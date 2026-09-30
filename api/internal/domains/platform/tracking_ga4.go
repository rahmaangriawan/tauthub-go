package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// tracking_ga4.go: port lib/tracking/service.ts recordTrackingEvent (rollup
// analytics_daily) + lib/ga4/service.ts forwardEventToGa4 (Measurement
// Protocol v2). Tulisio Analytics tetap sumber kebenaran; GA4 hanya mirror
// best-effort — kegagalan GA4 tidak boleh menggagalkan tracking.

// ga4CollectURL: endpoint Measurement Protocol v2. Variabel (bukan konstanta)
// supaya pengujian dapat mengarahkannya ke server tangkapan.
var ga4CollectURL = "https://www.google-analytics.com/mp/collect"

// ga4EventMap: salinan persis GA4_EVENT_MAP (lib/ga4/service.ts).
var ga4EventMap = map[string]string{
	"article_view":        "article_view",
	"article_scroll":      "article_scroll",
	"outbound_click":      "outbound_click",
	"cta_click":           "cta_click",
	"tracking_link_click": "tracking_link_click",
}

// trackingEventInput: input satu event tracking (parity TrackingEventInput).
type trackingEventInput struct {
	LinkID        string
	PublicationID string
	CampaignID    string
	BrandID       string
	EventType     string
	SessionID     string
	IPAddress     string
	UserAgent     string
	Referrer      string
	URL           string
	Metadata      map[string]any
}

// recordTrackingEvent: simpan event mentah + rollup analytics_daily
// (ON CONFLICT brand/date/type/publication → count+1), lalu mirror ke GA4
// best-effort. Parity recordTrackingEvent.
func (d *Deps) recordTrackingEvent(ctx context.Context, in trackingEventInput) error {
	eventDate := time.Now().UTC().Format("2006-01-02")

	var metadataJSON any
	if in.Metadata != nil {
		if b, err := json.Marshal(in.Metadata); err == nil {
			metadataJSON = string(b)
		}
	}

	if err := d.exec(ctx, `
		INSERT INTO tracking_events (id, link_id, publication_id, campaign_id, brand_id, event_type,
		                             session_id, ip_address, user_agent, referrer, url, metadata)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::jsonb)`,
		nullIfEmpty(in.LinkID), nullIfEmpty(in.PublicationID), nullIfEmpty(in.CampaignID), in.BrandID,
		in.EventType, nullIfEmpty(in.SessionID), nullIfEmpty(in.IPAddress),
		nullIfEmpty(truncateBytesStr(in.UserAgent, 300)), nullIfEmpty(in.Referrer), nullIfEmpty(in.URL),
		metadataJSON); err != nil {
		return err
	}

	if err := d.exec(ctx, `
		INSERT INTO analytics_daily (id, brand_id, publication_id, campaign_id, event_type, event_date, count)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5::date, 1)
		ON CONFLICT (brand_id, event_date, event_type, COALESCE(publication_id, ''))
		DO UPDATE SET count = analytics_daily.count + 1, updated_at = NOW()`,
		in.BrandID, nullIfEmpty(in.PublicationID), nullIfEmpty(in.CampaignID), in.EventType, eventDate); err != nil {
		return err
	}

	// GA4 mirror — hiccup GA4 tidak boleh menggagalkan tracking Tulisio.
	d.forwardEventToGa4(ctx, in)
	return nil
}

// ga4ConfigRow: baris ga4_configs yang relevan untuk forwarding.
type ga4ConfigRow struct {
	MeasurementID   string
	APISecret       string
	SealedAPISecret string
	Enabled         bool
	ConsentMode     string
	EventMapping    map[string]string
}

// loadGa4Config: config GA4 aktif milik brand (parity loadGa4Config).
func (d *Deps) loadGa4Config(ctx context.Context, brandID string) *ga4ConfigRow {
	row, err := d.queryRow(ctx,
		"SELECT measurement_id, api_secret, sealed_api_secret, enabled, consent_mode, event_mapping FROM ga4_configs WHERE brand_id = $1 AND enabled = TRUE LIMIT 1",
		brandID)
	if err != nil || row == nil {
		return nil
	}
	cfg := &ga4ConfigRow{
		MeasurementID:   row.strOr("measurement_id", ""),
		APISecret:       row.strOr("api_secret", ""),
		SealedAPISecret: row.strOr("sealed_api_secret", ""),
		Enabled:         row.boolOr("enabled", false),
		ConsentMode:     row.strOr("consent_mode", "required"),
		EventMapping:    map[string]string{},
	}
	if raw, ok := row.jsonField("event_mapping").(map[string]any); ok {
		for k, v := range raw {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				cfg.EventMapping[k] = strings.TrimSpace(s)
			}
		}
	}
	return cfg
}

