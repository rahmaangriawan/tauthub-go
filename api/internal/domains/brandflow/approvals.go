package brandflow

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Persetujuan belanja: bila biaya distribusi sebuah kampanye melebihi ambang
// brand dan pemesan bukan admin/owner, distribusi ditahan sampai admin atau
// owner menyetujuinya. Persetujuan berlaku 7 hari, untuk kampanye itu saja, dan
// hanya sekali pakai. Saldo tidak disentuh sebelum disetujui.

const approvalValidity = 7 * 24 * time.Hour

func canApproveSpend(role string) bool {
	return brandsdomain.RoleRank(role) >= brandsdomain.RoleRank("admin")
}

// ApprovalPolicyGet: GET /v1/brands/{brandId}/approval-policy
func (d *Deps) ApprovalPolicyGet(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "viewer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	row, err := d.queryRow(r.Context(),
		"SELECT enabled, threshold_tc FROM approval_policies WHERE brand_id = $1 LIMIT 1", brandRow.ID)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	out := map[string]any{"enabled": false, "threshold_tc": 0, "can_approve": canApproveSpend(brandRow.Role)}
	if row != nil {
		out["enabled"] = row.boolOr("enabled", false)
		out["threshold_tc"] = row.intOr("threshold_tc", 0)
	}
	writeNoStore(w, http.StatusOK, out)
}

