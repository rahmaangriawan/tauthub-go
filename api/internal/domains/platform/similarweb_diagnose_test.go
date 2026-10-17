package platform

import (
	"strings"
	"testing"
)

// Penolakan 403 "not subscribed" itu penyebab nyata gagalnya SimilarWeb:
// tasnya identik dengan key yang tidak dikenal, jadi hint-nya wajib menuntun
// ke langganan RapidAPI (bukan menyuruh ganti key untuk kasus ini).
func TestSwDiagnoseHint(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		body       string
		wantSubstr string
	}{
		{"200 sukses", 200, `{"domain":"x","visits":100}`, "Key valid dan berlangganan aktif"},
		{"404 berlangganan aktif", 404, `{"message":"Domain not found"}`, "Berlangganan aktif"},
		{"403 not subscribed", 403, `{"message":"You are not subscribed to this API."}`, "belum berlangganan listing SimilarWeb"},
		{"403 lain", 403, `{"message":"forbidden"}`, "menolak akses"},
		{"401 invalid key", 401, `{"message":"Invalid API key"}`, "Invalid API key"},
		{"429 kuota", 429, `{"message":"Too many requests"}`, "rate limit"},
		{"0 tidak terhubung", 0, "", "tidak dapat dihubungi"},
	}
	for _, c := range cases {
		if got := swDiagnoseHint(c.status, c.body); !strings.Contains(got, c.wantSubstr) {
			t.Fatalf("%s: %s tidak memuat %q", c.name, got, c.wantSubstr)
		}
	}
}
