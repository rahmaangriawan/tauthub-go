package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// link_guarantee.go: satu definisi garansi tautan yang dipakai pengecekan
// terjadwal, klaim klien, dan admin, supaya ketiganya tidak berbeda pendapat
// soal apa yang disebut pelanggaran dan apa yang terjadi pada uang.

// LinkViolationKind: jenis pelanggaran dari hasil pengecekan terakhir, atau ""
// bila tautan sehat. Nilainya sama dengan link_guardian_claims.detection_state.
// Urutan prioritas: artikel hilang, tautan dicabut, noindex, rel berubah.
func LinkViolationKind(linkState, inArticleState, relState, noindexState string) string {
	switch {
	case linkState == "gone":
		return "gone"
	case inArticleState == "removed":
		return "link_removed"
	case noindexState == "noindex":
		return "noindex"
	case relState == "changed":
		return "rel_changed"
	}
	return ""
}

// LinkViolationThreshold: pengecekan berturut-turut yang menemukan pelanggaran
// sebelum garansi terpicu. Sekali bisa cuma jaringan atau WAF yang berkedip.
const LinkViolationThreshold = 2

// LinkViolationActive: seperti LinkViolationKind, tapi juga menghitung bukti
// yang sudah terkumpul. Pengecekan yang tidak berinformasi (situs tak
// terjangkau) mengosongkan kolom hasil terbaru, padahal pelanggaran sudah
// terbaca dua kali; hitungan berturut-turut (violationStreak) mempertahankannya
// sampai pengecekan yang berinformasi menyatakan tautan sehat.
func LinkViolationActive(linkState, inArticleState, relState, noindexState, violationKind string, violationStreak int) string {
	if k := LinkViolationKind(linkState, inArticleState, relState, noindexState); k != "" {
		return k
	}
	if violationStreak >= LinkViolationThreshold {
		return violationKind
	}
	return ""
}

// LinkViolationLabel: kalimat untuk pengguna tentang jenis pelanggaran.
func LinkViolationLabel(kind string) string {
	switch kind {
	case "gone":
		return "artikel tidak ditemukan lagi (404)"
	case "link_removed":
		return "tautan di dalam artikel dicabut"
	case "noindex":
		return "artikel ditandai noindex sehingga tidak muncul di Google"
	case "rel_changed":
		return "atribut tautan berubah dari yang dijanjikan"
	}
	return "tautan bermasalah"
}

