package platform

// dewan_pers.go: review verifikasi Dewan Pers admin —
// GET /v1/platform/media/dewan-pers-reviews dan
// POST /v1/platform/media/dewan-pers-reviews/{reviewId}.
// Port app/v1/platform/media/dewan-pers-reviews/route.ts + [reviewId]/route.ts
// dan reviewDewanPersVerification di lib/media/dewan-pers.ts.
//
// Logika review sengaja DISALIN dari paket partner (yang menyimpan versi mitra)
// agar paket platform tidak mengimpor partner dan membentuk siklus.

import (
	"context"
	"net/http"
	"strings"
)

// mpDewanPersStatuses: status yang boleh difilter.
var mpDewanPersStatuses = []string{"pending", "verified", "rejected", "conflict", "superseded"}

// ---------- GET /v1/platform/media/dewan-pers-reviews ----------

func (d *Deps) MediaDewanPersReviews(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && !mpContains(mpDewanPersStatuses, status) {
		writeErr(w, d, errUnprocessable("Invalid status filter"))
		return
	}
	limit := mpClampInt(q.Get("limit"), 100, 1, 500)
	offset := mpClampInt(q.Get("offset"), 0, 0, 1<<30)

	where := []string{"v.status <> 'superseded'"}
	params := []any{}
	if status != "" {
		params = append(params, status)
		where = append(where, "v.status = $"+ptItoa(len(params)))
	}
	clause := "WHERE " + strings.Join(where, " AND ")

	countRow, err := d.queryRow(r.Context(), `
		SELECT COUNT(*) AS count
		FROM media_dewan_pers_verifications v
		`+clause, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	pageArgs := append(append([]any{}, params...), limit, offset)
	items, err := d.queryRows(r.Context(), `
		SELECT v.id, v.media_id, v.partner_id, v.certificate_no, v.status, v.rejection_reason, v.conflict_with,
		       v.submitted_at, v.reviewed_by, v.reviewed_at,
		       m.name AS media_name, m.website_url AS media_domain, m.dewan_pers_doc_key,
		       p.name AS partner_name, p.public_id AS partner_public_id,
		       conflict_media.name AS conflict_media_name
		FROM media_dewan_pers_verifications v
		JOIN media m ON m.id = v.media_id
		LEFT JOIN partners p ON p.id = v.partner_id
		LEFT JOIN media conflict_media ON conflict_media.id = v.conflict_with
		`+clause+`
		ORDER BY CASE v.status WHEN 'pending' THEN 0 WHEN 'conflict' THEN 1 WHEN 'rejected' THEN 2 ELSE 3 END, v.submitted_at DESC
		LIMIT $`+ptItoa(len(params)+1)+` OFFSET $`+ptItoa(len(params)+2), pageArgs...)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		out = append(out, map[string]any{
			"id":                  row.anyOrNil("id"),
			"media_id":            row.anyOrNil("media_id"),
			"media_name":          row.anyOrNil("media_name"),
			"media_domain":        row.anyOrNil("media_domain"),
			"dewan_pers_doc_key":  row.anyOrNil("dewan_pers_doc_key"),
			"partner_id":          row.anyOrNil("partner_id"),
			"partner_name":        row.anyOrNil("partner_name"),
			"partner_public_id":   row.anyOrNil("partner_public_id"),
			"certificate_no":      row.anyOrNil("certificate_no"),
			"status":              row.anyOrNil("status"),
			"rejection_reason":    row.anyOrNil("rejection_reason"),
			"conflict_with":       row.anyOrNil("conflict_with"),
			"conflict_media_name": row.anyOrNil("conflict_media_name"),
			"submitted_at":        row.anyOrNil("submitted_at"),
			"reviewed_at":         row.anyOrNil("reviewed_at"),
		})
	}
	total := 0
	if countRow != nil {
		total = ptRow(ptNormalizeMap(countRow)).intOr("count", 0)
	}
	writeJSON(w, http.StatusOK, map[string]any{"reviews": out, "total": total, "limit": limit, "offset": offset})
}

