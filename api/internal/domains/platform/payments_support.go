package platform

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/rahmaangriawan/tauthub-go/internal/ai"
	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/security"
)

// Port lib/payments/* (types.ts, credentials.ts, service.ts, adapters.ts).
// Awalan `pay` supaya tidak bentrok dengan helper modul platform lain.

// ---------- konstanta provider ----------

// payProviders: parity PAYMENT_PROVIDERS (urutan tampilan admin).
var payProviders = []string{"xendit", "duitku", "midtrans", "tripay", "sandbox", "sumopod"}

var payEnvironments = []string{"sandbox", "production"}

var payProviderLabels = map[string]string{
	"xendit":   "Xendit",
	"duitku":   "Duitku",
	"midtrans": "Midtrans",
	"tripay":   "Tripay",
	"sumopod":  "SumoPod",
	"sandbox":  "Sandbox (testing lokal)",
}

// payCredentialField: parity PROVIDER_CREDENTIAL_FIELDS.
type payCredentialField struct {
	Key         string
	Label       string
	Secret      bool
	Required    bool
	Placeholder string
}

var payCredentialFields = map[string][]payCredentialField{
	"xendit": {
		{"api_key", "API Key (x-api-key)", true, true, "xnd_development_…"},
		{"callback_token", "Webhook Callback Token", true, false, "Untuk verifikasi x-callback-token"},
	},
	"duitku": {
		{"merchant_code", "Merchant Code", false, true, "DXXXX"},
		{"api_key", "API Key", true, true, "b7f0…"},
	},
	"midtrans": {
		{"server_key", "Server Key", true, true, "SB-Mid-server-…"},
		{"client_key", "Client Key", false, false, "SB-Mid-client-…"},
	},
	"tripay": {
		{"merchant_code", "Merchant Code", false, true, "TXXXX"},
		{"api_key", "API Key", true, true, "…"},
		{"private_key", "Private Key", true, true, "Untuk verifikasi callback HMAC"},
	},
	"sumopod": {
		{"api_key", "API Key (X-Api-Key)", true, true, "e36a5f3d…"},
		{"webhook_token", "Webhook Token (X-Webhook-Token)", true, false, "whtok_…"},
		{"webhook_secret", "Webhook Signing Secret (Svix, whsec_)", true, false, "whsec_… (opsional, untuk verifikasi HMAC)"},
		{"payment_method_code", "Metode Pembayaran", false, false, "QRIS / QRIS_INSTANT (default QRIS)"},
	},
	"sandbox": {},
}

// payFinalStatuses: parity FINAL_PAYMENT_STATUSES.
var payFinalStatuses = map[string]bool{
	"paid": true, "failed": true, "expired": true, "cancelled": true, "refunded": true,
}

// payStatusMap: parity PROVIDER_STATUS_MAP (lookup case-insensitive).
var payStatusMap = map[string]map[string]string{
	"xendit": {
		"PENDING": "pending", "PROCESSING": "processing", "REVIEW": "processing",
		"PAID": "paid", "COMPLETED": "paid", "SETTLED": "paid",
		"FAILED": "failed", "EXPIRED": "expired", "VOIDED": "cancelled", "REFUNDED": "refunded",
	},
	"duitku": {
		"WAITING": "pending", "PROCESSING": "processing", "SUCCESS": "paid", "PAID": "paid",
		"FAILED": "failed", "EXPIRED": "expired", "CANCELLED": "cancelled",
	},
	"midtrans": {
		"PENDING": "pending", "CAPTURE": "processing", "AUTHORIZE": "processing", "CHALLENGE": "processing",
		"SETTLEMENT": "paid", "SUCCESS": "paid", "CANCEL": "cancelled", "EXPIRE": "expired",
		"DENY": "failed", "FAILURE": "failed", "REFUND": "refunded", "PARTIAL_REFUND": "refunded",
	},
	"tripay": {
		"UNPAID": "pending", "PAID": "paid", "EXPIRED": "expired", "FAILED": "failed",
		"CANCEL": "cancelled", "CANCELLED": "cancelled", "REFUND": "refunded",
	},
	"sumopod": {
		"PENDING": "pending", "PAID": "paid", "COMPLETED": "paid", "FAILED": "failed",
		"EXPIRED": "expired", "CANCELLED": "cancelled",
	},
	"sandbox": {
		"UNPAID": "pending", "PAID": "paid", "SETTLED": "paid", "COMPLETED": "paid",
		"FAILED": "failed", "EXPIRED": "expired", "CANCELLED": "cancelled",
	},
}

// payNormalizeStatus: parity normalizeProviderStatus.
func payNormalizeStatus(provider, raw string) string {
	if raw == "" {
		return "pending"
	}
	m := payStatusMap[provider]
	if status, ok := m[strings.ToUpper(raw)]; ok {
		return status
	}
	if status, ok := m[raw]; ok {
		return status
	}
	return "pending"
}

// ---------- kredensial ----------

// paySanitizeCredentials: parity sanitizeCredentials — hanya field yang dikenal.
func paySanitizeCredentials(provider string, input map[string]string) map[string]string {
	allowed := map[string]bool{}
	for _, f := range payCredentialFields[provider] {
		allowed[f.Key] = true
	}
	out := map[string]string{}
	for key, value := range input {
		if allowed[key] && strings.TrimSpace(value) != "" {
			out[key] = strings.TrimSpace(value)
		}
	}
	return out
}

// payValidateCredentials: parity validateCredentials → pesan error atau "".
func payValidateCredentials(provider string, credentials map[string]string) string {
	for _, f := range payCredentialFields[provider] {
		if f.Required && credentials[f.Key] == "" {
			return fmt.Sprintf("Field %q wajib diisi untuk %s.", f.Label, provider)
		}
	}
	return ""
}

// paySealCredentials: parity sealCredentials (JSON sanitized → t2:).
func (d *Deps) paySealCredentials(provider string, credentials map[string]string) (string, error) {
	jsonBody, err := json.Marshal(paySanitizeCredentials(provider, credentials))
	if err != nil {
		return "", err
	}
	return d.sealT2(string(jsonBody))
}

