package platform

import (
	"strings"
	"testing"
)

// Header provider Similar Web API (belchiorarkad di RapidAPI): cukup key
// RapidAPI + host. Tidak ada Authorization Bearer seperti provider lama.
func TestMpSimilarWebHeaders(t *testing.T) {
	h := mpSimilarWebHeaders("RAPIDAPI_KEY_PLACEHOLDER")
	if h["x-rapidapi-key"] != "RAPIDAPI_KEY_PLACEHOLDER" {
		t.Fatalf("x-rapidapi-key hilang: %v", h)
	}
	if h["x-rapidapi-host"] != mpSimilarWebHost {
		t.Fatalf("host salah: %v", h["x-rapidapi-host"])
	}
	if _, ok := h["authorization"]; ok {
		t.Fatalf("authorization tidak boleh dikirim: %v", h)
	}
	for k, v := range h {
		if strings.ContainsAny(v, "\r\n") {
			t.Fatalf("header %s memuat CRLF: %q", k, v)
		}
	}
}

// Penolakan 403 "not subscribed" identik dengan key yang tidak dikenal, jadi
// hint-nya wajib menuntun ke langganan RapidAPI.
func TestSwDiagnoseHint(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantSubstr string
	}{
		{"200 sukses", 200, `{"domain_analytics":{}}`, "Key valid dan berlangganan aktif"},
		{"404 kredensial lolos", 404, `{"message":"Domain not found"}`, "Kredensial lolos"},
		{"403 not subscribed", 403, `{"message":"You are not subscribed to this API."}`, "belum berlangganan listing Similar Web API"},
		{"403 lain", 403, `{"message":"forbidden"}`, "langganan"},
		{"401 kredensial ditolak", 401, `{"message":"Invalid API key"}`, "401"},
		{"429 kuota", 429, `{"message":"Too many requests"}`, "rate limit"},
		{"0 tidak terhubung", 0, "", "tidak dapat dihubungi"},
	}
	for _, c := range cases {
		if got := swDiagnoseHint(c.status, c.body); !strings.Contains(got, c.wantSubstr) {
			t.Fatalf("%s: %s tidak memuat %q", c.name, got, c.wantSubstr)
		}
	}
}
