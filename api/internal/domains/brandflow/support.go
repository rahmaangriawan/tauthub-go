package brandflow

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/db"
	emailqueue "github.com/rahmaangriawan/tauthub-go/internal/email"
	"github.com/rahmaangriawan/tauthub-go/internal/notify"
)

// Port lib/credits/wallet.ts + lib/publication/credits.ts + lib/jobs/queue.ts
// (bagian yang dipakai alur klien kampanye/publikasi). Semua unexported —
// fungsi tidak bisa diimpor lintas paket, jadi disalin dari clienttaut/support.go
// dengan tipe error milik paket ini.

// ---------- tc_config (port getTcConfig, cache 30s) ----------

type tcConfig struct {
	RateIDR int
}

var (
	tcConfigMu    sync.Mutex
	tcConfigCache *struct {
		value   tcConfig
		expires time.Time
	}
)

// getTcConfig: rate kredit platform (tc_config.rate_idr), minimal 1, default 1000.
func (d *Deps) getTcConfig(ctx context.Context) tcConfig {
	tcConfigMu.Lock()
	if tcConfigCache != nil && tcConfigCache.expires.After(time.Now()) {
		v := tcConfigCache.value
		tcConfigMu.Unlock()
		return v
	}
	tcConfigMu.Unlock()

	rate := 1000
	var raw *int
	if err := d.Pool.QueryRow(ctx, "SELECT rate_idr FROM tc_config WHERE id = 1 LIMIT 1").Scan(&raw); err == nil && raw != nil && *raw >= 1 {
		rate = *raw
	}
	value := tcConfig{RateIDR: rate}
	tcConfigMu.Lock()
	tcConfigCache = &struct {
		value   tcConfig
		expires time.Time
	}{value: value, expires: time.Now().Add(30 * time.Second)}
	tcConfigMu.Unlock()
	return value
}

// ---------- pricing (port lib/pricing/engine.ts) ----------

const defaultMarkupPercent = 15

var (
	markupMu     sync.Mutex
	markupCached *struct {
		value   float64
		expires time.Time
	}
)

func (d *Deps) getMarkupPercent(ctx context.Context) float64 {
	now := time.Now()
	markupMu.Lock()
	if markupCached != nil && markupCached.expires.After(now) {
		v := markupCached.value
		markupMu.Unlock()
		return v
	}
	markupMu.Unlock()

	value := float64(defaultMarkupPercent)
	var raw *float64
	if err := d.Pool.QueryRow(ctx, "SELECT markup_percent FROM pricing_config WHERE id = 1 LIMIT 1").Scan(&raw); err == nil && raw != nil {
		value = *raw
	}
	markupMu.Lock()
	markupCached = &struct {
		value   float64
		expires time.Time
	}{value: value, expires: now.Add(30 * time.Second)}
	markupMu.Unlock()
	return value
}

type pricingBreakdown struct {
	PartnerPrice  float64 `json:"partner_price"`
	MarkupPercent float64 `json:"markup_percent"`
	MarkupAmount  float64 `json:"markup_amount"`
	ClientPrice   float64 `json:"client_price"`
}

func computeClientPrice(partnerPrice, markupPercent float64) float64 {
	base := partnerPrice
	if base < 0 {
		base = 0
	}
	markup := markupPercent
	if markup < 0 {
		markup = 0
	}
	return round2(base * (1 + markup/100))
}

func breakdownFor(partnerPrice, markupPercent float64) pricingBreakdown {
	partner := partnerPrice
	if partner < 0 {
		partner = 0
	}
	markup := markupPercent
	if markup < 0 {
		markup = 0
	}
	client := computeClientPrice(partner, markup)
	return pricingBreakdown{
		PartnerPrice: partner, MarkupPercent: markup,
		MarkupAmount: round2(client - partner), ClientPrice: client,
	}
}

// creditFromClientPrice: parity creditFromClientPrice (min 1, ceil).
func creditFromClientPrice(clientPrice float64, rate int) int {
	if clientPrice <= 0 {
		return 0
	}
	safeRate := rate
	if safeRate < 1 {
		safeRate = 1
	}
	v := int(clientPrice / float64(safeRate))
	if float64(v)*float64(safeRate) < clientPrice {
		v++
	}
	if v < 1 {
		v = 1
	}
	return v
}

// creditCostForMedia: TC dari CLIENT price (pricing engine: partner price × markup).
func (d *Deps) creditCostForMedia(ctx context.Context, mediaID string) int {
	row, err := d.queryRow(ctx, "SELECT price FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil || row == nil {
		return 0
	}
	price := row.numOr("price")
	if price <= 0 {
		return 0
	}
	rate := d.getTcConfig(ctx).RateIDR
	markup := d.getMarkupPercent(ctx)
	return creditFromClientPrice(computeClientPrice(price, markup), rate)
}

// creditCostsForMedia: TC per media sekaligus (1 query price + config cache).
func (d *Deps) creditCostsForMedia(ctx context.Context, mediaIDs []string) map[string]int {
	out := map[string]int{}
	if len(mediaIDs) == 0 {
		return out
	}
	ph := make([]string, len(mediaIDs))
	args := make([]any, len(mediaIDs))
	for i, id := range mediaIDs {
		args[i] = id
		ph[i] = "$" + itoa(i+1)
	}
	rows, err := d.queryRows(ctx, "SELECT id, price FROM media WHERE id IN ("+strings.Join(ph, ", ")+")", args...)
	if err != nil {
		return out
	}
	rate := d.getTcConfig(ctx).RateIDR
	markup := d.getMarkupPercent(ctx)
	for _, row := range rows {
		price := row.numOr("price")
		out[row.strOr("id", "")] = creditFromClientPrice(computeClientPrice(price, markup), rate)
	}
	return out
}