// payUnsealCredentials: parity unsealCredentials — coba semua kandidat secret
// (ai.SecretCandidates, seperti loadAiRuntime) supaya config yang di-seal
// dengan secret lama tetap terbaca. Gagal semua → map kosong.
func (d *Deps) payUnsealCredentials(encrypted string) map[string]string {
	for _, secret := range ai.SecretCandidates() {
		plain, err := crypto.Unseal(encrypted, secret)
		if err != nil {
			continue
		}
		var parsed map[string]string
		if err := json.Unmarshal([]byte(plain), &parsed); err != nil {
			continue
		}
		if parsed == nil {
			return map[string]string{}
		}
		return parsed
	}
	return map[string]string{}
}

// payMaskedCredentials: parity maskedCredentials — presence + hint saja.
func payMaskedCredentials(credentials map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range credentials {
		if value == "" {
			out[key] = ""
			continue
		}
		if len(value) <= 4 {
			out[key] = "••••"
		} else {
			out[key] = "••••" + value[len(value)-4:]
		}
	}
	return out
}

// ---------- config gateway ----------

const payConfigSelect = `SELECT id, provider, environment, encrypted_credentials, is_configured, is_active,
	last_tested_at, last_test_ok, created_by, created_at, updated_at FROM payment_gateway_configs`

// payListConfigs: parity listGatewayConfigs.
func (d *Deps) payListConfigs(ctx context.Context) ([]rowMap, error) {
	rows, err := d.queryRows(ctx, payConfigSelect+" ORDER BY provider")
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// payGetConfig: parity getGatewayConfig.
func (d *Deps) payGetConfig(ctx context.Context, provider string) rowMap {
	row, err := d.queryRow(ctx, payConfigSelect+" WHERE provider = $1 LIMIT 1", provider)
	if err != nil {
		return nil
	}
	return row
}

// payConfigJSON: parity configJson.
func payConfigJSON(row rowMap) map[string]any {
	return map[string]any{
		"id":             row.strOr("id", ""),
		"provider":       row.strOr("provider", ""),
		"environment":    row.strOr("environment", ""),
		"is_configured":  row.boolOr("is_configured", false),
		"is_active":      row.boolOr("is_active", false),
		"last_tested_at": row.timePtr("last_tested_at"),
		"last_test_ok":   ptNullableBool(row, "last_test_ok"),
		"updated_at":     row.timePtr("updated_at"),
	}
}

// ptNullableBool: kolom boolean nullable → bool atau nil.
func ptNullableBool(row rowMap, key string) any {
	if !row.has(key) {
		return nil
	}
	return row.boolOr(key, false)
}

// payCredentialFieldsJSON: daftar field untuk UI admin.
func payCredentialFieldsJSON(provider string) []map[string]any {
	fields := payCredentialFields[provider]
	out := make([]map[string]any, 0, len(fields))
	for _, f := range fields {
		item := map[string]any{"key": f.Key, "label": f.Label}
		if f.Secret {
			item["secret"] = true
		}
		if f.Required {
			item["required"] = true
		}
		if f.Placeholder != "" {
			item["placeholder"] = f.Placeholder
		}
		out = append(out, item)
	}
	return out
}

// payUpsertGatewayConfig: parity upsertGatewayConfig — merge dengan kredensial
// tersimpan supaya field yang tidak dikirim tidak terhapus.
func (d *Deps) payUpsertGatewayConfig(ctx context.Context, provider, environment string, input map[string]string, actorUserID string) (rowMap, error) {
	existing := d.payGetConfig(ctx, provider)
	credentials := paySanitizeCredentials(provider, input)
	if existing != nil {
		stored := d.payUnsealCredentials(existing.strOr("encrypted_credentials", ""))
		merged := map[string]string{}
		for k, v := range stored {
			merged[k] = v
		}
		for k, v := range credentials {
			merged[k] = v
		}
		credentials = merged
	}
	if msg := payValidateCredentials(provider, credentials); msg != "" {
		// Error biasa (bukan adminshared.Err) — handler memetakan pesan
		// "…wajib diisi…" ke 422 seperti route lama.
		return nil, errors.New(msg)
	}
	encrypted, err := d.paySealCredentials(provider, credentials)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		if err := d.exec(ctx,
			"UPDATE payment_gateway_configs SET environment = $1, encrypted_credentials = $2, is_configured = TRUE, updated_at = NOW() WHERE id = $3",
			environment, encrypted, existing.strOr("id", "")); err != nil {
			return nil, err
		}
		return d.payGetConfig(ctx, provider), nil
	}
	if err := d.exec(ctx,
		"INSERT INTO payment_gateway_configs (id, provider, environment, encrypted_credentials, is_configured, created_by) VALUES ($1, $2, $3, $4, TRUE, $5)",
		uuid.NewString(), provider, environment, encrypted, actorUserID); err != nil {
		return nil, err
	}
	return d.payGetConfig(ctx, provider), nil
}

// payActivateGateway: parity activateGateway — satu UPDATE atomik.
func (d *Deps) payActivateGateway(ctx context.Context, provider string) (rowMap, error) {
	config := d.payGetConfig(ctx, provider)
	if config == nil || !config.boolOr("is_configured", false) {
		return nil, errBadRequest("Konfigurasi " + provider + " belum lengkap.")
	}
	if err := d.exec(ctx,
		"UPDATE payment_gateway_configs SET is_active = (provider = $1), updated_at = NOW()", provider); err != nil {
		return nil, err
	}
	return d.payGetConfig(ctx, provider), nil
}

// payGetActiveGateway: parity getActiveGateway.
func (d *Deps) payGetActiveGateway(ctx context.Context) rowMap {
	row, err := d.queryRow(ctx, payConfigSelect+" WHERE is_active = TRUE LIMIT 1")
	if err != nil {
		return nil
	}
	return row
}

