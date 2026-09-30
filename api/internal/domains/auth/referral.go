package authdomain

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// ReferralClick: publik — mencatat klik link referral (/register?ref=KODE).
//
// Parity app/register/route.ts: kode dinormalisasi, klik di-upsert ke
// referral_clicks, lalu audit_logs 'referral.click' ditulis. Route SvelteKit
// /register memanggil endpoint ini server-side untuk memutuskan tujuan redirect
// (halaman daftar sesuai peran atau /login) sekaligus men-set cookie
// tauthub_ref — cookie itu yang nanti dipakai form daftar sebagai atribusi.
//
// Kode tak dikenal tetap 200 dengan ok:false supaya halaman tetap bisa
// mengarahkan user ke /login?mode=register, sama seperti app lama.
func (d *Deps) ReferralClick(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ref string `json:"ref"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Kode referral wajib diisi.")
		return
	}
	ref := strings.TrimSpace(body.Ref)
	if ref == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Kode referral wajib diisi.")
		return
	}
	if len(ref) > 64 {
		ref = ref[:64]
	}

	ip := authRequestIp(r)
	if err := auth.CheckAuthRate(r.Context(), d.Pool, "referral:ip:"+orUnknown(ip), 60); err != nil {
		writeRateErr(w, err)
		return
	}

	code := auth.NormalizeReferralCode(ref)
	partnerID, ok := auth.RecordReferralClick(r.Context(), d.Pool, ref, ip, r.UserAgent())
	if !ok {
		writeReferralJSON(w, map[string]any{"ok": false, "code": code})
		return
	}

	// Audit actorless (parity INSERT audit_logs dengan actor_user_id NULL) —
	// best-effort, tidak boleh menggagalkan redirect.
	_, _ = d.Pool.Exec(r.Context(),
		"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, NULL, 'referral.click', 'partner', $2)",
		uuid.NewString(), partnerID)

	writeReferralJSON(w, map[string]any{"ok": true, "code": code})
}

func writeReferralJSON(w http.ResponseWriter, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(payload)
}