// brandCreditBalance: parity brandCreditBalance (paket legacy).
type brandBalance struct {
	Total        int
	Used         int
	Remaining    int
	PackageCount int
}

func (d *Deps) brandCreditBalance(ctx context.Context, brandID string) brandBalance {
	row, err := d.queryRow(ctx,
		`SELECT COALESCE(SUM(credit_total), 0)::int AS total,
		        COALESCE(SUM(credit_used), 0)::int AS used,
		        COUNT(*)::int AS count
		 FROM packages WHERE brand_id = $1 AND status = 'active'`, brandID)
	if err != nil || row == nil {
		return brandBalance{}
	}
	total := row.intOr("total", 0)
	used := row.intOr("used", 0)
	return brandBalance{Total: total, Used: used, Remaining: total - used, PackageCount: row.intOr("count", 0)}
}

// ---------- wallet TC (port lib/credits/wallet.ts) ----------

type walletRow struct {
	AvailableTC int
	ReservedTC  int
	CapturedTC  int
}

// getWallet: nil bila brand belum punya dompet TC.
func (d *Deps) getWallet(ctx context.Context, brandID string) *walletRow {
	row, err := d.queryRow(ctx,
		"SELECT available_tc, reserved_tc, captured_tc FROM credit_wallets WHERE brand_id = $1 LIMIT 1", brandID)
	if err != nil || row == nil {
		return nil
	}
	return &walletRow{
		AvailableTC: row.intOr("available_tc", 0),
		ReservedTC:  row.intOr("reserved_tc", 0),
		CapturedTC:  row.intOr("captured_tc", 0),
	}
}

// outstandingForPublication: sisa TC yang masih ter-reserve untuk publikasi.
func (d *Deps) outstandingForPublication(ctx context.Context, brandID, publicationID string) int {
	row, err := d.queryRow(ctx, `
		SELECT COALESCE(SUM(amount_tc), 0)::int AS net
		FROM credit_ledger_entries
		WHERE brand_id = $1 AND ref_id = $2 AND ref_type IN ('reserve','release','capture')`,
		brandID, publicationID)
	if err != nil || row == nil {
		return 0
	}
	net := row.intOr("net", 0)
	if net >= 0 {
		return 0
	}
	return -net
}

type ledgerEntry struct {
	BrandID        string
	RefType        string
	RefID          string
	AmountTC       int
	BalanceAfterTC int
	RateIDR        int
	AmountIDR      int
	Reason         string
	IdempotencyKey string
	CreatedBy      string
}

func insertLedger(ctx context.Context, tx pgx.Tx, e ledgerEntry) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO credit_ledger_entries
		  (id, brand_id, ref_type, ref_id, amount_tc, balance_after_tc, rate_idr, amount_idr, reason, idempotency_key, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		uuid.NewString(), e.BrandID, e.RefType, e.RefID, e.AmountTC, e.BalanceAfterTC,
		e.RateIDR, e.AmountIDR, e.Reason, e.IdempotencyKey, e.CreatedBy)
	return err
}