// payMarkGatewayTested: parity markGatewayTested.
func (d *Deps) payMarkGatewayTested(ctx context.Context, provider string, ok bool) {
	_ = d.exec(ctx,
		"UPDATE payment_gateway_configs SET last_tested_at = NOW(), last_test_ok = $1, updated_at = NOW() WHERE provider = $2",
		ok, provider)
}

// ---------- transaksi ----------

func payTransactionJSON(row rowMap) map[string]any {
	return map[string]any{
		"id":                      row.strOr("id", ""),
		"reference":               row.strOr("reference", ""),
		"gateway_provider":        row.strOr("gateway_provider", ""),
		"gateway_environment":     row.strOr("gateway_environment", ""),
		"amount":                  row.numOr("amount"),
		"currency":                row.strOr("currency", ""),
		"status":                  row.strOr("status", ""),
		"payment_method":          ptNullableStr(row, "payment_method"),
		"payment_url":             ptNullableStr(row, "payment_url"),
		"external_transaction_id": ptNullableStr(row, "external_transaction_id"),
		"provider_reference":      ptNullableStr(row, "provider_reference"),
		"purpose_ref_id":          ptNullableStr(row, "purpose_ref_id"),
		"created_at":              row.timePtr("created_at"),
		"paid_at":                 row.timePtr("paid_at"),
		"expired_at":              row.timePtr("expired_at"),
	}
}

// payGetTransaction: parity getPaymentTransaction.
func (d *Deps) payGetTransaction(ctx context.Context, id string) rowMap {
	row, err := d.queryRow(ctx, "SELECT * FROM payment_transactions WHERE id = $1 LIMIT 1", id)
	if err != nil {
		return nil
	}
	return row
}

// payListUserTransactions: parity listPaymentTransactions.
func (d *Deps) payListUserTransactions(ctx context.Context, userID string, limit, offset int) ([]rowMap, error) {
	return d.queryRows(ctx,
		"SELECT * FROM payment_transactions WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3",
		userID, limit, offset)
}

// payIsUniqueViolation: parity isUniqueViolation (kode PostgreSQL 23505).
func payIsUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return strings.Contains(err.Error(), "23505")
}

type payCreateInput struct {
	UserID         string
	BrandID        *string
	Amount         float64
	Currency       string
	Description    string
	PayerEmail     string
	CustomerName   string
	Purpose        string
	PurposeRefID   *string
	ReturnURL      *string
	InvoiceData    any
	IdempotencyKey *string
}

type payIntent struct {
	RawStatus         string
	PaymentURL        any
	PaymentMethod     any
	ProviderReference any
}

// payAttemptGatewayPayment: parity attemptGatewayPayment.
func (d *Deps) payAttemptGatewayPayment(ctx context.Context, input payCreateInput, gateway rowMap, existingID string) (rowMap, error) {
	transactionID := existingID
	provider := gateway.strOr("provider", "")
	if transactionID == "" {
		transactionID = uuid.NewString()
		var invoiceArg any
		if input.InvoiceData != nil {
			raw, err := json.Marshal(input.InvoiceData)
			if err != nil {
				return nil, err
			}
			invoiceArg = string(raw)
		}
		currency := input.Currency
		if currency == "" {
			currency = "IDR"
		}
		purpose := input.Purpose
		if purpose == "" {
			purpose = "generic"
		}
		_, err := d.Pool.Exec(ctx,
			`INSERT INTO payment_transactions (id, user_id, brand_id, gateway_provider, gateway_config_id, gateway_environment, external_transaction_id, amount, currency, status, purpose, purpose_ref_id, invoice_data, idempotency_key, reference)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending', $10, $11, $12::jsonb, $13,
			         'PAY-' || to_char(NOW(), 'YYYYMM') || '-' || lpad(nextval('payment_transaction_ref_seq')::text, 5, '0'))`,
			transactionID, input.UserID, input.BrandID, provider,
			gateway.strOr("id", ""), gateway.strOr("environment", ""), transactionID,
			input.Amount, currency, purpose, input.PurposeRefID, invoiceArg, input.IdempotencyKey)
		if err != nil {
			if payIsUniqueViolation(err) && input.BrandID != nil && input.IdempotencyKey != nil {
				existing, qErr := d.queryRow(ctx,
					"SELECT * FROM payment_transactions WHERE brand_id = $1 AND idempotency_key = $2 ORDER BY created_at DESC LIMIT 1",
					*input.BrandID, *input.IdempotencyKey)
				if qErr == nil && existing != nil {
					return existing, nil
				}
			}
			return nil, err
		}
	}

	credentials := d.payUnsealCredentials(gateway.strOr("encrypted_credentials", ""))
	intent, adapterErr := d.payAdapterCreatePayment(ctx, provider, gateway.strOr("environment", ""), credentials, input, transactionID)
	if adapterErr != nil {
		_ = d.exec(ctx, "UPDATE payment_transactions SET status = 'failed', updated_at = NOW() WHERE id = $1", transactionID)
		return d.payGetTransaction(ctx, transactionID), adapterErr
	}
	if err := d.exec(ctx,
		"UPDATE payment_transactions SET payment_url = $1, payment_method = $2, status = $3, provider_reference = $4, updated_at = NOW() WHERE id = $5",
		intent.PaymentURL, intent.PaymentMethod, payNormalizeStatus(provider, intent.RawStatus),
		intent.ProviderReference, transactionID); err != nil {
		return nil, err
	}
	updated := d.payGetTransaction(ctx, transactionID)
	if updated == nil {
		return nil, errNotFound("Payment transaction not found")
	}
	return updated, nil
}

