package platform

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	dbpkg "github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/imageconvert"
	"github.com/rahmaangriawan/tauthub-go/internal/notify"
)

// finNonAlnumRe: pembersih ekstensi file (parity replace(/[^a-z0-9]/g, "")).
var finNonAlnumRe = regexp.MustCompile(`[^a-z0-9]`)

func finAuthUserFromCtx(r *http.Request) *httpx.AuthUser { return httpx.AuthUserFromCtx(r) }

// finReadAllLimited: baca stream sampai max bytes; error bila melebihi.
func finReadAllLimited(reader io.Reader, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("file terlalu besar")
	}
	return data, nil
}

// finSaveUploadFile: parity saveUpload — tulis ke UPLOAD_DIR (.data/uploads).
func (d *Deps) finSaveUploadFile(storageKey string, data []byte) error {
	dir := strings.TrimSpace(os.Getenv("UPLOAD_DIR"))
	if dir == "" {
		dir = filepath.Join(".data", "uploads")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, storageKey), data, 0o644)
}

// Modul finance platform: credits/tc_config, packages, invoices, orders,
// revenue-shares, settlements, referrals, referral-audit, referred-domains,
// vip-partners, pricing. Port app/v1/platform/** terkait.

var finPublicationStatuses = []string{
	"pending", "waiting_partner", "partner_approved", "partner_rejected", "queued",
	"processing", "published", "failed", "retrying", "cancelled",
}

// ---------- credits (tc_config + wallet + adjustment) ----------

// finTCConfig: parity getTcConfig (rate, packages, minimum withdraw).
type finTCConfig struct {
	RateIDR            int
	TopupPackages      []map[string]any
	MinimumWithdrawIDR int
}

func (d *Deps) finTCConfig(ctx context.Context) finTCConfig {
	row, _ := d.queryRow(ctx,
		"SELECT rate_idr, topup_packages, minimum_withdraw_idr, withdraw_bank_fee_idr, withdraw_ewallet_fee_percent FROM tc_config WHERE id = 1 LIMIT 1")
	rate := 1000
	minWithdraw := 50000
	packages := []map[string]any{
		{"tc": 500, "idr": 500000},
		{"tc": 1000, "idr": 1000000},
		{"tc": 5000, "idr": 5000000},
	}
	if row != nil {
		rate = row.intOr("rate_idr", 1000)
		if rate < 1 {
			rate = 1000
		}
		minWithdraw = row.intOr("minimum_withdraw_idr", 50000)
		parsed := finParseTopupPackages(row.jsonField("topup_packages"))
		if len(parsed) > 0 {
			packages = parsed
		}
	}
	return finTCConfig{RateIDR: rate, TopupPackages: packages, MinimumWithdrawIDR: minWithdraw}
}

func finParseTopupPackages(v any) []map[string]any {
	arr, ok := v.([]any)
	if !ok {
		if s, isString := v.(string); isString {
			var parsed []any
			if err := json.Unmarshal([]byte(s), &parsed); err == nil {
				arr = parsed
			}
		}
	}
	if !ok && arr == nil {
		return nil
	}
	out := []map[string]any{}
	for _, item := range arr {
		obj, isObj := item.(map[string]any)
		if !isObj {
			continue
		}
		tc, tcOK := finNumInt(obj["tc"])
		idr, idrOK := finNumInt(obj["idr"])
		if !tcOK || !idrOK || tc <= 0 || idr <= 0 {
			continue
		}
		out = append(out, map[string]any{"tc": tc, "idr": idr})
	}
	return out
}

func finNumInt(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		if t != float64(int(t)) {
			return 0, false
		}
		return int(t), true
	case int:
		return t, true
	case int64:
		return int(t), true
	}
	return 0, false
}

// CreditsConfigGet: GET /v1/platform/credits
func (d *Deps) CreditsConfigGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "topup")
	if !ok {
		return
	}
	q := r.URL.Query()
	search := strings.TrimSpace(q.Get("q"))
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	clause := ""
	params := []any{}
	if search != "" {
		like := "%" + search + "%"
		params = append(params, like, like)
		clause = "AND (b.name ILIKE $1 OR b.client_name ILIKE $2)"
	}
	countRow, err := d.queryRow(r.Context(),
		"SELECT COUNT(*) AS count FROM credit_wallets w JOIN brands b ON b.id = w.brand_id WHERE 1 = 1 "+clause, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	args := append([]any{}, params...)
	args = append(args, limit, offset)
	items, err := d.queryRows(r.Context(),
		`SELECT w.*, b.name AS brand_name, b.client_name,
		        (SELECT COALESCE(SUM(amount_tc), 0)::int FROM credit_ledger_entries e WHERE e.brand_id = w.brand_id AND e.ref_type = 'topup') AS topup_tc,
		        (SELECT COALESCE(SUM(amount_idr), 0)::int FROM credit_ledger_entries e WHERE e.brand_id = w.brand_id AND e.ref_type = 'topup') AS topup_idr,
		        (SELECT COALESCE(SUM(e.amount_tc), 0)::bigint FROM credit_ledger_entries e WHERE e.brand_id = w.brand_id AND e.ref_type = 'capture' AND e.amount_tc > 0) AS pub_tc
		 FROM credit_wallets w JOIN brands b ON b.id = w.brand_id
		 WHERE 1 = 1 `+clause+`
		 ORDER BY w.updated_at DESC LIMIT $`+ptItoa(len(params)+1)+` OFFSET $`+ptItoa(len(params)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	wallets := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		available := int(row.numOr("available_tc"))
		reserved := int(row.numOr("reserved_tc"))
		captured := int(row.numOr("captured_tc"))
		wallets = append(wallets, map[string]any{
			"brand_id": row.strOr("brand_id", ""), "brand_name": ptNullableStr(row, "brand_name"),
			"client_name":  ptNullableStr(row, "client_name"),
			"available_tc": available, "reserved_tc": reserved, "captured_tc": captured,
			"total_tc": available + reserved + captured,
			"topup_tc": int(row.numOr("topup_tc")), "topup_idr": int(row.numOr("topup_idr")),
			// pub_tc = biaya publikasi saja (entri capture bertanda positif).
			// Entri capture bertanda negatif adalah biaya AI (Taut Pintar,
			// gambar, Profiler) yang ditagih langsung dari saldo, jadi tidak
			// boleh ikut dijumlahkan di sini.
			"pub_tc": int(row.numOr("pub_tc")), "updated_at": row.anyOrNil("updated_at"),
		})
	}
	cfg := d.finTCConfig(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"wallets": wallets, "total": countRow.intOr("count", 0),
		"tc_config": map[string]any{
			"rate_idr": cfg.RateIDR, "topup_packages": cfg.TopupPackages, "minimum_withdraw_idr": cfg.MinimumWithdrawIDR,
		},
	})
}

// CreditsConfigPut: PUT /v1/platform/credits
func (d *Deps) CreditsConfigPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "topup")
	if !ok {
		return
	}
	body := decodeJSON(r)
	rateIDR, hasRate := intField(body, "rate_idr")
	if !hasRate || rateIDR < 1 || rateIDR > 100000 {
		writeErr(w, d, errUnprocessable("Invalid TC config"))
		return
	}
	packagesRaw, hasPackages := rawField(body, "topup_packages")
	packagesArr, isArr := packagesRaw.([]any)
	if !hasPackages || !isArr || len(packagesArr) < 1 || len(packagesArr) > 20 {
		writeErr(w, d, errUnprocessable("Invalid TC config"))
		return
	}
	normalized := make([]map[string]any, 0, len(packagesArr))
	for _, item := range packagesArr {
		obj, isObj := item.(map[string]any)
		if !isObj {
			writeErr(w, d, errUnprocessable("Invalid TC config"))
			return
		}
		tc, tcOK := finNumInt(obj["tc"])
		idr, idrOK := finNumInt(obj["idr"])
		if !tcOK || !idrOK || tc < 1 || idr < 1 {
			writeErr(w, d, errUnprocessable("Invalid TC config"))
			return
		}
		if tc*rateIDR != idr {
			writeErr(w, d, errUnprocessable(fmt.Sprintf("Paket %d TC × rate %d = Rp%d, bukan Rp%d", tc, rateIDR, tc*rateIDR, idr)))
			return
		}
		normalized = append(normalized, map[string]any{"tc": tc, "idr": idr})
	}
	if err := d.exec(r.Context(), "UPDATE tc_config SET rate_idr = $1, updated_by = $2, updated_at = NOW() WHERE id = 1", rateIDR, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "platform.credits.rate_updated", "tc_config", "1")
	if err := d.exec(r.Context(), "UPDATE tc_config SET topup_packages = $1, updated_by = $2, updated_at = NOW() WHERE id = 1", ptJSONString(normalized), user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	cfg := d.finTCConfig(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		"tc_config": map[string]any{
			"rate_idr": cfg.RateIDR, "topup_packages": cfg.TopupPackages, "minimum_withdraw_idr": cfg.MinimumWithdrawIDR,
		},
	})
}

// CreditsAdjust: POST /v1/platform/credits
func (d *Deps) CreditsAdjust(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "topup")
	if !ok {
		return
	}
	body := decodeJSON(r)
	brandID, _ := strField(body, "brand_id")
	tc, hasTC := intField(body, "tc")
	direction, _ := strField(body, "direction")
	reason, _ := strField(body, "reason")
	if brandID == "" {
		writeErr(w, d, errUnprocessable("Invalid adjustment"))
		return
	}
	if !hasTC || tc < 1 || tc > 10000000 {
		writeErr(w, d, errUnprocessable("Invalid adjustment"))
		return
	}
	if direction != "credit" && direction != "debit" {
		writeErr(w, d, errUnprocessable("Invalid adjustment"))
		return
	}
	if len(reason) < 3 || len(reason) > 500 {
		writeErr(w, d, errUnprocessable("Invalid adjustment"))
		return
	}
	brand, err := d.queryRow(r.Context(), "SELECT id FROM brands WHERE id = $1 LIMIT 1", brandID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if brand == nil {
		writeErr(w, d, errNotFound("Brand not found"))
		return
	}
	signed := tc
	if direction == "debit" {
		signed = -tc
	}
	if err := d.finAdjustCredit(r.Context(), brandID, signed, fmt.Sprintf("Admin %s — %s", direction, reason), user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "platform.credits.adjusted", "brand", brandID)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok": true, "direction": direction, "tc": tc, "brand_id": brandID,
	})
}

// finAdjustCredit: port finAdjustCredit — idempotent, balance tidak boleh minus.
func (d *Deps) finAdjustCredit(ctx context.Context, brandID string, tc int, reason, userID string) error {
	if tc == 0 {
		return fmt.Errorf("Adjustment amount must be a non-zero integer.")
	}
	rate := d.finTCConfig(ctx).RateIDR
	return d.ptWithTx(ctx, func(tx *ptTx) error {
		if err := tx.exec("INSERT INTO credit_wallets (brand_id) VALUES ($1) ON CONFLICT (brand_id) DO NOTHING", brandID); err != nil {
			return err
		}
		updated, err := tx.queryRows(
			`UPDATE credit_wallets SET available_tc = available_tc + $1, updated_at = NOW()
			 WHERE brand_id = $2 AND available_tc + $1 >= 0
			 RETURNING available_tc, reserved_tc, captured_tc`,
			tc, brandID)
		if err != nil {
			return err
		}
		if len(updated) == 0 {
			avail, _ := tx.queryRows("SELECT available_tc FROM credit_wallets WHERE brand_id = $1", brandID)
			available := 0
			if len(avail) > 0 {
				available = int(avail[0].numOr("available_tc"))
			}
			if available < 0 {
				available = 0
			}
			return fmt.Errorf("Insufficient credits: need %d, available %d", finAbsInt(tc), available)
		}
		row := updated[0]
		total := int(row.numOr("available_tc")) + int(row.numOr("reserved_tc")) + int(row.numOr("captured_tc"))
		var amountIDR any
		if tc >= 0 {
			amountIDR = tc * rate
		}
		idempotencyKey := "adjustment:" + uuid.NewString()
		return tx.exec(
			`INSERT INTO credit_ledger_entries (id, brand_id, ref_type, ref_id, amount_tc, balance_after_tc, rate_idr, amount_idr, reason, idempotency_key, created_by)
			 VALUES ($1, $2, 'adjustment', NULL, $3, $4, $5, $6, $7, $8, $9)`,
			uuid.NewString(), brandID, tc, total, rate, amountIDR, reason, idempotencyKey, userID)
	})
}

func finAbsInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ---------- packages ----------

func pkgJSON(row ptRow) map[string]any {
	total := int(row.numOr("credit_total"))
	used := int(row.numOr("credit_used"))
	return map[string]any{
		"id": row.strOr("id", ""), "brand_id": row.strOr("brand_id", ""),
		"brand_name": ptNullableStr(row, "brand_name"), "name": row.strOr("name", ""),
		"credit_total": total, "credit_used": used, "credit_remaining": total - used,
		"price": int(row.numOr("price")), "status": row.strOr("status", ""),
		"starts_at": row.anyOrNil("starts_at"), "ends_at": row.anyOrNil("ends_at"),
		"created_by": ptNullableStr(row, "created_by"),
		"created_at": row.anyOrNil("created_at"), "updated_at": row.anyOrNil("updated_at"),
	}
}

