package platform

import (
	"context"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Pencatatan penarikan profit Tauthub dari rekening escrow ke rekening
// operasional. Aplikasi hanya mencatat; transfer uang tetap manual lewat
// m-banking owner.
//
// Profit gross = 25% × SUM(client_price) semua publikasi 'published'
// (rumus sederhana sesuai brainstorming owner — tidak dikurangi fee transfer
// atau fee gateway; fee ditampilkan terpisah di dashboard).

// adminProfitPosition: posisi profit seumur hidup (gross / sudah ditarik /
// tersedia) dengan rumus dan pengecualian yang sama persis dengan kartu Profit
// Bersih di dashboard. Dipakai daftar penarikan dan validasi POST withdraw.
func (d *Deps) adminProfitPosition(ctx context.Context) (gross, withdrawn, available int64, err error) {
	pr, err := d.finProfitInRange(ctx, time.Time{}, time.Now().Add(time.Minute), d.finLoadFilter(ctx))
	if err != nil {
		return 0, 0, 0, err
	}
	return pr.gross, pr.withdrawn, pr.available, nil
}

// adminProfitJSON: bentuk JSON satu baris withdrawal (seragam list/create).
func adminProfitJSON(row rowMap) map[string]any {
	createdAt, _ := row.raw("created_at").(time.Time)
	voidedAt, _ := row.raw("voided_at").(time.Time)
	var voided any
	if !voidedAt.IsZero() {
		voided = isoMilli(voidedAt)
	}
	return map[string]any{
		"id":          row.strOr("id", ""),
		"amount":      int64(math.Round(row.numOr("amount"))),
		"note":        row.strOr("note", ""),
		"bank_from":   row.strOr("bank_from", ""),
		"bank_to":     row.strOr("bank_to", ""),
		"reference":   row.strOr("reference", ""),
		"receipt_url": row.strOr("receipt_url", ""),
		"status":      row.strOr("status", "paid"),
		"created_by":  row.strOr("created_by", ""),
		"created_by_name": row.strOr("created_by_name", ""),
		"created_at":  isoMilli(createdAt),
		"voided_at":   voided,
		"void_reason": row.strOr("void_reason", ""),
	}
}

// AdminProfitWithdrawalsList: GET /v1/platform/finance/admin-withdrawals
func (d *Deps) AdminProfitWithdrawalsList(w http.ResponseWriter, r *http.Request) {
	_, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	q := r.URL.Query()
	limit := clampInt(q.Get("limit"), 20, 1, 200)
	offset := clampInt(q.Get("offset"), 0, 0, 1<<30)

	countRow, err := d.queryRow(r.Context(),
		`SELECT COUNT(*)::int AS n FROM admin_profit_withdrawals`)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	rows, err := d.queryRows(r.Context(),
		`SELECT w.id::text AS id, w.amount, w.note, w.bank_from, w.bank_to,
		        w.reference, w.receipt_url, w.status,
		        w.created_by::text AS created_by,
		        COALESCE(u.display_name, u.email, '') AS created_by_name,
		        w.created_at, w.voided_at, w.void_reason
		   FROM admin_profit_withdrawals w
		   LEFT JOIN users u ON u.id::text = w.created_by::text
		  ORDER BY w.created_at DESC
		  LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	entries := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, adminProfitJSON(row))
	}

	gross, withdrawn, available, err := d.adminProfitPosition(r.Context())
	if err != nil {
		writeErr(w, d, err)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, map[string]any{
		"entries":     entries,
		"total":       countRow.intOr("n", 0),
		"limit":       limit,
		"offset":      offset,
		"profit_gross":     gross,
		"profit_withdrawn": withdrawn,
		"profit_available": available,
	})
}

// AdminProfitWithdrawalCreate: POST /v1/platform/finance/admin-withdrawals
//
// Payload: {amount, note, bank_from, bank_to, reference}. Amount divalidasi
// ulang di server (<= available) supaya frontend tidak bisa maksa overdraft.
func (d *Deps) AdminProfitWithdrawalCreate(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	body := decodeJSON(r)

	amountF, hasAmount := numField(body, "amount")
	if !hasAmount || amountF <= 0 {
		writeErr(w, d, errUnprocessable("Jumlah penarikan harus lebih dari 0"))
		return
	}
	amount := int64(math.Round(amountF))

	note, _ := strField(body, "note")
	bankFrom, _ := strField(body, "bank_from")
	bankTo, _ := strField(body, "bank_to")
	reference, _ := strField(body, "reference")
	receiptURL, _ := strField(body, "receipt_url")

	if len(note) > 2000 || len(bankFrom) > 200 || len(bankTo) > 200 ||
		len(reference) > 200 || len(receiptURL) > 500 {
		writeErr(w, d, errUnprocessable("Panjang field melebihi batas"))
		return
	}

	_, _, available, err := d.adminProfitPosition(r.Context())
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if amount > available {
		writeErr(w, d, errUnprocessable("Jumlah melebihi profit yang tersedia untuk ditarik"))
		return
	}

	id := uuid.NewString()
	// created_by di-cast ke uuid; user.ID disimpan sebagai character(36) tapi
	// kolomnya native uuid (sesuai pola media_violations migrasi 292).
	if err := d.exec(r.Context(),
		`INSERT INTO admin_profit_withdrawals
		   (id, amount, note, bank_from, bank_to, reference, receipt_url, status, created_by)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, 'paid', NULLIF($8, '')::uuid)`,
		id, amount, note, bankFrom, bankTo, reference, receiptURL, user.ID); err != nil {
		writeErr(w, d, err)
		return
	}
	d.auditWith(r.Context(), user.ID, "finance.admin_withdrawal.created",
		"admin_profit_withdrawal", id,
		map[string]any{"amount": amount, "reference": reference})

	row, err := d.queryRow(r.Context(),
		`SELECT w.id::text AS id, w.amount, w.note, w.bank_from, w.bank_to,
		        w.reference, w.receipt_url, w.status,
		        w.created_by::text AS created_by,
		        COALESCE(u.display_name, u.email, '') AS created_by_name,
		        w.created_at, w.voided_at, w.void_reason
		   FROM admin_profit_withdrawals w
		   LEFT JOIN users u ON u.id::text = w.created_by::text
		  WHERE w.id::text = $1`, id)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if row == nil {
		writeErr(w, d, errNotFound("Pencatatan tidak ditemukan setelah dibuat"))
		return
	}
	writeJSON(w, http.StatusCreated, adminProfitJSON(row))
}

// AdminProfitWithdrawalVoid: PUT /v1/platform/finance/admin-withdrawals/{id}/void
//
// Membatalkan pencatatan supaya `profit_available` kembali naik sebanyak
// amount-nya. Status permanent (tidak bisa un-void).
func (d *Deps) AdminProfitWithdrawalVoid(w http.ResponseWriter, r *http.Request) {
	user, _, ok := d.adminGuard(w, r, "payments")
	if !ok {
		return
	}
	id := chiParam(r, "withdrawalId")
	if strings.TrimSpace(id) == "" {
		writeErr(w, d, errUnprocessable("ID penarikan wajib diisi"))
		return
	}
	body := decodeJSON(r)
	reason, _ := strField(body, "reason")
	if len(reason) > 2000 {
		writeErr(w, d, errUnprocessable("Alasan void terlalu panjang"))
		return
	}

	existing, err := d.queryRow(r.Context(),
		`SELECT id::text AS id, status FROM admin_profit_withdrawals WHERE id::text = $1 LIMIT 1`, id)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	if existing == nil {
		writeErr(w, d, errNotFound("Pencatatan tidak ditemukan"))
		return
	}
	if existing.strOr("status", "") == "void" {
		writeErr(w, d, errConflict("Pencatatan sudah dibatalkan"))
		return
	}

	if err := d.exec(r.Context(),
		`UPDATE admin_profit_withdrawals
		    SET status = 'void', voided_at = NOW(), void_reason = $1
		  WHERE id::text = $2`, reason, id); err != nil {
		writeErr(w, d, err)
		return
	}
	d.auditWith(r.Context(), user.ID, "finance.admin_withdrawal.voided",
		"admin_profit_withdrawal", id,
		map[string]any{"reason": reason})

	row, err := d.queryRow(r.Context(),
		`SELECT w.id::text AS id, w.amount, w.note, w.bank_from, w.bank_to,
		        w.reference, w.receipt_url, w.status,
		        w.created_by::text AS created_by,
		        COALESCE(u.display_name, u.email, '') AS created_by_name,
		        w.created_at, w.voided_at, w.void_reason
		   FROM admin_profit_withdrawals w
		   LEFT JOIN users u ON u.id::text = w.created_by::text
		  WHERE w.id::text = $1`, id)
	if err != nil {
		writeErr(w, d, err)
		return
	}
	writeJSON(w, http.StatusOK, adminProfitJSON(row))
}