// reserveCredit: available → reserved, atomik + idempoten per publikasi
// (`reserve:{publicationId}`). insufficientCreditsError bila saldo kurang.
func (d *Deps) reserveCredit(ctx context.Context, brandID, publicationID string, tc int, userID string) (int, error) {
	if tc <= 0 {
		return 0, nil
	}
	rate := d.getTcConfig(ctx).RateIDR
	applied := false
	err := db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		key := "reserve:" + publicationID
		var exists int
		if err := tx.QueryRow(ctx, "SELECT 1 FROM credit_ledger_entries WHERE idempotency_key = $1 LIMIT 1", key).Scan(&exists); err == nil {
			return nil
		}
		if _, err := tx.Exec(ctx, "INSERT INTO credit_wallets (brand_id) VALUES ($1) ON CONFLICT (brand_id) DO NOTHING", brandID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE credit_wallets SET available_tc = available_tc - $1, reserved_tc = reserved_tc + $1, updated_at = NOW()
			WHERE brand_id = $2 AND available_tc >= $1`, tc, brandID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var available int
			_ = tx.QueryRow(ctx, "SELECT available_tc FROM credit_wallets WHERE brand_id = $1", brandID).Scan(&available)
			return &insufficientCreditsError{Needed: tc, Available: available}
		}
		var available, reserved, captured int
		if err := tx.QueryRow(ctx,
			"SELECT available_tc, reserved_tc, captured_tc FROM credit_wallets WHERE brand_id = $1", brandID).
			Scan(&available, &reserved, &captured); err != nil {
			return err
		}
		applied = true
		return insertLedger(ctx, tx, ledgerEntry{
			BrandID: brandID, RefType: "reserve", RefID: publicationID, AmountTC: -tc,
			BalanceAfterTC: available + reserved + captured, RateIDR: rate, AmountIDR: tc * rate,
			Reason: "Reservasi kredit untuk publikasi", IdempotencyKey: key, CreatedBy: userID,
		})
	})
	if err != nil {
		return 0, err
	}
	if !applied {
		return 0, nil
	}
	return tc, nil
}

// reserveCycleSuffix: siklus reservasi publikasi. Siklus pertama memakai kunci
// polos (`release:{id}` / `capture:{id}`, kompatibel dengan data lama); siklus
// berikutnya (setelah retry yang menahan kredit ulang) diberi akhiran `:{n}`
// supaya release/capture tidak tertolak oleh kunci siklus sebelumnya.
func reserveCycleSuffix(ctx context.Context, tx pgx.Tx, publicationID string) string {
	var n int
	if err := tx.QueryRow(ctx,
		"SELECT COUNT(*)::int FROM credit_ledger_entries WHERE ref_id = $1 AND ref_type = 'reserve'",
		publicationID).Scan(&n); err != nil || n <= 1 {
		return ""
	}
	return ":" + itoa(n)
}

// reacquireReservation: dipakai retry publikasi yang gagal. Saat gagal permanen
// worker melepas reservasi (release +N); mengantrekan ulang tanpa menahan kredit
// lagi membuat artikel terbit tanpa tagihan. Fungsi ini menahan ulang jumlah
// reservasi terakhir secara atomik (kunci `reserve:{id}:{n}`), dengan baris
// wallet dikunci sehingga dua retry paralel hanya menahan satu kali.
//
// Mengembalikan (true, nil) bila reservasi baru dibuat, (false, nil) bila tidak
// perlu (reservasi masih berjalan, atau publikasi tidak memakai wallet TC), dan
// insufficientCreditsError bila saldo kurang.
func (d *Deps) reacquireReservation(ctx context.Context, brandID, publicationID, userID string) (bool, error) {
	rate := d.getTcConfig(ctx).RateIDR
	applied := false
	err := db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO credit_wallets (brand_id) VALUES ($1) ON CONFLICT (brand_id) DO NOTHING", brandID); err != nil {
			return err
		}
		var lockedAvail int
		if err := tx.QueryRow(ctx, "SELECT available_tc FROM credit_wallets WHERE brand_id = $1 FOR UPDATE", brandID).Scan(&lockedAvail); err != nil {
			return err
		}
		var net, captured int
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount_tc), 0)::int,
			       COALESCE(SUM(CASE WHEN ref_type = 'capture' THEN 1 ELSE 0 END), 0)::int
			FROM credit_ledger_entries
			WHERE brand_id = $1 AND ref_id = $2 AND ref_type IN ('reserve','release','capture')`,
			brandID, publicationID).Scan(&net, &captured); err != nil {
			return err
		}
		if net < 0 || captured > 0 {
			return nil // masih ter-reserve, atau sudah ditagih
		}
		var reserveCount, lastReserve int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)::int,
			       COALESCE((SELECT -amount_tc FROM credit_ledger_entries
			                  WHERE brand_id = $1 AND ref_id = $2 AND ref_type = 'reserve'
			                  ORDER BY created_at DESC LIMIT 1), 0)::int
			FROM credit_ledger_entries
			WHERE brand_id = $1 AND ref_id = $2 AND ref_type = 'reserve'`,
			brandID, publicationID).Scan(&reserveCount, &lastReserve); err != nil {
			return err
		}
		if reserveCount == 0 || lastReserve <= 0 {
			return nil // tidak pernah memakai reservasi wallet
		}
		if lockedAvail < lastReserve {
			return &insufficientCreditsError{Needed: lastReserve, Available: lockedAvail}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE credit_wallets SET available_tc = available_tc - $1, reserved_tc = reserved_tc + $1, updated_at = NOW()
			WHERE brand_id = $2`, lastReserve, brandID); err != nil {
			return err
		}
		var available, reserved, capturedTC int
		if err := tx.QueryRow(ctx,
			"SELECT available_tc, reserved_tc, captured_tc FROM credit_wallets WHERE brand_id = $1", brandID).
			Scan(&available, &reserved, &capturedTC); err != nil {
			return err
		}
		applied = true
		return insertLedger(ctx, tx, ledgerEntry{
			BrandID: brandID, RefType: "reserve", RefID: publicationID, AmountTC: -lastReserve,
			BalanceAfterTC: available + reserved + capturedTC, RateIDR: rate, AmountIDR: lastReserve * rate,
			Reason:         "Reservasi kredit ulang (retry publikasi)",
			IdempotencyKey: "reserve:" + publicationID + ":" + itoa(reserveCount+1), CreatedBy: userID,
		})
	})
	return applied, err
}

// releaseCredit: reserved → available (publikasi batal/ditolak). Idempoten.
func (d *Deps) releaseCredit(ctx context.Context, brandID, publicationID, userID string) int {
	outstanding := d.outstandingForPublication(ctx, brandID, publicationID)
	if outstanding <= 0 {
		return 0
	}
	rate := d.getTcConfig(ctx).RateIDR
	applied := false
	err := db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		key := "release:" + publicationID + reserveCycleSuffix(ctx, tx, publicationID)
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
		applied = true
		return insertLedger(ctx, tx, ledgerEntry{
			BrandID: brandID, RefType: "release", RefID: publicationID, AmountTC: outstanding,
			BalanceAfterTC: available + reserved + captured, RateIDR: rate, AmountIDR: outstanding * rate,
			Reason: "Kredit reservasi dikembalikan (publikasi batal/ditolak)", IdempotencyKey: key, CreatedBy: userID,
		})
	})
	if err != nil || !applied {
		return 0
	}
	return outstanding
}