func (d *Deps) pkgScoped(r *http.Request, packageID string) (ptRow, error) {
	row, err := d.queryRow(r.Context(), "SELECT * FROM packages WHERE id = $1 LIMIT 1", packageID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Package not found")
	}
	return ptRow(ptNormalizeMap(row)), nil
}

// CreditPackagesList: GET /v1/platform/packages
func (d *Deps) CreditPackagesList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	brandID := strings.TrimSpace(q.Get("brand_id"))
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && status != "active" && status != "exhausted" && status != "revoked" {
		writeErr(w, d, errUnprocessable("Invalid package status filter"))
		return
	}
	limit := clampInt(q.Get("limit"), 100, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	where := []string{}
	params := []any{}
	if brandID != "" {
		params = append(params, brandID)
		where = append(where, "p.brand_id = $"+ptItoa(len(params)))
	}
	if status != "" {
		params = append(params, status)
		where = append(where, "p.status = $"+ptItoa(len(params)))
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM packages p "+clause, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	args := append([]any{}, params...)
	args = append(args, limit, offset)
	items, err := d.queryRows(r.Context(),
		"SELECT p.*, b.name AS brand_name FROM packages p LEFT JOIN brands b ON b.id = p.brand_id "+
			clause+" ORDER BY p.created_at DESC LIMIT $"+ptItoa(len(params)+1)+" OFFSET $"+ptItoa(len(params)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	packages := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		packages = append(packages, pkgJSON(ptRow(ptNormalizeMap(raw))))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"packages": packages, "total": countRow.intOr("count", 0), "limit": limit, "offset": offset,
	})
}

// CreditPackageCreate: POST /v1/platform/packages
func (d *Deps) CreditPackageCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	brandID, _ := strField(body, "brand_id")
	name, _ := strField(body, "name")
	creditTotal, hasCredit := intField(body, "credit_total")
	if brandID == "" || len(name) < 2 || len(name) > 160 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if !hasCredit || creditTotal < 1 || creditTotal > 100000000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	price := 0
	if p, hasPrice := numField(body, "price"); hasPrice {
		if p < 0 || p > 1_000_000_000_000 {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		price = int(p)
	}
	status := "active"
	if s, hasStatus := strField(body, "status"); hasStatus && s != "" {
		if s != "active" && s != "revoked" {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		status = s
	}
	startsAt, hasStarts := strField(body, "starts_at")
	endsAt, hasEnds := strField(body, "ends_at")
	brand, err := d.queryRow(r.Context(), "SELECT id FROM brands WHERE id = $1 LIMIT 1", brandID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if brand == nil {
		writeErr(w, d, errNotFound("Brand not found"))
		return
	}
	id := uuid.NewString()
	if err := d.ptWithTx(r.Context(), func(tx *ptTx) error {
		if err := tx.exec(
			"INSERT INTO packages (id, brand_id, name, credit_total, price, status, starts_at, ends_at, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
			id, brandID, name, creditTotal, price, status, finNullableTime(startsAt, hasStarts), finNullableTime(endsAt, hasEnds), user.ID); err != nil {
			return err
		}
		return tx.exec(
			"INSERT INTO credit_transactions (id, brand_id, package_id, amount, reason, balance_after, created_by) VALUES ($1, $2, $3, $4, 'package.grant', $5, $6)",
			uuid.NewString(), brandID, id, creditTotal, creditTotal, user.ID)
	}); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "package.created", "package", id)
	created, err := d.pkgScoped(r, id)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusCreated, pkgJSON(created))
}

func finNullableTime(raw string, has bool) any {
	if !has || raw == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t
	}
	return nil
}