// payCreatePayment: parity createPayment — gateway aktif + idempotency key.
func (d *Deps) payCreatePayment(ctx context.Context, input payCreateInput) (rowMap, error) {
	active := d.payGetActiveGateway(ctx)
	if active == nil {
		return nil, errBadRequest("Belum ada payment gateway aktif. Hubungi admin.")
	}
	key := ""
	if input.IdempotencyKey != nil {
		key = strings.TrimSpace(*input.IdempotencyKey)
		if len(key) > 100 {
			key = key[:100]
		}
		input.IdempotencyKey = &key
	}
	if key == "" {
		input.IdempotencyKey = nil
	}

	if key != "" && input.BrandID != nil {
		existing, err := d.queryRow(ctx,
			"SELECT * FROM payment_transactions WHERE brand_id = $1 AND idempotency_key = $2 ORDER BY created_at DESC LIMIT 1",
			*input.BrandID, key)
		if err == nil && existing != nil {
			if existing.strOr("status", "") == "failed" && !existing.has("payment_url") {
				return d.payAttemptGatewayPayment(ctx, input, active, existing.strOr("id", ""))
			}
			return existing, nil
		}
	}
	return d.payAttemptGatewayPayment(ctx, input, active, "")
}

// ---------- apply webhook (port applyWebhook + applyVerifiedPayment) ----------

type payWebhookVerification struct {
	Valid                 bool
	ExternalTransactionID *string
	RawStatus             *string
	Payload               any
}

// payApplyWebhook: parity applyWebhook.
func (d *Deps) payApplyWebhook(ctx context.Context, provider string, v payWebhookVerification) (rowMap, error) {
	if !v.Valid || v.ExternalTransactionID == nil || v.RawStatus == nil {
		return nil, nil
	}
	status := payNormalizeStatus(provider, *v.RawStatus)
	tx, _, _, err := d.payApplyVerifiedPayment(ctx, provider, *v.ExternalTransactionID, status, v.Payload)
	return tx, err
}

// payApplyVerifiedPayment: parity applyVerifiedPayment — UPDATE order +
// kredit TC dalam SATU transaksi DB, idempoten lewat unique
// payment_webhook_events + ledger `topup:{tx.id}`.
func (d *Deps) payApplyVerifiedPayment(ctx context.Context, provider, externalID, status string, payload any) (rowMap, bool, bool, error) {
	tx, err := d.queryRow(ctx,
		"SELECT * FROM payment_transactions WHERE gateway_provider = $1 AND external_transaction_id = $2 LIMIT 1",
		provider, externalID)
	if err != nil {
		return nil, false, false, err
	}
	if tx == nil {
		return nil, false, false, nil
	}
	currentStatus := tx.strOr("status", "")
	if payFinalStatuses[currentStatus] && currentStatus == status {
		return tx, false, true, nil
	}

	var payloadArg any
	if payload != nil {
		raw, mErr := json.Marshal(payload)
		if mErr == nil {
			payloadArg = string(raw)
		}
	}

	credited := false
	duplicate := false
	txErr := db.WithPoolTx(ctx, d.Pool, func(dbTx pgx.Tx) error {
		var lockedStatus string
		if err := dbTx.QueryRow(ctx,
			"SELECT status FROM payment_transactions WHERE id = $1 AND gateway_provider = $2 FOR UPDATE",
			tx.strOr("id", ""), provider).Scan(&lockedStatus); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				duplicate = true
				return nil
			}
			return err
		}

		var eventID string
		err := dbTx.QueryRow(ctx,
			`INSERT INTO payment_webhook_events (id, gateway_provider, external_transaction_id, status, payload)
			 VALUES ($1, $2, $3, $4, $5::jsonb)
			 ON CONFLICT (gateway_provider, external_transaction_id, status) DO NOTHING
			 RETURNING id`,
			uuid.NewString(), provider, externalID, status, payloadArg).Scan(&eventID)
		eventInserted := err == nil && eventID != ""
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !eventInserted && lockedStatus == status {
			duplicate = true
			return nil
		}

		tag, err := dbTx.Exec(ctx,
			`UPDATE payment_transactions
			 SET status = $1, raw_callback = $2::jsonb, paid_at = CASE WHEN $1 = 'paid' THEN COALESCE(paid_at, NOW()) ELSE paid_at END, updated_at = NOW()
			 WHERE id = $3 AND gateway_provider = $4 AND status IN ('pending', 'processing')`,
			status, payloadArg, tx.strOr("id", ""), provider)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			duplicate = true
			return nil
		}

		if status == "paid" && tx.strOr("purpose", "") == "topup" && tx.has("brand_id") {
			brandID := tx.strOr("brand_id", "")
			userID := tx.strOr("user_id", "")
			amountIDR := int(math.Round(tx.numOr("amount")))
			outcome, creditErr := d.payPostTopUpOnTx(ctx, dbTx, brandID, userID, tx.strOr("id", ""), amountIDR)
			if creditErr != nil {
				return creditErr
			}
			credited = !outcome.Duplicate
			duplicate = outcome.Duplicate
		}
		return nil
	})
	if txErr != nil {
		return nil, false, false, txErr
	}
	return d.payGetTransaction(ctx, tx.strOr("id", "")), credited, duplicate, nil
}

type payTopUpOutcome struct {
	TC        int
	RateIDR   int
	Duplicate bool
}

