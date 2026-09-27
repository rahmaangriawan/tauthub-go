package partner

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/db"
)

// Port lib/credits/wallet.ts (bagian yang dipakai alur mitra): capture saat
// publikasi terbit, release saat ditolak. Keduanya best-effort dari sisi
// pemanggil — kegagalan kredit tidak boleh menggagalkan transisi status.
//
// Idempotensi mengikuti app lama: kunci `capture:{publicationId}` /
// `release:{publicationId}` dijaga unique index credit_ledger_entries.
// idempotency_key.

// reservedForPublication: sisa TC yang masih ter-reserve untuk publikasi ini.
func (d *Deps) reservedForPublication(ctx context.Context, brandID, publicationID string) (int, error) {
	row, err := d.queryRow(ctx, `
		SELECT COALESCE(SUM(amount_tc), 0)::int AS net
		FROM credit_ledger_entries
		WHERE brand_id = $1 AND ref_id = $2 AND ref_type IN ('reserve','release','capture')`,
		brandID, publicationID)
	if err != nil {
		return 0, err
	}
	net := row.intOr("net", 0)
	if net > 0 {
		return 0, nil
	}
	return -net, nil
}

// tcRateIDR: rate kredit platform (tc_config.rate_idr), minimal 1.
func (d *Deps) tcRateIDR(ctx context.Context) int {
	row, _ := d.queryRow(ctx, "SELECT rate_idr FROM tc_config WHERE id = 1 LIMIT 1")
	rate := row.intOr("rate_idr", 1000)
	if rate < 1 {
		rate = 1000
	}
	return rate
}

// captureCredit: reserved → captured (publikasi terbit). Mengembalikan jumlah
// TC yang ditangkap (0 bila tidak ada reservasi / sudah pernah diproses).
func (d *Deps) captureCredit(ctx context.Context, brandID, publicationID, userID string) int {
	outstanding, err := d.reservedForPublication(ctx, brandID, publicationID)
	if err != nil || outstanding <= 0 {
		return 0
	}
	rate := d.tcRateIDR(ctx)
	applied := false
	err = db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		key := "capture:" + publicationID
		var exists int
		if err := tx.QueryRow(ctx, "SELECT 1 FROM credit_ledger_entries WHERE idempotency_key = $1 LIMIT 1", key).Scan(&exists); err == nil {
			return nil // sudah diproses
		}
		if _, err := tx.Exec(ctx, `
			UPDATE credit_wallets SET reserved_tc = GREATEST(reserved_tc - $1, 0), captured_tc = captured_tc + $1, updated_at = NOW()
			WHERE brand_id = $2`, outstanding, brandID); err != nil {
			return err
		}
		var available, reserved, captured int
		if err := tx.QueryRow(ctx,
			"SELECT available_tc, reserved_tc, captured_tc FROM credit_wallets WHERE brand_id = $1", brandID).
			Scan(&available, &reserved, &captured); err != nil {
			return err
		}
		total := available + reserved + captured
		if _, err := tx.Exec(ctx, `
			INSERT INTO credit_ledger_entries (id, brand_id, ref_type, ref_id, amount_tc, balance_after_tc, rate_idr, amount_idr, reason, idempotency_key, created_by)
			VALUES ($1, $2, 'capture', $3, $4, $5, $6, $7, $8, $9, $10)`,
			uuid.NewString(), brandID, publicationID, outstanding, total, rate, outstanding*rate,
			"Kredit terpakai untuk publikasi terbit", key, userID); err != nil {
			return err
		}
		applied = true
		return nil
	})
	if err != nil || !applied {
		return 0
	}
	return outstanding
}

// releaseCredit: reserved → available (publikasi ditolak/dibatalkan).
func (d *Deps) releaseCredit(ctx context.Context, brandID, publicationID, userID string) int {
	outstanding, err := d.reservedForPublication(ctx, brandID, publicationID)
	if err != nil || outstanding <= 0 {
		return 0
	}
	rate := d.tcRateIDR(ctx)
	applied := false
	err = db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		key := "release:" + publicationID
		var exists int
		if err := tx.QueryRow(ctx, "SELECT 1 FROM credit_ledger_entries WHERE idempotency_key = $1 LIMIT 1", key).Scan(&exists); err == nil {
			return nil
		}
		if _, err := tx.Exec(ctx, `
			UPDATE credit_wallets SET reserved_tc = GREATEST(reserved_tc - $1, 0), available_tc = available_tc + $1, updated_at = NOW()
			WHERE brand_id = $2`, outstanding, brandID); err != nil {
			return err
		}
		var available, reserved, captured int
		if err := tx.QueryRow(ctx,
			"SELECT available_tc, reserved_tc, captured_tc FROM credit_wallets WHERE brand_id = $1", brandID).
			Scan(&available, &reserved, &captured); err != nil {
			return err
		}
		total := available + reserved + captured
		if _, err := tx.Exec(ctx, `
			INSERT INTO credit_ledger_entries (id, brand_id, ref_type, ref_id, amount_tc, balance_after_tc, rate_idr, amount_idr, reason, idempotency_key, created_by)
			VALUES ($1, $2, 'release', $3, $4, $5, $6, $7, $8, $9, $10)`,
			uuid.NewString(), brandID, publicationID, outstanding, total, rate, outstanding*rate,
			"Kredit reservasi dikembalikan (publikasi batal/ditolak)", key, userID); err != nil {
			return err
		}
		applied = true
		return nil
	})
	if err != nil || !applied {
		return 0
	}
	return outstanding
}

// capturePublicationCredit: variante best-effort untuk recordPublicationAttempt
// (status published) — parity blok "captureCredit" di publication/service.ts.
func (d *Deps) capturePublicationCredit(ctx context.Context, publicationID string) {
	if strings.TrimSpace(publicationID) == "" {
		return
	}
	row, err := d.queryRow(ctx, "SELECT brand_id FROM publications WHERE id = $1 LIMIT 1", publicationID)
	if err != nil || row == nil {
		return
	}
	brandID := row.strOr("brand_id", "")
	if brandID == "" {
		return
	}
	d.captureCredit(ctx, brandID, publicationID, "system")
}