// ---------- POST /v1/platform/media/dewan-pers-reviews/{reviewId} ----------

func (d *Deps) MediaDewanPersReviewDecide(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	reviewID := chiParam(r, "reviewId")
	body := decodeJSON(r)
	decision, hasDecision := strField(body, "decision")
	if !hasDecision || (decision != "approve" && decision != "reject") {
		writeErr(w, d, errUnprocessable(mpZEnum([]string{"approve", "reject"})))
		return
	}
	reason, hasReason := strField(body, "reason")
	if hasReason && len(reason) > 1000 {
		writeErr(w, d, errUnprocessable("Too big: expected string to have <=1000 characters"))
		return
	}
	if decision == "reject" && reason == "" {
		// superRefine zod → ZodError → 422 (bukan 409); 409 hanya untuk error lib
		// (sudah diproses / nomor dipakai media lain / tidak ditemukan).
		writeErr(w, d, errUnprocessable("Alasan penolakan wajib diisi."))
		return
	}

	verification, err := d.mpReviewDewanPersVerification(r.Context(), reviewID, decision, reason, user.ID)
	if err != nil {
		writeErr(w, d, err)
		return
	}

	// Notifikasi ke mitra pemilik media (best-effort).
	media, _ := d.queryRow(r.Context(), "SELECT name, partner_id FROM media WHERE id = $1 LIMIT 1", verification.strOr("media_id", ""))
	partner, _ := d.queryRow(r.Context(), "SELECT public_id FROM partners WHERE id = $1 LIMIT 1", verification.strOr("partner_id", ""))
	partnerID := verification.strOr("partner_id", "")
	linkTarget := partnerID
	if partner != nil {
		if pid := ptRow(ptNormalizeMap(partner)).strOr("public_id", ""); pid != "" {
			linkTarget = pid
		}
	}
	link := "/mitra/" + linkTarget + "/media"
	mediaName := "Media"
	if media != nil {
		if n := ptRow(ptNormalizeMap(media)).strOr("name", ""); n != "" {
			mediaName = n
		}
	}
	if decision == "approve" {
		d.mpNotifyPartner(r.Context(), partnerID, "media.dewan_pers.approved", "Verifikasi Dewan Pers disetujui",
			`Media "`+mediaName+`" telah diverifikasi oleh Admin Tulisio.`, &link)
	} else {
		d.mpNotifyPartner(r.Context(), partnerID, "media.dewan_pers.rejected", "Verifikasi Dewan Pers ditolak",
			`Verifikasi Dewan Pers untuk "`+mediaName+`" ditolak. Alasan: `+reason+`. Silakan perbaiki lalu ajukan kembali.`, &link)
	}

	// Email follow-up (best-effort): partner user + email.
	event := "media.dewan_pers.rejected"
	if decision == "approve" {
		event = "media.dewan_pers.approved"
	}
	var partnerUserID, partnerEmail string
	if emailRow, err := d.queryRow(r.Context(),
		"SELECT email FROM users WHERE id = (SELECT user_id FROM partners WHERE id = $1) LIMIT 1", partnerID); err == nil && emailRow != nil {
		partnerEmail = ptRow(ptNormalizeMap(emailRow)).strOr("email", "")
	}
	if partnerRow, err := d.queryRow(r.Context(), "SELECT user_id FROM partners WHERE id = $1 LIMIT 1", partnerID); err == nil && partnerRow != nil {
		partnerUserID = ptRow(ptNormalizeMap(partnerRow)).strOr("user_id", "")
	}
	d.mpEmailFollowUp(r.Context(), event, partnerUserID, partnerEmail, verification.strOr("media_id", ""), map[string]string{
		"media_name": mediaName,
		"cert_no":    verification.strOr("certificate_no", ""),
		"reason":     reason,
		"link":       link,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":               true,
		"status":           verification.anyOrNil("status"),
		"certificate_no":   verification.anyOrNil("certificate_no"),
		"media_id":         verification.anyOrNil("media_id"),
		"rejection_reason": verification.anyOrNil("rejection_reason"),
	})
}

