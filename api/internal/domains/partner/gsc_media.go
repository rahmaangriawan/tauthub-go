package partner

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
)

// gsc_media.go: Search Console per media untuk Pusat Penjual.
//
// Endpoint lama (/partner/gsc/*) mengambil satu snapshot untuk satu properti
// terpilih. Pusat Penjual butuh gambaran per media: properti mana yang cocok
// dengan situs media, snapshot terbaru, dan tren dari pengambilan mingguan.
// Pencocokan properti ke media memakai host: properti domain ("sc-domain:") atau
// URL-prefix yang hostnya sama dengan situs media, atau domain induknya.
//
// Search Console tidak memberi cakupan indeks lewat API ini, hanya klik,
// tayangan, CTR, dan posisi rata-rata. Tampilan tidak boleh menyebut "terindeks".

const (
	// gscSnapshotPeriod: jendela 28 hari, sama dengan tampilan bawaan Search Console.
	gscSnapshotPeriod = 28
	// gscDataLagDays: data Search Console tertinggal sekitar dua hari.
	gscDataLagDays = 2
	// gscManualCooldown: jeda minimal antar pengambilan manual per media.
	gscManualCooldown = 30 * time.Minute

	jobKindGSCSnapshot = "gsc_snapshot"
)

// gscSiteHost: host dari site_url Search Console ("sc-domain:contoh.id" atau
// "https://www.contoh.id/").
func gscSiteHost(siteURL string) string {
	s := strings.TrimSpace(siteURL)
	if rest, ok := strings.CutPrefix(s, "sc-domain:"); ok {
		return strings.TrimPrefix(strings.ToLower(strings.TrimSpace(rest)), "www.")
	}
	return hostnameOf(s)
}

// gscPropertyCoversMedia: properti mencakup situs media bila hostnya sama atau
// host media adalah subdomain dari host properti (properti domain mencakup
// subdomain). Tidak dibalik: properti subdomain tidak mencakup domain induknya.
func gscPropertyCoversMedia(siteURL, websiteURL string) bool {
	propertyHost := gscSiteHost(siteURL)
	mediaHost := hostnameOf(websiteURL)
	if propertyHost == "" || mediaHost == "" {
		return false
	}
	return propertyHost == mediaHost || strings.HasSuffix(mediaHost, "."+propertyHost)
}

type gscTarget struct {
	PropertyID   string
	SiteURL      string
	ConnectionID string
	SealedToken  string
}

// gscTargetForMedia: properti aktif milik mitra yang mencakup situs media.
// Properti yang persis sama dengan host media diutamakan atas properti induk.
func (d *Deps) gscTargetForMedia(ctx context.Context, partnerID, websiteURL string) (*gscTarget, error) {
	rows, err := d.queryRows(ctx, `
		SELECT pr.id AS property_id, pr.site_url, pr.permission_level, c.id AS connection_id, c.sealed_refresh_token
		FROM gsc_properties pr
		JOIN gsc_connections c ON c.id = pr.connection_id
		WHERE c.partner_id = $1 AND c.status = 'active'
		ORDER BY pr.site_url ASC`, partnerID)
	if err != nil {
		return nil, err
	}
	var best *gscTarget
	bestExact := false
	mediaHost := hostnameOf(websiteURL)
	for _, row := range rows {
		site := row.strOr("site_url", "")
		if !gscPropertyCoversMedia(site, websiteURL) {
			continue
		}
		if strings.EqualFold(row.strOr("permission_level", ""), "siteUnverifiedUser") {
			continue
		}
		exact := gscSiteHost(site) == mediaHost
		if best == nil || (exact && !bestExact) {
			best = &gscTarget{
				PropertyID:   row.strOr("property_id", ""),
				SiteURL:      site,
				ConnectionID: row.strOr("connection_id", ""),
				SealedToken:  row.strOr("sealed_refresh_token", ""),
			}
			bestExact = exact
		}
	}
	return best, nil
}

