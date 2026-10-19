package clienttaut

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
)

// Port lib/credits/wallet.ts + lib/jobs/queue.ts + lib/notifications/service.ts
// (bagian yang dipakai alur Taut Pintar klien). Semua fungsi unexported —
// sama seperti paket partner, fungsi-fungsi ini tidak bisa diimpor lintas paket.

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

// invalidateTcConfigCache: dipanggil setelah admin mengubah rate.
func invalidateTcConfigCache() {
	tcConfigMu.Lock()
	tcConfigCache = nil
	tcConfigMu.Unlock()
}

// outstandingForPublication: sisa TC yang masih ter-reserve untuk publikasi.
// Ledger: reserve -tc, release +tc, capture +tc → net 3 tipe = 0 bila selesai.
func (d *Deps) outstandingForPublication(ctx context.Context, brandID, publicationID string) int {
	row, err := d.queryRow(ctx, `
		SELECT COALESCE(SUM(amount_tc), 0)::int AS net
		FROM credit_ledger_entries
		WHERE brand_id = $1 AND ref_id = $2 AND ref_type IN ('reserve','release','capture')`,
		brandID, publicationID)
	if err != nil {
		return 0
	}
	net := row.intOr("net", 0)
	if net >= 0 {
		return 0
	}
	return -net
}

// reserveCredit: available → reserved, atomik + idempoten per publikasi
// (`reserve:{publicationId}`). InsufficientCreditsError bila saldo kurang.
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

// releaseCredit: reserved → available (publikasi batal/ditolak). Idempoten.
func (d *Deps) releaseCredit(ctx context.Context, brandID, publicationID, userID string) int {
	outstanding := d.outstandingForPublication(ctx, brandID, publicationID)
	if outstanding <= 0 {
		return 0
	}
	rate := d.getTcConfig(ctx).RateIDR
	applied := false
	err := db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
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

// chargeCredit: debit langsung dari available (bukan reserved) — idempoten per
// refId (`charge:{refId}`). Dipakai Taut Pintar untuk Profiler/Writer.
func (d *Deps) chargeCredit(ctx context.Context, brandID, refID string, tc int, reason, userID string) error {
	if tc <= 0 {
		return nil
	}
	rate := d.getTcConfig(ctx).RateIDR
	key := "charge:" + refID
	return db.WithPoolTx(ctx, d.Pool, func(tx pgx.Tx) error {
		var exists int
		if err := tx.QueryRow(ctx, "SELECT 1 FROM credit_ledger_entries WHERE idempotency_key = $1 LIMIT 1", key).Scan(&exists); err == nil {
			return nil // sudah diproses (retry)
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
			BrandID: brandID, RefType: "capture", RefID: refID, AmountTC: -tc,
			BalanceAfterTC: available + reserved + captured, RateIDR: rate, AmountIDR: tc * rate,
			Reason: reason, IdempotencyKey: key, CreatedBy: userID,
		})
	})
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

// splitTcAcrossPublications: port lib/credits/split.ts — floor per publikasi,
// sisa pembagian ke publikasi pertama agar Σ = total.
func splitTcAcrossPublications(totalTC, count int) []int {
	if count <= 0 {
		return nil
	}
	safeTotal := totalTC
	if safeTotal < 0 {
		safeTotal = 0
	}
	base := safeTotal / count
	remainder := safeTotal - base*count
	out := make([]int, count)
	for i := range out {
		if i == 0 {
			out[i] = base + remainder
		} else {
			out[i] = base
		}
	}
	return out
}

// ---------- jobs (port enqueueJob lib/jobs/queue.ts) ----------

func (d *Deps) enqueueJob(ctx context.Context, kind string, payload map[string]any, maxAttempts int) (string, error) {
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
	var id string
	err = d.Pool.QueryRow(ctx,
		"INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at) VALUES (gen_random_uuid(), $1, $2::jsonb, 'pending', 0, $3, NOW()) RETURNING id",
		kind, string(raw), maxAttempts).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

// enqueueJobAt: varian dengan run_at (publication_execution terjadwal).
func (d *Deps) enqueueJobAt(ctx context.Context, kind string, payload map[string]any, runAt *time.Time, maxAttempts int) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	if runAt == nil {
		now := time.Now()
		runAt = &now
	}
	var id string
	err = d.Pool.QueryRow(ctx,
		"INSERT INTO jobs (id, kind, payload, status, attempts, max_attempts, run_at) VALUES (gen_random_uuid(), $1, $2::jsonb, 'pending', 0, $3, $4) RETURNING id",
		kind, string(raw), maxAttempts, *runAt).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (d *Deps) enqueueJobBestEffort(ctx context.Context, kind string, payload map[string]any, maxAttempts int) {
	if _, err := d.enqueueJob(ctx, kind, payload, maxAttempts); err != nil {
		slog.Warn("[clienttaut] enqueue job gagal", "kind", kind, "err", err)
	}
}

// ---------- notifications (port lib/notifications/service.ts) ----------

func (d *Deps) createNotification(ctx context.Context, userID, notifType, title, body string, link *string, publicationID *string) {
	if err := d.exec(ctx,
		"INSERT INTO notifications (id, user_id, type, title, body, link, publication_id) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6)",
		userID, notifType, title, body, link, publicationID); err != nil {
		slog.Warn("[clienttaut] notifikasi gagal", "type", notifType, "err", err)
	}
}

func (d *Deps) notifyBrandMembers(ctx context.Context, brandID, notifType, title, body string, link *string, publicationID *string) {
	if err := d.exec(ctx,
		`INSERT INTO notifications (id, user_id, type, title, body, link, publication_id)
		 SELECT gen_random_uuid(), bm.user_id, $1, $2, $3, $4, $5
		 FROM brand_memberships bm JOIN users u ON u.id = bm.user_id
		 WHERE bm.brand_id = $6 AND u.role <> 'admin'`,
		notifType, title, body, link, publicationID, brandID); err != nil {
		slog.Warn("[clienttaut] notifikasi brand gagal", "type", notifType, "err", err)
	}
}

// publicationWaitingPartnerNotification: parity publicationEventNotification
// event "publication.waiting_partner" — kirim ke user mitra media (best-effort).
func (d *Deps) publicationWaitingPartnerNotification(ctx context.Context, publicationID string) {
	row, err := d.queryRow(ctx, `
		SELECT p.id, p.brand_id, m.name AS media_name, pa.user_id AS partner_user_id, pa.public_id AS partner_public_id,
		       b.name AS brand_name, b.client_name, b.public_id AS brand_public_id, d.title AS document_title
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
	brandName := row.strOr("client_name", row.strOr("brand_name", "Klien"))
	partnerPublicID := row.strOr("partner_public_id", "")
	link := "/mitra/" + partnerPublicID + "/publications?pub=" + publicationID
	title := "Publikasi menunggu persetujuan"
	body := "\"" + article + "\" oleh " + brandName + " menunggu persetujuan Anda untuk terbit di " + mediaName + "."
	pubID := publicationID
	d.createNotification(ctx, partnerUserID, "publication.waiting_partner", title, body, &link, &pubID)
}