// mpReviewDewanPersVerification: port reviewDewanPersVerification.
// Pesan error mengikuti route lama: "Verifikasi tidak ditemukan",
// "Verifikasi sudah diproses (bukan pending).", "Alasan penolakan wajib diisi.",
// lalu 409 karena polanya cocok dengan /wajib diisi|sudah digunakan|bukan pending|tidak ditemukan/i.
func (d *Deps) mpReviewDewanPersVerification(ctx context.Context, reviewID, decision, reason, actorUserID string) (ptRow, error) {
	existing, err := d.queryRow(ctx, "SELECT * FROM media_dewan_pers_verifications WHERE id = $1 LIMIT 1", reviewID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errConflict("Verifikasi tidak ditemukan")
	}
	row := ptRow(ptNormalizeMap(existing))
	if status := row.strOr("status", ""); status != "pending" && status != "conflict" {
		return nil, errConflict("Verifikasi sudah diproses (bukan pending).")
	}

	if decision == "reject" {
		trimmed := strings.TrimSpace(reason)
		if trimmed == "" {
			return nil, errConflict("Alasan penolakan wajib diisi.")
		}
		if err := d.exec(ctx,
			"UPDATE media_dewan_pers_verifications SET status = 'rejected', rejection_reason = $1, reviewed_by = $2, reviewed_at = NOW(), updated_at = NOW() WHERE id = $3",
			trimmed, actorUserID, reviewID); err != nil {
			return nil, err
		}
	} else {
		// Approve: guard ganda — nomor tidak boleh sedang diklaim media lain
		// (verified/pending).
		certificateNo := row.strOr("certificate_no", "")
		mediaID := row.strOr("media_id", "")
		other, err := d.queryRow(ctx, `
			SELECT * FROM media_dewan_pers_verifications
			WHERE certificate_no = $1 AND media_id <> $2 AND status IN ('pending','verified')
			ORDER BY submitted_at DESC LIMIT 1`, certificateNo, mediaID)
		if err != nil {
			return nil, err
		}
		if other != nil {
			if row.strOr("status", "") != "conflict" {
				if err := d.exec(ctx,
					"UPDATE media_dewan_pers_verifications SET status = 'conflict', conflict_with = $1, reviewed_by = $2, reviewed_at = NOW(), updated_at = NOW() WHERE id = $3",
					ptRow(ptNormalizeMap(other)).anyOrNil("media_id"), actorUserID, reviewID); err != nil {
					return nil, err
				}
			}
			return nil, errConflict("Nomor sertifikat ini sudah digunakan oleh media lain dan memerlukan pemeriksaan Admin.")
		}
		if err := d.exec(ctx,
			"UPDATE media_dewan_pers_verifications SET status = 'verified', rejection_reason = NULL, conflict_with = NULL, reviewed_by = $1, reviewed_at = NOW(), updated_at = NOW() WHERE id = $2",
			actorUserID, reviewID); err != nil {
			return nil, err
		}
	}

	updated, err := d.queryRow(ctx, "SELECT * FROM media_dewan_pers_verifications WHERE id = $1 LIMIT 1", reviewID)
	if err != nil || updated == nil {
		return nil, err
	}
	urow := ptRow(ptNormalizeMap(updated))
	d.mpSyncDewanPersMediaColumns(ctx, urow.strOr("media_id", ""))

	action := "media.dewan_pers.rejected"
	if decision == "approve" {
		action = "media.dewan_pers.approved"
	}
	writeAuditBestEffort(d, ctx, actorUserID, action, "media", urow.strOr("media_id", ""))
	return urow, nil
}
