package platform

import (
	"strings"
	"testing"
)

// Header provider Similarweb API Pro (DataBee): dua kredensial terpisah —
// key RapidAPI dan token DataBee via Authorization Bearer. Token wajib
// dikirim, tapi tidak boleh bocor ke luar header.
func TestMpSimilarWebHeaders(t *testing.T) {
	h := mpSimilarWebHeaders("RAPIDAPI_KEY_PLACEHOLDER", "DATABEE_TOKEN_PLACEHOLDER")
	if h["x-rapidapi-key"] != "RAPIDAPI_KEY_PLACEHOLDER" {
		t.Fatalf("x-rapidapi-key hilang: %v", h)
	}
	if h["x-rapidapi-host"] != mpSimilarWebHost {
		t.Fatalf("host salah: %v", h["x-rapidapi-host"])
	}
	if got := h["authorization"]; got != "Bearer DATABEE_TOKEN_PLACEHOLDER" {
		t.Fatalf("authorization salah: %q", got)
	}

	// Tanpa DataBee token: header tidak diberangkatkan (bukan "Bearer " kosong).
	noBearer := mpSimilarWebHeaders("RAPIDAPI_KEY_PLACEHOLDER", "")
	if _, ok := noBearer["authorization"]; ok {
		t.Fatalf("authorization tidak boleh hadir tanpa token: %v", noBearer)
	}
	for k, v := range noBearer {
		if strings.ContainsAny(v, "\r\n") {
			t.Fatalf("header %s memuat CRLF: %q", k, v)
		}
	}
}

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
		{"200 sukses", 200, `{"region":"US"}`, "Key valid dan berlangganan aktif"},
		{"404 kredensial lolos", 404, `{"message":"Domain not found"}`, "Kredensial lolos"},
		{"403 not subscribed", 403, `{"message":"You are not subscribed to this API."}`, "belum berlangganan listing SimilarWeb"},
		{"403 lain (dugaan DataBee)", 403, `{"message":"forbidden"}`, "DataBee Token"},
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