// CreditPackageGet: GET /v1/platform/packages/{packageId}
func (d *Deps) CreditPackageGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.pkgScoped(r, chiParam(r, "packageId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	brandName := any(nil)
	brand, _ := d.queryRow(r.Context(), "SELECT name FROM brands WHERE id = $1 LIMIT 1", row.strOr("brand_id", ""))
	if brand != nil {
		brandName = brand.strOr("name", "")
	}
	row["brand_name"] = brandName
	writeJSON(w, http.StatusOK, pkgJSON(row))
}

// CreditPackageUpdate: PUT /v1/platform/packages/{packageId}
func (d *Deps) CreditPackageUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	packageID := chiParam(r, "packageId")
	if _, err := d.pkgScoped(r, packageID); err != nil {
		writeErr(w, d, err)
		return
	}
	body := decodeJSON(r)
	name, hasName := strField(body, "name")
	creditTotal, hasCredit := intField(body, "credit_total")
	price, hasPrice := intField(body, "price")
	status, hasStatus := strField(body, "status")
	startsAt, hasStarts := strField(body, "starts_at")
	endsAt, hasEnds := strField(body, "ends_at")
	if hasName && (len(name) < 2 || len(name) > 160) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasCredit && (creditTotal < 1 || creditTotal > 100000000) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasPrice && price < 0 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasStatus {
		if status != "active" && status != "exhausted" && status != "revoked" {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
	}
	if err := d.exec(r.Context(),
		`UPDATE packages SET
		   name = COALESCE($1, name),
		   credit_total = COALESCE($2, credit_total),
		   price = COALESCE($3, price),
		   status = COALESCE($4, status),
		   starts_at = COALESCE($5, starts_at),
		   ends_at = COALESCE($6, ends_at),
		   updated_at = NOW()
		 WHERE id = $7`,
		ptNullableArgStr(name, hasName), ptNullableAnyInt(creditTotal, hasCredit),
		ptNullableAnyInt(price, hasPrice), ptNullableArgStr(status, hasStatus && status != ""),
		finNullableTime(startsAt, hasStarts), finNullableTime(endsAt, hasEnds), packageID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "package.updated", "package", packageID)
	updated, err := d.pkgScoped(r, packageID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, pkgJSON(updated))
}

// CreditPackageRevoke: DELETE /v1/platform/packages/{packageId} (soft revoke).
func (d *Deps) CreditPackageRevoke(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	packageID := chiParam(r, "packageId")
	if _, err := d.pkgScoped(r, packageID); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(r.Context(), "UPDATE packages SET status = 'revoked', updated_at = NOW() WHERE id = $1", packageID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "package.revoked", "package", packageID)
	updated, err := d.pkgScoped(r, packageID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, pkgJSON(updated))
}

// ---------- invoices ----------

func invJSON(row ptRow) map[string]any {
	return map[string]any{
		"id": row.strOr("id", ""), "brand_id": row.strOr("brand_id", ""),
		"brand_name": ptNullableStr(row, "brand_name"), "campaign_id": ptNullableStr(row, "campaign_id"),
		"number": row.strOr("number", ""), "status": row.strOr("status", ""),
		"amount": int(row.numOr("amount")), "issued_at": row.anyOrNil("issued_at"),
		"due_at": row.anyOrNil("due_at"), "paid_at": row.anyOrNil("paid_at"),
		"notes": ptNullableStr(row, "notes"), "created_by": ptNullableStr(row, "created_by"),
		"created_at": row.anyOrNil("created_at"), "updated_at": row.anyOrNil("updated_at"),
	}
}

func (d *Deps) invScoped(r *http.Request, invoiceID string) (ptRow, error) {
	row, err := d.queryRow(r.Context(),
		"SELECT i.*, b.name AS brand_name FROM invoices i LEFT JOIN brands b ON b.id = i.brand_id WHERE i.id = $1 LIMIT 1", invoiceID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Invoice not found")
	}
	return ptRow(ptNormalizeMap(row)), nil
}

// InvoicesList: GET /v1/platform/invoices
func (d *Deps) InvoicesList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	q := r.URL.Query()
	brandID := strings.TrimSpace(q.Get("brand_id"))
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && status != "draft" && status != "issued" && status != "paid" && status != "void" {
		writeErr(w, d, errUnprocessable("Invalid invoice status filter"))
		return
	}
	limit := clampInt(q.Get("limit"), 100, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	where := []string{}
	params := []any{}
	if brandID != "" {
		params = append(params, brandID)
		where = append(where, "i.brand_id = $"+ptItoa(len(params)))
	}
	if status != "" {
		params = append(params, status)
		where = append(where, "i.status = $"+ptItoa(len(params)))
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM invoices i "+clause, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	args := append([]any{}, params...)
	args = append(args, limit, offset)
	items, err := d.queryRows(r.Context(),
		"SELECT i.*, b.name AS brand_name FROM invoices i LEFT JOIN brands b ON b.id = i.brand_id "+
			clause+" ORDER BY i.created_at DESC LIMIT $"+ptItoa(len(params)+1)+" OFFSET $"+ptItoa(len(params)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	invoices := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		invoices = append(invoices, invJSON(ptRow(ptNormalizeMap(raw))))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"invoices": invoices, "total": countRow.intOr("count", 0), "limit": limit, "offset": offset,
	})
}

// InvoiceCreate: POST /v1/platform/invoices
func (d *Deps) InvoiceCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	body := decodeJSON(r)
	brandID, _ := strField(body, "brand_id")
	if brandID == "" {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	campaignID, hasCampaign := strField(body, "campaign_id")
	amount, hasAmount := numField(body, "amount")
	number, hasNumber := strField(body, "number")
	status := "draft"
	if s, hasStatus := strField(body, "status"); hasStatus && s != "" {
		if s != "draft" && s != "issued" && s != "paid" && s != "void" {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		status = s
	}
	issuedAt, hasIssued := strField(body, "issued_at")
	dueAt, hasDue := strField(body, "due_at")
	notes, hasNotes := strField(body, "notes")
	if hasNumber && len(number) > 40 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasNotes && len(notes) > 2000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	brand, err := d.queryRow(r.Context(), "SELECT id FROM brands WHERE id = $1 LIMIT 1", brandID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if brand == nil {
		writeErr(w, d, errNotFound("Brand not found"))
		return
	}
	if hasCampaign && campaignID != "" {
		campaign, cErr := d.queryRow(r.Context(),
			"SELECT id FROM campaigns WHERE id = $1 AND brand_id = $2 LIMIT 1", campaignID, brandID)
		if cErr != nil {
			writeErr(w, d, cErr)
			return
		}
		if campaign == nil {
			writeErr(w, d, errNotFound("Campaign not found for this brand"))
			return
		}
	}
	if !(hasCampaign && campaignID != "") && !hasAmount {
		writeErr(w, d, errUnprocessable("amount is required for manual invoices"))
		return
	}
	amountValue := int64(0)
	if hasCampaign && campaignID != "" {
		sumRow, _ := d.queryRow(r.Context(),
			"SELECT COALESCE(SUM(client_price), 0)::numeric(14,2) AS total FROM publications WHERE campaign_id = $1 AND status = 'published'", campaignID)
		if sumRow != nil {
			amountValue = int64(sumRow.numOr("total"))
		}
	} else {
		amountValue = int64(amount)
	}
	id := uuid.NewString()
	numberValue := number
	if !hasNumber || number == "" {
		numberValue = d.invNextNumber(r)
	}
	issuedAtValue := finNullableTime(issuedAt, hasIssued)
	if !hasIssued && (status == "issued" || status == "paid") {
		issuedAtValue = time.Now()
	}
	if err := d.exec(r.Context(),
		"INSERT INTO invoices (id, brand_id, campaign_id, number, status, amount, issued_at, due_at, notes, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)",
		id, brandID, ptNullableArgStr(campaignID, hasCampaign && campaignID != ""), numberValue, status,
		amountValue, issuedAtValue, finNullableTime(dueAt, hasDue), ptNullableArgStr(notes, hasNotes), user.ID); err != nil {
		if mcIsDuplicate(err) {
			writeErr(w, d, errConflict("Invoice number already exists"))
			return
		}
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "finance.invoice.created", "invoice", id)
	created, err := d.invScoped(r, id)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusCreated, invJSON(created))
}

// invNextNumber: parity invNextNumber (INV-YYYYMM-NNNN).
func (d *Deps) invNextNumber(r *http.Request) string {
	now := time.Now()
	prefix := fmt.Sprintf("INV-%d%02d", now.Year(), int(now.Month()))
	row, _ := d.queryRow(r.Context(), "SELECT COUNT(*)::int AS count FROM invoices WHERE number LIKE $1", prefix+"%")
	count := 0
	if row != nil {
		count = row.intOr("count", 0)
	}
	return fmt.Sprintf("%s-%04d", prefix, count+1)
}

// InvoiceGet: GET /v1/platform/invoices/{invoiceId}
func (d *Deps) InvoiceGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	row, err := d.invScoped(r, chiParam(r, "invoiceId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, invJSON(row))
}

// InvoiceUpdate: PUT /v1/platform/invoices/{invoiceId}
func (d *Deps) InvoiceUpdate(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	invoiceID := chiParam(r, "invoiceId")
	if _, err := d.invScoped(r, invoiceID); err != nil {
		writeErr(w, d, err)
		return
	}
	body := decodeJSON(r)
	amount, hasAmount := numField(body, "amount")
	status, hasStatus := strField(body, "status")
	issuedAt, hasIssued := strField(body, "issued_at")
	dueAt, hasDue := strField(body, "due_at")
	paidAt, hasPaid := strField(body, "paid_at")
	notes, hasNotes := strField(body, "notes")
	if hasStatus && status != "draft" && status != "issued" && status != "paid" && status != "void" {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasNotes && len(notes) > 2000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	var paidAtValue any
	if hasStatus && status == "paid" {
		if hasPaid && paidAt != "" {
			paidAtValue = finNullableTime(paidAt, true)
		} else {
			paidAtValue = time.Now()
		}
	} else if hasPaid {
		paidAtValue = finNullableTime(paidAt, true)
	}
	if err := d.exec(r.Context(),
		`UPDATE invoices SET
		   amount = COALESCE($1, amount),
		   status = COALESCE($2, status),
		   issued_at = COALESCE($3, issued_at),
		   due_at = COALESCE($4, due_at),
		   paid_at = COALESCE($5, paid_at),
		   notes = COALESCE($6, notes),
		   updated_at = NOW()
		 WHERE id = $7`,
		ptNullableAnyInt(int(amount), hasAmount), ptNullableArgStr(status, hasStatus && status != ""),
		finNullableTime(issuedAt, hasIssued), finNullableTime(dueAt, hasDue), paidAtValue,
		ptNullableArgStr(notes, hasNotes), invoiceID); err != nil {
		writeErr(w, d, err)
		return
	}
	updated, err := d.invScoped(r, invoiceID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, invJSON(updated))
}

// InvoiceVoid: DELETE /v1/platform/invoices/{invoiceId} (soft).
func (d *Deps) InvoiceVoid(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	invoiceID := chiParam(r, "invoiceId")
	invoice, err := d.invScoped(r, invoiceID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if invoice.strOr("status", "") == "paid" {
		writeErr(w, d, errConflict("Paid invoices cannot be voided"))
		return
	}
	if err := d.exec(r.Context(), "UPDATE invoices SET status = 'void', updated_at = NOW() WHERE id = $1", invoiceID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "finance.invoice.voided", "invoice", invoiceID)
	updated, err := d.invScoped(r, invoiceID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, invJSON(updated))
}

// ---------- orders ----------

// OrdersList: GET /v1/platform/orders
func (d *Deps) OrdersList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && !ptContains(finPublicationStatuses, status) {
		writeErr(w, d, errUnprocessable("Invalid publication status filter"))
		return
	}
	problemOnly := q.Get("problem") == "1"
	search := strings.TrimSpace(q.Get("q"))
	limit := clampInt(q.Get("limit"), 50, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	clauses := []string{}
	baseParams := []any{}
	if status != "" {
		baseParams = append(baseParams, status)
		clauses = append(clauses, "p.status = $"+ptItoa(len(baseParams)))
	}
	if problemOnly {
		clauses = append(clauses, "(p.status IN ('failed','retrying','partner_rejected','cancelled') OR (p.scheduled_at IS NOT NULL AND p.published_at IS NULL AND p.scheduled_at < NOW() - INTERVAL '48 hours'))")
	}
	if search != "" {
		like := "%" + search + "%"
		baseParams = append(baseParams, like, like, like, like, like, like)
		clauses = append(clauses, "(p.order_code ILIKE $"+ptItoa(len(baseParams)-5)+" OR c.order_code ILIKE $"+ptItoa(len(baseParams)-4)+" OR d.title ILIKE $"+ptItoa(len(baseParams)-3)+" OR b.name ILIKE $"+ptItoa(len(baseParams)-2)+" OR m.name ILIKE $"+ptItoa(len(baseParams)-1)+" OR p.id ILIKE $"+ptItoa(len(baseParams))+")")
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}
	selectColumns := `p.id AS publication_id, p.order_code, p.status, p.publish_status, p.client_price, p.scheduled_at, p.published_at, p.published_url, p.created_at,
	                  c.order_code AS campaign_order_code, c.name AS campaign_name,
	                  b.name AS brand_name, b.public_id AS brand_public_id,
	                  m.name AS media_name, m.integration_type AS media_integration_type,
	                  d.title AS document_title,
	                  (SELECT pa.error_code FROM publication_attempts pa WHERE pa.publication_id = p.id ORDER BY pa.attempt_number DESC LIMIT 1) AS error_code,
	                  (SELECT pa.error_message FROM publication_attempts pa WHERE pa.publication_id = p.id ORDER BY pa.attempt_number DESC LIMIT 1) AS error_message`
	fromClause := `FROM publications p
	   LEFT JOIN campaigns c ON c.id = p.campaign_id
	   JOIN brands b ON b.id = p.brand_id
	   JOIN media m ON m.id = p.media_id
	   JOIN content_documents d ON d.id = p.document_id
	   ` + where
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count "+fromClause, baseParams...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	args := append([]any{}, baseParams...)
	args = append(args, limit, offset)
	items, err := d.queryRows(r.Context(),
		"SELECT "+selectColumns+" "+fromClause+" ORDER BY p.created_at DESC LIMIT $"+ptItoa(len(baseParams)+1)+" OFFSET $"+ptItoa(len(baseParams)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	orders := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		orders = append(orders, map[string]any{
			"publication_id": row.strOr("publication_id", ""), "order_code": ptNullableStr(row, "order_code"),
			"campaign_order_code": ptNullableStr(row, "campaign_order_code"), "campaign_name": ptNullableStr(row, "campaign_name"),
			"status": row.strOr("status", ""), "publish_status": row.anyOrNil("publish_status"),
			"client_price": int(row.numOr("client_price")), "scheduled_at": row.anyOrNil("scheduled_at"),
			"published_at": row.anyOrNil("published_at"), "published_url": ptNullableStr(row, "published_url"),
			"created_at": row.anyOrNil("created_at"), "brand_name": ptNullableStr(row, "brand_name"),
			"brand_public_id": ptNullableStr(row, "brand_public_id"), "media_name": ptNullableStr(row, "media_name"),
			"media_integration_type": ptNullableStr(row, "media_integration_type"), "document_title": ptNullableStr(row, "document_title"),
			"error_code": ptNullableStr(row, "error_code"), "error_message": ptNullableStr(row, "error_message"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"orders": orders, "total": countRow.intOr("count", 0), "limit": limit, "offset": offset,
	})
}

// ---------- orders per kampanye (level order CAM-) ----------

// OrdersCampaignsList: GET /v1/platform/orders/campaigns?q=&limit=&offset=
// Daftar kampanye sebagai entitas order: Rencana = SUM harga media terorder
// (COALESCE(tcm.price, m.price)), fallback publikasi untuk kampanye legacy
// tanpa taut_campaign_media; Terpakai = SUM publications.client_price
// non-cancelled (paritas dengan Ringkasan klien totalCostIdr).
func (d *Deps) OrdersCampaignsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	search := strings.TrimSpace(q.Get("q"))
	limit := clampInt(q.Get("limit"), 25, 1, 200)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	where := ""
	params := []any{}
	if search != "" {
		like := "%" + search + "%"
		params = append(params, like, like, like)
		where = "WHERE c.order_code ILIKE $1 OR c.name ILIKE $2 OR b.name ILIKE $3"
	}
	from := `FROM campaigns c JOIN brands b ON b.id = c.brand_id ` + where
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count "+from, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	plannedSub := `CASE WHEN EXISTS (SELECT 1 FROM taut_campaign_media tcm WHERE tcm.campaign_id = c.id AND tcm.status = 'selected')
	     THEN (SELECT COALESCE(SUM(COALESCE(tcm2.price, m2.price, 0)), 0) FROM taut_campaign_media tcm2 JOIN media m2 ON m2.id = tcm2.media_id WHERE tcm2.campaign_id = c.id AND tcm2.status = 'selected')
	     ELSE (SELECT COALESCE(SUM(p2.client_price), 0) FROM publications p2 WHERE p2.campaign_id = c.id AND p2.status <> 'cancelled') END`
	selectColumns := `c.id, c.order_code, c.name, c.taut_status, c.status AS campaign_status, c.budget, c.created_at,
	                  b.name AS brand_name, b.public_id AS brand_public_id,
	                  (SELECT COUNT(DISTINCT tcm.media_id) FROM taut_campaign_media tcm WHERE tcm.campaign_id = c.id AND tcm.status = 'selected') AS media_count,
	                  (SELECT COUNT(*) FROM taut_campaign_media tcm WHERE tcm.campaign_id = c.id AND tcm.status = 'selected') AS article_count,
	                  (SELECT COUNT(*) FROM publications p WHERE p.campaign_id = c.id AND p.status <> 'cancelled') AS publication_count,
	                  (SELECT COUNT(*) FROM publications p WHERE p.campaign_id = c.id AND p.status = 'published') AS published_count,
	                  (SELECT COALESCE(SUM(p.client_price), 0) FROM publications p WHERE p.campaign_id = c.id AND p.status <> 'cancelled') AS realized_cost,
	                  ` + plannedSub + ` AS planned_cost`
	args := append([]any{}, params...)
	args = append(args, limit, offset)
	items, err := d.queryRows(r.Context(),
		"SELECT "+selectColumns+" "+from+" ORDER BY c.created_at DESC LIMIT $"+ptItoa(len(params)+1)+" OFFSET $"+ptItoa(len(params)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		out = append(out, map[string]any{
			"campaign_id": row.strOr("id", ""), "order_code": ptNullableStr(row, "order_code"),
			"name": row.strOr("name", ""), "taut_status": ptNullableStr(row, "taut_status"),
			"campaign_status": row.strOr("campaign_status", ""), "budget": int(row.numOr("budget")),
			"created_at":      row.anyOrNil("created_at"),
			"brand_name":      ptNullableStr(row, "brand_name"), "brand_public_id": ptNullableStr(row, "brand_public_id"),
			"media_count": int(row.numOr("media_count")), "article_count": int(row.numOr("article_count")),
			"publication_count": int(row.numOr("publication_count")),
			"published_count": int(row.numOr("published_count")),
			"planned_cost":    int(row.numOr("planned_cost")), "realized_cost": int(row.numOr("realized_cost")),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"campaigns": out, "total": countRow.intOr("count", 0), "limit": limit, "offset": offset,
	})
}

// OrdersCampaignDetail: GET /v1/platform/orders/campaigns/{campaignId}
func (d *Deps) OrdersCampaignDetail(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	campaignID := chiParam(r, "campaignId")
	camp, err := d.queryRow(r.Context(), `SELECT c.id, c.order_code, c.name, c.taut_status, c.status AS campaign_status, c.budget,
	       c.starts_at, c.ends_at, c.created_at, b.name AS brand_name, b.public_id AS brand_public_id
	    FROM campaigns c JOIN brands b ON b.id = c.brand_id WHERE c.id = $1 LIMIT 1`, campaignID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if camp == nil {
		writeErr(w, d, errNotFound("Campaign not found"))
		return
	}
	campRow := ptRow(ptNormalizeMap(camp))

	items, err := d.queryRows(r.Context(), `
		SELECT tcm.id::text AS tcm_id, tcm.media_id, m.name AS media_name, m.website_url,
		       tcm.category AS tcm_category, m.category AS media_category,
		       COALESCE(tcm.price, m.price, 0) AS price,
		       d.id AS document_id, d.title AS article_title, tcm.article_status,
		       pub.id AS publication_id, pub.status AS pub_status, pub.client_price AS pub_client_price,
		       pub.published_url, pub.published_at, pub.scheduled_at
		FROM taut_campaign_media tcm
		JOIN media m ON m.id = tcm.media_id
		LEFT JOIN content_documents d ON d.id = tcm.article_document_id
		LEFT JOIN publications pub ON pub.document_id = tcm.article_document_id AND pub.media_id = tcm.media_id
		WHERE tcm.campaign_id = $1
		ORDER BY tcm.match_rank ASC NULLS LAST, m.name ASC`, campaignID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	mode := "campaign_media"
	if len(items) == 0 {
		// Kampanye legacy (jalur lama) menyimpan medianya langsung di
		// publications tanpa baris taut_campaign_media — fallback grup per
		// publikasi, harga per baris = client_price publikasinya.
		mode = "publications_legacy"
		items, err = d.queryRows(r.Context(), `
			SELECT p.id AS publication_id, p.media_id, m.name AS media_name, m.website_url,
			       m.category AS media_category, COALESCE(p.client_price, 0) AS price,
			       p.document_id, d.title AS article_title, p.status AS article_status,
			       p.id AS publication_id, p.status AS pub_status, p.client_price AS pub_client_price,
			       p.published_url, p.published_at, p.scheduled_at
			FROM publications p
			JOIN media m ON m.id = p.media_id
			LEFT JOIN content_documents d ON d.id = p.document_id
			WHERE p.campaign_id = $1
			ORDER BY p.created_at ASC`, campaignID)
		if err != nil {
			writeErr(w, d, err)
			return
		}
	}

	monthly := make([]map[string]any, 0, len(items))
	seenMedia := map[string]int{}
	planned, realized, published, pubCount := 0, 0, 0, 0
	for _, raw := range items {
		row := ptRow(ptNormalizeMap(raw))
		// Pub join doc+media bisa mengembalikan beberapa revisi publikasi per
		// baris media — dedupe: baris yang sudah punya publikasi menang.
		key := row.strOr("tcm_id", "")
		if key == "" {
			key = row.strOr("media_id", "")
		}
		hasPub := row.strOr("publication_id", "") != ""
		if idx, ok := seenMedia[key]; ok {
			if hasPub && monthly[idx]["publication_id"] == "" {
				monthly[idx] = campaignMediaRow(row)
			}
			continue
		}
		seenMedia[key] = len(monthly)
		entry := campaignMediaRow(row)
		monthly = append(monthly, entry)

		price := int(row.numOr("price"))
		if price < 0 {
			price = 0
		}
		planned += price
		pubStatus := row.strOr("pub_status", "")
		if hasPub && pubStatus != "" && pubStatus != "cancelled" {
			pubCount++
			pubPrice := int(row.numOr("pub_client_price"))
			if pubPrice == 0 {
				pubPrice = price
			}
			realized += pubPrice
			if pubStatus == "published" {
				published++
			}
		}
	}
	budget := int(campRow.numOr("budget"))
	remaining := planned - realized
	if remaining < 0 {
		remaining = 0
	}
	if budget > 0 {
		remaining = budget - realized
		if remaining < 0 {
			remaining = 0
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"campaign": map[string]any{
			"id": campRow.strOr("id", ""), "order_code": ptNullableStr(campRow, "order_code"),
			"name": campRow.strOr("name", ""), "taut_status": ptNullableStr(campRow, "taut_status"),
			"campaign_status": campRow.strOr("campaign_status", ""), "brand_name": ptNullableStr(campRow, "brand_name"),
			"brand_public_id": ptNullableStr(campRow, "brand_public_id"), "budget": budget,
			"starts_at": campRow.anyOrNil("starts_at"), "ends_at": campRow.anyOrNil("ends_at"),
			"created_at": campRow.anyOrNil("created_at"),
		},
		"mode": mode,
		"media": monthly,
		"totals": map[string]any{
			"planned": planned, "realized": realized, "remaining": remaining, "budget": budget,
			"media_count": len(monthly), "publication_count": pubCount, "published_count": published,
		},
	})
}

func campaignMediaRow(row ptRow) map[string]any {
	category := row.strOr("tcm_category", "")
	if category == "" {
		category = row.strOr("media_category", "")
	}
	return map[string]any{
		"media_id": row.strOr("media_id", ""), "media_name": row.strOr("media_name", ""),
		"website_url": row.strOr("website_url", ""), "category": category,
		"price": int(row.numOr("price")), "document_id": ptNullableStr(row, "document_id"),
		"article_title": ptNullableStr(row, "article_title"), "article_status": row.strOr("article_status", ""),
		"publication_id": ptNullableStr(row, "publication_id"), "pub_status": row.strOr("pub_status", ""),
		"published_url": ptNullableStr(row, "published_url"), "published_at": row.anyOrNil("published_at"),
		"scheduled_at": row.anyOrNil("scheduled_at"),
	}
}

// ---------- revenue shares ----------

func revShareJSON(row ptRow) map[string]any {
	return map[string]any{
		"id": row.strOr("id", ""), "partner_id": ptNullableStr(row, "partner_id"),
		"media_id": ptNullableStr(row, "media_id"), "partner_name": ptNullableStr(row, "partner_name"),
		"media_name": ptNullableStr(row, "media_name"), "name": row.strOr("name", ""),
		"share_type": row.strOr("share_type", ""), "tulisio_percent": row.numOr("tulisio_percent"),
		"partner_percent": row.numOr("partner_percent"), "partner_fixed": int(row.numOr("partner_fixed")),
		"status": row.strOr("status", ""), "created_by": ptNullableStr(row, "created_by"),
		"created_at": row.anyOrNil("created_at"), "updated_at": row.anyOrNil("updated_at"),
	}
}

const revShareSelect = `SELECT r.*, p.name AS partner_name, m.name AS media_name
   FROM revenue_shares r
   LEFT JOIN partners p ON p.id = r.partner_id
   LEFT JOIN media m ON m.id = r.media_id`

func (d *Deps) revShareScoped(r *http.Request, ruleID string) (ptRow, error) {
	row, err := d.queryRow(r.Context(), revShareSelect+" WHERE r.id = $1 LIMIT 1", ruleID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Revenue share rule not found")
	}
	return ptRow(ptNormalizeMap(row)), nil
}

// RevenueSharesList: GET /v1/platform/revenue-shares
func (d *Deps) RevenueSharesList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && status != "active" && status != "archived" {
		writeErr(w, d, errUnprocessable("Invalid revenue share status filter"))
		return
	}
	limit := clampInt(q.Get("limit"), 100, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)
	clause := ""
	params := []any{}
	if status != "" {
		params = append(params, status)
		clause = "WHERE r.status = $1"
	}
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM revenue_shares r "+clause, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	args := append([]any{}, params...)
	args = append(args, limit, offset)
	items, err := d.queryRows(r.Context(),
		revShareSelect+" "+clause+" ORDER BY r.created_at DESC LIMIT $"+ptItoa(len(params)+1)+" OFFSET $"+ptItoa(len(params)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		out = append(out, revShareJSON(ptRow(ptNormalizeMap(raw))))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"revenue_shares": out, "total": countRow.intOr("count", 0), "limit": limit, "offset": offset,
	})
}

// RevenueShareCreate: POST /v1/platform/revenue-shares
func (d *Deps) RevenueShareCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	body := decodeJSON(r)
	name, _ := strField(body, "name")
	if len(name) < 2 || len(name) > 160 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	partnerID, hasPartner := strField(body, "partner_id")
	mediaID, hasMedia := strField(body, "media_id")
	shareType := "percentage"
	if s, hasShare := strField(body, "share_type"); hasShare && s != "" {
		if s != "percentage" && s != "fixed" {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		shareType = s
	}
	tulisioPercent := 75.0
	if v, hasV := numField(body, "tulisio_percent"); hasV {
		if v < 0 || v > 100 {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		tulisioPercent = v
	}
	partnerPercent := 25.0
	if v, hasV := numField(body, "partner_percent"); hasV {
		if v < 0 || v > 100 {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		partnerPercent = v
	}
	partnerFixed := 0.0
	if v, hasV := numField(body, "partner_fixed"); hasV {
		if v < 0 || v > 1_000_000_000 {
			writeErr(w, d, errUnprocessable("Payload tidak valid"))
			return
		}
		partnerFixed = v
	}
	if hasPartner && partnerID != "" {
		partner, pErr := d.queryRow(r.Context(), "SELECT id FROM partners WHERE id = $1 LIMIT 1", partnerID)
		if pErr != nil {
			writeErr(w, d, pErr)
			return
		}
		if partner == nil {
			writeErr(w, d, errNotFound("Partner not found"))
			return
		}
	}
	if hasMedia && mediaID != "" {
		media, mErr := d.queryRow(r.Context(), "SELECT id FROM media WHERE id = $1 LIMIT 1", mediaID)
		if mErr != nil {
			writeErr(w, d, mErr)
			return
		}
		if media == nil {
			writeErr(w, d, errNotFound("Media not found"))
			return
		}
	}
	id := uuid.NewString()
	if err := d.exec(r.Context(),
		"INSERT INTO revenue_shares (id, partner_id, media_id, name, share_type, tulisio_percent, partner_percent, partner_fixed, created_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		id, ptNullableArgStr(partnerID, hasPartner && partnerID != ""), ptNullableArgStr(mediaID, hasMedia && mediaID != ""),
		name, shareType, tulisioPercent, partnerPercent, partnerFixed, user.ID); err != nil {
		if mcIsDuplicate(err) {
			writeErr(w, d, errNotFound("Partner or media not found"))
			return
		}
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "finance.revenue_share.created", "revenue_share", id)
	created, err := d.revShareScoped(r, id)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusCreated, revShareJSON(created))
}

// RevenueShareGet: GET /v1/platform/revenue-shares/{ruleId}
func (d *Deps) RevenueShareGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	row, err := d.revShareScoped(r, chiParam(r, "ruleId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, revShareJSON(row))
}

// RevenueShareUpdate: PUT /v1/platform/revenue-shares/{ruleId}
func (d *Deps) RevenueShareUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	ruleID := chiParam(r, "ruleId")
	if _, err := d.revShareScoped(r, ruleID); err != nil {
		writeErr(w, d, err)
		return
	}
	body := decodeJSON(r)
	name, hasName := strField(body, "name")
	shareType, hasShareType := strField(body, "share_type")
	tulisioPercent, hasTulisio := numField(body, "tulisio_percent")
	partnerPercent, hasPartnerPercent := numField(body, "partner_percent")
	partnerFixed, hasPartnerFixed := numField(body, "partner_fixed")
	status, hasStatus := strField(body, "status")
	if hasName && (len(name) < 2 || len(name) > 160) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasShareType && shareType != "percentage" && shareType != "fixed" {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if (hasTulisio && (tulisioPercent < 0 || tulisioPercent > 100)) ||
		(hasPartnerPercent && (partnerPercent < 0 || partnerPercent > 100)) ||
		(hasPartnerFixed && (partnerFixed < 0 || partnerFixed > 1_000_000_000)) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasStatus && status != "active" && status != "archived" {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if err := d.exec(r.Context(),
		`UPDATE revenue_shares SET
		   name = COALESCE($1, name),
		   share_type = COALESCE($2, share_type),
		   tulisio_percent = COALESCE($3, tulisio_percent),
		   partner_percent = COALESCE($4, partner_percent),
		   partner_fixed = COALESCE($5, partner_fixed),
		   status = COALESCE($6, status),
		   updated_at = NOW()
		 WHERE id = $7`,
		ptNullableArgStr(name, hasName), ptNullableArgStr(shareType, hasShareType && shareType != ""),
		finNullableAnyFloat(tulisioPercent, hasTulisio), finNullableAnyFloat(partnerPercent, hasPartnerPercent),
		finNullableAnyFloat(partnerFixed, hasPartnerFixed), ptNullableArgStr(status, hasStatus && status != ""), ruleID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "finance.revenue_share.updated", "revenue_share", ruleID)
	updated, err := d.revShareScoped(r, ruleID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, revShareJSON(updated))
}

func finNullableAnyFloat(v float64, has bool) any {
	if !has {
		return nil
	}
	return v
}

// RevenueShareArchive: DELETE /v1/platform/revenue-shares/{ruleId} (soft).
func (d *Deps) RevenueShareArchive(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	ruleID := chiParam(r, "ruleId")
	if _, err := d.revShareScoped(r, ruleID); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(r.Context(), "UPDATE revenue_shares SET status = 'archived', updated_at = NOW() WHERE id = $1", ruleID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "finance.revenue_share.archived", "revenue_share", ruleID)
	updated, err := d.revShareScoped(r, ruleID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, revShareJSON(updated))
}

// ---------- settlements ----------

// finFeeCfg: biaya penarikan dari tc_config — dipakai jalur payout. Default
/// sama seperti partner/partner.go (bank Rp2.400, e-wallet 0,7%) bila baris
// config belum ada.
type finFeeCfg struct {
	bankIDR    float64
	ewalletPct float64
}

func (d *Deps) finFeeCfg(ctx context.Context) finFeeCfg {
	cfg := finFeeCfg{bankIDR: 2400, ewalletPct: 0.7}
	row, err := d.queryRow(ctx,
		"SELECT withdraw_bank_fee_idr, withdraw_ewallet_fee_percent FROM tc_config WHERE id = 1 LIMIT 1")
	if err != nil || row == nil {
		return cfg
	}
	m := ptRow(ptNormalizeMap(row))
	cfg.bankIDR = m.numOr("withdraw_bank_fee_idr")
	cfg.ewalletPct = m.numOr("withdraw_ewallet_fee_percent")
	return cfg
}

// settPayout: jumlah yang benar-benar harus ditransfer ke mitra. Baris yang
// mengandung net_amount_idr (dibuat mitra / admin versi baru) dipakai apa
// adanya; baris lama buatan admin (belum ada fee/net) dihitung runtime —
// aturannya identik partner/finance.go: bank = fee flat, ewallet = persen.
func settPayout(row ptRow, cfg finFeeCfg) (method string, fee, gross, net float64) {
	gross = row.numOr("total_amount")
	method = row.strOr("payout_method", "bank")
	if f, ok := row.num("net_amount_idr"); ok {
		// net tersimpan (jalur mitra / admin pasca-fix): pakai apa adanya.
		net = f
		fee = row.numOr("transfer_fee_idr")
		return method, fee, gross, round2(net)
	}
	if method == "ewallet" {
		fee = round2(gross * cfg.ewalletPct / 100)
	} else {
		fee = cfg.bankIDR
	}
	net = round2(gross - fee)
	if net < 0 {
		net = 0
	}
	return method, fee, gross, net
}

func settJSON(row ptRow, cfg finFeeCfg) map[string]any {
	method, fee, gross, net := settPayout(row, cfg)
	return map[string]any{
		"id": row.strOr("id", ""), "partner_id": row.strOr("partner_id", ""),
		"partner_name": ptNullableStr(row, "partner_name"), "period_start": row.anyOrNil("period_start"),
		"period_end": row.anyOrNil("period_end"), "status": row.strOr("status", ""),
		"total_amount": gross, "notes": ptNullableStr(row, "notes"),
		"payout_method": method, "transfer_fee_idr": fee, "net_amount_idr": net,
		"invoice_number":  ptNullableStr(row, "invoice_number"),
		"bank_name":       ptNullableStr(row, "bank_name"),
		"bank_account_number": ptNullableStr(row, "bank_account_number"),
		"bank_account_name":   ptNullableStr(row, "bank_account_name"),
		"receipt_url": ptNullableStr(row, "receipt_url"), "payment_reference": ptNullableStr(row, "payment_reference"),
		"rejection_reason": ptNullableStr(row, "rejection_reason"), "paid_at": row.anyOrNil("paid_at"),
		"created_by": ptNullableStr(row, "created_by"),
		"created_at": row.anyOrNil("created_at"), "updated_at": row.anyOrNil("updated_at"),
	}
}

func (d *Deps) settScoped(r *http.Request, settlementID string) (ptRow, error) {
	row, err := d.queryRow(r.Context(),
		"SELECT s.*, p.name AS partner_name, p.bank_name, p.bank_account_number, p.bank_account_name, p.payout_type AS partner_payout_type "+
			"FROM settlements s LEFT JOIN partners p ON p.id = s.partner_id WHERE s.id = $1 LIMIT 1", settlementID)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, errNotFound("Settlement not found")
	}
	return ptRow(ptNormalizeMap(row)), nil
}

// SettlementsList: GET /v1/platform/settlements
func (d *Deps) SettlementsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "settlements")
	if !ok {
		return
	}
	q := r.URL.Query()
	partnerID := strings.TrimSpace(q.Get("partner_id"))
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && !ptContains([]string{"pending", "approved", "processing", "paid", "failed"}, status) {
		writeErr(w, d, errUnprocessable("Invalid settlement status filter"))
		return
	}
	search := strings.TrimSpace(q.Get("q"))
	if len(search) > 120 {
		search = search[:120]
	}
	limit := clampInt(q.Get("limit"), 100, 1, 500)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	where := []string{}
	params := []any{}
	if partnerID != "" {
		params = append(params, partnerID)
		where = append(where, "s.partner_id = $"+ptItoa(len(params)))
	}
	if status != "" {
		params = append(params, status)
		where = append(where, "s.status = $"+ptItoa(len(params)))
	}
	if search != "" {
		params = append(params, "%"+search+"%")
		where = append(where, "p.name ILIKE $"+ptItoa(len(params)))
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	countRow, err := d.queryRow(r.Context(),
		"SELECT COUNT(*) AS count FROM settlements s LEFT JOIN partners p ON p.id = s.partner_id "+clause, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	args := append([]any{}, params...)
	args = append(args, limit, offset)
	items, err := d.queryRows(r.Context(),
		"SELECT s.*, p.name AS partner_name, p.bank_name, p.bank_account_number, p.bank_account_name, p.payout_type AS partner_payout_type "+
			"FROM settlements s LEFT JOIN partners p ON p.id = s.partner_id "+
			clause+" ORDER BY s.created_at DESC LIMIT $"+ptItoa(len(params)+1)+" OFFSET $"+ptItoa(len(params)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	feeCfg := d.finFeeCfg(r.Context())
	out := make([]map[string]any, 0, len(items))
	for _, raw := range items {
		out = append(out, settJSON(ptRow(ptNormalizeMap(raw)), feeCfg))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"settlements": out, "total": countRow.intOr("count", 0), "limit": limit, "offset": offset,
	})
}

// SettlementCreate: POST /v1/platform/settlements
func (d *Deps) SettlementCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "settlements")
	if !ok {
		return
	}
	body := decodeJSON(r)
	partnerID, _ := strField(body, "partner_id")
	periodStart, hasStart := strField(body, "period_start")
	periodEnd, hasEnd := strField(body, "period_end")
	if partnerID == "" || !hasStart || !hasEnd || periodStart == "" || periodEnd == "" {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	notes, hasNotes := strField(body, "notes")
	if hasNotes && len(notes) > 2000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	partner, err := d.queryRow(r.Context(), "SELECT id FROM partners WHERE id = $1 LIMIT 1", partnerID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if partner == nil {
		writeErr(w, d, errNotFound("Partner not found"))
		return
	}
	start, startErr := time.Parse(time.RFC3339, periodStart)
	if startErr != nil {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	end, endErr := time.Parse(time.RFC3339, periodEnd)
	if endErr != nil {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	created, err := d.finCreateSettlement(r, partnerID, start, end, ptNullableArgStr(notes, hasNotes && notes != ""), user.ID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "finance.settlement.created", "settlement", created.strOr("id", ""))
	settlement, err := d.settScoped(r, created.strOr("id", ""))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusCreated, settJSON(settlement, d.finFeeCfg(r.Context())))
}

// finCreateSettlement: port finCreateSettlement (locks pending earnings in period).
// Konsisten jalur mitra: fee transfer & net dihitung dari payout_type partner
// (bank = flat, ewallet = persen) sehingga Total yang dilihat admin sama
// dengan yang dilihat mitra di dashboard-nya.
func (d *Deps) finCreateSettlement(r *http.Request, partnerID string, start, end time.Time, notes any, userID string) (ptRow, error) {
	earnings, err := d.queryRows(r.Context(),
		"SELECT id, amount FROM partner_earnings WHERE partner_id = $1 AND status = 'pending' AND settlement_id IS NULL AND created_at >= $2 AND created_at < $3 AND created_at <= "+dbpkg.PartnerEarningHoldCutoffSQL,
		partnerID, start, end.Add(24*time.Hour))
	if err != nil {
		return nil, err
	}
	total := 0.0
	for _, row := range earnings {
		total += row.numOr("amount")
	}
	total = round2(total)
	id := uuid.NewString()

	// Metode + fee + net mengikuti pengaturan mitra & tc_config (aturan sama
	// dengan PartnerPost /finance.go).
	pool, _ := d.queryRow(r.Context(),
		"SELECT payout_type FROM partners WHERE id = $1 LIMIT 1", partnerID)
	cfg := d.finFeeCfg(r.Context())
	payoutMethod := "bank"
	fee := cfg.bankIDR
	if pool != nil && strings.EqualFold(ptRow(ptNormalizeMap(pool)).strOr("payout_type", "bank"), "ewallet") {
		payoutMethod = "ewallet"
		fee = round2(total * cfg.ewalletPct / 100)
	}
	net := round2(total - fee)
	if net < 0 {
		net = 0
	}
	// Nomor invoice — format sama dengan jalur mitra supaya riwayatnya seragam.
	invCount, _ := d.queryRow(r.Context(),
		"SELECT COUNT(*) AS count FROM settlements WHERE partner_id = $1 AND invoice_number IS NOT NULL", partnerID)
	invoiceNumber := fmt.Sprintf("INV/%s/%s", partnerID, ptPadLeft(ptItoa(invCount.intOr("count", 0)+1), 3))

	if err := d.exec(r.Context(),
		"INSERT INTO settlements (id, partner_id, period_start, period_end, status, total_amount, payout_method, transfer_fee_idr, net_amount_idr, invoice_number, notes, created_by) VALUES ($1, $2, $3, $4, 'pending', $5, $6, $7, $8, $9, $10, $11)",
		id, partnerID, start, end, total, payoutMethod, fee, net, invoiceNumber, notes, userID); err != nil {
		return nil, err
	}
	if len(earnings) > 0 {
		ids := make([]any, 0, len(earnings)+1)
		ids = append(ids, id)
		placeholders := make([]string, 0, len(earnings))
		for i, row := range earnings {
			ids = append(ids, row.strOr("id", ""))
			placeholders = append(placeholders, "$"+ptItoa(i+2))
		}
		if err := d.exec(r.Context(),
			"UPDATE partner_earnings SET status = 'settled', settlement_id = $1 WHERE id IN ("+strings.Join(placeholders, ",")+")", ids...); err != nil {
			return nil, err
		}
	}
	created, err := d.queryRow(r.Context(), "SELECT * FROM settlements WHERE id = $1 LIMIT 1", id)
	if err != nil || created == nil {
		return nil, err
	}
	return ptRow(ptNormalizeMap(created)), nil
}

// settTransitions: parity SETTLEMENT_TRANSITIONS.
var settTransitions = map[string][]string{
	"pending": {"approved", "failed"},
	// approved -> paid langsung: alur admin "Setujui" membuka popup kirim bukti
	// dan penyelesaiannya menandai paid tanpa mampir ke 'processing'.
	"approved":   {"processing", "paid", "failed"},
	"processing": {"paid", "failed"},
	"paid":       {},
	// failed bersifat final: earning-nya sudah dilepas ke saldo mitra, jadi
	// dibuka kembali = klaim ganda atas dana yang sama. Mitra mengajukan baru.
	"failed": {},
}

// finTransitionSettlement: port finTransitionSettlement (books payout on paid).
//
// Seluruh transisi berjalan dalam satu transaksi dengan baris settlement
// dikunci (FOR UPDATE): status ditulis hanya bila masih sama dengan status
// yang dibaca (cek rows-affected), payout hanya dibukukan bila transisi ke
// paid benar-benar terjadi, dan transisi ke status yang sama adalah no-op.
func (d *Deps) finTransitionSettlement(r *http.Request, settlementID, to, userID string) (ptRow, error) {
	ctx := r.Context()
	txErr := d.ptWithTx(ctx, func(tx *ptTx) error {
		rows, err := tx.queryRows("SELECT * FROM settlements WHERE id = $1 LIMIT 1 FOR UPDATE", settlementID)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return errConflict("Settlement not found")
		}
		settlement := rows[0]
		current := settlement.strOr("status", "")
		if current == to {
			return nil // no-op: tidak ada payout/pelepasan earning kedua
		}
		if !ptContains(settTransitions[current], to) {
			return errConflict(fmt.Sprintf("Settlement cannot transition from %s to %s", current, to))
		}
		if to == "paid" {
			payout := settlement.numOr("total_amount")
			// Earning yang terlampir wajib menutup jumlah yang dibayarkan; kalau
			// sudah dilepas/diklaim ulang, jangan bayar dari saldo kosong.
			sumRows, err := tx.queryRows(
				"SELECT COALESCE(SUM(amount), 0) AS total FROM partner_earnings WHERE settlement_id = $1", settlementID)
			if err != nil {
				return err
			}
			attached := 0.0
			if len(sumRows) > 0 {
				attached = sumRows[0].numOr("total")
			}
			if round2(attached) < round2(payout) {
				return errConflict("Earning yang terlampir pada settlement tidak menutup jumlah pembayaran; payout ditolak.")
			}
			// Baris lama buatan admin belum bere fee/net: isikan sekarang (aturan
			// sama dengan jalur mitra) supaya jumlah payout = jumlah yang
			// diterima mitra (setelah potongan transfer).
			if settlement.raw("net_amount_idr") == nil {
				cfg := d.finFeeCfg(ctx)
				method := settlement.strOr("payout_method", "bank")
				fee := cfg.bankIDR
				if strings.EqualFold(method, "ewallet") {
					fee = round2(payout * cfg.ewalletPct / 100)
				}
				net := round2(payout - fee)
				if net < 0 {
					net = 0
				}
				if err := tx.exec(
					"UPDATE settlements SET payout_method = $2, transfer_fee_idr = $3, net_amount_idr = $4 WHERE id = $1",
					settlementID, method, fee, net); err != nil {
					return err
				}
				payout = net
			} else {
				payout = settlement.numOr("net_amount_idr")
			}
			tag, err := tx.tx.Exec(ctx,
				"UPDATE settlements SET status = 'paid', paid_at = NOW(), updated_at = NOW() WHERE id = $1 AND status = $2", settlementID, current)
			if err != nil {
				return err
			}
			if tag.RowsAffected() != 1 {
				return errConflict("Settlement status changed concurrently")
			}
			if payout > 0 {
				if err := tx.exec(
					"INSERT INTO transactions (id, partner_id, amount, type, created_by) VALUES ($1, $2, $3, 'settlement.payout', $4)",
					uuid.NewString(), settlement.strOr("partner_id", ""), -payout, userID); err != nil {
					return err
				}
			}
			return nil
		}
		tag, err := tx.tx.Exec(ctx,
			"UPDATE settlements SET status = $1, updated_at = NOW() WHERE id = $2 AND status = $3", to, settlementID, current)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return errConflict("Settlement status changed concurrently")
		}
		if to == "failed" {
			if err := tx.exec(
				"UPDATE partner_earnings SET status = 'pending', settlement_id = NULL WHERE settlement_id = $1", settlementID); err != nil {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	updated, err := d.queryRow(ctx, "SELECT * FROM settlements WHERE id = $1 LIMIT 1", settlementID)
	if err != nil || updated == nil {
		return nil, err
	}
	return ptRow(ptNormalizeMap(updated)), nil
}

// SettlementGet: GET /v1/platform/settlements/{settlementId}
func (d *Deps) SettlementGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "settlements")
	if !ok {
		return
	}
	row, err := d.settScoped(r, chiParam(r, "settlementId"))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, settJSON(row, d.finFeeCfg(r.Context())))
}

// SettlementUpdate: PUT /v1/platform/settlements/{settlementId}
func (d *Deps) SettlementUpdate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "settlements")
	if !ok {
		return
	}
	settlementID := chiParam(r, "settlementId")
	settlement, err := d.settScoped(r, settlementID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	body := decodeJSON(r)
	status, hasStatus := strField(body, "status")
	notes, hasNotes := strField(body, "notes")
	receiptURL, hasReceipt := strField(body, "receipt_url")
	paymentRef, hasPaymentRef := strField(body, "payment_reference")
	rejectionReason, hasRejection := strField(body, "rejection_reason")
	if hasStatus && !ptContains([]string{"pending", "approved", "processing", "paid", "failed"}, status) {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasNotes && len(notes) > 2000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasReceipt && len(receiptURL) > 500 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasPaymentRef && len(paymentRef) > 200 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasRejection && len(rejectionReason) > 1000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if hasStatus && status != "" {
		if status == "failed" && rejectionReason == "" {
			writeErr(w, d, errUnprocessable("Alasan penolakan wajib diisi saat menolak settlement."))
			return
		}
		if _, tErr := d.finTransitionSettlement(r, settlementID, status, user.ID); tErr != nil {
			writeErr(w, d, tErr)
			return
		}
		d.audit(r.Context(), user.ID, "finance.settlement."+status, "settlement", settlementID)
		if hasNotes && notes != "" {
			_ = d.exec(r.Context(), "UPDATE settlements SET notes = $1, updated_at = NOW() WHERE id = $2", notes, settlementID)
		}
		if hasReceipt && receiptURL != "" {
			_ = d.exec(r.Context(), "UPDATE settlements SET receipt_url = $1, updated_at = NOW() WHERE id = $2", receiptURL, settlementID)
		}
		if hasPaymentRef && paymentRef != "" {
			_ = d.exec(r.Context(), "UPDATE settlements SET payment_reference = $1, updated_at = NOW() WHERE id = $2", paymentRef, settlementID)
		}
		if hasRejection && rejectionReason != "" {
			_ = d.exec(r.Context(), "UPDATE settlements SET rejection_reason = $1, updated_at = NOW() WHERE id = $2", rejectionReason, settlementID)
		}
		d.settNotifyPartner(r.Context(), settlement, status, rejectionReason)
		updated, uErr := d.settScoped(r, settlementID)
		if uErr != nil {
			writeErr(w, d, uErr)
			return
		}
		writeJSON(w, http.StatusOK, settJSON(updated, d.finFeeCfg(r.Context())))
		return
	}
	if hasNotes && notes != "" {
		_ = d.exec(r.Context(), "UPDATE settlements SET notes = $1, updated_at = NOW() WHERE id = $2", notes, settlementID)
	}
	if hasReceipt && receiptURL != "" {
		_ = d.exec(r.Context(), "UPDATE settlements SET receipt_url = $1, updated_at = NOW() WHERE id = $2", receiptURL, settlementID)
	}
	if hasPaymentRef && paymentRef != "" {
		_ = d.exec(r.Context(), "UPDATE settlements SET payment_reference = $1, updated_at = NOW() WHERE id = $2", paymentRef, settlementID)
	}
	if hasRejection && rejectionReason != "" {
		_ = d.exec(r.Context(), "UPDATE settlements SET rejection_reason = $1, updated_at = NOW() WHERE id = $2", rejectionReason, settlementID)
	}
	updated, uErr := d.settScoped(r, settlementID)
	if uErr != nil {
		writeErr(w, d, uErr)
		return
	}
	writeJSON(w, http.StatusOK, settJSON(updated, d.finFeeCfg(r.Context())))
}

// settNotifyPartner: notifikasi withdrawal.* ke user mitra (best-effort).
func (d *Deps) settNotifyPartner(ctx context.Context, settlement ptRow, to, rejectionReason string) {
	partnerID := settlement.strOr("partner_id", "")
	if partnerID == "" {
		return
	}
	amount := "Rp " + finFormatThousands(int64(settlement.numOr("total_amount")))
	partner, err := d.queryRow(ctx,
		"SELECT user_id, public_id, (SELECT email FROM users WHERE id = user_id) AS email FROM partners WHERE id = $1 LIMIT 1", partnerID)
	if err != nil || partner == nil {
		return
	}
	partnerPublicID := partner.strOr("public_id", partnerID)
	link := "/mitra/" + partnerPublicID + "/settlements"
	var notifType, title, body string
	switch to {
	case "paid":
		notifType, title = "withdrawal.paid", "Withdrawal dibayarkan"
		body = "Penarikan dana " + amount + " telah dibayarkan oleh Admin."
	case "processing":
		notifType, title = "withdrawal.processing", "Withdrawal sedang diproses"
		body = "Penarikan dana " + amount + " sedang diproses oleh Admin."
	case "failed":
		notifType, title = "withdrawal.rejected", "Withdrawal ditolak"
		body = "Penarikan dana " + amount + " ditolak."
		if rejectionReason != "" {
			body += " Alasan: " + rejectionReason
		}
		body += " Saldo telah dikembalikan."
	default:
		return
	}
	userID := partner.strOr("user_id", "")
	if userID != "" {
		d.finNotifyUser(ctx, userID, notifType, title, body, link, notifType+":"+settlement.strOr("id", ""))
		// WA ke mitra memakai template audiens mitra. Tanpa nomor/izin, atau
		// saat template dinonaktifkan admin, pengiriman dilewati tanpa error.
		statusText := ""
		if to == "failed" {
			statusText = rejectionReason
		}
		notify.EnqueueUserWA(ctx, d.Pool, notify.UserWAOptions{
			Audience:    notify.AudiencePartner,
			Event:       notifType,
			UserID:      userID,
			ReferenceID: settlement.strOr("id", ""),
			Data:        map[string]string{"nama": "", "jumlah": amount, "status": statusText},
			Link:        link,
			BaseURL:     notify.BaseURL(),
		})
	}
}

// finNotifyUser: INSERT notifications (dedupe_key opsional).
func (d *Deps) finNotifyUser(ctx context.Context, userID, notifType, title, body, link, dedupeKey string) {
	if err := d.exec(ctx,
		"INSERT INTO notifications (id, user_id, type, title, body, link, dedupe_key) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (user_id, dedupe_key) DO NOTHING",
		uuid.NewString(), userID, notifType, title, body, link, nullIfEmpty(dedupeKey)); err != nil {
		// best-effort
	}
}

func finFormatThousands(n int64) string {
	negative := n < 0
	if negative {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	out := make([]string, 0, len(s)+len(s)/3)
	count := 0
	for i := len(s) - 1; i >= 0; i-- {
		out = append(out, string(s[i]))
		count++
		if count%3 == 0 && i > 0 {
			out = append(out, ".")
		}
	}
	result := ""
	for i := len(out) - 1; i >= 0; i-- {
		result += out[i]
	}
	if negative {
		result = "-" + result
	}
	return result
}

// SettlementConfigGet: GET /v1/platform/settlements/config
func (d *Deps) SettlementConfigGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "settlements")
	if !ok {
		return
	}
	cfg := d.finTCConfig(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"minimum_withdraw_idr": cfg.MinimumWithdrawIDR})
}

// SettlementConfigPut: PUT /v1/platform/settlements/config
func (d *Deps) SettlementConfigPut(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "settlements")
	if !ok {
		return
	}
	body := decodeJSON(r)
	minimum, hasMinimum := intField(body, "minimum_withdraw_idr")
	if !hasMinimum || minimum < 1 || minimum > 100_000_000 {
		writeErr(w, d, errUnprocessable("Payload tidak valid"))
		return
	}
	if err := d.exec(r.Context(), "UPDATE tc_config SET minimum_withdraw_idr = $1, updated_by = $2, updated_at = NOW() WHERE id = 1", minimum, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "finance.withdraw.minimum_changed", "config", "tc_config")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "minimum_withdraw_idr": minimum})
}

// SettlementVoid: DELETE /v1/platform/settlements/{settlementId}
func (d *Deps) SettlementVoid(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "settlements")
	if !ok {
		return
	}
	settlementID := chiParam(r, "settlementId")
	settlement, err := d.settScoped(r, settlementID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if settlement.strOr("status", "") == "paid" {
		writeErr(w, d, errConflict("Paid settlements cannot be deleted"))
		return
	}
	updated, tErr := d.finTransitionSettlement(r, settlementID, "failed", user.ID)
	if tErr != nil {
		writeErr(w, d, tErr)
		return
	}
	d.audit(r.Context(), user.ID, "finance.settlement.voided", "settlement", settlementID)
	refreshed, err := d.settScoped(r, updated.strOr("id", settlementID))
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, settJSON(refreshed, d.finFeeCfg(r.Context())))
}

// SettlementReceiptUpload: POST /v1/platform/settlements/{settlementId}/receipt
// (multipart/form-data; file PDF/JPG/PNG maks 10 MB).
func (d *Deps) SettlementReceiptUpload(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "settlements")
	if !ok {
		return
	}
	settlementID := chiParam(r, "settlementId")
	if _, err := d.settScoped(r, settlementID); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "File tidak ditemukan."})
		return
	}
	file, header, fErr := r.FormFile("file")
	if fErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "File tidak ditemukan."})
		return
	}
	defer file.Close()
	raw, readErr := finReadAllLimited(file, 10<<20)
	if readErr != nil {
		writeErr(w, d, errUnprocessable("Ukuran file maksimal 10 MB."))
		return
	}
	contentType := header.Header.Get("Content-Type")
	lowerName := strings.ToLower(header.Filename)
	isPDF := contentType == "application/pdf" || strings.HasSuffix(lowerName, ".pdf")
	isImage := strings.HasPrefix(contentType, "image/")
	if !isPDF && !isImage {
		writeErr(w, d, errUnprocessable("Format file harus berupa PDF, JPG, atau PNG."))
		return
	}
	if len(raw) > 10*1024*1024 {
		writeErr(w, d, errUnprocessable("Ukuran file maksimal 10 MB."))
		return
	}
	// Gambar dinormalisasi ke WebP (parity app lama); PDF apa adanya.
	payload := raw
	ext := "jpg"
	if isPDF {
		ext = "pdf"
	} else if converted, ok := imageconvert.ConvertToWebP(raw, contentType); ok {
		payload = converted
		ext = "webp"
	} else if idx := strings.LastIndex(lowerName, "."); idx >= 0 {
		candidate := finNonAlnumRe.ReplaceAllString(lowerName[idx+1:], "")
		if candidate != "" {
			ext = candidate
		}
	}
	storageKey := uuid.NewString() + "." + ext
	receiptURL := "/uploads/" + storageKey
	if err := d.finSaveUploadFile(storageKey, payload); err != nil {
		writeErr(w, d, err)
		return
	}
	if err := d.exec(r.Context(), "UPDATE settlements SET receipt_url = $1, updated_at = NOW() WHERE id = $2", receiptURL, settlementID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.audit(r.Context(), user.ID, "finance.settlement.receipt_uploaded", "settlement", settlementID)
	updated, err := d.settScoped(r, settlementID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusCreated, settJSON(updated, d.finFeeCfg(r.Context())))
}

// ---------- referrals / referred-domains / vip-partners ----------

// ReferralsList: GET /v1/platform/referrals
func (d *Deps) ReferralsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	commissions, err := d.queryRows(r.Context(),
		`SELECT c.id, p1.name AS referrer, u1.email AS referrer_email, p2.name AS referee, u2.email AS referee_email, c.amount, c.rate_bps, c.status, c.created_at
		 FROM partner_referral_commissions c
		 JOIN partners p1 ON p1.id = c.referrer_partner_id
		 JOIN partners p2 ON p2.id = c.referee_partner_id
		 JOIN users u1 ON u1.id = p1.user_id
		 JOIN users u2 ON u2.id = p2.user_id
		 ORDER BY c.created_at DESC LIMIT 500`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"commissions": ptRows(commissions)})
}

// ReferralAuditList: GET /v1/platform/referral-audit
func (d *Deps) ReferralAuditList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	orderFilter := strings.TrimSpace(q.Get("orders"))
	if orderFilter != "" && orderFilter != "with" && orderFilter != "without" {
		writeErr(w, d, errUnprocessable("orders harus with atau without"))
		return
	}
	domainStatus := strings.TrimSpace(q.Get("domain_status"))
	if domainStatus != "" && !ptContains([]string{"pending", "approved", "rejected"}, domainStatus) {
		writeErr(w, d, errUnprocessable("domain_status harus pending, approved, atau rejected"))
		return
	}
	referrers, err := d.queryRows(r.Context(),
		`SELECT
		   p.id AS partner_id, p.name AS name, u.email AS email,
		   p.referral_code AS referral_code, p.vip_tier, p.is_vip,
		   COALESCE((SELECT SUM(rc.click_count) FROM referral_clicks rc WHERE rc.referrer_partner_id = p.id), 0)::int AS clicks,
		   (SELECT COUNT(*) FROM referral_attributions ra WHERE ra.referrer_partner_id = p.id)::int AS signups,
		   (SELECT COUNT(*) FROM referral_attributions ra JOIN users ru ON ru.id = ra.referee_user_id WHERE ra.referrer_partner_id = p.id AND ru.role = 'partner')::int AS mitra_count,
		   (SELECT COUNT(*) FROM referral_attributions ra JOIN users ru ON ru.id = ra.referee_user_id WHERE ra.referrer_partner_id = p.id AND ru.role = 'client')::int AS klien_count,
		   COALESCE((SELECT SUM(c.amount) FROM partner_referral_commissions c WHERE c.referrer_partner_id = p.id), 0)::numeric(14,2) AS commission_total,
		   COALESCE((SELECT COUNT(*) FROM referred_domains rd WHERE rd.referrer_partner_id = p.id), 0)::int AS domains_joined,
		   COALESCE((SELECT COUNT(*) FROM referred_domains rd
		              WHERE rd.referrer_partner_id = p.id AND EXISTS (
		                SELECT 1 FROM publications pub WHERE pub.media_id = rd.media_id AND pub.status = 'published'
		              )), 0)::int AS domains_with_orders,
		   COALESCE((SELECT COUNT(*) FROM publications pub
		              WHERE pub.media_id IN (
		                SELECT rd2.media_id FROM referred_domains rd2
		                 WHERE rd2.referrer_partner_id = p.id AND rd2.media_id IS NOT NULL
		              ) AND pub.status = 'published'), 0)::int AS orders_published_total,
		   COALESCE((SELECT SUM(pub.client_price) FROM publications pub
		              WHERE pub.media_id IN (
		                SELECT rd2.media_id FROM referred_domains rd2
		                 WHERE rd2.referrer_partner_id = p.id AND rd2.media_id IS NOT NULL
		              ) AND pub.status = 'published'), 0)::numeric(14,2) AS gmv_published_total
		 FROM partners p
		 JOIN users u ON u.id = p.user_id
		 WHERE p.status = 'active'
		   AND p.is_vip = false
		   AND p.vip_tier NOT IN ('brand_ambassador', 'vip_reguler')
		 ORDER BY commission_total DESC, signups DESC, p.name ASC`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	attributions, err := d.queryRows(r.Context(),
		`SELECT
		   ra.id, ra.code,
		   p.name AS referrer_name, p.id AS referrer_partner_id,
		   u.email AS referee_email, u.display_name AS referee_name, u.role AS referee_role,
		   ra.ip::text AS ip, ra.notes AS fraud_note, ra.created_at
		 FROM referral_attributions ra
		 JOIN partners p ON p.id = ra.referrer_partner_id
		 LEFT JOIN users u ON u.id = ra.referee_user_id
		 ORDER BY ra.created_at DESC LIMIT 500`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	domainFilters := []string{}
	domainValues := []any{}
	if domainStatus != "" {
		domainValues = append(domainValues, domainStatus)
		domainFilters = append(domainFilters, "rd.status = $"+ptItoa(len(domainValues)))
	}
	if orderFilter == "with" {
		domainFilters = append(domainFilters, "EXISTS (SELECT 1 FROM publications p2 WHERE p2.media_id = rd.media_id AND p2.status = 'published')")
	} else if orderFilter == "without" {
		domainFilters = append(domainFilters, "NOT EXISTS (SELECT 1 FROM publications p2 WHERE p2.media_id = rd.media_id AND p2.status = 'published')")
	}
	domainWhere := ""
	if len(domainFilters) > 0 {
		domainWhere = "WHERE " + strings.Join(domainFilters, " AND ")
	}
	domainCount, err := d.queryRow(r.Context(),
		"SELECT COUNT(*)::int AS total FROM referred_domains rd "+domainWhere, domainValues...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	domains, err := d.queryRows(r.Context(),
		`SELECT
		   rd.id, rd.domain_name, rd.status, rd.rejection_reason, rd.is_tld,
		   rd.cpa_paid, rd.cpa_amount, rd.created_at,
		   p.name AS referrer_name, p.id AS referrer_partner_id, p.vip_tier AS referrer_vip_tier, p.is_vip AS referrer_is_vip,
		   u.email AS owner_email, u.display_name AS owner_name,
		   m.name AS media_name, m.website_url AS media_website_url,
		   COALESCE((SELECT COUNT(*) FROM publications pub WHERE pub.media_id = rd.media_id), 0)::int AS orders_total,
		   COALESCE((SELECT COUNT(*) FROM publications pub WHERE pub.media_id = rd.media_id AND pub.status = 'published'), 0)::int AS orders_published,
		   COALESCE((SELECT SUM(pub.client_price) FROM publications pub
		              WHERE pub.media_id = rd.media_id AND pub.status = 'published'), 0)::numeric(14,2) AS gmv_published,
		   (SELECT MAX(pub.published_at) FROM publications pub
		     WHERE pub.media_id = rd.media_id AND pub.status = 'published') AS last_order_at
		 FROM referred_domains rd
		 LEFT JOIN partners p ON p.id = rd.referrer_partner_id
		 LEFT JOIN users u ON u.id = rd.owner_user_id
		 LEFT JOIN media m ON m.id = rd.media_id
		 `+domainWhere+`
		 ORDER BY rd.created_at DESC LIMIT 500`, domainValues...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"referrers": ptRows(referrers), "attributions": ptRows(attributions),
		"domains": ptRows(domains), "domains_total": domainCount.intOr("total", 0),
	})
}

// ReferredDomainsList: GET /v1/platform/referred-domains
func (d *Deps) ReferredDomainsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	q := r.URL.Query()
	status := strings.TrimSpace(q.Get("status"))
	where := []string{}
	params := []any{}
	if status != "" && ptContains([]string{"pending", "approved", "rejected"}, status) {
		params = append(params, status)
		where = append(where, "rd.status = $"+ptItoa(len(params)))
	}
	clause := ""
	if len(where) > 0 {
		clause = "WHERE " + strings.Join(where, " AND ")
	}
	limit := clampInt(q.Get("limit"), 50, 1, 200)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)
	countRow, err := d.queryRow(r.Context(), "SELECT COUNT(*) AS count FROM referred_domains rd "+clause, params...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	args := append([]any{}, params...)
	args = append(args, limit, offset)
	items, err := d.queryRows(r.Context(),
		`SELECT rd.*, p.name AS referrer_name, u.email AS owner_email FROM referred_domains rd
		 LEFT JOIN partners p ON p.id = rd.referrer_partner_id
		 LEFT JOIN users u ON u.id = rd.owner_user_id
		 `+clause+` ORDER BY rd.created_at DESC LIMIT $`+ptItoa(len(params)+1)+` OFFSET $`+ptItoa(len(params)+2), args...)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": ptRows(items), "total": countRow.intOr("count", 0), "limit": limit, "offset": offset,
	})
}

// ReferredDomainsReview: PUT /v1/platform/referred-domains
func (d *Deps) ReferredDomainsReview(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	id, _ := strField(body, "id")
	status, _ := strField(body, "status")
	if id == "" || (status != "approved" && status != "rejected") {
		writeErr(w, d, errUnprocessable("id dan status approved|rejected wajib"))
		return
	}
	reason, hasReason := strField(body, "rejection_reason")
	if status == "rejected" && (!hasReason || len(strings.TrimSpace(reason)) < 5) {
		writeErr(w, d, errUnprocessable("Alasan penolakan minimal 5 karakter"))
		return
	}
	existing, err := d.queryRow(r.Context(), "SELECT id, status FROM referred_domains WHERE id = $1 LIMIT 1", id)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if existing == nil {
		writeErr(w, d, errNotFound("Domain tidak ditemukan"))
		return
	}
	rejectionArg := any(nil)
	if status == "rejected" {
		rejectionArg = reason
	}
	if err := d.exec(r.Context(),
		"UPDATE referred_domains SET status = $1, rejection_reason = $2, updated_at = NOW() WHERE id = $3",
		status, rejectionArg, id); err != nil {
		writeErr(w, d, err)
		return
	}
	if status == "rejected" {
		if err := d.exec(r.Context(),
			`INSERT INTO notifications (id, user_id, type, title, body, link, dedupe_key)
			 SELECT $1, owner_user_id, 'referral.domain.rejected', 'Domain ditolak', $2, '/mitra', $3 FROM referred_domains WHERE id = $4
			 ON CONFLICT (user_id, dedupe_key) DO NOTHING`,
			uuid.NewString(), finCoalesceStr(reason, "Domain ditolak"), "referred_domain:rejected:"+id, id); err != nil {
			// best-effort
		}
	} else {
		if err := d.exec(r.Context(),
			`INSERT INTO notifications (id, user_id, type, title, body, link, dedupe_key)
			 SELECT $1, owner_user_id, 'referral.domain.approved', 'Domain disetujui', $2, '/mitra', $3 FROM referred_domains WHERE id = $4
			 ON CONFLICT (user_id, dedupe_key) DO NOTHING`,
			uuid.NewString(), "Domain Anda disetujui — CPA akan diproses.", "referred_domain:approved:"+id, id); err != nil {
			// best-effort
		}
		d.finCreditCPAForDomain(r.Context(), id)
	}
	d.audit(r.Context(), user.ID, map[bool]string{true: "domain.approved", false: "domain.rejected"}[status == "approved"], "referred_domain", id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func finCoalesceStr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// VipPartnersList: GET /v1/platform/vip-partners
func (d *Deps) VipPartnersList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	partners, err := d.queryRows(r.Context(),
		`SELECT
		   p.id AS partner_id, p.name, u.email, p.vip_tier, p.is_vip,
		   p.vip_enabled_at, p.vip_contract_months, p.vip_contract_days, p.vip_expires_at,
		   a.commission_rate_bp, a.cpa_rate, a.cpa_tier_threshold, a.cpa_tier1_rate, a.cpa_tier2_rate,
		   COALESCE((SELECT COUNT(*) FROM referred_domains rd WHERE rd.referrer_partner_id = p.id), 0)::int AS domains_total,
		   COALESCE((SELECT COUNT(*) FROM referred_domains rd WHERE rd.referrer_partner_id = p.id AND rd.status = 'approved'), 0)::int AS domains_approved,
		   COALESCE((SELECT COUNT(*) FROM referred_domains rd WHERE rd.referrer_partner_id = p.id AND rd.cpa_paid = true), 0)::int AS domains_paid,
		   COALESCE((SELECT COUNT(*) FROM referred_domains rd
		              WHERE rd.referrer_partner_id = p.id AND rd.cpa_paid = true
		                AND (p.vip_enabled_at IS NULL OR rd.created_at >= p.vip_enabled_at)), 0)::int AS quota_used,
		   COALESCE((SELECT SUM(rd.cpa_amount) FROM referred_domains rd
		              WHERE rd.referrer_partner_id = p.id AND rd.cpa_paid = true), 0)::numeric(14,2) AS fee_paid_total,
		   COALESCE((SELECT COUNT(DISTINCT rd.id) FROM referred_domains rd
		              WHERE rd.referrer_partner_id = p.id AND EXISTS (
		                SELECT 1 FROM publications pub WHERE pub.media_id = rd.media_id AND pub.status = 'published'
		              )), 0)::int AS domains_with_orders,
		   COALESCE((SELECT COUNT(*) FROM publications pub
		              WHERE pub.media_id IN (
		                SELECT rd2.media_id FROM referred_domains rd2
		                 WHERE rd2.referrer_partner_id = p.id AND rd2.media_id IS NOT NULL
		              ) AND pub.status = 'published'), 0)::int AS orders_published_total,
		   COALESCE((SELECT SUM(pub.client_price) FROM publications pub
		              WHERE pub.media_id IN (
		                SELECT rd2.media_id FROM referred_domains rd2
		                 WHERE rd2.referrer_partner_id = p.id AND rd2.media_id IS NOT NULL
		              ) AND pub.status = 'published'), 0)::numeric(14,2) AS gmv_published_total,
		   COALESCE((SELECT SUM(pe.amount) FROM partner_earnings pe
		              WHERE pe.partner_id = p.id AND pe.kind = 'transaction_commission' AND pe.status <> 'voided'), 0)::numeric(14,2) AS tx_commission_total,
		   COALESCE((SELECT SUM(c.amount) FROM partner_referral_commissions c
		              WHERE c.referrer_partner_id = p.id), 0)::numeric(14,2) AS referral_commission_total
		 FROM partners p
		 JOIN users u ON u.id = p.user_id
		 LEFT JOIN affiliate_settings a ON a.partner_id = p.id
		 WHERE p.vip_tier IN ('brand_ambassador', 'vip_reguler') OR p.is_vip = true OR p.vip_enabled_at IS NOT NULL
		 ORDER BY p.is_vip DESC, p.vip_enabled_at DESC NULLS LAST, p.name ASC`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	now := time.Now()
	items := make([]map[string]any, 0, len(partners))
	for _, raw := range partners {
		row := ptRow(ptNormalizeMap(raw))
		tier := row.strOr("vip_tier", "regular")
		isReguler := tier == "vip_reguler"
		var expiresAt *time.Time
		if t, isTime := row["vip_expires_at"].(time.Time); isTime {
			expiresAt = &t
		}
		isActive := row.boolOr("is_vip", false) && (expiresAt == nil || expiresAt.After(now))
		var daysRemaining any
		if expiresAt != nil {
			daysRemaining = int((expiresAt.Sub(now).Hours() + 23) / 24)
		}
		threshold := 1000
		if row.has("cpa_tier_threshold") {
			threshold = int(row.numOr("cpa_tier_threshold"))
		}
		commissionRate := 500
		if isReguler {
			commissionRate = 100
		}
		if row.has("commission_rate_bp") {
			commissionRate = int(row.numOr("commission_rate_bp"))
		}
		cpaRate := 0
		if row.has("cpa_rate") {
			cpaRate = int(row.numOr("cpa_rate"))
		}
		cpaTier1 := 10000
		if row.has("cpa_tier1_rate") {
			cpaTier1 = int(row.numOr("cpa_tier1_rate"))
		}
		cpaTier2 := 5000
		if row.has("cpa_tier2_rate") {
			cpaTier2 = int(row.numOr("cpa_tier2_rate"))
		}
		items = append(items, map[string]any{
			"partner_id": row.strOr("partner_id", ""), "name": ptNullableStr(row, "name"),
			"email": ptNullableStr(row, "email"), "tier": tier, "is_active": isActive,
			"vip_enabled_at": row.anyOrNil("vip_enabled_at"), "vip_expires_at": row.anyOrNil("vip_expires_at"),
			"days_remaining": daysRemaining, "contract_days": row.intPtr("vip_contract_days"),
			"contract_months": row.intPtr("vip_contract_months"), "commission_rate_bp": commissionRate,
			"cpa_rate": cpaRate, "cpa_tier_threshold": threshold, "cpa_tier1_rate": cpaTier1, "cpa_tier2_rate": cpaTier2,
			"domains_total": int(row.numOr("domains_total")), "domains_approved": int(row.numOr("domains_approved")),
			"domains_paid": int(row.numOr("domains_paid")), "domains_with_orders": int(row.numOr("domains_with_orders")),
			"orders_published_total": int(row.numOr("orders_published_total")),
			"gmv_published_total":    row.numOr("gmv_published_total"), "quota_used": int(row.numOr("quota_used")),
			"fee_paid_total": row.numOr("fee_paid_total"), "tx_commission_total": row.numOr("tx_commission_total"),
			"referral_commission_total": row.numOr("referral_commission_total"),
		})
	}
	summary := map[string]any{
		"total": len(items), "active": 0, "vvip": 0, "vip_reguler": 0,
		"fee_paid_total": 0.0, "commission_total": 0.0, "domains_total": 0,
		"domains_with_orders": 0, "orders_published_total": 0, "gmv_published_total": 0.0,
	}
	feePaid, commissionTotal, gmvTotal := 0.0, 0.0, 0.0
	for _, item := range items {
		if item["is_active"] == true {
			summary["active"] = summary["active"].(int) + 1
		}
		if item["tier"] == "brand_ambassador" {
			summary["vvip"] = summary["vvip"].(int) + 1
		}
		if item["tier"] == "vip_reguler" {
			summary["vip_reguler"] = summary["vip_reguler"].(int) + 1
		}
		feePaid += item["fee_paid_total"].(float64)
		commissionTotal += item["tx_commission_total"].(float64) + item["referral_commission_total"].(float64)
		gmvTotal += item["gmv_published_total"].(float64)
		summary["domains_total"] = summary["domains_total"].(int) + item["domains_total"].(int)
		summary["domains_with_orders"] = summary["domains_with_orders"].(int) + item["domains_with_orders"].(int)
		summary["orders_published_total"] = summary["orders_published_total"].(int) + item["orders_published_total"].(int)
	}
	summary["fee_paid_total"] = feePaid
	summary["commission_total"] = commissionTotal
	summary["gmv_published_total"] = gmvTotal
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "summary": summary})
}

// VipPartnerDomains: GET /v1/platform/vip-partners/{partnerId}/domains
func (d *Deps) VipPartnerDomains(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	partnerID := chiParam(r, "partnerId")
	q := r.URL.Query()
	limit := clampInt(q.Get("limit"), 50, 1, 200)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)
	orderFilter := strings.TrimSpace(q.Get("orders"))
	if orderFilter != "" && orderFilter != "with" && orderFilter != "without" {
		writeErr(w, d, errUnprocessable("orders harus with atau without"))
		return
	}
	partner, err := d.queryRow(r.Context(), "SELECT id, name FROM partners WHERE id = $1 LIMIT 1", partnerID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if partner == nil {
		writeErr(w, d, errNotFound("Mitra tidak ditemukan"))
		return
	}
	havingClause := ""
	if orderFilter == "with" {
		havingClause = "AND EXISTS (SELECT 1 FROM publications p2 WHERE p2.media_id = rd.media_id AND p2.status = 'published')"
	} else if orderFilter == "without" {
		havingClause = "AND NOT EXISTS (SELECT 1 FROM publications p2 WHERE p2.media_id = rd.media_id AND p2.status = 'published')"
	}
	countRow, err := d.queryRow(r.Context(),
		"SELECT COUNT(*)::int AS total FROM referred_domains rd WHERE rd.referrer_partner_id = $1 "+havingClause, partnerID)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	domains, err := d.queryRows(r.Context(),
		`SELECT
		   rd.id, rd.domain_name, rd.status, rd.rejection_reason, rd.cpa_paid, rd.cpa_amount, rd.created_at,
		   u.email AS owner_email, u.display_name AS owner_name,
		   m.name AS media_name, m.website_url AS media_website_url,
		   COALESCE((SELECT COUNT(*) FROM publications p WHERE p.media_id = rd.media_id), 0)::int AS orders_total,
		   COALESCE((SELECT COUNT(*) FROM publications p WHERE p.media_id = rd.media_id AND p.status = 'published'), 0)::int AS orders_published,
		   COALESCE((SELECT SUM(p.client_price) FROM publications p WHERE p.media_id = rd.media_id AND p.status = 'published'), 0)::numeric(14,2) AS gmv_published,
		   (SELECT MAX(p.published_at) FROM publications p WHERE p.media_id = rd.media_id AND p.status = 'published') AS last_order_at
		 FROM referred_domains rd
		 LEFT JOIN users u ON u.id = rd.owner_user_id
		 LEFT JOIN media m ON m.id = rd.media_id
		 WHERE rd.referrer_partner_id = $1 `+havingClause+`
		 ORDER BY rd.created_at DESC
		 LIMIT $2 OFFSET $3`, partnerID, limit, offset)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	out := make([]map[string]any, 0, len(domains))
	for _, raw := range domains {
		row := ptRow(ptNormalizeMap(raw))
		out = append(out, map[string]any{
			"id": row.strOr("id", ""), "domain_name": row.strOr("domain_name", ""),
			"owner_email": ptNullableStr(row, "owner_email"), "owner_name": ptNullableStr(row, "owner_name"),
			"media_name": ptNullableStr(row, "media_name"), "media_website_url": ptNullableStr(row, "media_website_url"),
			"status": row.strOr("status", "pending"), "rejection_reason": ptNullableStr(row, "rejection_reason"),
			"cpa_paid": row.boolOr("cpa_paid", false), "cpa_amount": row.numPtr("cpa_amount"),
			"orders_total": int(row.numOr("orders_total")), "orders_published": int(row.numOr("orders_published")),
			"gmv_published": row.numOr("gmv_published"), "created_at": row.anyOrNil("created_at"),
			"last_order_at": row.anyOrNil("last_order_at"),
		})
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"partner_id": partnerID, "partner_name": ptNullableStr(ptRow(ptNormalizeMap(partner)), "name"),
		"domains": out, "total": countRow.intOr("total", 0), "limit": limit, "offset": offset,
	})
}

// finCreditCPAForDomain: port creditCpaForDomain (best-effort).
func (d *Deps) finCreditCPAForDomain(ctx context.Context, referredDomainID string) {
	rd, err := d.queryRow(ctx, "SELECT * FROM referred_domains WHERE id = $1 LIMIT 1", referredDomainID)
	if err != nil || rd == nil {
		return
	}
	if rd.strOr("status", "") != "approved" || rd.boolOr("cpa_paid", false) {
		return
	}
	referrerPartnerID := rd.strOr("referrer_partner_id", "")
	if referrerPartnerID == "" {
		return
	}
	cpaAmount := d.finResolveCPAAmount(ctx, referrerPartnerID)
	if cpaAmount <= 0 {
		return
	}
	mediaID := rd.strOr("media_id", "")
	if mediaID == "" {
		mediaID = referredDomainID
	}
	// Earning ditulis dulu, baru domain ditandai lunas. Kalau insert gagal,
	// domain tetap belum lunas dan bisa dicoba lagi; kunci unik
	// (publication_id, kind) mencegah CPA ganda untuk domain yang sama.
	if err := d.exec(ctx,
		`INSERT INTO partner_earnings (id, partner_id, publication_id, media_id, rule_id, amount, gross_amount, client_price, status, kind, source_referred_domain_id)
		 VALUES ($1, $2, $3, $4, NULL, $5, $5, $6, 'pending', 'cpa', $7) ON CONFLICT (publication_id, kind) DO NOTHING`,
		uuid.NewString(), referrerPartnerID, "cpa:"+referredDomainID, mediaID, cpaAmount, cpaAmount, referredDomainID); err != nil {
		slog.Warn("[finance] CPA earning gagal", "domain", referredDomainID, "err", err)
		return
	}
	if err := d.exec(ctx, "UPDATE referred_domains SET cpa_paid = true, cpa_amount = $1 WHERE id = $2 AND cpa_paid = false", cpaAmount, referredDomainID); err != nil {
		return
	}
	partner, err := d.queryRow(ctx, "SELECT user_id, public_id, name FROM partners WHERE id = $1 LIMIT 1", referrerPartnerID)
	if err == nil && partner != nil {
		userID := partner.strOr("user_id", "")
		publicID := partner.strOr("public_id", referrerPartnerID)
		if userID != "" {
			refLink := "/mitra/" + publicID + "/referral"
			d.finNotifyUser(ctx, userID, "referral.cpa.credited",
				"CPA Rp "+finFormatThousands(int64(cpaAmount))+" dikreditkan",
				"Domain "+rd.strOr("domain_name", "")+" approved.",
				refLink, "referral:cpa:"+referredDomainID)
			notify.EnqueueUserWA(ctx, d.Pool, notify.UserWAOptions{
				Audience:    notify.AudiencePartner,
				Event:       "referral.cpa.credited",
				UserID:      userID,
				ReferenceID: referredDomainID,
				Data: map[string]string{
					"nama":   partner.strOr("name", ""),
					"judul":  rd.strOr("domain_name", ""),
					"jumlah": "Rp " + finFormatThousands(int64(cpaAmount)),
				},
				Link:    refLink,
				BaseURL: notify.BaseURL(),
			})
		}
	}
}

// finResolveCPAAmount: parity resolveCpaAmountForReferrer — flat (brand_ambassador)
// atau bertingkat (VIP Reguler); selain itu 0.
func (d *Deps) finResolveCPAAmount(ctx context.Context, partnerID string) int {
	partner, err := d.queryRow(ctx,
		"SELECT vip_tier, is_vip, vip_enabled_at FROM partners WHERE id = $1 LIMIT 1", partnerID)
	if err != nil || partner == nil {
		return 0
	}
	settings, _ := d.queryRow(ctx,
		"SELECT cpa_rate, cpa_tier_threshold, cpa_tier1_rate, cpa_tier2_rate FROM affiliate_settings WHERE partner_id = $1 LIMIT 1", partnerID)
	rate := 0
	threshold := 1000
	tier1 := 10000
	tier2 := 5000
	if settings != nil {
		rate = int(settings.numOr("cpa_rate"))
		if settings.has("cpa_tier_threshold") {
			threshold = int(settings.numOr("cpa_tier_threshold"))
		}
		if settings.has("cpa_tier1_rate") {
			tier1 = int(settings.numOr("cpa_tier1_rate"))
		}
		if settings.has("cpa_tier2_rate") {
			tier2 = int(settings.numOr("cpa_tier2_rate"))
		}
	}
	tier := partner.strOr("vip_tier", "regular")
	if tier == "brand_ambassador" && rate > 0 {
		return rate
	}
	if tier == "vip_reguler" {
		quotaUsed := 0
		countRow, cErr := d.queryRow(ctx,
			"SELECT COUNT(*)::int AS c FROM referred_domains rd WHERE rd.referrer_partner_id = $1 AND rd.cpa_paid = true", partnerID)
		if cErr == nil && countRow != nil {
			quotaUsed = countRow.intOr("c", 0)
		}
		if quotaUsed < threshold {
			return tier1
		}
		return tier2
	}
	if rate > 0 {
		return rate
	}
	return 0
}

// ---------- pricing ----------

const finDefaultMarkupPercent = 15.0

var finPricingCache = newTTLCache(30 * time.Second)

func (d *Deps) finMarkupPercent(ctx context.Context) float64 {
	if cached, ok := finPricingCache.get("markup"); ok {
		if v, isFloat := cached.(float64); isFloat {
			return v
		}
	}
	value := finDefaultMarkupPercent
	row, err := d.queryRow(ctx, "SELECT markup_percent FROM pricing_config WHERE id = 1 LIMIT 1")
	if err == nil && row != nil && row.has("markup_percent") {
		value = row.numOr("markup_percent")
	}
	finPricingCache.set("markup", value)
	return value
}

// PricingGet: GET /v1/platform/pricing
func (d *Deps) PricingGet(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"markup_percent": d.finMarkupPercent(r.Context()), "default_markup_percent": finDefaultMarkupPercent,
	})
}

// PricingPut: PUT /v1/platform/pricing
func (d *Deps) PricingPut(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "")
	if !ok {
		return
	}
	body := decodeJSON(r)
	markup, hasMarkup := numField(body, "markup_percent")
	if !hasMarkup || markup < 0 || markup > 500 {
		writeErr(w, d, errUnprocessable("Invalid markup_percent"))
		return
	}
	if err := d.exec(r.Context(),
		"INSERT INTO pricing_config (id, markup_percent, updated_by, updated_at) VALUES (1, $1, $2, NOW()) ON CONFLICT (id) DO UPDATE SET markup_percent = EXCLUDED.markup_percent, updated_by = EXCLUDED.updated_by, updated_at = NOW()",
		markup, finCurrentUserID(r)); err != nil {
		writeErr(w, d, ptErr(http.StatusInternalServerError, "Gagal menyimpan konfigurasi pricing"))
		return
	}
	finPricingCache.invalidate("markup")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "markup_percent": d.finMarkupPercent(r.Context())})
}

// finCurrentUserID: user dari context guard (untuk audit inline tanpa guard kedua).
func finCurrentUserID(r *http.Request) string {
	user := finAuthUserFromCtx(r)
	if user == nil {
		return ""
	}
	return user.ID
}
