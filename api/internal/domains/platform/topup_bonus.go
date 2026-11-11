package platform

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Bonus top-up. Dicatat sebagai baris ledger terpisah (ref_type 'bonus', amount_idr 0)
// supaya nominal pembayaran, faktur, dan angka pendapatan top-up tidak berubah.
// Tiap bonus berjalan di savepoint sendiri: kegagalan bonus tidak boleh membatalkan
// kredit saldo yang sudah dibayar. Idempoten lewat idempotency_key.

// payGrantBonusesOnTx memberi bonus paket lalu bonus kode promo; mengembalikan total TC bonus.
func (d *Deps) payGrantBonusesOnTx(ctx context.Context, tx pgx.Tx, brandID, userID, paymentTxID string, purchasedTC, rate int) int {
	total := 0
	if pkg := d.payPackageBonus(ctx, tx, purchasedTC); pkg > 0 {
		if d.payCreditBonus(ctx, tx, brandID, userID, paymentTxID, pkg, rate, "pkg", "Bonus paket top up") {
			total += pkg
		}
	}
	var code *string
	if err := tx.QueryRow(ctx, "SELECT promo_code FROM payment_transactions WHERE id = $1", paymentTxID).Scan(&code); err == nil && code != nil && *code != "" {
		if bonus := d.payRedeemPromo(ctx, tx, brandID, paymentTxID, *code, purchasedTC); bonus > 0 {
			if d.payCreditBonus(ctx, tx, brandID, userID, paymentTxID, bonus, rate, "promo", "Bonus kode promo "+*code) {
				total += bonus
			}
		}
	}
	return total
}

func (d *Deps) payPackageBonus(ctx context.Context, tx pgx.Tx, purchasedTC int) int {
	var raw []byte
	if err := tx.QueryRow(ctx, "SELECT topup_packages FROM tc_config WHERE id = 1").Scan(&raw); err != nil {
		return 0
	}
	var pkgs []map[string]any
	if json.Unmarshal(raw, &pkgs) != nil {
		return 0
	}
	for _, p := range pkgs {
		tc, ok1 := finNumInt(p["tc"])
		bonus, ok2 := finNumInt(p["bonus_tc"])
		if ok1 && ok2 && tc == purchasedTC && bonus > 0 {
			return bonus
		}
	}
	return 0
}

// payRedeemPromo: klaim satu pemakaian kode di savepoint. Gagal (kedaluwarsa, habis,
// di bawah minimal, atau brand sudah pernah memakai) = tanpa bonus.
func (d *Deps) payRedeemPromo(ctx context.Context, tx pgx.Tx, brandID, paymentTxID, code string, purchasedTC int) int {
	sp, err := tx.Begin(ctx)
	if err != nil {
		return 0
	}
	var bonus int
	err = sp.QueryRow(ctx, `
		UPDATE promo_codes SET used_count = used_count + 1
		WHERE code = $1 AND active AND min_topup_tc <= $2
		  AND (valid_until IS NULL OR valid_until > NOW())
		  AND (max_uses IS NULL OR used_count < max_uses)
		RETURNING bonus_tc`, code, purchasedTC).Scan(&bonus)
	if err == nil {
		_, err = sp.Exec(ctx,
			"INSERT INTO promo_redemptions (code, brand_id, payment_transaction_id, bonus_tc) VALUES ($1, $2, $3, $4)",
			code, brandID, paymentTxID, bonus)
	}
	if err != nil {
		_ = sp.Rollback(ctx)
		return 0
	}
	if err := sp.Commit(ctx); err != nil {
		return 0
	}
	return bonus
}

func (d *Deps) payCreditBonus(ctx context.Context, tx pgx.Tx, brandID, userID, paymentTxID string, bonus, rate int, kind, reason string) bool {
	sp, err := tx.Begin(ctx)
	if err != nil {
		return false
	}
	key := "bonus:" + kind + ":" + paymentTxID
	var exists int
	if sp.QueryRow(ctx, "SELECT 1 FROM credit_ledger_entries WHERE idempotency_key = $1", key).Scan(&exists) == nil {
		_ = sp.Rollback(ctx)
		return false
	}
	var available, reserved, captured int
	if err := sp.QueryRow(ctx,
		"UPDATE credit_wallets SET available_tc = available_tc + $1, updated_at = NOW() WHERE brand_id = $2 RETURNING available_tc, reserved_tc, captured_tc",
		bonus, brandID).Scan(&available, &reserved, &captured); err != nil {
		_ = sp.Rollback(ctx)
		slog.Warn("[bonus] gagal menambah saldo", "payment", paymentTxID, "err", err)
		return false
	}
	if _, err := sp.Exec(ctx,
		`INSERT INTO credit_ledger_entries (id, brand_id, ref_type, ref_id, amount_tc, balance_after_tc, rate_idr, amount_idr, reason, idempotency_key, created_by)
		 VALUES ($1, $2, 'bonus', $3, $4, $5, $6, 0, $7, $8, $9)`,
		uuid.NewString(), brandID, paymentTxID, bonus, available+reserved+captured, rate, reason, key, userID); err != nil {
		_ = sp.Rollback(ctx)
		slog.Warn("[bonus] gagal mencatat ledger", "payment", paymentTxID, "err", err)
		return false
	}
	return sp.Commit(ctx) == nil
}
