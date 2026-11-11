package platform

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
)

// link_guarantee.go: sisi platform dari garansi tautan, yaitu pemrosesan pilihan
// klien "artikel pengganti". Pilihan "kredit kembali" selesai di sisi klien
// (brandflow/link_guarantee.go) karena hanya memindahkan saldo; pembuatan
// publikasi pengganti memakai pemilihan media milik paket ini
// (mpCreateReplacementPublication), jadi dijalankan sebagai job dari sini.

// jobKindLinkGuaranteeReplace: payload {claimId}.
const jobKindLinkGuaranteeReplace = "link_guarantee_replace"

func (d *Deps) registerLinkGuaranteeJobs() {
	jobs.Register(jobKindLinkGuaranteeReplace, func(ctx context.Context, job jobs.JobRow) error {
		id, _ := job.Payload["claimId"].(string)
		if id == "" {
			return fmt.Errorf("payload claimId kosong")
		}
		return d.LinkGuaranteeReplaceJob(ctx, id)
	})
}

// LinkGuaranteeReplaceJob: buat artikel pengganti untuk klaim yang klien-nya
// memilih "pengganti". Tidak bergantung pada pengaturan auto-replace brand:
// garansi adalah janji platform, bukan fitur opsional. Satu klaim per
// publikasi (UNIQUE), jadi paling banyak satu pengganti per artikel.
//
// Bila tidak ada media pengganti yang cocok, pilihan klien dikosongkan dan
// klien diberi tahu supaya bisa memilih kredit kembali; klaim tidak menggantung.
func (d *Deps) LinkGuaranteeReplaceJob(ctx context.Context, claimID string) error {
	claim, err := d.queryRow(ctx, `
		SELECT id, publication_id, brand_id, status, client_choice, replacement_publication_id
		FROM link_guardian_claims WHERE id = $1 LIMIT 1`, claimID)
	if err != nil {
		return err
	}
	if claim == nil {
		return nil
	}
	switch claim.strOr("status", "") {
	case "refunded", "rejected", "replaced":
		return nil
	}
	if claim.strOr("client_choice", "") != "replacement" || claim.strOr("replacement_publication_id", "") != "" {
		return nil
	}
	publicationID := claim.strOr("publication_id", "")
	brandID := claim.strOr("brand_id", "")

	pub, err := d.queryRow(ctx, `
		SELECT p.id, p.brand_id, p.document_id, p.campaign_id, p.client_price, p.publish_status, p.created_by,
		       d.title AS document_title, m.name AS media_name, m.category AS media_category, m.partner_id
		FROM publications p
		JOIN content_documents d ON d.id = p.document_id
		JOIN media m ON m.id = p.media_id
		WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil {
		return err
	}
	if pub == nil {
		return nil
	}
	row := ptRow(ptNormalizeMap(pub))
	mediaName := row.strOr("media_name", "media")
	link := "/klien/" + brandID + "/link-guardian"

	created, candidate, reason := d.mpCreateReplacementPublication(ctx, row, publicationID)
	if created == "" {
		slog.Warn("[link-guarantee] artikel pengganti tidak dibuat", "claim", claimID, "reason", reason)
		if _, err := d.execRows(ctx, `
			UPDATE link_guardian_claims
			SET client_choice = NULL, client_chosen_at = NULL, updated_at = NOW()
			WHERE id = $1 AND status IN ('submitted', 'checking', 'contacted')`, claimID); err != nil {
			return err
		}
		d.mpNotifyBrandMembers(ctx, brandID, "publication.guarantee_update",
			"Belum ada media pengganti yang cocok",
			"Kami belum menemukan media pengganti untuk artikel di "+mediaName+". Silakan pilih kredit kembali ke saldo di halaman Link Guardian.", &link)
		return nil
	}

	var reversal db.PartnerEarningReversal
	if err := db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		var rErr error
		if reversal, rErr = db.ReversePartnerEarningTx(ctx, tx, publicationID); rErr != nil {
			return rErr
		}
		_, rErr = tx.Exec(ctx, `
			UPDATE link_guardian_claims
			SET status = 'replaced', replacement_publication_id = $1,
			    resolution_note = COALESCE(resolution_note, 'Klien memilih artikel pengganti'),
			    updated_at = NOW()
			WHERE id = $2`, created, claimID)
		return rErr
	}); err != nil {
		// Pengganti sudah dibuat; jangan buat lagi pada percobaan ulang.
		slog.Error("[link-guarantee] pengganti dibuat tapi klaim gagal diperbarui", "claim", claimID, "replacement", created, "err", err)
		_, _ = d.execRows(ctx, "UPDATE link_guardian_claims SET replacement_publication_id = $1 WHERE id = $2", created, claimID)
		return nil
	}

	d.lgNotifyOutcome(ctx, brandID, row.strOr("partner_id", ""), mediaName, reversal,
		"Artikel pengganti dibuat",
		"\""+row.strOr("document_title", "Artikel")+"\" dialihkan ke "+candidate.Name+" untuk diproses ulang, tanpa biaya tambahan.")
	return nil
}
