package brandflow

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pembatalan oleh admin operasi.

var (
	// ErrPublicationPublished: artikel sudah terbit, pembatalan tidak lagi
	// aman karena ada URL publik dan potensi sengketa dengan media.
	ErrPublicationPublished = errors.New("publikasi sudah terbit")
	// ErrPublicationAlreadyCancelled: idempoten untuk UI, bukan error sistem.
	ErrPublicationAlreadyCancelled = errors.New("publikasi sudah dibatalkan")
	// ErrPublicationStatusChanged: status berubah di antara baca dan tulis
	// (mis. media menyetujui tepat saat admin menekan Batal).
	ErrPublicationStatusChanged = errors.New("status publikasi berubah")
)

// CancelPublicationAsAdmin membatalkan publikasi dari panel operasi admin.
//
// Memakai jalur pengembalian saldo yang sama dengan pembatalan klien dan sweep
// SLA (settleCancelledPublication), supaya tidak ada jalur refund kedua yang
// bisa menyimpang. Admin boleh membatalkan status apa pun kecuali yang sudah
// terbit atau sudah batal; aturan state machine klien/ mitra tidak dipakai di
// sini karena ini override operasional.
func CancelPublicationAsAdmin(ctx context.Context, pool *pgxpool.Pool, publicationID, adminUserID, reason string) error {
	if pool == nil {
		return errors.New("database tidak siap")
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "Dibatalkan oleh admin."
	}
	if runes := []rune(reason); len(runes) > 500 {
		reason = string(runes[:500])
	}

	d := &Deps{Pool: pool}
	row, err := d.queryRow(ctx, `
		SELECT p.brand_id, p.status, m.name AS media_name, d.title AS document_title,
		       pa.user_id AS partner_user_id, pa.public_id AS partner_public_id,
		       b.public_id AS brand_public_id
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN content_documents d ON d.id = p.document_id
		JOIN brands b ON b.id = p.brand_id
		LEFT JOIN partners pa ON pa.id = m.partner_id
		WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil {
		return err
	}
	if row == nil {
		return errNotFound("Publication not found")
	}
	current := row.strOr("status", "")
	switch current {
	case "published":
		return ErrPublicationPublished
	case "cancelled":
		return ErrPublicationAlreadyCancelled
	}

	brandID := row.strOr("brand_id", "")
	n, err := d.execRows(ctx,
		`UPDATE publications
		    SET status = 'cancelled', cancel_reason = $1, updated_at = NOW()
		  WHERE id = $2 AND status = $3`, reason, publicationID, current)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrPublicationStatusChanged
	}

	d.settleCancelledPublication(ctx, brandID, publicationID, adminUserID, "publication.cancelled_by_admin")

	article := row.strOr("document_title", "Artikel")
	mediaName := row.strOr("media_name", "Media")

	if partnerUserID := row.strOr("partner_user_id", ""); partnerUserID != "" {
		link := "/mitra/publications?pub=" + publicationID
		if publicID := row.strOr("partner_public_id", ""); publicID != "" {
			link = "/mitra/" + publicID + "/publications?pub=" + publicationID
		}
		body := "Pesanan \"" + article + "\" untuk " + mediaName +
			" dibatalkan oleh admin. Alasan: " + reason
		d.insertNotificationOnce(ctx, partnerUserID, "publication.cancelled_by_admin",
			"Pesanan dibatalkan oleh admin", body, link, publicationID, "admin_cancel:"+publicationID)
	}
	if brandID != "" {
		brandPublicID := row.strOr("brand_public_id", brandID)
		brandLink := "/klien/" + brandPublicID + "/publications?pub=" + publicationID
		brandBody := "Pesanan \"" + article + "\" ke " + mediaName +
			" dibatalkan oleh admin. Alasan: " + reason +
			" Kredit reservasi sudah dikembalikan ke Saldo Akun."
		d.notifyBrandMembersOnPub(ctx, brandID, "publication.cancelled_by_admin",
			"Pesanan dibatalkan oleh admin", brandBody, &brandLink, publicationID)
	}
	return nil
}
