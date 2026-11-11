package brandflow

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// link_guarantee.go: pilihan klien saat garansi tautan terpicu.
//
// Klaim dibuka otomatis oleh pengecekan terjadwal (job_link_check.go,
// openGuaranteeClaim) setelah dua pengecekan berturut-turut menemukan
// pelanggaran. Klien lalu memilih sendiri remedinya:
//   - "credit": kredit artikel itu kembali ke saldo, selesai di sini dalam satu
//     transaksi (refund, pembatalan penghasilan mitra, klaim ditutup);
//   - "replacement": artikel dibuatkan ulang di media lain. Pemilihan media ada
//     di paket platform, jadi di sini hanya mencatat pilihan dan mengantre job
//     link_guarantee_replace (platform/link_guarantee.go).
//
// Kelayakan dinilai ulang di server (bukan hanya di tampilan) dengan definisi
// yang sama dengan admin: masih dalam masa garansi media, media terverifikasi,
// dan pelanggaran masih terbaca pada pengecekan terakhir.

var errGuaranteeNoCredit = errors.New("Tidak ada kredit yang bisa dikembalikan untuk artikel ini.")
var errGuaranteeTaken = errors.New("Klaim ini sudah dipilih atau selesai diproses.")

// LinkGuaranteeChoose: POST /v1/brands/{brandId}/link-monitor/guarantee/choose
// Body: {publicationId, choice: "credit" | "replacement"}.
func (d *Deps) LinkGuaranteeChoose(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	publicationID, _ := body["publicationId"].(string)
	if !bsIsUUID(publicationID) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid UUID")
		return
	}
	choice, _ := body["choice"].(string)
	if choice != "credit" && choice != "replacement" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Pilihan harus 'credit' atau 'replacement'.")
		return
	}

	ctx := r.Context()
	claim, err := d.queryRow(ctx, `
		SELECT c.id, c.status, c.client_choice,
		       m.partner_id, m.name AS media_name, d.title AS document_title,
		       COALESCE(m.domain_verified, FALSE) AS domain_verified,
		       lc.state AS link_state, lc.in_article_state, lc.rel_state, lc.noindex_state,
		       lc.violation_kind, lc.violation_streak,
		       `+db.LinkWarrantySQL("p", "m")+` AS in_warranty
		FROM link_guardian_claims c
		JOIN publications p ON p.id = c.publication_id
		JOIN media m ON m.id = p.media_id
		LEFT JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN publication_link_checks lc ON lc.publication_id = p.id
		WHERE c.publication_id = $1 AND p.brand_id = $2
		LIMIT 1`, publicationID, brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if claim == nil {
		httpx.Error(w, http.StatusNotFound, "Belum ada klaim garansi untuk artikel ini.")
		return
	}
	claimID := claim.strOr("id", "")

	switch claim.strOr("status", "") {
	case "submitted", "checking", "contacted":
	default:
		httpx.Error(w, http.StatusConflict, errGuaranteeTaken.Error())
		return
	}
	if claim.strOr("client_choice", "") != "" {
		httpx.Error(w, http.StatusConflict, errGuaranteeTaken.Error())
		return
	}
	if !claim.boolOr("in_warranty", false) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Masa garansi tayang media ini sudah habis untuk artikel ini.")
		return
	}
	if !claim.boolOr("domain_verified", false) {
		httpx.Error(w, http.StatusUnprocessableEntity, "Artikel ini diterbitkan di media yang belum terverifikasi, jadi tidak masuk garansi.")
		return
	}
	if db.LinkViolationActive(claim.strOr("link_state", ""), claim.strOr("in_article_state", ""),
		claim.strOr("rel_state", ""), claim.strOr("noindex_state", ""),
		claim.strOr("violation_kind", ""), claim.intOr("violation_streak", 0)) == "" {
		httpx.Error(w, http.StatusUnprocessableEntity, "Tautan artikel ini sudah sehat kembali, jadi tidak ada yang perlu diganti.")
		return
	}

	mediaName := claim.strOr("media_name", "media")
	link := "/klien/" + brandRow.ID + "/link-guardian"

	if choice == "replacement" {
		tag, err := d.execRows(ctx, `
			UPDATE link_guardian_claims
			SET client_choice = 'replacement', client_chosen_at = NOW(), updated_by = $2, updated_at = NOW()
			WHERE id = $1 AND status IN ('submitted', 'checking', 'contacted') AND client_choice IS NULL`,
			claimID, user.ID)
		if err != nil {
			d.writeErr(w, err)
			return
		}
		if tag == 0 {
			httpx.Error(w, http.StatusConflict, errGuaranteeTaken.Error())
			return
		}
		if _, err := d.enqueueJobAt(ctx, "link_guarantee_replace", map[string]any{"claimId": claimID}, nil, 2); err != nil {
			// Antrean gagal: lepas pilihan supaya klien bisa mencoba lagi.
			_, _ = d.execRows(ctx, `
				UPDATE link_guardian_claims SET client_choice = NULL, client_chosen_at = NULL, updated_at = NOW()
				WHERE id = $1 AND status IN ('submitted', 'checking', 'contacted')`, claimID)
			d.writeErr(w, err)
			return
		}
		d.audit(ctx, brandRow.ID, user.ID, "publication.guarantee.replacement_chosen", "publication", publicationID)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "choice": "replacement", "publication_id": publicationID})
		return
	}

	var tc, idr int
	var reversal db.PartnerEarningReversal
	txErr := db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		// Klaim dikunci lebih dulu: dua klik bersamaan tidak bisa sama-sama lolos.
		tag, err := tx.Exec(ctx, `
			UPDATE link_guardian_claims
			SET client_choice = 'credit', client_chosen_at = NOW(), updated_by = $2, updated_at = NOW()
			WHERE id = $1 AND status IN ('submitted', 'checking', 'contacted') AND client_choice IS NULL`,
			claimID, user.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errGuaranteeTaken
		}
		if tc, idr, err = db.RefundCapturedCreditTx(ctx, tx, brandRow.ID, publicationID, user.ID); err != nil {
			return err
		}
		if tc <= 0 {
			return errGuaranteeNoCredit
		}
		if reversal, err = db.ReversePartnerEarningTx(ctx, tx, publicationID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE link_guardian_claims
			SET status = 'refunded', refund_tc = $1, refund_idr = $2, refunded_at = NOW(),
			    resolution_note = COALESCE(resolution_note, 'Klien memilih kredit kembali ke saldo')
			WHERE id = $3`, tc, idr, claimID)
		return err
	})
	switch {
	case errors.Is(txErr, errGuaranteeTaken):
		httpx.Error(w, http.StatusConflict, errGuaranteeTaken.Error())
		return
	case errors.Is(txErr, errGuaranteeNoCredit):
		httpx.Error(w, http.StatusUnprocessableEntity, errGuaranteeNoCredit.Error())
		return
	case txErr != nil:
		d.writeErr(w, txErr)
		return
	}

	d.notifyBrandMembersOnPub(ctx, brandRow.ID, "publication.guarantee_update", "Kredit dikembalikan: garansi tautan",
		strings.TrimSpace(itoa(tc)+" TC dikembalikan ke saldo Anda karena tautan artikel di "+mediaName+" tidak bertahan sesuai garansi."),
		&link, publicationID)
	if partnerID := claim.strOr("partner_id", ""); partnerID != "" && reversal.Action != "none" && reversal.Action != "" {
		msg := "Penghasilan artikel di " + mediaName + " dibatalkan karena tautan tidak bertahan sesuai garansi."
		if reversal.Action == "charged" {
			msg = "Penghasilan artikel di " + mediaName + " sudah ditarik, jadi nilainya dipotong dari saldo Anda berikutnya karena tautan tidak bertahan sesuai garansi."
		}
		_ = d.exec(ctx, `
			INSERT INTO notifications (id, user_id, type, title, body, link)
			SELECT gen_random_uuid(), user_id, 'partner.guarantee_reversed', 'Penghasilan artikel dibatalkan', $1, '/mitra'
			FROM partners WHERE id = $2 AND user_id IS NOT NULL`, msg, partnerID)
	}
	d.audit(ctx, brandRow.ID, user.ID, "publication.guarantee.credit_chosen", "publication", publicationID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "choice": "credit", "publication_id": publicationID, "refund_tc": tc, "refund_idr": idr})
}