// gscSnapshotForMedia: ambil klik, tayangan, CTR, dan posisi 28 hari untuk satu
// media lalu simpan sebagai snapshot milik media itu. Dipakai tombol "Ambil
// data" dan job mingguan. Mengembalikan (nil, nil) bila tidak ada properti yang
// cocok (bukan galat: mitra memang belum menambahkan situsnya).
func (d *Deps) gscSnapshotForMedia(ctx context.Context, partnerID, mediaID string) (map[string]any, *PartnerError) {
	media, err := d.queryRow(ctx,
		"SELECT id, website_url FROM media WHERE id = $1 AND partner_id = $2 LIMIT 1", mediaID, partnerID)
	if err != nil {
		return nil, &PartnerError{http.StatusInternalServerError, "Gagal memuat media."}
	}
	if media == nil {
		return nil, errNotFound("Media tidak ditemukan")
	}
	target, err := d.gscTargetForMedia(ctx, partnerID, media.strOr("website_url", ""))
	if err != nil {
		return nil, &PartnerError{http.StatusInternalServerError, "Gagal memuat properti Search Console."}
	}
	if target == nil {
		return nil, nil
	}
	if strings.TrimSpace(target.SealedToken) == "" {
		return nil, &PartnerError{http.StatusConflict, "Refresh token tidak tersedia. Hubungkan ulang Search Console."}
	}
	refreshToken, unsealErr := d.unsealT2(target.SealedToken)
	if unsealErr != nil {
		slog.Error("[gsc] refresh token gagal di-unseal (APP_SECRET berubah?)", "partner_id", partnerID, "err", unsealErr)
		_ = d.exec(ctx, "UPDATE gsc_connections SET status = 'error', updated_at = NOW() WHERE id = $1", target.ConnectionID)
		return nil, &PartnerError{http.StatusInternalServerError, "Kredensial tersimpan tidak dapat dibuka. Hubungkan ulang Search Console."}
	}

	// Fungsi Google yang sudah ada menerima *http.Request hanya untuk konteksnya.
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	if reqErr != nil {
		return nil, &PartnerError{http.StatusInternalServerError, "Gagal menyiapkan permintaan."}
	}
	cfg, cfgErr := d.loadGSCPlatformConfig(req)
	if cfgErr != nil {
		return nil, cfgErr
	}
	accessToken, refreshErr := d.refreshGSCToken(req, refreshToken, cfg.ClientID, cfg.ClientSecret)
	if refreshErr != nil {
		_ = d.exec(ctx, "UPDATE gsc_connections SET status = 'error', updated_at = NOW() WHERE id = $1", target.ConnectionID)
		return nil, &PartnerError{http.StatusUnauthorized, "Akses ke Search Console ditolak. Hubungkan ulang."}
	}

	end := time.Now().UTC().AddDate(0, 0, -gscDataLagDays)
	start := end.AddDate(0, 0, -(gscSnapshotPeriod - 1))
	analytics, aErr := d.fetchSearchAnalytics(req, accessToken, target.SiteURL, start.Format("2006-01-02"), end.Format("2006-01-02"))
	if aErr != nil {
		if pe, ok := aErr.(*PartnerError); ok {
			return nil, pe
		}
		return nil, &PartnerError{http.StatusBadGateway, aErr.Error()}
	}
	id := uuid.NewString()
	if err := d.exec(ctx, `
		INSERT INTO gsc_snapshots (id, property_id, media_id, period_days, clicks, impressions, ctr, position)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		id, target.PropertyID, mediaID, gscSnapshotPeriod,
		analytics["clicks"], analytics["impressions"], analytics["ctr"], analytics["position"]); err != nil {
		return nil, &PartnerError{http.StatusInternalServerError, "Gagal menyimpan snapshot."}
	}
	out := map[string]any{"snapshot_id": id, "site_url": target.SiteURL, "period_days": gscSnapshotPeriod}
	for k, v := range analytics {
		out[k] = v
	}
	return out, nil
}

// GSCMediaSnapshot: POST /v1/partner/media/{mediaId}/gsc-snapshot
func (d *Deps) GSCMediaSnapshot(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	mediaID := chiParam(r, "mediaId")
	recent, _ := d.queryRow(r.Context(), `
		SELECT 1 AS recent FROM gsc_snapshots
		WHERE media_id = $1 AND fetched_at > NOW() - make_interval(secs => $2)
		LIMIT 1`, mediaID, gscManualCooldown.Seconds())
	if recent != nil {
		httpx.Error(w, http.StatusConflict, "Data baru saja diambil. Coba lagi beberapa menit lagi.")
		return
	}
	result, pErr := d.gscSnapshotForMedia(r.Context(), partner.ID, mediaID)
	if pErr != nil {
		httpx.Error(w, pErr.Status, pErr.Message)
		return
	}
	if result == nil {
		httpx.Error(w, http.StatusUnprocessableEntity,
			"Situs ini belum ada di Search Console akun Anda. Tambahkan dan verifikasi situsnya di Search Console, lalu hubungkan ulang.")
		return
	}
	result["ok"] = true
	writeJSON(w, http.StatusOK, result)
}

// Traffic: GET /v1/partner/traffic
// Gambaran trafik semua media mitra: apakah Search Console tersambung, properti
// yang cocok per media, tren snapshot mingguan, dan kunjungan SimilarWeb (bila
// ada). Data yang belum ada dikirim kosong, tidak diperkirakan.
func (d *Deps) Traffic(w http.ResponseWriter, r *http.Request) {
	_, partner, ok := d.auth(w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	var connections int
	_ = d.Pool.QueryRow(ctx,
		"SELECT COUNT(*)::int FROM gsc_connections WHERE partner_id = $1 AND status = 'active'", partner.ID).Scan(&connections)

	mediaRows, err := d.queryRows(ctx, `
		SELECT id, name, website_url FROM media
		WHERE partner_id = $1 AND status <> 'hidden' ORDER BY created_at DESC LIMIT 200`, partner.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	propRows, _ := d.queryRows(ctx, `
		SELECT pr.id AS property_id, pr.site_url, pr.permission_level
		FROM gsc_properties pr JOIN gsc_connections c ON c.id = pr.connection_id
		WHERE c.partner_id = $1 AND c.status = 'active'`, partner.ID)

	snapRows, _ := d.queryRows(ctx, `
		SELECT s.media_id, s.fetched_at, s.clicks, s.impressions, s.ctr::float8 AS ctr, s.position::float8 AS position
		FROM gsc_snapshots s JOIN media m ON m.id = s.media_id
		WHERE m.partner_id = $1 AND s.period_days = $2 AND s.fetched_at > NOW() - INTERVAL '120 days'
		ORDER BY s.fetched_at ASC`, partner.ID, gscSnapshotPeriod)
	snapsByMedia := map[string][]map[string]any{}
	for _, s := range snapRows {
		id := s.strOr("media_id", "")
		snapsByMedia[id] = append(snapsByMedia[id], map[string]any{
			"fetched_at": s.timePtr("fetched_at"), "clicks": s.intOr("clicks", 0),
			"impressions": s.intOr("impressions", 0), "ctr": s.numOr("ctr"), "position": s.numOr("position"),
		})
	}

	swRows, _ := d.queryRows(ctx, `
		SELECT s.media_id, s.snapshot_date,
		       CASE WHEN s.insights->>'visits' ~ '^-?[0-9]+([.][0-9]+)?$'
		            THEN ((s.insights->>'visits')::numeric)::bigint END AS visits
		FROM media_similarweb_snapshots s JOIN media m ON m.id = s.media_id
		WHERE m.partner_id = $1
		ORDER BY s.created_at ASC`, partner.ID)
	swByMedia := map[string][]map[string]any{}
	for _, s := range swRows {
		if !s.has("visits") {
			continue
		}
		id := s.strOr("media_id", "")
		swByMedia[id] = append(swByMedia[id], map[string]any{"date": s.timePtr("snapshot_date"), "visits": s.numOr("visits")})
	}

	items := make([]map[string]any, 0, len(mediaRows))
	for _, m := range mediaRows {
		id := m.strOr("id", "")
		website := m.strOr("website_url", "")
		matchedSite := ""
		for _, p := range propRows {
			if strings.EqualFold(p.strOr("permission_level", ""), "siteUnverifiedUser") {
				continue
			}
			if gscPropertyCoversMedia(p.strOr("site_url", ""), website) {
				matchedSite = p.strOr("site_url", "")
				if gscSiteHost(matchedSite) == hostnameOf(website) {
					break
				}
			}
		}
		sw := swByMedia[id]
		if len(sw) > 12 {
			sw = sw[len(sw)-12:]
		}
		items = append(items, map[string]any{
			"media_id": id, "name": m.raw("name"), "website_url": m.raw("website_url"),
			"matched": matchedSite != "", "site_url": matchedSite,
			"gsc": snapsByMedia[id], "similarweb": sw,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"connected":        connections > 0,
		"connections":      connections,
		"period_days":      gscSnapshotPeriod,
		"cooldown_minutes": int(gscManualCooldown.Minutes()),
		"media":            items,
	})
}

// RegisterJobs: processor job milik paket partner. Dipanggil jobwire.Register.
func (d *Deps) RegisterJobs() {
	jobs.Register(jobKindGSCSnapshot, func(ctx context.Context, job jobs.JobRow) error {
		mediaID, _ := job.Payload["mediaId"].(string)
		if mediaID == "" {
			return fmt.Errorf("payload mediaId kosong")
		}
		return d.GSCSnapshotJob(ctx, mediaID)
	})
}

// GSCSnapshotJob: snapshot Search Console mingguan untuk satu media. Media tanpa
// properti yang cocok atau tanpa koneksi aktif dilewati tanpa galat; galat
// Google (token dicabut, kuota) dilaporkan ke runner supaya dicoba ulang.
func (d *Deps) GSCSnapshotJob(ctx context.Context, mediaID string) error {
	media, err := d.queryRow(ctx, "SELECT partner_id FROM media WHERE id = $1 AND status = 'active' LIMIT 1", mediaID)
	if err != nil {
		return err
	}
	if media == nil || media.strOr("partner_id", "") == "" {
		return nil
	}
	_, pErr := d.gscSnapshotForMedia(ctx, media.strOr("partner_id", ""), mediaID)
	if pErr != nil {
		// 401/409: koneksi bermasalah dan sudah ditandai; mencoba lagi tidak membantu.
		if pErr.Status == http.StatusUnauthorized || pErr.Status == http.StatusConflict || pErr.Status == http.StatusUnprocessableEntity {
			slog.Warn("[gsc] snapshot mingguan dilewati", "media", mediaID, "reason", pErr.Message)
			return nil
		}
		return pErr
	}
	return nil
}