// captureCredit: reserved → captured saat artikel terbit. Idempoten.
func (d *Deps) captureCredit(ctx context.Context, brandID, publicationID, userID string) int {
	outstanding := d.outstandingForPublication(ctx, brandID, publicationID)
	if outstanding <= 0 {
		return 0
	}
	rate := d.getTcConfig(ctx).RateIDR
	applied := false
	err := db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		key := "capture:" + publicationID + reserveCycleSuffix(ctx, tx, publicationID)
		var exists int
		if err := tx.QueryRow(ctx, "SELECT 1 FROM credit_ledger_entries WHERE idempotency_key = $1 LIMIT 1", key).Scan(&exists); err == nil {
			return nil
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
		applied = true
		return insertLedger(ctx, tx, ledgerEntry{
			BrandID: brandID, RefType: "capture", RefID: publicationID, AmountTC: outstanding,
			BalanceAfterTC: available + reserved + captured, RateIDR: rate, AmountIDR: outstanding * rate,
			Reason: "Kredit publikasi ditagihkan (artikel terbit)", IdempotencyKey: key, CreatedBy: userID,
		})
	})
	if err != nil || !applied {
		return 0
	}
	return outstanding
}

// chargeCredit: debit langsung dari available — idempoten per refID (`charge:{refId}`).
func (d *Deps) chargeCredit(ctx context.Context, brandID, refID string, tc int, reason, userID string) error {
	if tc <= 0 {
		return nil
	}
	rate := d.getTcConfig(ctx).RateIDR
	key := "charge:" + refID
	return db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		var exists int
		if err := tx.QueryRow(ctx, "SELECT 1 FROM credit_ledger_entries WHERE idempotency_key = $1 LIMIT 1", key).Scan(&exists); err == nil {
			return nil
		}
		if _, err := tx.Exec(ctx, "INSERT INTO credit_wallets (brand_id) VALUES ($1) ON CONFLICT (brand_id) DO NOTHING", brandID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE credit_wallets
			SET available_tc = available_tc - $1, captured_tc = captured_tc + $1, updated_at = NOW()
			WHERE brand_id = $2 AND available_tc >= $1`, tc, brandID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var available int
			_ = tx.QueryRow(ctx, "SELECT available_tc FROM credit_wallets WHERE brand_id = $1", brandID).Scan(&available)
			return &insufficientCreditsError{Needed: tc, Available: available}
		}
		var available, reserved, captured int
		if err := tx.QueryRow(ctx,
			"SELECT available_tc, reserved_tc, captured_tc FROM credit_wallets WHERE brand_id = $1", brandID).
			Scan(&available, &reserved, &captured); err != nil {
			return err
		}
		return insertLedger(ctx, tx, ledgerEntry{
			BrandID: brandID, RefType: "adjustment", RefID: refID, AmountTC: -tc,
			BalanceAfterTC: available + reserved + captured, RateIDR: rate, AmountIDR: tc * rate,
			Reason: reason, IdempotencyKey: key, CreatedBy: userID,
		})
	})
}

// adjustCredit: port adjustCredit lib/credits/wallet.ts — penyesuaian integer
// (negatif = potong available). idempotencyKey kosong → kunci acak (sekali pakai).
func (d *Deps) adjustCredit(ctx context.Context, brandID string, tc int, reason, userID, idempotencyKey string) error {
	if tc == 0 {
		return errBadRequest("Adjustment amount must be a non-zero integer.")
	}
	rate := d.getTcConfig(ctx).RateIDR
	key := idempotencyKey
	if key == "" {
		key = "adjustment:" + uuid.NewString()
	}
	return db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		var exists int
		if err := tx.QueryRow(ctx, "SELECT 1 FROM credit_ledger_entries WHERE idempotency_key = $1 LIMIT 1", key).Scan(&exists); err == nil {
			return nil
		}
		if _, err := tx.Exec(ctx, "INSERT INTO credit_wallets (brand_id) VALUES ($1) ON CONFLICT (brand_id) DO NOTHING", brandID); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx,
			`UPDATE credit_wallets SET available_tc = available_tc + $1, updated_at = NOW()
			 WHERE brand_id = $2 AND available_tc + $1 >= 0`, tc, brandID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var available int
			_ = tx.QueryRow(ctx, "SELECT available_tc FROM credit_wallets WHERE brand_id = $1", brandID).Scan(&available)
			needed := tc
			if needed < 0 {
				needed = -needed
			}
			if available < 0 {
				available = 0
			}
			return &insufficientCreditsError{Needed: needed, Available: available}
		}
		var available, reserved, captured int
		if err := tx.QueryRow(ctx,
			"SELECT available_tc, reserved_tc, captured_tc FROM credit_wallets WHERE brand_id = $1", brandID).
			Scan(&available, &reserved, &captured); err != nil {
			return err
		}
		var amountIDR any
		if tc >= 0 {
			amountIDR = tc * rate
		}
		safeReason := reason
		if safeReason == "" {
			safeReason = "Penyesuaian manual"
		}
		return insertLedger(ctx, tx, ledgerEntry{
			BrandID: brandID, RefType: "adjustment", RefID: "", AmountTC: tc,
			BalanceAfterTC: available + reserved + captured, RateIDR: rate, AmountIDR: intOrZero(amountIDR),
			Reason: safeReason, IdempotencyKey: key, CreatedBy: userID,
		})
	})
}

func intOrZero(v any) int {
	if n, ok := v.(int); ok {
		return n
	}
	return 0
}

// ---------- paket legacy (port lib/publication/credits.ts) ----------

// reservePackageCredits: pakai kredit dari paket aktif (FIFO). No-op tanpa paket.
func (d *Deps) reservePackageCredits(ctx context.Context, brandID, publicationID string, amount int, userID string) (int, error) {
	if amount <= 0 {
		return 0, nil
	}
	packages, err := d.queryRows(ctx,
		"SELECT id, credit_total, credit_used FROM packages WHERE brand_id = $1 AND status = 'active' ORDER BY created_at ASC", brandID)
	if err != nil {
		return 0, err
	}
	if len(packages) == 0 {
		return 0, nil
	}
	available := 0
	for _, pkg := range packages {
		available += pkg.intOr("credit_total", 0) - pkg.intOr("credit_used", 0)
	}
	if available < amount {
		return 0, &insufficientCreditsError{Needed: amount, Available: available}
	}

	needed := amount
	remainingAfter := available
	err = d.withTx(ctx, func(tx pgx.Tx) error {
		for _, pkg := range packages {
			if needed <= 0 {
				break
			}
			pkgRemaining := pkg.intOr("credit_total", 0) - pkg.intOr("credit_used", 0)
			take := needed
			if pkgRemaining < take {
				take = pkgRemaining
			}
			if take <= 0 {
				continue
			}
			if _, err := tx.Exec(ctx, "UPDATE packages SET credit_used = credit_used + $1, updated_at = NOW() WHERE id = $2", take, pkg.strOr("id", "")); err != nil {
				return err
			}
			remainingAfter -= take
			if _, err := tx.Exec(ctx,
				"INSERT INTO credit_transactions (id, brand_id, package_id, publication_id, amount, reason, balance_after, created_by) VALUES ($1, $2, $3, $4, $5, 'publication.usage', $6, $7)",
				uuid.NewString(), brandID, pkg.strOr("id", ""), publicationID, -take, remainingAfter, userID); err != nil {
				return err
			}
			needed -= take
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return amount, nil
}

// refundPackageCredits: kembalikan kredit paket untuk publikasi batal.
func (d *Deps) refundPackageCredits(ctx context.Context, brandID, publicationID, userID string) int {
	usages, err := d.queryRows(ctx,
		"SELECT package_id, (-amount)::int AS usage_amount FROM credit_transactions WHERE publication_id = $1 AND reason = 'publication.usage'", publicationID)
	if err != nil || len(usages) == 0 {
		return 0
	}
	usageTotal := 0
	for _, u := range usages {
		usageTotal += u.intOr("usage_amount", 0)
	}
	refundRow, _ := d.queryRow(ctx,
		"SELECT COALESCE(SUM(amount), 0)::int AS refunded FROM credit_transactions WHERE publication_id = $1 AND reason = 'publication.refund'", publicationID)
	refunded := 0
	if refundRow != nil {
		refunded = refundRow.intOr("refunded", 0)
	}
	outstanding := usageTotal - refunded
	if outstanding <= 0 {
		return 0
	}
	balance := d.brandCreditBalance(ctx, brandID)
	remainingAfter := balance.Remaining
	_ = d.withTx(ctx, func(tx pgx.Tx) error {
		for _, u := range usages {
			if outstanding <= 0 {
				break
			}
			usage := u.intOr("usage_amount", 0)
			refund := outstanding
			if usage < refund {
				refund = usage
			}
			if refund <= 0 {
				continue
			}
			if _, err := tx.Exec(ctx, "UPDATE packages SET credit_used = GREATEST(credit_used - $1, 0), updated_at = NOW() WHERE id = $2", refund, u.strOr("package_id", "")); err != nil {
				return err
			}
			remainingAfter += refund
			if _, err := tx.Exec(ctx,
				"INSERT INTO credit_transactions (id, brand_id, package_id, publication_id, amount, reason, balance_after, created_by) VALUES ($1, $2, $3, $4, $5, 'publication.refund', $6, $7)",
				uuid.NewString(), brandID, u.strOr("package_id", ""), publicationID, refund, remainingAfter, userID); err != nil {
				return err
			}
			outstanding -= refund
		}
		return nil
	})
	return usageTotal - outstanding
}

// ---------- jobs (port enqueueJob lib/jobs/queue.ts) ----------

// enqueueJobAt: attempts default 3 untuk kind "*generation*", else 2.
func (d *Deps) enqueueJobAt(ctx context.Context, kind string, payload map[string]any, runAt *time.Time, maxAttempts int) (string, error) {
	if maxAttempts <= 0 {
		if strings.Contains(kind, "generation") {
			maxAttempts = 3
		} else {
			maxAttempts = 2
		}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	when := time.Now()
	if runAt != nil && !runAt.IsZero() {
		when = *runAt
	}
	var id string
	err = d.Pool.QueryRow(ctx,
		"INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at) VALUES (gen_random_uuid(), $1, $2::jsonb, 'pending', 0, $3, $4) RETURNING id",
		kind, string(raw), maxAttempts, when).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *Deps) enqueueJobBestEffort(ctx context.Context, kind string, payload map[string]any, maxAttempts int) {
	if _, err := d.enqueueJobAt(ctx, kind, payload, nil, maxAttempts); err != nil {
		slog.Warn("[brandflow] enqueue job gagal", "kind", kind, "err", err)
	}
}

// ---------- notifications (port lib/notifications/service.ts) ----------

func (d *Deps) createNotification(ctx context.Context, userID, notifType, title, body string, link *string, publicationID *string) {
	if err := d.exec(ctx,
		"INSERT INTO notifications (id, user_id, type, title, body, link, publication_id) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)",
		userID, notifType, title, body, link, publicationID); err != nil {
		slog.Warn("[brandflow] notifikasi gagal", "type", notifType, "err", err)
	}
}

// notifyBrandMembersOnPub: fan-out notifikasi + publication_id.
func (d *Deps) notifyBrandMembersOnPub(ctx context.Context, brandID, notifType, title, body string, link *string, publicationID string) {
	pubID := publicationID
	if err := d.exec(ctx,
		`INSERT INTO notifications (id, user_id, type, title, body, link, publication_id)
		 SELECT gen_random_uuid(), bm.user_id, $1, $2, $3, $4, $5
		 FROM brand_memberships bm JOIN users u ON u.id = bm.user_id
		 WHERE bm.brand_id = $6 AND u.role <> 'admin'`,
		notifType, title, body, link, &pubID, brandID); err != nil {
		slog.Warn("[brandflow] notifikasi brand gagal", "type", notifType, "err", err)
	}
}

// publicationWaitingPartnerNotification: parity event "publication.waiting_partner".
func (d *Deps) publicationWaitingPartnerNotification(ctx context.Context, publicationID string) {
	row, err := d.queryRow(ctx, `
		SELECT p.id, p.brand_id, m.name AS media_name, pa.user_id AS partner_user_id, pa.public_id AS partner_public_id,
		       b.name AS brand_name, b.client_name, d.title AS document_title
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN brands b ON b.id = p.brand_id
		JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN partners pa ON pa.id = m.partner_id
		WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil || row == nil {
		return
	}
	partnerUserID := row.strOr("partner_user_id", "")
	if partnerUserID == "" {
		return
	}
	article := row.strOr("document_title", "Artikel")
	mediaName := row.strOr("media_name", "Media")
	partnerPublicID := row.strOr("partner_public_id", "")
	link := "/mitra/" + partnerPublicID + "/publications?pub=" + publicationID
	if partnerPublicID == "" {
		link = ""
	}
	var linkArg *string
	if link != "" {
		linkArg = &link
	}
	title := "Publikasi menunggu persetujuan"
	body := "\"" + article + "\" menunggu persetujuan Anda untuk terbit di " + mediaName + "."
	d.createNotification(ctx, partnerUserID, "publication.waiting_partner", title, body, linkArg, &publicationID)
}

// publicationPublishedNotification: parity event "publication.published"
// (draft/publish + notifikasi brand + mitra).
func (d *Deps) publicationPublishedNotification(ctx context.Context, publicationID string) {
	row, err := d.queryRow(ctx, `
		SELECT p.id, p.brand_id, p.publish_status, p.published_url,
		       m.name AS media_name, d.title AS document_title,
		       b.name AS brand_name, b.client_name, b.public_id AS brand_public_id,
		       pa.user_id AS partner_user_id, pa.public_id AS partner_public_id
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN brands b ON b.id = p.brand_id
		JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN partners pa ON pa.id = m.partner_id
		WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil || row == nil {
		return
	}
	article := row.strOr("document_title", "Artikel")
	mediaName := row.strOr("media_name", "Media")
	brandName := row.strOr("client_name", row.strOr("brand_name", "Klien"))
	brandPublicID := row.strOr("brand_public_id", row.strOr("brand_id", ""))
	brandLink := "/klien/" + brandPublicID + "/publications?pub=" + publicationID
	draftDelivery := row.strOr("publish_status", "draft") == "draft"
	urlSuffix := ""
	if u := row.strOr("published_url", ""); u != "" {
		if draftDelivery {
			urlSuffix = " Draf di: " + u
		} else {
			urlSuffix = " Terbit di: " + u
		}
	}
	title := "Artikel terbit"
	body := "\"" + article + "\" berhasil terbit di " + mediaName + "." + urlSuffix
	if draftDelivery {
		title = "Artikel terkirim sebagai draf"
		body = "\"" + article + "\" berhasil dikirim ke " + mediaName + " sebagai DRAFT dan menunggu tindakan Anda untuk diterbitkan." + urlSuffix
	}
	// Draf punya tipe sendiri supaya email/WA klien tidak menyebutnya "terbit".
	clientEvent := "publication.published"
	if draftDelivery {
		clientEvent = "publication.draft_sent"
	}
	d.notifyBrandMembersOnPub(ctx, row.strOr("brand_id", ""), clientEvent, title, body, &brandLink, publicationID)

	if partnerUserID := row.strOr("partner_user_id", ""); partnerUserID != "" {
		partnerLink := ""
		if pp := row.strOr("partner_public_id", ""); pp != "" {
			partnerLink = "/mitra/" + pp + "/publications?pub=" + publicationID
		}
		partnerTitle := "Artikel berhasil diterbitkan"
		partnerBody := "\"" + article + "\" dari " + brandName + " berhasil diterbitkan di " + mediaName + urlSuffix
		if draftDelivery {
			partnerTitle = "Artikel baru menunggu review"
			partnerBody = "\"" + article + "\" dari " + brandName + " dikirim ke " + mediaName + " sebagai draf dan menunggu tindakan Anda." + urlSuffix
		}
		var partnerLinkArg *string
		if partnerLink != "" {
			partnerLinkArg = &partnerLink
		}
		partnerEvent := "publication.published"
		if draftDelivery {
			partnerEvent = "publication.draft_sent"
		}
		d.createNotification(ctx, partnerUserID, partnerEvent, partnerTitle, partnerBody, partnerLinkArg, &publicationID)
		// WA ke mitra: terbit atau draf yang menunggu tindakan mitra.
		notify.EnqueueUserWA(ctx, d.Pool, notify.UserWAOptions{
			Audience:    notify.AudiencePartner,
			Event:       partnerEvent,
			UserID:      partnerUserID,
			ReferenceID: publicationID,
			Data:        map[string]string{"nama": "", "judul": article, "status": "", "media": mediaName},
			Link:        partnerLink,
			BaseURL:     notify.BaseURL(),
		})
	}

	// WA ke klien (semua anggota brand) dengan template audiens klien.
	d.publicationWAtoBrandMembers(ctx, row.strOr("brand_id", ""), clientEvent, publicationID, article, mediaName, brandLink, "")
	// Laporan ke tim admin (grup WA / admin ber-nomor) dengan tautan halaman admin:
	// artikel terbit, atau terkirim sebagai draf dan menunggu mitra.
	adminEvent := notify.AdminEventPublicationPublished
	if draftDelivery {
		adminEvent = notify.AdminEventPublicationDraft
	}
	adminLink := "/admin/publications/" + publicationID
	notify.EnqueueAdminsWA(ctx, d.Pool, notify.AdminWAOptions{
		Event:       adminEvent,
		ReferenceID: publicationID,
		Data:        map[string]string{"nama": brandName, "judul": article, "media": mediaName},
		Link:        adminLink,
		BaseURL:     notify.BaseURL(),
	})
	// Email ke admin hanya untuk draf (artikel terbit cukup lewat WA grup).
	if draftDelivery {
		emailqueue.EnqueueAdminNotice(ctx, d.Pool, emailqueue.AdminNoticeOptions{
			Template:    "admin-publication-draft",
			Event:       "admin.publication_draft",
			ReferenceID: publicationID,
			Subject:     "Artikel terkirim sebagai draf",
			Paragraphs: []string{
				"\"" + article + "\" dari " + brandName + " sudah dikirim ke " + mediaName + " sebagai draf dan menunggu mitra menerbitkannya.",
			},
			Link:     adminLink,
			CTALabel: "Cek di admin",
		})
	}
}

// publicationWAtoBrandMembers: antrekan WA event publikasi ke setiap anggota
// brand (klien). Best-effort; kegagalan tidak menggagalkan notifikasi in-app.
func (d *Deps) publicationWAtoBrandMembers(ctx context.Context, brandID, event, referenceID, article, mediaName, link, status string) {
	if brandID == "" {
		return
	}
	rows, err := d.queryRows(ctx,
		`SELECT bm.user_id FROM brand_memberships bm JOIN users u ON u.id = bm.user_id
		 WHERE bm.brand_id = $1 AND u.role <> 'admin'`, brandID)
	if err != nil {
		return
	}
	for _, row := range rows {
		userID := row.strOr("user_id", "")
		if userID == "" {
			continue
		}
		notify.EnqueueUserWA(ctx, d.Pool, notify.UserWAOptions{
			Audience:    notify.AudienceClient,
			Event:       event,
			UserID:      userID,
			ReferenceID: referenceID,
			Data:        map[string]string{"nama": "", "judul": article, "status": status, "media": mediaName},
			Link:        link,
			BaseURL:     notify.BaseURL(),
		})
	}
}

// publicationFailedNotification: parity event "publication.failed" (brand + mitra).
func (d *Deps) publicationFailedNotification(ctx context.Context, publicationID string) {
	row, err := d.queryRow(ctx, `
		SELECT p.id, p.brand_id, m.name AS media_name, d.title AS document_title,
		       b.public_id AS brand_public_id,
		       pa.user_id AS partner_user_id, pa.public_id AS partner_public_id,
		       (SELECT error_message FROM publication_attempts WHERE publication_id = p.id ORDER BY attempt_number DESC LIMIT 1) AS error_message
		FROM publications p
		JOIN media m ON m.id = p.media_id
		JOIN brands b ON b.id = p.brand_id
		JOIN content_documents d ON d.id = p.document_id
		LEFT JOIN partners pa ON pa.id = m.partner_id
		WHERE p.id = $1 LIMIT 1`, publicationID)
	if err != nil || row == nil {
		return
	}
	article := row.strOr("document_title", "Artikel")
	mediaName := row.strOr("media_name", "Media")
	safe := truncateString(row.strOr("error_message", ""), 200)
	safeSuffix := ""
	if safe != "" {
		safeSuffix = " " + safe
	}
	brandPublicID := row.strOr("brand_public_id", row.strOr("brand_id", ""))
	brandLink := "/klien/" + brandPublicID + "/publications?pub=" + publicationID
	body := "\"" + article + "\" gagal dikirim ke " + mediaName + "."
	if safe != "" {
		body += " " + safe
	} else {
		body += " Silakan coba lagi atau hubungi dukungan."
	}
	d.notifyBrandMembersOnPub(ctx, row.strOr("brand_id", ""), "publication.failed", "Publikasi gagal", body, &brandLink, publicationID)

	if partnerUserID := row.strOr("partner_user_id", ""); partnerUserID != "" {
		partnerLink := ""
		if pp := row.strOr("partner_public_id", ""); pp != "" {
			partnerLink = "/mitra/" + pp + "/publications?pub=" + publicationID
		}
		var partnerLinkArg *string
		if partnerLink != "" {
			partnerLinkArg = &partnerLink
		}
		d.createNotification(ctx, partnerUserID, "publication.failed", "Publikasi gagal",
			"Artikel \""+article+"\" gagal dikirim ke media Anda ("+mediaName+")."+safeSuffix, partnerLinkArg, &publicationID)
		notify.EnqueueUserWA(ctx, d.Pool, notify.UserWAOptions{
			Audience:    notify.AudiencePartner,
			Event:       "publication.failed",
			UserID:      partnerUserID,
			ReferenceID: publicationID,
			Data:        map[string]string{"nama": "", "judul": article, "status": safe, "media": mediaName},
			Link:        partnerLink,
			BaseURL:     notify.BaseURL(),
		})
	}

	d.publicationWAtoBrandMembers(ctx, row.strOr("brand_id", ""), "publication.failed", publicationID, article, mediaName, brandLink, safe)
}

// ---------- standard check (port lib/standard/service.ts) ----------

type mediaStandard struct {
	MediaID           string
	MinWords          int
	MaxWords          int
	JournalisticStyle string
	HardSellingTol    string
	QuoteRequired     bool
	MinQuotes         int
	ImageRequired     bool
	CategoryRequired  bool
	RequiredInfo      []string
	ForbiddenContent  []string
}

type standardCheck struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type standardCheckResult struct {
	MediaID   string          `json:"media_id"`
	MediaName any             `json:"media_name"`
	Status    string          `json:"status"`
	Score     int             `json:"score"`
	Checks    []standardCheck `json:"checks"`
	Failed    []string        `json:"failed"`
	Warnings  []string        `json:"warnings"`
}

// mediaStandardRow: standard aktif satu media (nil bila tidak ada).
func (d *Deps) mediaStandardRow(ctx context.Context, mediaID string) *mediaStandard {
	row, err := d.queryRow(ctx,
		`SELECT media_id, min_words, max_words, journalistic_style, hard_selling_tolerance, quote_required,
		        min_quotes, image_required, category_required, required_info, forbidden_content
		 FROM media_content_standards WHERE media_id = $1 AND status = 'active' LIMIT 1`, mediaID)
	if err != nil || row == nil {
		return nil
	}
	return &mediaStandard{
		MediaID:           row.strOr("media_id", mediaID),
		MinWords:          row.intOr("min_words", 0),
		MaxWords:          row.intOr("max_words", 0),
		JournalisticStyle: row.strOr("journalistic_style", "news"),
		HardSellingTol:    row.strOr("hard_selling_tolerance", "low"),
		QuoteRequired:     row.boolOr("quote_required", false),
		MinQuotes:         row.intOr("min_quotes", 0),
		ImageRequired:     row.boolOr("image_required", false),
		CategoryRequired:  row.boolOr("category_required", false),
		RequiredInfo:      jsonStringList(row.jsonField("required_info")),
		ForbiddenContent:  jsonStringList(row.jsonField("forbidden_content")),
	}
}

// articleContentFor: versi konten terbaru dokumen (scope brand).
func (d *Deps) articleContentFor(ctx context.Context, brandID, documentID string) (string, error) {
	doc, err := d.queryRow(ctx,
		"SELECT id, current_version FROM content_documents WHERE id = $1 AND brand_id = $2 LIMIT 1", documentID, brandID)
	if err != nil || doc == nil {
		return "", nil
	}
	ver, err := d.queryRow(ctx,
		"SELECT content FROM content_versions WHERE document_id = $1 AND version = $2 LIMIT 1",
		documentID, doc.intOr("current_version", 1))
	if err != nil || ver == nil {
		return "", err
	}
	return ver.strOr("content", ""), nil
}

// finalStandardCheck: Conflict bila standard aktif media gagal (parity finalStandardCheck).
func (d *Deps) finalStandardCheck(ctx context.Context, brandID, documentID, mediaID string) error {
	standard := d.mediaStandardRow(ctx, mediaID)
	if standard == nil {
		return nil
	}
	article, err := d.articleContentFor(ctx, brandID, documentID)
	if err != nil {
		return err
	}
	media, err := d.queryRow(ctx, "SELECT name FROM media WHERE id = $1 LIMIT 1", mediaID)
	if err != nil {
		return err
	}
	var mediaName any
	if media != nil && media.has("name") {
		mediaName = media.strOr("name", "")
	}
	result := runStandardCheck(*standard, article, mediaName)
	if result.Status == "fail" {
		name := "media"
		if s, ok := mediaName.(string); ok && s != "" {
			name = s
		}
		return errConflict("Artikel belum memenuhi standar editorial " + name + ": " +
			strings.Join(capStrings(result.Failed, 3), "; ") + ". Perbaiki artikel atau jalankan standard check untuk detail.")
	}
	return nil
}

// finalStandardCheckMany: parity finalStandardCheckMany — semua media dievaluasi.
func (d *Deps) finalStandardCheckMany(ctx context.Context, brandID, documentID string, mediaIDs []string) ([]standardCheckResult, error) {
	article, err := d.articleContentFor(ctx, brandID, documentID)
	if err != nil {
		return nil, err
	}
	out := make([]standardCheckResult, 0, len(mediaIDs))
	for _, mediaID := range mediaIDs {
		standard := d.mediaStandardRow(ctx, mediaID)
		if standard == nil {
			continue
		}
		media, _ := d.queryRow(ctx, "SELECT name FROM media WHERE id = $1 LIMIT 1", mediaID)
		var mediaName any
		if media != nil && media.has("name") {
			mediaName = media.strOr("name", "")
		}
		out = append(out, runStandardCheck(*standard, article, mediaName))
	}
	return out, nil
}

// ---------- misc ----------

func newUUID() string { return uuid.NewString() }