// payPostTopUpOnTx: port postTopUpOnConn — kredit dompet TC idempoten
// `topup:{txId}` di koneksi transaksi yang sama.
func (d *Deps) payPostTopUpOnTx(ctx context.Context, tx pgx.Tx, brandID, userID, paymentTxID string, amountIDR int) (payTopUpOutcome, error) {
	rate := d.finTCConfig(ctx).RateIDR
	if rate < 1 {
		rate = 1000
	}
	tc := amountIDR / rate
	if tc <= 0 {
		return payTopUpOutcome{}, errors.New("Payment amount is below one Tulisio Credit.")
	}
	key := "topup:" + paymentTxID
	var exists int
	if err := tx.QueryRow(ctx, "SELECT 1 FROM credit_ledger_entries WHERE idempotency_key = $1", key).Scan(&exists); err == nil {
		return payTopUpOutcome{TC: tc, RateIDR: rate, Duplicate: true}, nil
	}
	if _, err := tx.Exec(ctx, "INSERT INTO credit_wallets (brand_id) VALUES ($1) ON CONFLICT (brand_id) DO NOTHING", brandID); err != nil {
		return payTopUpOutcome{}, err
	}
	var available, reserved, captured int
	if err := tx.QueryRow(ctx,
		"UPDATE credit_wallets SET available_tc = available_tc + $1, updated_at = NOW() WHERE brand_id = $2 RETURNING available_tc, reserved_tc, captured_tc",
		tc, brandID).Scan(&available, &reserved, &captured); err != nil {
		return payTopUpOutcome{}, err
	}
	reason := "Top Up Tulisio Credit — pembayaran " + payShortID(paymentTxID, 8)
	if _, err := tx.Exec(ctx,
		`INSERT INTO credit_ledger_entries (id, brand_id, ref_type, ref_id, amount_tc, balance_after_tc, rate_idr, amount_idr, reason, idempotency_key, created_by)
		 VALUES ($1, $2, 'topup', $3, $4, $5, $6, $7, $8, $9, $10)`,
		uuid.NewString(), brandID, paymentTxID, tc, available+reserved+captured, rate, tc*rate,
		reason, key, userID); err != nil {
		return payTopUpOutcome{}, err
	}
	return payTopUpOutcome{TC: tc, RateIDR: rate}, nil
}

func payShortID(v string, n int) string {
	if len(v) <= n {
		return v
	}
	return v[:n]
}

// ---------- rekonsiliasi ----------

// payReconcileStatus: parity reconcilePaymentStatus — fallback saat webhook
// hilang. Hanya provider dengan verifyPaymentStatus (SumoPod).
func (d *Deps) payReconcileStatus(ctx context.Context, transactionID string) rowMap {
	tx := d.payGetTransaction(ctx, transactionID)
	if tx == nil || !tx.has("external_transaction_id") {
		return tx
	}
	if payFinalStatuses[tx.strOr("status", "")] {
		return tx
	}
	provider := tx.strOr("gateway_provider", "")
	if provider != "sumopod" {
		return tx
	}
	config := d.payGetConfig(ctx, provider)
	if config == nil || !config.boolOr("is_configured", false) {
		return tx
	}
	credentials := d.payUnsealCredentials(config.strOr("encrypted_credentials", ""))
	result := d.payVerifyPaymentStatus(ctx, provider, credentials, payVerifyInput{
		ExternalTransactionID: tx.strOr("external_transaction_id", ""),
		ProviderReference:     tx.strOr("provider_reference", ""),
		Amount:                int(math.Round(tx.numOr("amount"))),
		Environment:           tx.strOr("gateway_environment", ""),
	})
	if result == nil || !result.Verified {
		return tx
	}
	if result.Status == tx.strOr("status", "") {
		return tx
	}
	updated, _, _, err := d.payApplyVerifiedPayment(ctx, provider, tx.strOr("external_transaction_id", ""), result.Status, result.Payload)
	if err != nil || updated == nil {
		return tx
	}
	return updated
}

// ---------- HTTP provider (port providerFetch + adapter methods) ----------

const payTimeout = 15 * time.Second

// payFetch: parity providerFetch — timeout 15s, kembalikan status + body.
func payFetch(ctx context.Context, method, target string, headers map[string]string, body string) (int, string) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return 0, ""
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := security.SafeClient(payTimeout).Do(req)
	if err != nil {
		return 0, ""
	}
	defer resp.Body.Close()
	text, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, string(text)
}

func payReadJSON(body string) map[string]any {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		return map[string]any{}
	}
	return parsed
}

func payAsString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

// payBaseURL: parity baseUrl per provider + environment.
func payBaseURL(provider, environment string) string {
	switch provider {
	case "duitku":
		if environment == "sandbox" {
			return "https://sandbox.duitku.com"
		}
		return "https://duitku.com"
	case "midtrans":
		if environment == "sandbox" {
			return "https://app.sandbox.midtrans.com"
		}
		return "https://app.midtrans.com"
	case "tripay":
		if environment == "sandbox" {
			return "https://tripay.co.id/api-sandbox"
		}
		return "https://tripay.co.id/api"
	case "sumopod":
		if environment == "sandbox" {
			return "https://api-pay-sandbox.sumopod.com"
		}
		return "https://api-pay.sumopod.com"
	case "xendit":
		return "https://api.xendit.co"
	case "sandbox":
		return ""
	}
	return ""
}