// ApprovalPolicyPut: PUT /v1/brands/{brandId}/approval-policy (admin)
func (d *Deps) ApprovalPolicyPut(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	enabled, _ := body["enabled"].(bool)
	threshold := 0
	if v, ok := body["threshold_tc"].(float64); ok {
		threshold = int(v)
	}
	if threshold < 1 || threshold > 100000000 {
		httpx.Error(w, http.StatusUnprocessableEntity, "Ambang persetujuan harus berupa angka TC lebih dari 0.")
		return
	}
	if err := d.exec(r.Context(), `
		INSERT INTO approval_policies (brand_id, enabled, threshold_tc, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (brand_id) DO UPDATE SET enabled = EXCLUDED.enabled,
		  threshold_tc = EXCLUDED.threshold_tc, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		brandRow.ID, enabled, threshold, user.ID); err != nil {
		d.writeErr(w, err)
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{"enabled": enabled, "threshold_tc": threshold, "can_approve": true})
}

// ApprovalsList: GET /v1/brands/{brandId}/approvals?status=pending|all
// Admin/owner melihat semua permintaan brand; peran lain hanya miliknya sendiri.
func (d *Deps) ApprovalsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	statusFilter := "AND a.status = 'pending'"
	if r.URL.Query().Get("status") == "all" {
		statusFilter = ""
	}
	ownOnly := ""
	args := []any{brandRow.ID}
	if !canApproveSpend(brandRow.Role) {
		ownOnly = "AND a.requested_by = $2"
		args = append(args, user.ID)
	}
	rows, err := d.queryRows(r.Context(), `
		SELECT a.id, a.campaign_id, c.name AS campaign_name, a.cost_tc, a.status,
		       a.created_at, a.decided_at, a.decision_note, a.expires_at,
		       COALESCE(NULLIF(u.display_name, ''), u.email) AS requester
		FROM campaign_approvals a
		JOIN campaigns c ON c.id = a.campaign_id
		JOIN users u ON u.id = a.requested_by
		WHERE a.brand_id = $1 `+statusFilter+` `+ownOnly+`
		ORDER BY a.created_at DESC LIMIT 100`, args...)
	if err != nil {
		d.writeErr(w, err)
		return
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id": row.strOr("id", ""), "campaign_id": row.strOr("campaign_id", ""),
			"campaign_name": row.strOr("campaign_name", ""), "cost_tc": row.intOr("cost_tc", 0),
			"status": row.strOr("status", ""), "requester": row.strOr("requester", ""),
			"created_at": row.raw("created_at"), "decided_at": row.raw("decided_at"),
			"decision_note": row.raw("decision_note"), "expires_at": row.raw("expires_at"),
		})
	}
	writeNoStore(w, http.StatusOK, map[string]any{"items": items, "can_approve": canApproveSpend(brandRow.Role)})
}

// ApprovalDecide: POST /v1/brands/{brandId}/approvals/{approvalId}/decide (admin)
// Body: {"decision":"approve"|"reject","note":"…"}
func (d *Deps) ApprovalDecide(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	body := decodeJSON(r)
	decision, _ := body["decision"].(string)
	note, _ := body["note"].(string)
	note = strings.TrimSpace(note)
	if len(note) > 500 {
		note = note[:500]
	}
	var newStatus string
	switch decision {
	case "approve":
		newStatus = "approved"
	case "reject":
		newStatus = "rejected"
	default:
		httpx.Error(w, http.StatusUnprocessableEntity, "Keputusan harus approve atau reject.")
		return
	}
	row, err := d.queryRow(r.Context(), `
		UPDATE campaign_approvals SET status = $3, decided_by = $4, decided_at = NOW(),
		  decision_note = NULLIF($5, ''),
		  expires_at = CASE WHEN $3 = 'approved' THEN NOW() + ($6 || ' seconds')::interval ELSE NULL END
		WHERE id = $1 AND brand_id = $2 AND status = 'pending'
		RETURNING requested_by, campaign_id, cost_tc`,
		chiParam(r, "approvalId"), brandRow.ID, newStatus, user.ID, note, int(approvalValidity.Seconds()))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if row == nil {
		httpx.Error(w, http.StatusNotFound, "Permintaan tidak ditemukan atau sudah diputuskan.")
		return
	}
	title := "Pengajuan belanja disetujui"
	text := "Kampanye boleh didistribusikan dalam 7 hari."
	if newStatus == "rejected" {
		title = "Pengajuan belanja ditolak"
		text = "Distribusi kampanye tidak disetujui."
		if note != "" {
			text += " Catatan: " + note
		}
	}
	d.approvalNotifyUser(r.Context(), row.strOr("requested_by", ""), "approval.decided", title, text,
		"/klien/"+brandRow.ID+"/persetujuan")
	writeNoStore(w, http.StatusOK, map[string]any{"ok": true, "status": newStatus})
}

// ApprovalCancel: POST /v1/brands/{brandId}/approvals/{approvalId}/cancel
// Pemohon (atau admin) menarik permintaan yang masih menunggu.
func (d *Deps) ApprovalCancel(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	brandRow, err := d.access(r, chiParam(r, "brandId"), user.ID, "writer")
	if err != nil {
		d.writeErr(w, err)
		return
	}
	n, err := d.execRows(r.Context(), `
		UPDATE campaign_approvals SET status = 'cancelled', decided_at = NOW()
		WHERE id = $1 AND brand_id = $2 AND status = 'pending'
		  AND (requested_by = $3 OR $4)`,
		chiParam(r, "approvalId"), brandRow.ID, user.ID, canApproveSpend(brandRow.Role))
	if err != nil {
		d.writeErr(w, err)
		return
	}
	if n == 0 {
		httpx.Error(w, http.StatusNotFound, "Permintaan tidak ditemukan atau sudah diputuskan.")
		return
	}
	writeNoStore(w, http.StatusOK, map[string]any{"ok": true})
}

// approvalGateResult: hasil pemeriksaan sebelum distribusi.
type approvalGateResult struct {
	Blocked    bool
	ApprovalID string // persetujuan yang dipakai bila lolos lewat persetujuan
	Code       string // approval_required | approval_pending
	Threshold  int
	RequestID  string
}

// approvalGate: dipanggil handler distribusi setelah biaya diketahui. Admin dan
// owner tidak ditahan. Gagal membaca kebijakan dihitung sebagai tidak ada
// kebijakan hanya bila tabelnya kosong; error basis data diteruskan supaya
// distribusi tidak lolos diam-diam.
func (d *Deps) approvalGate(ctx context.Context, brandID, campaignID, userID, role string, costTC int) (approvalGateResult, error) {
	var res approvalGateResult
	if canApproveSpend(role) || costTC <= 0 {
		return res, nil
	}
	pol, err := d.queryRow(ctx, "SELECT enabled, threshold_tc FROM approval_policies WHERE brand_id = $1 LIMIT 1", brandID)
	if err != nil {
		return res, err
	}
	if pol == nil || !pol.boolOr("enabled", false) {
		return res, nil
	}
	res.Threshold = pol.intOr("threshold_tc", 0)
	if res.Threshold <= 0 || costTC <= res.Threshold {
		return res, nil
	}
	ok, err := d.queryRow(ctx, `
		SELECT id FROM campaign_approvals
		WHERE campaign_id = $1 AND brand_id = $2 AND status = 'approved'
		  AND expires_at > NOW() AND cost_tc >= $3
		ORDER BY decided_at DESC LIMIT 1`, campaignID, brandID, costTC)
	if err != nil {
		return res, err
	}
	if ok != nil {
		res.ApprovalID = ok.strOr("id", "")
		return res, nil
	}
	res.Blocked = true
	pending, err := d.queryRow(ctx,
		"SELECT id FROM campaign_approvals WHERE campaign_id = $1 AND status = 'pending' LIMIT 1", campaignID)
	if err != nil {
		return res, err
	}
	if pending != nil {
		res.Code = "approval_pending"
		res.RequestID = pending.strOr("id", "")
		return res, nil
	}
	created, err := d.queryRow(ctx, `
		INSERT INTO campaign_approvals (brand_id, campaign_id, requested_by, cost_tc)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (campaign_id) WHERE status = 'pending' DO NOTHING
		RETURNING id`, brandID, campaignID, userID, costTC)
	if err != nil {
		return res, err
	}
	res.Code = "approval_required"
	if created != nil {
		res.RequestID = created.strOr("id", "")
		d.approvalNotifyApprovers(ctx, brandID, campaignID, costTC)
	}
	return res, nil
}

func (d *Deps) approvalConsume(ctx context.Context, approvalID string) {
	if approvalID == "" {
		return
	}
	if err := d.exec(ctx, "UPDATE campaign_approvals SET status = 'used' WHERE id = $1 AND status = 'approved'", approvalID); err != nil {
		slog.Warn("[approval] gagal menandai terpakai", "approval", approvalID, "err", err)
	}
}

func (d *Deps) approvalNotifyApprovers(ctx context.Context, brandID, campaignID string, costTC int) {
	name := ""
	if row, err := d.queryRow(ctx, "SELECT name FROM campaigns WHERE id = $1", campaignID); err == nil && row != nil {
		name = row.strOr("name", "")
	}
	if err := d.exec(ctx, `
		INSERT INTO notifications (id, user_id, type, title, body, link)
		SELECT gen_random_uuid(), bm.user_id, 'approval.requested', $2, $3, $4
		FROM brand_memberships bm JOIN users u ON u.id = bm.user_id
		WHERE bm.brand_id = $1 AND bm.role IN ('owner', 'admin') AND u.role <> 'admin'`,
		brandID, "Pengajuan belanja butuh persetujuan",
		"Kampanye "+name+" senilai "+itoa(costTC)+" TC menunggu persetujuan Anda.",
		"/klien/"+brandID+"/persetujuan"); err != nil {
		slog.Warn("[approval] notifikasi gagal", "brand", brandID, "err", err)
	}
}

func (d *Deps) approvalNotifyUser(ctx context.Context, userID, notifType, title, body, link string) {
	if userID == "" {
		return
	}
	if err := d.exec(ctx,
		"INSERT INTO notifications (id, user_id, type, title, body, link) VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)",
		userID, notifType, title, body, link); err != nil {
		slog.Warn("[approval] notifikasi gagal", "user", userID, "err", err)
	}
}