// shouldForwardToGa4: consent_mode 'none' → semua event; selain itu hanya
// event dengan metadata.consent == "granted" (parity shouldForwardToGa4).
func shouldForwardToGa4(cfg *ga4ConfigRow, metadata map[string]any) bool {
	if cfg == nil || !cfg.Enabled {
		return false
	}
	if cfg.ConsentMode == "none" {
		return true
	}
	if metadata == nil {
		return false
	}
	consent, _ := metadata["consent"].(string)
	return consent == "granted"
}

// ga4ClientID: client id format GA deterministik dari session (parity
// ga4ClientId — hash 31 + timestamp, tanpa PII).
func ga4ClientID(sessionID string) string {
	seed := sessionID
	if seed == "" {
		seed = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	var hash uint32
	for i := 0; i < len(seed); i++ {
		hash = hash*31 + uint32(seed[i])
	}
	return fmt.Sprintf("%d.%d", hash%1_000_000_000, time.Now().UnixMilli()%1_000_000_000)
}

// ga4APISecret: sealed_api_secret diutamakan (unseal v2), fallback plaintext
// legacy api_secret — parity ga4ApiSecret.
func (d *Deps) ga4APISecret(cfg *ga4ConfigRow) string {
	if cfg.SealedAPISecret != "" {
		if plain, err := d.unsealT2(cfg.SealedAPISecret); err == nil && plain != "" {
			return plain
		}
	}
	return cfg.APISecret
}

// forwardEventToGa4: muat config brand lalu kirim event (best-effort).
func (d *Deps) forwardEventToGa4(ctx context.Context, in trackingEventInput) {
	cfg := d.loadGa4Config(ctx, in.BrandID)
	if !shouldForwardToGa4(cfg, in.Metadata) {
		return
	}
	if err := sendGa4Event(ctx, cfg.MeasurementID, d.ga4APISecret(cfg), cfg.EventMapping, in); err != nil {
		slog.Warn("[ga4] forward gagal", "brand", in.BrandID, "err", err)
	}
}

// sendGa4Event: kirim satu event ke Measurement Protocol v2. Tanpa akses DB
// supaya dapat diuji langsung terhadap server tangkapan.
func sendGa4Event(ctx context.Context, measurementID, apiSecret string, mapping map[string]string, in trackingEventInput) error {
	if measurementID == "" || apiSecret == "" {
		return nil
	}

	eventName := in.EventType
	if mapped, ok := mapping[in.EventType]; ok && mapped != "" {
		eventName = mapped
	} else if mapped, ok := ga4EventMap[in.EventType]; ok {
		eventName = mapped
	}

	params := map[string]any{
		"event_type":   in.EventType,
		"event_source": "tulisio",
	}
	if in.PublicationID != "" {
		params["publication_id"] = in.PublicationID
	}
	if in.CampaignID != "" {
		params["campaign_id"] = in.CampaignID
	}
	if in.URL != "" {
		params["url"] = in.URL
	}
	body, err := json.Marshal(map[string]any{
		"client_id": ga4ClientID(in.SessionID),
		"events":    []any{map[string]any{"name": eventName, "params": params}},
	})
	if err != nil {
		return err
	}

	endpoint := ga4CollectURL + "?measurement_id=" + url.QueryEscape(measurementID) +
		"&api_secret=" + url.QueryEscape(apiSecret)
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, rErr := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(body))
	if rErr != nil {
		return rErr
	}
	req.Header.Set("Content-Type", "application/json")
	resp, hErr := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if hErr != nil {
		return hErr
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 200))
		return fmt.Errorf("GA4 collect returned %d: %s", resp.StatusCode, string(snippet))
	}
	return nil
}
