package platform

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestSendGa4EventPayload: kirim event ke server tangkapan lalu verifikasi
// URL (measurement_id + api_secret) dan bentuk body Measurement Protocol v2.
func TestSendGa4EventPayload(t *testing.T) {
	var gotPath, gotQuery, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotPath, gotQuery, gotBody = r.URL.Path, r.URL.RawQuery, string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := ga4CollectURL
	ga4CollectURL = srv.URL + "/mp/collect"
	defer func() { ga4CollectURL = orig }()

	err := sendGa4Event(context.Background(), "G-TEST123", "secret-abc", nil, trackingEventInput{
		BrandID: "brand-1", EventType: "article_view", SessionID: "sess-9",
		PublicationID: "pub-1", CampaignID: "camp-1", URL: "https://contoh.id/artikel",
	})
	if err != nil {
		t.Fatalf("sendGa4Event error: %v", err)
	}

	if gotPath != "/mp/collect" {
		t.Errorf("path = %q, ingin /mp/collect", gotPath)
	}
	q, _ := url.ParseQuery(gotQuery)
	if q.Get("measurement_id") != "G-TEST123" {
		t.Errorf("measurement_id = %q", q.Get("measurement_id"))
	}
	if q.Get("api_secret") != "secret-abc" {
		t.Errorf("api_secret = %q", q.Get("api_secret"))
	}

	var body struct {
		ClientID string `json:"client_id"`
		Events   []struct {
			Name   string         `json:"name"`
			Params map[string]any `json:"params"`
		} `json:"events"`
	}
	if err := json.Unmarshal([]byte(gotBody), &body); err != nil {
		t.Fatalf("body bukan JSON: %v (%s)", err, gotBody)
	}
	if body.ClientID == "" {
		t.Error("client_id kosong")
	}
	if len(body.Events) != 1 {
		t.Fatalf("events = %d, ingin 1", len(body.Events))
	}
	if body.Events[0].Name != "article_view" {
		t.Errorf("nama event = %q, ingin article_view", body.Events[0].Name)
	}
	if body.Events[0].Params["event_source"] != "tulisio" {
		t.Errorf("event_source = %v", body.Events[0].Params["event_source"])
	}
}

// TestSendGa4EventMappingOverride: event_mapping per brand menimpa nama bawaan.
func TestSendGa4EventMappingOverride(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	orig := ga4CollectURL
	ga4CollectURL = srv.URL + "/mp/collect"
	defer func() { ga4CollectURL = orig }()

	mapping := map[string]string{"article_view": "kustom_view"}
	if err := sendGa4Event(context.Background(), "G-1", "s", mapping, trackingEventInput{
		BrandID: "b", EventType: "article_view",
	}); err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(gotBody, "kustom_view") {
		t.Errorf("override tidak dipakai: %s", gotBody)
	}
}

// TestSendGa4EventNonOK: respons non-2xx harus menjadi error (agar tercatat,
// tapi tidak pernah menggagalkan tracking karena pemanggil menelannya).
func TestSendGa4EventNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("bad"))
	}))
	defer srv.Close()

	orig := ga4CollectURL
	ga4CollectURL = srv.URL + "/mp/collect"
	defer func() { ga4CollectURL = orig }()

	err := sendGa4Event(context.Background(), "G-1", "s", nil, trackingEventInput{EventType: "article_view"})
	if err == nil {
		t.Fatal("ingin error untuk status 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("pesan error = %v", err)
	}
}

// TestSendGa4EventSkipsWithoutCredentials: tanpa id/secret tidak ada request.
func TestSendGa4EventSkipsWithoutCredentials(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	defer srv.Close()
	orig := ga4CollectURL
	ga4CollectURL = srv.URL
	defer func() { ga4CollectURL = orig }()

	if err := sendGa4Event(context.Background(), "", "", nil, trackingEventInput{EventType: "article_view"}); err != nil {
		t.Fatalf("error: %v", err)
	}
	if called {
		t.Error("tidak boleh memanggil GA4 tanpa kredensial")
	}
}

// TestShouldForwardToGa4Consent: aturan consent (parity shouldForwardToGa4).
func TestShouldForwardToGa4Consent(t *testing.T) {
	required := &ga4ConfigRow{Enabled: true, ConsentMode: "required"}
	none := &ga4ConfigRow{Enabled: true, ConsentMode: "none"}
	off := &ga4ConfigRow{Enabled: false, ConsentMode: "none"}

	cases := []struct {
		name     string
		cfg      *ga4ConfigRow
		metadata map[string]any
		want     bool
	}{
		{"required tanpa metadata", required, nil, false},
		{"required consent granted", required, map[string]any{"consent": "granted"}, true},
		{"required consent denied", required, map[string]any{"consent": "denied"}, false},
		{"none tanpa metadata", none, nil, true},
		{"disabled selalu false", off, map[string]any{"consent": "granted"}, false},
		{"cfg nil", nil, map[string]any{"consent": "granted"}, false},
	}
	for _, c := range cases {
		if got := shouldForwardToGa4(c.cfg, c.metadata); got != c.want {
			t.Errorf("%s: got %v ingin %v", c.name, got, c.want)
		}
	}
}

// TestGa4ClientIDDeterministic: session sama → hash sama (bagian sebelum titik).
func TestGa4ClientIDDeterministic(t *testing.T) {
	a := ga4ClientID("session-xyz")
	b := ga4ClientID("session-xyz")
	ha := strings.Split(a, ".")[0]
	hb := strings.Split(b, ".")[0]
	if ha != hb {
		t.Errorf("hash tidak deterministik: %s vs %s", ha, hb)
	}
	if strings.Split(ga4ClientID("lain"), ".")[0] == ha {
		t.Error("session berbeda menghasilkan hash sama")
	}
}

// TestTrackingEventTypesMatchDbCheck: daftar event harus sama dengan CHECK
// constraint tracking_events.event_type (5 nilai, tanpa 'impression' dkk).
func TestTrackingEventTypesMatchDbCheck(t *testing.T) {
	want := []string{"article_view", "article_scroll", "outbound_click", "cta_click", "tracking_link_click"}
	if len(trackingEventTypes) != len(want) {
		t.Fatalf("jumlah event type = %d, ingin %d (%v)", len(trackingEventTypes), len(want), trackingEventTypes)
	}
	for i, v := range want {
		if trackingEventTypes[i] != v {
			t.Errorf("event type[%d] = %q, ingin %q", i, trackingEventTypes[i], v)
		}
	}
}
