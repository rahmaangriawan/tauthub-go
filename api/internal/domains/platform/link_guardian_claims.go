package platform

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/db"
)

// link_guardian_claims.go: klaim garansi tayang Link Guardian.
//
// Alur kerja admin (kebijakan produk): cek kondisi domain → hubungi pemilik
// domain → bila tidak bisa diselesaikan, kembalikan saldo ke akun pengguna
// senilai pembelian artikel yang bermasalah. Pengembalian itu memindahkan kredit
// yang sudah TERCAPTURE kembali ke saldo tersedia dan dicatat di ledger sebagai
// ref_type='refund' bernilai negatif, sehingga pemasukan Tauthub ikut turun.
//
// Kelayakan ditegakkan di server, bukan hanya di tampilan:
//   - artikel masih dalam masa garansi tayang medianya (12 bulan, atau tanpa
//     batas untuk media yang tautannya permanen — media.link_duration_months),
//   - medianya terverifikasi (bukan domain makelar),
//   - kerusakannya 'gone' (404/410) atau tautan di dalam artikel dicabut.

// LinkGuardianClaimsList: GET /v1/platform/link-guardian/claims?status=
func (d *Deps) LinkGuardianClaimsList(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := d.adminGuard(w, r, ""); !ok {
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	args := []any{}
	clause := ""
	if status != "" {
		if !ptContains([]string{"submitted", "checking", "contacted", "refunded", "rejected", "replaced"}, status) {
			writeErr(w, d, errUnprocessable("status klaim tidak dikenal"))
			return
		}
		args = append(args, status)
		clause = " AND c.status = $1"
	}
	rows, err := d.queryRows(r.Context(), `
		SELECT c.id, c.publication_id, c.brand_id, c.status, c.detection_state,
		       c.anchor_expected, c.domain_check_note, c.owner_contact_note, c.resolution_note,
		       c.rejection_reason, c.refund_tc, c.refund_idr, c.refunded_at, c.created_at, c.updated_at,
		       p.published_at, p.published_url, p.client_price,
		       m.name AS media_name, COALESCE(m.domain_verified, FALSE) AS domain_verified,
		       b.name AS brand_name,
		       lc.state AS link_state, lc.in_article_state, lc.rel_state, lc.noindex_state,
		       lc.violation_kind, lc.violation_streak, lc.last_checked_at,
		       c.auto_detected, c.client_choice, c.client_chosen_at,
		       m.link_duration_months AS link_duration_months,
		       `+db.LinkWarrantySQL("p", "m")+` AS in_warranty
		FROM link_guardian_claims c
		JOIN publications p ON p.id = c.publication_id
		LEFT JOIN media m ON m.id = p.media_id
		LEFT JOIN brands b ON b.id = c.brand_id
		LEFT JOIN publication_link_checks lc ON lc.publication_id = c.publication_id
		WHERE TRUE`+clause+`
		ORDER BY c.created_at DESC
		LIMIT 200`, args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		reason := d.lgEligibilityReason(row)
		items = append(items, map[string]any{
			"id":                 row.strOr("id", ""),
			"publication_id":     row.strOr("publication_id", ""),
			"brand_id":           row.strOr("brand_id", ""),
			"brand_name":         ptNullableStr(row, "brand_name"),
			"status":             row.strOr("status", ""),
			"detection_state":    ptNullableStr(row, "detection_state"),
			"anchor_expected":    ptNullableStr(row, "anchor_expected"),
			"domain_check_note":  ptNullableStr(row, "domain_check_note"),
			"owner_contact_note": ptNullableStr(row, "owner_contact_note"),
			"resolution_note":    ptNullableStr(row, "resolution_note"),
			"rejection_reason":   ptNullableStr(row, "rejection_reason"),
			"refund_tc":          row.intPtr("refund_tc"),
			"refund_idr":         row.intPtr("refund_idr"),
			"refunded_at":        row.timePtr("refunded_at"),
			"created_at":         row.timePtr("created_at"),
			"updated_at":         row.timePtr("updated_at"),
			"published_at":       row.timePtr("published_at"),
			"published_url":      ptNullableStr(row, "published_url"),
			"client_price":       row.intPtr("client_price"),
			"media_name":         ptNullableStr(row, "media_name"),
			"domain_verified":    row.boolOr("domain_verified", false),
			"in_warranty":        row.boolOr("in_warranty", false),
			"link_state":         ptNullableStr(row, "link_state"),
			"in_article_state":   ptNullableStr(row, "in_article_state"),
			"rel_state":          ptNullableStr(row, "rel_state"),
			"noindex_state":      ptNullableStr(row, "noindex_state"),
			"auto_detected":      row.boolOr("auto_detected", false),
			"client_choice":      ptNullableStr(row, "client_choice"),
			"client_chosen_at":   row.timePtr("client_chosen_at"),
			"last_checked_at":    row.timePtr("last_checked_at"),
			"eligible":           reason == "",
			"ineligible_reason":  reason,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"claims": items, "total": len(items)})
}

// LinkGuardianClaimStep: POST /v1/platform/link-guardian/claims/{claimId}/step
// Menyimpan progres dua langkah awal: hasil cek kondisi domain dan catatan
// menghubungi pemilik domain.
func (d *Deps) LinkGuardianClaimStep(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	claimID := chiParam(r, "claimId")
	body := decodeJSON(r)
	status, _ := body["status"].(string)
	if status == "" {
		status = "checking"
	}
	if !ptContains([]string{"checking", "contacted"}, status) {
		writeErr(w, d, errUnprocessable("Status langkah hanya 'checking' atau 'contacted'."))
		return
	}
	domainNote, _ := body["domain_check_note"].(string)
	ownerNote, _ := body["owner_contact_note"].(string)
	resolutionNote, _ := body["resolution_note"].(string)

	tag, err := d.execRows(r.Context(), `
		UPDATE link_guardian_claims
		SET status = $1,
		    domain_check_note  = COALESCE(NULLIF($2, ''), domain_check_note),
		    owner_contact_note = COALESCE(NULLIF($3, ''), owner_contact_note),
		    resolution_note    = COALESCE(NULLIF($4, ''), resolution_note),
		    updated_by = $5, updated_at = NOW()
		WHERE id = $6 AND status IN ('submitted', 'checking', 'contacted')`,
		status, strings.TrimSpace(domainNote), strings.TrimSpace(ownerNote), strings.TrimSpace(resolutionNote),
		user.ID, claimID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if tag == 0 {
		writeErr(w, d, errUnprocessable("Klaim tidak ditemukan atau sudah selesai diproses."))
		return
	}
	if info, _ := d.queryRow(r.Context(), `
		SELECT c.brand_id, m.name AS media_name
		FROM link_guardian_claims c
		JOIN publications p ON p.id = c.publication_id
		LEFT JOIN media m ON m.id = p.media_id
		WHERE c.id = $1 LIMIT 1`, claimID); info != nil {
		progress := "Tim Tauthub sedang memeriksa kondisi domain"
		if status == "contacted" {
			progress = "Tim Tauthub sudah menghubungi pemilik domain"
		}
		link := "/klien/" + info.strOr("brand_id", "") + "/link-guardian"
		d.mpNotifyBrandMembers(r.Context(), info.strOr("brand_id", ""), "publication.guarantee_update",
			"Klaim garansi tautan diproses",
			progress+" "+info.strOr("media_name", "media")+" untuk klaim garansi Anda.", &link)
	}
	d.audit(r.Context(), user.ID, "platform.link_guardian.claim_step", "link_guardian_claim", claimID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "status": status})
}

// LinkGuardianClaimRefund: POST /v1/platform/link-guardian/claims/{claimId}/refund
// Mengembalikan saldo senilai pembelian artikel yang bermasalah. Idempoten per
// publikasi, dan hanya untuk klaim yang belum selesai.
func (d *Deps) LinkGuardianClaimRefund(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	claimID := chiParam(r, "claimId")
	body := decodeJSON(r)
	resolutionNote, _ := body["resolution_note"].(string)

	claim, err := d.queryRow(r.Context(), `
		SELECT c.id, c.publication_id, c.brand_id, c.status,
		       COALESCE(m.domain_verified, FALSE) AS domain_verified,
		       m.partner_id, m.name AS media_name,
		       lc.state AS link_state, lc.in_article_state, lc.rel_state, lc.noindex_state,
		       lc.violation_kind, lc.violation_streak,
		       m.link_duration_months AS link_duration_months,
		       `+db.LinkWarrantySQL("p", "m")+` AS in_warranty
		FROM link_guardian_claims c
		JOIN publications p ON p.id = c.publication_id
		LEFT JOIN media m ON m.id = p.media_id
		LEFT JOIN publication_link_checks lc ON lc.publication_id = c.publication_id
		WHERE c.id = $1 LIMIT 1`, claimID)
	if err != nil || claim == nil {
		writeErr(w, d, errNotFound("Klaim tidak ditemukan"))
		return
	}
	switch claim.strOr("status", "") {
	case "refunded", "rejected", "replaced":
		writeErr(w, d, errUnprocessable("Klaim ini sudah selesai diproses."))
		return
	}
	if reason := d.lgEligibilityReason(claim); reason != "" {
		writeErr(w, d, errUnprocessable(reason))
		return
	}

	brandID := claim.strOr("brand_id", "")
	publicationID := claim.strOr("publication_id", "")
	var tc, idr int
	var reversal db.PartnerEarningReversal
	if err := db.WithPoolTx(r.Context(), d.Pool, func(tx pgx.Tx) error {
		var rErr error
		tc, idr, rErr = db.RefundCapturedCreditTx(r.Context(), tx, brandID, publicationID, user.ID)
		if rErr != nil {
			return rErr
		}
		if tc <= 0 {
			return errUnprocessable("Tidak ada kredit yang bisa dikembalikan untuk artikel ini.")
		}
		if reversal, rErr = db.ReversePartnerEarningTx(r.Context(), tx, publicationID); rErr != nil {
			return rErr
		}
		_, rErr = tx.Exec(r.Context(), `
			UPDATE link_guardian_claims
			SET status = 'refunded', refund_tc = $1, refund_idr = $2, refunded_at = NOW(),
			    resolution_note = COALESCE(NULLIF($3, ''), resolution_note),
			    updated_by = $4, updated_at = NOW()
			WHERE id = $5`, tc, idr, strings.TrimSpace(resolutionNote), user.ID, claimID)
		return rErr
	}); err != nil {
		writeErr(w, d, err)
		return
	}
	d.lgNotifyOutcome(r.Context(), brandID, claim.strOr("partner_id", ""), claim.strOr("media_name", ""), reversal,
		"Kredit dikembalikan: garansi tautan",
		fmt.Sprintf("%d TC dikembalikan ke saldo Anda karena tautan artikel di %s tidak bertahan sesuai garansi.", tc, claim.strOr("media_name", "media")))
	d.audit(r.Context(), user.ID, "platform.link_guardian.refund", "link_guardian_claim", claimID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "refund_tc": tc, "refund_idr": idr})
}

// LinkGuardianClaimReject: POST /v1/platform/link-guardian/claims/{claimId}/reject
func (d *Deps) LinkGuardianClaimReject(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	claimID := chiParam(r, "claimId")
	body := decodeJSON(r)
	reason, _ := body["reason"].(string)
	reason = strings.TrimSpace(reason)
	if len(reason) < 5 {
		writeErr(w, d, errUnprocessable("Alasan penolakan minimal 5 karakter."))
		return
	}
	tag, err := d.execRows(r.Context(), `
		UPDATE link_guardian_claims
		SET status = 'rejected', rejection_reason = $1, updated_by = $2, updated_at = NOW()
		WHERE id = $3 AND status IN ('submitted', 'checking', 'contacted')`,
		reason, user.ID, claimID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if tag == 0 {
		writeErr(w, d, errUnprocessable("Klaim tidak ditemukan atau sudah selesai diproses."))
		return
	}
	if info, _ := d.queryRow(r.Context(), `
		SELECT c.brand_id, c.rejection_reason, m.name AS media_name
		FROM link_guardian_claims c
		JOIN publications p ON p.id = c.publication_id
		LEFT JOIN media m ON m.id = p.media_id
		WHERE c.id = $1 LIMIT 1`, claimID); info != nil {
		link := "/klien/" + info.strOr("brand_id", "") + "/link-guardian"
		d.mpNotifyBrandMembers(r.Context(), info.strOr("brand_id", ""), "publication.guarantee_update",
			"Klaim garansi tautan ditolak",
			"Klaim untuk artikel di "+info.strOr("media_name", "media")+" ditolak: "+reason, &link)
	}
	d.audit(r.Context(), user.ID, "platform.link_guardian.reject", "link_guardian_claim", claimID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// lgEligibilityReason: alasan klaim tidak layak digaransi ("" = layak).
// Semua penilaian memakai kolom yang sudah dihitung di SQL supaya tidak ada
// perhitungan tanggal yang tersebar di Go.
func (d *Deps) lgEligibilityReason(row rowMap) string {
	if !row.boolOr("in_warranty", false) {
		months := row.intOr("link_duration_months", 12)
		return fmt.Sprintf("Masa garansi tayang media ini (%d bulan) sudah habis untuk artikel ini.", months)
	}
	if !row.boolOr("domain_verified", false) {
		return "Artikel ini diterbitkan di media yang belum terverifikasi, jadi tidak masuk garansi."
	}
	if db.LinkViolationActive(row.strOr("link_state", ""), row.strOr("in_article_state", ""),
		row.strOr("rel_state", ""), row.strOr("noindex_state", ""),
		row.strOr("violation_kind", ""), row.intOr("violation_streak", 0)) == "" {
		return "Artikel masih sehat: tidak ada 404, tautan dicabut, noindex, maupun perubahan atribut tautan."
	}
	return ""
}

// lgNotifyOutcome: beri tahu klien hasil garansi dan, bila penghasilan mitra
// dibatalkan atau dipotong, beri tahu mitranya juga. Best-effort.
func (d *Deps) lgNotifyOutcome(ctx context.Context, brandID, partnerID, mediaName string, reversal db.PartnerEarningReversal, title, body string) {
	link := "/klien/" + brandID + "/link-guardian"
	d.mpNotifyBrandMembers(ctx, brandID, "publication.guarantee_update", title, body, &link)
	if partnerID == "" || reversal.Action == "none" || reversal.Action == "" {
		return
	}
	partnerLink := "/mitra"
	msg := "Penghasilan artikel di " + mediaName + " dibatalkan karena tautan tidak bertahan sesuai garansi."
	if reversal.Action == "charged" {
		msg = "Penghasilan artikel di " + mediaName + " sudah ditarik, jadi nilainya dipotong dari saldo Anda berikutnya karena tautan tidak bertahan sesuai garansi."
	}
	d.mpNotifyPartner(ctx, partnerID, "partner.guarantee_reversed", "Penghasilan artikel dibatalkan", msg, &partnerLink)
}