// payAdapterCreatePayment: port createPayment per adapter.
func (d *Deps) payAdapterCreatePayment(ctx context.Context, provider, environment string, credentials map[string]string, input payCreateInput, externalID string) (*payIntent, error) {
	base := payBaseURL(provider, environment)
	currency := input.Currency
	if currency == "" {
		currency = "IDR"
	}
	description := input.Description
	if description == "" {
		description = "Pembayaran Tulisio"
	}
	amount := int(math.Round(input.Amount))

	switch provider {
	case "xendit":
		payload, _ := json.Marshal(map[string]any{
			"external_id": externalID, "amount": amount,
			"payer_email": nullIfEmpty(input.PayerEmail), "description": description, "currency": currency,
		})
		headers := map[string]string{"Content-Type": "application/json", "x-api-key": credentials["api_key"]}
		status, body := payFetch(ctx, http.MethodPost, base+"/v2/invoices", headers, string(payload))
		data := payReadJSON(body)
		if status < 200 || status >= 300 {
			return nil, errors.New("Xendit: " + aiOrEmpty(payAsString(data["message"]), aiOrEmpty(payAsString(data["error"]), "HTTP "+ptItoa(status))))
		}
		return &payIntent{
			RawStatus:         aiOrEmpty(payAsString(data["status"]), "PENDING"),
			PaymentURL:        ptNullableStr(rowMap(data), "invoice_url"),
			PaymentMethod:     "invoice",
			ProviderReference: ptNullableStr(rowMap(data), "id"),
		}, nil

	case "duitku":
		form := url.Values{}
		form.Set("merchantCode", credentials["merchant_code"])
		form.Set("merchantOrderId", externalID)
		form.Set("paymentAmount", ptItoa(amount))
		form.Set("productDetails", description)
		form.Set("email", input.PayerEmail)
		customerName := input.CustomerName
		if customerName == "" {
			customerName = "Customer Tauthub"
		}
		form.Set("customerVaName", customerName)
		returnURL := "https://app.tauthub.com/"
		if input.ReturnURL != nil {
			returnURL = *input.ReturnURL
		}
		form.Set("merchantUrl", returnURL)
		statusURL := returnURL
		if input.ReturnURL == nil {
			statusURL = "https://app.tauthub.com/payments/status"
		}
		form.Set("returnUrl", statusURL)
		headers := map[string]string{"Content-Type": "application/x-www-form-urlencoded"}
		status, body := payFetch(ctx, http.MethodPost, base+"/webapi/api/merchant/v2/inquiry", headers, form.Encode())
		data := payReadJSON(body)
		if status < 200 || status >= 300 || payAsString(data["statusCode"]) != "00" {
			return nil, errors.New("Duitku: " + aiOrEmpty(payAsString(data["statusMessage"]), aiOrEmpty(payAsString(data["message"]), "HTTP "+ptItoa(status))))
		}
		return &payIntent{
			RawStatus:         "WAITING",
			PaymentURL:        ptNullableStr(rowMap(data), "paymentUrl"),
			PaymentMethod:     ptNullableStr(rowMap(data), "paymentMethod"),
			ProviderReference: ptNullableStr(rowMap(data), "reference"),
		}, nil

	case "midtrans":
		basic := base64.StdEncoding.EncodeToString([]byte(credentials["server_key"] + ":"))
		payload, _ := json.Marshal(map[string]any{
			"transaction_details": map[string]any{"order_id": externalID, "gross_amount": amount},
			"item_details": []map[string]any{{
				"id": "tauthub-payment", "price": amount, "quantity": 1, "name": description,
			}},
			"customer_details": map[string]any{"first_name": aiOrEmpty(input.CustomerName, "Customer"), "email": input.PayerEmail},
		})
		headers := map[string]string{"Content-Type": "application/json", "Authorization": "Basic " + basic}
		status, body := payFetch(ctx, http.MethodPost, base+"/snap/v1/transactions", headers, string(payload))
		data := payReadJSON(body)
		if status < 200 || status >= 300 {
			detail := payAsString(data["status_message"])
			if arr, ok := data["error_messages"].([]any); ok && len(arr) > 0 {
				detail = payAsString(arr[0])
			}
			if detail == "" {
				detail = "HTTP " + ptItoa(status)
			}
			return nil, errors.New("Midtrans: " + detail)
		}
		return &payIntent{
			RawStatus: "pending", PaymentURL: ptNullableStr(rowMap(data), "redirect_url"),
			PaymentMethod: "snap",
		}, nil

	case "tripay":
		customerName := input.CustomerName
		if customerName == "" {
			customerName = "Customer Tauthub"
		}
		payload, _ := json.Marshal(map[string]any{
			"method": "QRIS", "merchant_ref": externalID, "amount": amount,
			"customer_name": customerName, "customer_email": input.PayerEmail,
			"order_items": []map[string]any{{"sku": "tauthub-payment", "name": description, "price": amount, "quantity": 1}},
		})
		headers := map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + credentials["api_key"]}
		status, body := payFetch(ctx, http.MethodPost, base+"/transaction/create", headers, string(payload))
		data := payReadJSON(body)
		item, _ := data["data"].(map[string]any)
		if status < 200 || status >= 300 || data["success"] == false {
			return nil, errors.New("Tripay: " + aiOrEmpty(payAsString(data["message"]), "HTTP "+ptItoa(status)))
		}
		rawStatus := "UNPAID"
		if item != nil {
			rawStatus = aiOrEmpty(payAsString(item["status"]), "UNPAID")
		}
		return &payIntent{
			RawStatus: rawStatus, PaymentURL: ptNullableStr(rowMap(item), "checkout_url"),
			PaymentMethod:     "QRIS",
			ProviderReference: ptNullableStr(rowMap(item), "reference"),
		}, nil

	case "sumopod":
		appBase := strings.TrimRight(d.Cfg.PublicAppURL, "/")
		if input.ReturnURL != nil {
			base := strings.TrimRight(*input.ReturnURL, "/")
			if base != "" {
				appBase = base
			}
		}
		toHTTPS := func(v string) string {
			if strings.HasPrefix(strings.ToLower(v), "http://") {
				return "https://" + v[len("http://"):]
			}
			return v
		}
		brandSegment := ""
		if input.BrandID != nil {
			brandSegment = "/" + url.PathEscape(*input.BrandID)
		}
		body := map[string]any{
			"order_id": externalID, "amount": amount, "currency": currency,
			"expires_in_hours":         24,
			"payment_method_type_code": aiOrEmpty(strings.TrimSpace(credentials["payment_method_code"]), "QRIS"),
		}
		if appBase != "" {
			body["success_return_url"] = toHTTPS(appBase + "/payments/success" + brandSegment)
			body["cancel_return_url"] = toHTTPS(appBase + "/payments/cancelled" + brandSegment)
		}
		payload, _ := json.Marshal(body)
		headers := map[string]string{"Content-Type": "application/json", "X-Api-Key": credentials["api_key"]}
		status, raw := payFetch(ctx, http.MethodPost, base+"/api/v1/payments", headers, string(payload))
		data := payReadJSON(raw)
		if status < 200 || status >= 300 {
			msg := aiOrEmpty(payAsString(data["message"]), aiOrEmpty(payAsString(data["error"]), "HTTP "+ptItoa(status)))
			return nil, errors.New("SumoPod: " + msg)
		}
		return &payIntent{
			RawStatus:         aiOrEmpty(payAsString(data["status"]), "pending"),
			PaymentURL:        ptNullableStr(rowMap(data), "payment_link_url"),
			PaymentMethod:     "QRIS",
			ProviderReference: ptNullableStr(rowMap(data), "payment_id"),
		}, nil

	case "sandbox":
		return &payIntent{
			RawStatus: "UNPAID", PaymentURL: "/payments/sandbox/" + externalID,
			PaymentMethod: "sandbox", ProviderReference: externalID,
		}, nil
	}
	return nil, errors.New("Unknown payment provider")
}

