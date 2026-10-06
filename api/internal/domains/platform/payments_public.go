package platform

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Modul pembayaran publik — port app/v1/payments/** (4 endpoint). Hanya
// memerlukan Bearer user biasa (bukan admin), kecuali status & webhook yang
// benar-benar publik.

var payPublicFinalStatuses = payFinalStatuses

// PublicPaymentsList: GET /v1/payments
func (d *Deps) PublicPaymentsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	limit := clampInt(r.URL.Query().Get("limit"), 20, 1, 100)
	offset := clampInt(r.URL.Query().Get("offset"), 0, 0, 1<<30)
	txs, err := d.payListUserTransactions(r.Context(), user.ID, limit, offset)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := make([]map[string]any, 0, len(txs))
	for _, tx := range txs {
		out = append(out, payTransactionJSON(tx))
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": out})
}

// PublicPaymentsCreate: POST /v1/payments
func (d *Deps) PublicPaymentsCreate(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	ctx := r.Context()
	// parity jsonBody + createSchema.safeParse: body kosong/rusak → 422
	// "Invalid input: expected object, received null".
	body, okBody := aiStrictObject(w, d, r)
	if !okBody {
		return
	}

	// createSchema: amount coerce number positive max 1e9, currency 3..8 default
	// IDR, description trim max 500 nullable optional, brand_id uuid nullable optional.
	amountRaw, hasAmount := body["amount"]
	if !hasAmount || amountRaw == nil {
		writeErr(w, d, aiZodInvalidType("number", "undefined"))
		return
	}
	amount := 0.0
	switch t := amountRaw.(type) {
	case float64:
		amount = t
	case string:
		parsed, pErr := aiParseFloat(t)
		if pErr != nil {
			writeErr(w, d, aiZodInvalidType("number", "string"))
			return
		}
		amount = parsed
	default:
		writeErr(w, d, aiZodInvalidType("number", aiZodReceivedType(amountRaw)))
		return
	}
	if amount <= 0 {
		writeErr(w, d, errUnprocessable("Too small: expected number to be >0"))
		return
	}
	if amount > 1000000000 {
		writeErr(w, d, errUnprocessable("Too big: expected number to be <=1000000000"))
		return
	}
	currency, _, err := aiStringField(body, "currency", 3, 8, false, false)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if currency == "" {
		currency = "IDR"
	}
	description, _, err := aiStringField(body, "description", 0, 500, true, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	var brandID *string
	// parity z.string().uuid().nullable().optional().
	brandRaw, hasBrand, err := aiStringField(body, "brand_id", 0, 0, false, true)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if hasBrand {
		if !aiIsUUID(brandRaw) {
			writeErr(w, d, errUnprocessable("Invalid UUID"))
			return
		}
		brandID = &brandRaw
	}

	purpose := "generic"
	desc := description
	if desc == "" {
		desc = "Pembayaran Tulisio"
	}

	transaction, createErr := d.payCreatePayment(ctx, payCreateInput{
		UserID:       user.ID,
		BrandID:      brandID,
		Amount:       amount,
		Currency:     currency,
		Description:  desc,
		PayerEmail:   user.Email,
		CustomerName: user.DisplayName,
		Purpose:      purpose,
	})
	if createErr != nil {
		if strings.Contains(createErr.Error(), "Belum ada payment gateway aktif") {
			writeErr(w, d, errBadRequest(createErr.Error()))
			return
		}
		writeErr(w, d, createErr)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transaction": payTransactionJSON(transaction)})
}

// PublicPaymentGatewayActive: GET /v1/payments/gateway/active
func (d *Deps) PublicPaymentGatewayActive(w http.ResponseWriter, r *http.Request) {
	if httpx.AuthUserFromCtx(r) == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	active := d.payGetActiveGateway(r.Context())
	if active == nil {
		writeErr(w, d, errNotFound("No active payment gateway"))
		return
	}
	provider := active.strOr("provider", "")
	writeJSON(w, http.StatusOK, map[string]any{
		"provider":    provider,
		"label":       payProviderLabels[provider],
		"environment": active.strOr("environment", ""),
	})
}

// PublicPaymentStatus: GET /v1/payments/status/{transactionId} — PUBLIK.
func (d *Deps) PublicPaymentStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Publik tanpa auth — dibatasi longgar supaya polling halaman pembayaran
	// sah tidak terganggu, tapi pengurasan/recon massal tertahan.
	if !d.pubRateLimit(w, r, "payments-status", 120) {
		return
	}
	transactionID := chiParam(r, "transactionId")

	loadTx := func(id string) (rowMap, error) {
		return d.queryRow(ctx,
			`SELECT t.id, t.status, t.purpose, t.purpose_ref_id, t.amount, t.paid_at, t.gateway_provider, t.reference,
			        b.name AS brand_name, b.id AS brand_id
			 FROM payment_transactions t
			 LEFT JOIN brands b ON b.id = t.brand_id
			 WHERE t.id = $1 OR t.provider_reference = $2 OR t.external_transaction_id = $3
			 LIMIT 1`,
			id, id, id)
	}

	tx, loadErr := loadTx(transactionID)
	if loadErr != nil {
		// parity catch → 500 "Gagal memuat status pembayaran".
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": "Gagal memuat status pembayaran"})
		return
	}
	if tx == nil {
		// Fallback: redirect gateway membawa payment id provider yang belum
		// tercatat. Verifikasi ke API publik provider (echo order_id internal).
		gateway := d.payGetActiveGateway(ctx)
		if gateway != nil {
			orderID, resolved := d.payResolveProviderPaymentID(ctx, gateway.strOr("provider", ""),
				gateway.strOr("environment", ""), transactionID)
			if resolved {
				candidate, qErr := d.queryRow(ctx,
					`SELECT t.id, t.status, t.purpose, t.purpose_ref_id, t.amount, t.paid_at, t.gateway_provider, t.reference,
					        b.name AS brand_name, b.id AS brand_id
					 FROM payment_transactions t
					 LEFT JOIN brands b ON b.id = t.brand_id
					 WHERE t.external_transaction_id = $1 AND t.gateway_provider = $2
					 LIMIT 1`,
					orderID, gateway.strOr("provider", ""))
				if qErr == nil && candidate != nil {
					// Rekam payment id provider biar lookup berikutnya langsung kena.
					_ = d.exec(ctx,
						"UPDATE payment_transactions SET provider_reference = COALESCE(provider_reference, $1), updated_at = NOW() WHERE id = $2",
						transactionID, candidate.strOr("id", ""))
					tx = candidate
				}
			}
		}
	}
	if tx == nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Pembayaran tidak ditemukan"})
		return
	}

	if !payPublicFinalStatuses[tx.strOr("status", "")] {
		// Rekonsiliasi server-side (throttle 10 detik per transaksi).
		txID := tx.strOr("id", "")
		now := time.Now()
		last, hasLast := payReconcileCache.get(txID)
		lastAt, _ := last.(time.Time)
		if !hasLast || now.Sub(lastAt) >= 10*time.Second {
			payReconcileCache.set(txID, now)
			// Rekonsiliasi memakai id internal; baris dimuat ulang lewat loadTx
			// supaya brand_name/brand_id ikut terbawa.
			d.payReconcileStatus(ctx, txID)
			if refreshed, rErr := loadTx(txID); rErr == nil && refreshed != nil {
				tx = refreshed
			}
		}
		if payPublicFinalStatuses[tx.strOr("status", "")] {
			payReconcileCache.invalidate(txID)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"transaction_id":   tx.strOr("id", ""),
		"status":           tx.strOr("status", ""),
		"purpose":          ptNullableStr(tx, "purpose"),
		"purpose_ref_id":   ptNullableStr(tx, "purpose_ref_id"),
		"amount":           tx.numOr("amount"),
		"paid_at":          tx.timePtr("paid_at"),
		"gateway_provider": ptNullableStr(tx, "gateway_provider"),
		"reference":        ptNullableStr(tx, "reference"),
		"brand_name":       ptNullableStr(tx, "brand_name"),
		"brand_id":         ptNullableStr(tx, "brand_id"),
	})
}

// payReconcileCache: throttle rekonsiliasi per transaksi (parity lastReconcileAt).
var payReconcileCache = newTTLCache(10 * time.Second)

// PaymentWebhook: POST /v1/payments/webhooks/{provider} — PUBLIK (trust dari
// signature provider, diverifikasi per adapter).
func (d *Deps) PaymentWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	provider := chiParam(r, "provider")
	if !aiContains(payProviders, provider) {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Unknown payment provider"})
		return
	}
	config := d.payGetConfig(ctx, provider)
	if config == nil || !config.boolOr("is_configured", false) {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Gateway not configured"})
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "Invalid webhook signature"})
		return
	}
	rawBody := string(raw)
	credentials := d.payUnsealCredentials(config.strOr("encrypted_credentials", ""))
	verification := payVerifyWebhook(provider, rawBody, r.Header, credentials)
	if !verification.Valid {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "Invalid webhook signature"})
		return
	}
	transaction, applyErr := d.payApplyWebhook(ctx, provider, verification)
	if applyErr != nil {
		// 500 → provider retry; transaksi sudah di-rollback (idempoten).
		// Tanpa log ini penyebab kegagalan kredit tidak bisa dilacak dari luar.
		externalID := ""
		if verification.ExternalTransactionID != nil {
			externalID = *verification.ExternalTransactionID
		}
		slog.Error("[payments] gagal menerapkan webhook",
			"provider", provider, "external_id", externalID, "err", applyErr)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": "Failed to apply payment"})
		return
	}
	var txID any
	if transaction != nil {
		txID = transaction.strOr("id", "")
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "transaction_id": txID})
}
