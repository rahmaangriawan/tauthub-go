package platform

import (
	"net/http"
	"strings"
)

// Modul payments admin — port app/v1/platform/payments/** (4 endpoint).
// Semua butuh permission "payments" (parity hasAdminPermission → 404).

// PaymentsGateways: GET /v1/platform/payments/gateways
func (d *Deps) PaymentsGateways(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	configs, err := d.payListConfigs(r.Context())
	if err != nil {
		writeErr(w, d, err)
		return
	}
	byProvider := map[string]rowMap{}
	for _, config := range configs {
		byProvider[config.strOr("provider", "")] = config
	}
	// Semua provider selalu dikembalikan — yang belum dikonfigurasi tampil
	// sebagai shell supaya admin bisa mengisi kredensial tanpa baris DB.
	gateways := make([]map[string]any, 0, len(payProviders))
	for _, provider := range payProviders {
		config := byProvider[provider]
		item := map[string]any{
			"id":                "shell-" + provider,
			"provider":          provider,
			"label":             payProviderLabels[provider],
			"environment":       "sandbox",
			"is_configured":     false,
			"is_active":         false,
			"last_tested_at":    nil,
			"last_test_ok":      nil,
			"updated_at":        nil,
			"credentials":       map[string]any{},
			"credential_fields": payCredentialFieldsJSON(provider),
		}
		if config != nil {
			item["id"] = config.strOr("id", "shell-"+provider)
			item["environment"] = config.strOr("environment", "sandbox")
			item["is_configured"] = config.boolOr("is_configured", false)
			item["is_active"] = config.boolOr("is_active", false)
			item["last_tested_at"] = config.timePtr("last_tested_at")
			item["last_test_ok"] = ptNullableBool(config, "last_test_ok")
			item["updated_at"] = config.timePtr("updated_at")
			if config.boolOr("is_configured", false) {
				item["credentials"] = payMaskedCredentials(d.payUnsealCredentials(config.strOr("encrypted_credentials", "")))
			}
		}
		gateways = append(gateways, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"gateways": gateways})
}