// ---------- signature webhook (port verifyWebhook per adapter) ----------

const paySandboxSecret = "tauthub-sandbox-secret"

// payVerifyWebhook: parity getAdapter(provider).verifyWebhook(rawBody, headers, credentials).
func payVerifyWebhook(provider, rawBody string, headers http.Header, credentials map[string]string) payWebhookVerification {
	invalid := payWebhookVerification{}
	switch provider {
	case "xendit":
		if credentials["callback_token"] != "" {
			token := headers.Get("x-callback-token")
			if token == "" || token != credentials["callback_token"] {
				return invalid
			}
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(rawBody), &payload); err != nil {
			return invalid
		}
		external, _ := payload["external_id"].(string)
		if external == "" {
			return invalid
		}
		// parity `payload.status ?? null` — status hilang bukan "PENDING".
		return payWebhookVerification{Valid: true, ExternalTransactionID: &external,
			RawStatus: ptStrOrNil(payAsString(payload["status"])), Payload: payload}

	case "duitku":
		var payload map[string]any
		if err := json.Unmarshal([]byte(rawBody), &payload); err != nil {
			return invalid
		}
		orderID, _ := payload["merchantOrderId"].(string)
		if orderID == "" {
			return invalid
		}
		merchantCode := payAsString(payload["merchantCode"])
		if merchantCode == "" {
			merchantCode = credentials["merchant_code"]
		}
		sum := md5.Sum([]byte(merchantCode + orderID + payAsString(payload["amount"]) + credentials["api_key"]))
		expected := hex.EncodeToString(sum[:])
		signature := strings.ToLower(payAsString(payload["signature"]))
		if signature == "" || signature != expected {
			return invalid
		}
		rawStatus := "FAILED"
		switch payAsString(payload["resultCode"]) {
		case "00":
			rawStatus = "SUCCESS"
		case "01":
			rawStatus = "FAILED"
		case "02":
			rawStatus = "CANCELLED"
		case "03":
			rawStatus = "EXPIRED"
		}
		return payWebhookVerification{Valid: true, ExternalTransactionID: &orderID,
			RawStatus: &rawStatus, Payload: payload}

	case "midtrans":
		var payload map[string]any
		if err := json.Unmarshal([]byte(rawBody), &payload); err != nil {
			return invalid
		}
		orderID, _ := payload["order_id"].(string)
		if orderID == "" {
			return invalid
		}
		// signature_key = SHA512(order_id + status_code + gross_amount + server_key)
		expected := fmt.Sprintf("%x", sha512.Sum512([]byte(
			orderID+payAsString(payload["status_code"])+payAsString(payload["gross_amount"])+credentials["server_key"])))
		if payAsString(payload["signature_key"]) == "" || payAsString(payload["signature_key"]) != expected {
			return invalid
		}
		rawStatus := payAsString(payload["transaction_status"])
		return payWebhookVerification{Valid: true, ExternalTransactionID: &orderID,
			RawStatus: ptStrOrNil(rawStatus), Payload: payload}

	case "tripay":
		signature := headers.Get("x-callback-signature")
		if signature == "" {
			return invalid
		}
		mac := hmac.New(sha256.New, []byte(credentials["private_key"]))
		mac.Write([]byte(rawBody))
		expected := hex.EncodeToString(mac.Sum(nil))
		if signature != expected {
			return invalid
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(rawBody), &payload); err != nil {
			return invalid
		}
		merchantRef, _ := payload["merchant_ref"].(string)
		if merchantRef == "" {
			return invalid
		}
		rawStatus := payAsString(payload["status"])
		return payWebhookVerification{Valid: true, ExternalTransactionID: &merchantRef,
			RawStatus: ptStrOrNil(rawStatus), Payload: payload}

	case "sumopod":
		tokenOk := false
		if credentials["webhook_token"] != "" {
			token := headers.Get("x-webhook-token")
			if token != "" && token == credentials["webhook_token"] {
				tokenOk = true
			}
		}
		svixOk := false
		if !tokenOk {
			svixID := headers.Get("svix-id")
			svixTs := headers.Get("svix-timestamp")
			svixSig := headers.Get("svix-signature")
			secret := strings.TrimPrefix(credentials["webhook_secret"], "whsec_")
			if svixID != "" && svixTs != "" && svixSig != "" && secret != "" {
				key, err := base64.StdEncoding.DecodeString(secret)
				if err == nil {
					mac := hmac.New(sha256.New, key)
					mac.Write([]byte(svixID + "." + svixTs + "." + rawBody))
					expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
					for _, part := range strings.Split(svixSig, " ") {
						pieces := strings.Split(part, ",")
						if len(pieces) > 1 && pieces[1] == expected {
							svixOk = true
						}
					}
				}
			}
		}
		if !tokenOk && !svixOk {
			return invalid
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(rawBody), &payload); err != nil {
			return invalid
		}
		data, _ := payload["data"].(map[string]any)
		orderID := ""
		if data != nil {
			orderID = payAsString(data["order_id"])
		}
		if orderID == "" {
			orderID = payAsString(payload["order_id"])
		}
		eventType := payAsString(payload["event_type"])
		if eventType == "payment.test" {
			return payWebhookVerification{Valid: true, Payload: payload}
		}
		if orderID == "" {
			return invalid
		}
		rawStatus := ""
		switch eventType {
		case "payment.completed":
			rawStatus = "paid"
		case "payment.failed":
			rawStatus = "failed"
		case "payment.expired":
			rawStatus = "expired"
		default:
			if data != nil {
				rawStatus = payAsString(data["status"])
			}
		}
		return payWebhookVerification{Valid: true, ExternalTransactionID: &orderID,
			RawStatus: ptStrOrNil(rawStatus), Payload: payload}

	case "sandbox":
		signature := headers.Get("x-sandbox-signature")
		if signature == "" {
			return invalid
		}
		mac := hmac.New(sha256.New, []byte(paySandboxSecret))
		mac.Write([]byte(rawBody))
		expected := hex.EncodeToString(mac.Sum(nil))
		if strings.ToLower(signature) != expected {
			return invalid
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(rawBody), &payload); err != nil {
			return invalid
		}
		orderID, _ := payload["order_id"].(string)
		if orderID == "" {
			return invalid
		}
		rawStatus := payAsString(payload["status"])
		return payWebhookVerification{Valid: true, ExternalTransactionID: &orderID,
			RawStatus: ptStrOrNil(rawStatus), Payload: payload}
	}
	return invalid
}