// RefundCapturedCreditTx: pindahkan kredit yang sudah tercapture kembali ke
// saldo tersedia dan catat di ledger sebagai pengembalian (amount negatif).
// Idempoten per publikasi lewat kunci "guardian-refund:<publicationId>", sama
// dengan jalur admin, sehingga dua jalur tidak bisa mengembalikan dua kali.
// Mengembalikan (0, 0, nil) bila tidak ada kredit tersisa atau sudah pernah.
func RefundCapturedCreditTx(ctx context.Context, tx pgx.Tx, brandID, publicationID, userID string) (int, int, error) {
	var captured, refunded int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_tc) FILTER (WHERE ref_type = 'capture'), 0)::int,
		       COALESCE(SUM(-amount_tc) FILTER (WHERE ref_type = 'refund'), 0)::int
		FROM credit_ledger_entries
		WHERE ref_id = $1 AND brand_id = $2`, publicationID, brandID).Scan(&captured, &refunded); err != nil {
		return 0, 0, err
	}
	outstanding := captured - refunded
	if outstanding <= 0 {
		return 0, 0, nil
	}

	key := "guardian-refund:" + publicationID
	var exists int
	if err := tx.QueryRow(ctx, "SELECT 1 FROM credit_ledger_entries WHERE idempotency_key = $1 LIMIT 1", key).Scan(&exists); err == nil {
		return 0, 0, nil
	} else if err != pgx.ErrNoRows {
		return 0, 0, err
	}

	rate := 1000
	if err := tx.QueryRow(ctx, "SELECT rate_idr FROM tc_config WHERE id = 1 LIMIT 1").Scan(&rate); err != nil && err != pgx.ErrNoRows {
		return 0, 0, err
	}
	if rate < 1 {
		rate = 1000
	}

	var available, reserved, capturedAfter int
	if err := tx.QueryRow(ctx, `
		UPDATE credit_wallets
		SET captured_tc = GREATEST(captured_tc - $1, 0), available_tc = available_tc + $1, updated_at = NOW()
		WHERE brand_id = $2
		RETURNING available_tc, reserved_tc, captured_tc`, outstanding, brandID).Scan(&available, &reserved, &capturedAfter); err != nil {
		if err == pgx.ErrNoRows {
			return 0, 0, fmt.Errorf("Dompet kredit brand tidak ditemukan.")
		}
		return 0, 0, err
	}
	total := available + reserved + capturedAfter
	if _, err := tx.Exec(ctx, `
		INSERT INTO credit_ledger_entries (id, brand_id, ref_type, ref_id, amount_tc, balance_after_tc, rate_idr, amount_idr, reason, idempotency_key, created_by)
		VALUES ($1, $2, 'refund', $3, $4, $5, $6, $7, $8, $9, $10)`,
		uuid.NewString(), brandID, publicationID, -outstanding, total, rate, -(outstanding * rate),
		"Pengembalian saldo klaim Link Guardian (garansi tayang)", key, userID); err != nil {
		return 0, 0, err
	}
	return outstanding, outstanding * rate, nil
}

// PartnerEarningReversal: hasil membatalkan penghasilan mitra atas satu publikasi.
type PartnerEarningReversal struct {
	PartnerID string
	Amount    float64
	// Action: "none" (tidak ada penghasilan / sudah dibatalkan), "voided"
	// (belum ditarik, statusnya dibatalkan), atau "charged" (sudah masuk
	// pengajuan penarikan, jadi dicatat sebagai potongan dari saldo berikutnya).
	Action string
}

// ReversePartnerEarningTx: batalkan penghasilan mitra atas publikasi yang
// garansinya jatuh (klien dikembalikan dana atau dibuatkan artikel pengganti).
//
//   - pending, atau settled yang belum masuk penarikan  → status 'voided'.
//   - settled yang sudah masuk penarikan                 → baris di
//     partner_service_charges (service 'link_guarantee'), yang sudah dikurangkan
//     dari saldo tersedia mitra (lihat partnerServiceChargesTotal).
//
// Idempoten: panggilan kedua tidak mengubah apa pun.
func ReversePartnerEarningTx(ctx context.Context, tx pgx.Tx, publicationID string) (PartnerEarningReversal, error) {
	out := PartnerEarningReversal{Action: "none"}
	var earningID, status string
	var settlementID *string
	err := tx.QueryRow(ctx, `
		SELECT id, partner_id, amount::float8, status, settlement_id
		FROM partner_earnings
		WHERE publication_id = $1 AND kind = 'publication'
		FOR UPDATE`, publicationID).Scan(&earningID, &out.PartnerID, &out.Amount, &status, &settlementID)
	if err == pgx.ErrNoRows {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if status == "voided" || out.Amount <= 0 {
		return out, nil
	}
	if settlementID == nil || *settlementID == "" {
		if _, err := tx.Exec(ctx, "UPDATE partner_earnings SET status = 'voided' WHERE id = $1", earningID); err != nil {
			return out, err
		}
		out.Action = "voided"
		return out, nil
	}

	var exists int
	if err := tx.QueryRow(ctx, `
		SELECT 1 FROM partner_service_charges
		WHERE service = 'link_guarantee' AND reference_id = $1 LIMIT 1`, publicationID).Scan(&exists); err == nil {
		return out, nil
	} else if err != pgx.ErrNoRows {
		return out, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO partner_service_charges (id, partner_id, service, amount_idr, reference_id, reference_type, notes)
		VALUES ($1, $2, 'link_guarantee', $3, $4, 'publication', $5)`,
		uuid.NewString(), out.PartnerID, out.Amount, publicationID,
		"Garansi tautan jatuh: penghasilan artikel yang sudah ditarik dipotong dari saldo berikutnya"); err != nil {
		return out, err
	}
	out.Action = "charged"
	return out, nil
}