// PaymentGatewaySave: POST /v1/platform/payments/gateways
func (d *Deps) PaymentGatewaySave(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	ctx := r.Context()
	// parity route lama: jsonBody → null → safeParse null → 422 pesan issue pertama.
	body, okBody := aiStrictObject(w, d, r)
	if !okBody {
		return
	}
	providerRaw, hasProvider, err := aiStringField(body, "provider", 0, 0, false, false)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if !hasProvider {
		writeErr(w, d, aiZodInvalidType("string", "undefined"))
		return
	}
	if !aiContains(payProviders, providerRaw) {
		writeErr(w, d, aiZodInvalidOption(payProviders...))
		return
	}
	provider := providerRaw
	environment, err := aiEnumField(body, "environment", "sandbox", payEnvironments...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	credentialsRaw, hasCredentials, err := aiObjectField(body, "credentials")
	if err != nil {
		writeErr(w, d, err)
		return
	}
	credentials := map[string]string{}
	if hasCredentials {
		for key, value := range credentialsRaw {
			s, isString := value.(string)
			if !isString {
				writeErr(w, d, aiZodInvalidType("string", aiZodReceivedType(value)))
				return
			}
			credentials[key] = s
		}
	}

	config, saveErr := d.payUpsertGatewayConfig(ctx, provider, environment, credentials, user.ID)
	if saveErr != nil {
		// parity: error validasi "…wajib diisi…" → 422, sisanya 500.
		if payIsRequiredFieldMessage(saveErr.Error()) {
			writeErr(w, d, errUnprocessable(saveErr.Error()))
			return
		}
		writeErr(w, d, saveErr)
		return
	}
	payload := payConfigJSON(config)
	payload["label"] = payProviderLabels[provider]
	payload["credentials"] = payMaskedCredentials(d.payUnsealCredentials(config.strOr("encrypted_credentials", "")))
	payload["credential_fields"] = payCredentialFieldsJSON(provider)
	writeJSON(w, http.StatusOK, payload)
}

// payIsRequiredFieldMessage: parity /wajib diisi/i pada pesan error validasi.
func payIsRequiredFieldMessage(msg string) bool {
	return strings.Contains(strings.ToLower(msg), "wajib diisi")
}

// PaymentGatewayActivate: POST /v1/platform/payments/gateways/{gatewayId}/activate
func (d *Deps) PaymentGatewayActivate(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	ctx := r.Context()
	gatewayID := chiParam(r, "gatewayId")
	configs, err := d.payListConfigs(ctx)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	var target rowMap
	for _, config := range configs {
		if config.strOr("id", "") == gatewayID {
			target = config
			break
		}
	}
	if target == nil {
		writeErr(w, d, errNotFound("Payment gateway not found"))
		return
	}
	if !target.boolOr("is_configured", false) {
		writeErr(w, d, errUnprocessable("Konfigurasi gateway belum lengkap."))
		return
	}
	provider := target.strOr("provider", "")
	active, err := d.payActivateGateway(ctx, provider)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	refreshed, err := d.payListConfigs(ctx)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	activePayload := payConfigJSON(active)
	activePayload["label"] = payProviderLabels[provider]
	activePayload["credentials"] = payMaskedCredentials(d.payUnsealCredentials(active.strOr("encrypted_credentials", "")))

	gateways := make([]map[string]any, 0, len(refreshed))
	for _, config := range refreshed {
		item := payConfigJSON(config)
		item["label"] = payProviderLabels[config.strOr("provider", "")]
		if config.boolOr("is_configured", false) {
			item["credentials"] = payMaskedCredentials(d.payUnsealCredentials(config.strOr("encrypted_credentials", "")))
		} else {
			item["credentials"] = map[string]any{}
		}
		gateways = append(gateways, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"active": activePayload, "gateways": gateways})
}

// PaymentGatewayTest: POST /v1/platform/payments/gateways/{gatewayId}/test
func (d *Deps) PaymentGatewayTest(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	ctx := r.Context()
	gatewayID := chiParam(r, "gatewayId")
	configs, err := d.payListConfigs(ctx)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	var target rowMap
	for _, config := range configs {
		if config.strOr("id", "") == gatewayID {
			target = config
			break
		}
	}
	if target == nil {
		writeErr(w, d, errNotFound("Payment gateway not found"))
		return
	}
	if !target.boolOr("is_configured", false) {
		writeErr(w, d, errUnprocessable("Konfigurasi gateway belum lengkap."))
		return
	}
	provider := target.strOr("provider", "")
	credentials := d.payUnsealCredentials(target.strOr("encrypted_credentials", ""))
	testOK, message := d.payTestConnection(ctx, provider, credentials)
	d.payMarkGatewayTested(ctx, provider, testOK)
	writeJSON(w, http.StatusOK, map[string]any{"ok": testOK, "message": message})
}

// PaymentTransactions: GET /v1/platform/payments/transactions
func (d *Deps) PaymentTransactions(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	limit := clampInt(r.URL.Query().Get("limit"), 50, 1, 200)
	txs, err := d.queryRows(r.Context(),
		`SELECT id, reference, gateway_provider, gateway_environment, amount, currency, status,
		        purpose, purpose_ref_id, payment_method,
		        (invoice_data IS NOT NULL) AS has_invoice, created_at, paid_at
		 FROM payment_transactions
		 ORDER BY created_at DESC
		 LIMIT $1`, limit)
	if err != nil {
		// parity: route lama membalas pesan generik tanpa detail error.
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Gagal memuat log transaksi."))
		return
	}
	out := make([]map[string]any, 0, len(txs))
	for _, t := range txs {
		out = append(out, map[string]any{
			"id":             t.strOr("id", ""),
			"reference":      t.strOr("reference", ""),
			"provider":       t.strOr("gateway_provider", ""),
			"environment":    t.strOr("gateway_environment", ""),
			"amount":         t.numOr("amount"),
			"currency":       t.strOr("currency", ""),
			"status":         t.strOr("status", ""),
			"purpose":        t.strOr("purpose", ""),
			"purpose_ref_id": ptNullableStr(t, "purpose_ref_id"),
			"payment_method": ptNullableStr(t, "payment_method"),
			"has_invoice":    t.boolOr("has_invoice", false),
			"created_at":     t.timePtr("created_at"),
			"paid_at":        t.timePtr("paid_at"),
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"transactions": out})
}