// ---------- verifyPaymentStatus / testConnection / resolve ----------

type payVerifyInput struct {
	ExternalTransactionID string
	ProviderReference     string
	Amount                int
	Environment           string
}

type payVerifyResult struct {
	Verified bool
	Status   string
	OrderID  *string
	Payload  any
}

// payVerifyPaymentStatus: port adapter.verifyPaymentStatus (hanya SumoPod yang
// punya endpoint status publik).
func (d *Deps) payVerifyPaymentStatus(ctx context.Context, provider string, credentials map[string]string, input payVerifyInput) *payVerifyResult {
	if provider != "sumopod" {
		return nil
	}
	if input.ProviderReference == "" {
		return nil
	}
	base := payBaseURL(provider, input.Environment)
	status, body := payFetch(ctx, http.MethodGet,
		base+"/api/v1/public/payments/"+url.PathEscape(input.ProviderReference)+"/status", nil, "")
	if status < 200 || status >= 300 {
		return nil
	}
	data := payReadJSON(body)
	orderID := payAsString(data["order_id"])
	amount, _ := data["amount"].(float64)
	verified := orderID != "" && orderID == input.ExternalTransactionID && int(amount) == input.Amount
	var orderIDPtr *string
	if orderID != "" {
		orderIDPtr = &orderID
	}
	return &payVerifyResult{
		Verified: verified, Status: payNormalizeStatus(provider, payAsString(data["status"])),
		OrderID: orderIDPtr, Payload: data,
	}
}

// payResolveProviderPaymentID: port resolveProviderPaymentId (fallback halaman
// status saat lookup DB tidak menemukan transaksi).
func (d *Deps) payResolveProviderPaymentID(ctx context.Context, provider, environment, paymentID string) (string, bool) {
	if provider != "sumopod" {
		return "", false
	}
	base := payBaseURL(provider, environment)
	status, body := payFetch(ctx, http.MethodGet,
		base+"/api/v1/public/payments/"+url.PathEscape(paymentID)+"/status", nil, "")
	if status < 200 || status >= 300 {
		return "", false
	}
	orderID := payAsString(payReadJSON(body)["order_id"])
	if orderID == "" {
		return "", false
	}
	return orderID, true
}

// payTestConnection: port adapter.testConnection.
func (d *Deps) payTestConnection(ctx context.Context, provider string, credentials map[string]string) (bool, string) {
	switch provider {
	case "sandbox":
		return true, "Gateway sandbox lokal selalu tersedia."
	case "xendit":
		status, _ := payFetch(ctx, http.MethodGet, "https://api.xendit.co/v1/balance",
			map[string]string{"x-api-key": credentials["api_key"]}, "")
		if status >= 200 && status < 300 {
			return true, "Koneksi berhasil (balance OK)."
		}
		return false, "Xendit menolak kredensial (HTTP " + ptItoa(status) + ")."
	case "duitku":
		target := "https://duitku.com/webapi/api/merchant/v2/getPaymentMethod?merchantCode=" +
			url.QueryEscape(credentials["merchant_code"]) + "&apiKey=" + url.QueryEscape(credentials["api_key"])
		status, _ := payFetch(ctx, http.MethodGet, target, nil, "")
		if status >= 200 && status < 300 {
			return true, "Koneksi berhasil (payment methods OK)."
		}
		return false, "Duitku menolak kredensial (HTTP " + ptItoa(status) + ")."
	case "midtrans":
		basic := base64.StdEncoding.EncodeToString([]byte(credentials["server_key"] + ":"))
		status, _ := payFetch(ctx, http.MethodGet,
			"https://app.midtrans.com/v1/transactions/nonexistent/status",
			map[string]string{"Authorization": "Basic " + basic}, "")
		if status == 401 || status == 403 {
			return false, "Midtrans menolak kredensial (HTTP " + ptItoa(status) + ")."
		}
		return true, "Koneksi berhasil (server key diterima)."
	case "tripay":
		status, _ := payFetch(ctx, http.MethodGet,
			"https://tripay.co.id/api/merchant/transactions?page=1&per_page=1",
			map[string]string{"Authorization": "Bearer " + credentials["api_key"]}, "")
		if status >= 200 && status < 300 {
			return true, "Koneksi berhasil (merchant OK)."
		}
		return false, "Tripay menolak kredensial (HTTP " + ptItoa(status) + ")."
	case "sumopod":
		payload, _ := json.Marshal(map[string]any{
			"order_id": "test-" + ptItoa(int(time.Now().UnixMilli())), "amount": 10000,
			"currency":                 "IDR",
			"payment_method_type_code": aiOrEmpty(strings.TrimSpace(credentials["payment_method_code"]), "QRIS"),
		})
		headers := map[string]string{"Content-Type": "application/json", "X-Api-Key": credentials["api_key"]}
		status, _ := payFetch(ctx, http.MethodPost,
			"https://api-pay-sandbox.sumopod.com/api/v1/payments", headers, string(payload))
		if status >= 200 && status < 300 {
			return true, "Koneksi berhasil (payment link dibuat)."
		}
		return false, "SumoPod menolak kredensial (HTTP " + ptItoa(status) + ")."
	}
	return false, "Provider tidak dikenal."
}

// ---------- util ----------

func ptStrOrNil(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
