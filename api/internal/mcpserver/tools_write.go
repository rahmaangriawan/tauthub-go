package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Executor: fungsi yang dijalankan BOT setelah admin menekan "Setujui".
// Menerima user_id admin sebagai actAsUser supaya API internal memakai
// identitas yang benar (adminGuard tetap berjalan).
type Executor func(ctx context.Context, s *Server, actAsUser string, agentCtx *AgentHeader, params json.RawMessage) (any, error)

var executors = map[string]Executor{}

// registerWriteTools: daftar tool tulis prioritas. Semua tool tulis
// mengikuti pola:
//   1. decode & validasi args
//   2. susun summary dalam bahasa Indonesia
//   3. Propose → simpan ke agent_actions
//   4. kirim kartu Telegram (kalau bot aktif)
//   5. kembalikan {status:"awaiting_approval", code}
//
// Eksekusi terjadi di bot (handler callback). Pemetaan nama tool →
// executor ada di `executors` map; register-nya di bawah.
func (s *Server) registerWriteTools() {
	s.Register(&Tool{
		Name:        "review_media",
		Description: "Usulkan persetujuan atau penolakan satu media yang sedang menunggu review. Eksekusi hanya setelah admin menekan tombol Setujui di Telegram.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"media_id": map[string]any{"type": "string", "description": "UUID media."},
				"decision": map[string]any{"type": "string", "enum": []string{"approve", "reject"}},
				"reason":   map[string]any{"type": "string", "description": "Wajib untuk reject; opsional untuk approve. Maks 1000 karakter."},
			},
			"required":             []string{"media_id", "decision"},
			"additionalProperties": false,
		},
		Handle: proposeReviewMedia,
	})
	executors["review_media"] = execReviewMedia

	s.Register(&Tool{
		Name:        "suspend_media",
		Description: "Tangguhkan satu media (menandai pelanggaran, memblokir dari katalog). Menunggu persetujuan admin.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"media_id": map[string]any{"type": "string"},
				"reason":   map[string]any{"type": "string"},
			},
			"required":             []string{"media_id", "reason"},
			"additionalProperties": false,
		},
		Handle: proposeSuspendMedia,
	})
	executors["suspend_media"] = execSuspendMedia

	s.Register(&Tool{
		Name:        "unsuspend_media",
		Description: "Cabut penangguhan media.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"media_id": map[string]any{"type": "string"},
				"reason":   map[string]any{"type": "string"},
			},
			"required":             []string{"media_id"},
			"additionalProperties": false,
		},
		Handle: proposeUnsuspendMedia,
	})
	executors["unsuspend_media"] = execUnsuspendMedia

	s.Register(&Tool{
		Name:        "update_settlement_status",
		Description: "Ubah status penarikan mitra: approved, processing, paid (butuh payment_reference), failed. Tandai lunas = status 'paid'. Butuh izin admin 'settlements'.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"settlement_id":     map[string]any{"type": "string"},
				"status":            map[string]any{"type": "string", "enum": []string{"approved", "processing", "paid", "failed"}},
				"payment_reference": map[string]any{"type": "string", "description": "Wajib bila status=paid."},
				"note":              map[string]any{"type": "string"},
			},
			"required":             []string{"settlement_id", "status"},
			"additionalProperties": false,
		},
		Handle: proposeUpdateSettlementStatus,
	})
	executors["update_settlement_status"] = execUpdateSettlementStatus

	s.Register(&Tool{
		Name:        "adjust_credit",
		Description: "Sesuaikan saldo (TC) sebuah brand (klien). Nilai negatif = kurangi. Dibatasi oleh MCP_MAX_CREDIT_ADJUST_TC. Butuh izin admin 'topup'.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"brand_id": map[string]any{"type": "string"},
				"tc":       map[string]any{"type": "integer", "description": "Bilangan bulat, non-nol. Negatif untuk pengurangan. Satuan: TC (1 TC = Rp1)."},
				"reason":   map[string]any{"type": "string", "minLength": 3, "maxLength": 500},
			},
			"required":             []string{"brand_id", "tc", "reason"},
			"additionalProperties": false,
		},
		Handle: proposeAdjustCredit,
	})
	executors["adjust_credit"] = execAdjustCredit

	s.Register(&Tool{
		Name:        "review_credit_report",
		Description: "Review klaim top-up gagal yang diajukan klien: approve (kredit saldo) atau reject. Butuh izin admin 'topup'.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"report_id": map[string]any{"type": "string"},
				"decision":  map[string]any{"type": "string", "enum": []string{"approve", "reject"}},
				"note":      map[string]any{"type": "string"},
			},
			"required":             []string{"report_id", "decision"},
			"additionalProperties": false,
		},
		Handle: proposeReviewCreditReport,
	})
	executors["review_credit_report"] = execReviewCreditReport

	s.Register(&Tool{
		Name:        "set_user_status",
		Description: "Aktifkan / nonaktifkan / approve akun user (klien atau mitra). Tidak boleh dipakai pada akun admin.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"user_id":   map[string]any{"type": "string"},
				"is_active": map[string]any{"type": "boolean"},
				"approve":   map[string]any{"type": "boolean", "description": "true = sekalian mengisi approved_at."},
				"note":      map[string]any{"type": "string"},
			},
			"required":             []string{"user_id", "is_active"},
			"additionalProperties": false,
		},
		Handle: proposeSetUserStatus,
	})
	executors["set_user_status"] = execSetUserStatus

	s.Register(&Tool{
		Name:        "set_partner_status",
		Description: "Ubah status mitra (active / inactive). Nonaktifkan = menangguhkan mitra.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"partner_id": map[string]any{"type": "string"},
				"status":     map[string]any{"type": "string", "enum": []string{"active", "inactive"}},
				"note":       map[string]any{"type": "string"},
			},
			"required":             []string{"partner_id", "status"},
			"additionalProperties": false,
		},
		Handle: proposeSetPartnerStatus,
	})
	executors["set_partner_status"] = execSetPartnerStatus

	s.Register(&Tool{
		Name:        "cancel_publication",
		Description: "Batalkan publikasi (order klien) yang masih pending / processing. Mengembalikan kredit yang direserve.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"publication_id": map[string]any{"type": "string"},
				"reason":         map[string]any{"type": "string"},
			},
			"required":             []string{"publication_id", "reason"},
			"additionalProperties": false,
		},
		Handle: proposeCancelPublication,
	})
	executors["cancel_publication"] = execCancelPublication

	s.Register(&Tool{
		Name:        "retry_publication",
		Description: "Jalankan ulang publikasi yang gagal (reset state ke retrying dan antre ulang job eksekusinya).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"publication_id": map[string]any{"type": "string"},
				"note":           map[string]any{"type": "string"},
			},
			"required":             []string{"publication_id"},
			"additionalProperties": false,
		},
		Handle: proposeRetryPublication,
	})
	executors["retry_publication"] = execRetryPublication

	s.Register(&Tool{
		Name:        "reply_ticket",
		Description: "Kirim balasan ke thread tiket dukungan atas nama admin yang menyetujui.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ticket_id": map[string]any{"type": "string"},
				"message":   map[string]any{"type": "string", "minLength": 1, "maxLength": 4000},
				"internal":  map[string]any{"type": "boolean", "description": "true = catatan internal (tidak terlihat klien/mitra)."},
			},
			"required":             []string{"ticket_id", "message"},
			"additionalProperties": false,
		},
		Handle: proposeReplyTicket,
	})
	executors["reply_ticket"] = execReplyTicket

	s.Register(&Tool{
		Name:        "refund_link_guardian",
		Description: "Setujui klaim Link Guardian: kembalikan TC yang sudah di-capture ke saldo klien. Tindakan ini membebaskan kredit publikasi dan mencatat negative ledger refund.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"claim_id": map[string]any{"type": "string"},
				"note":     map[string]any{"type": "string"},
			},
			"required":             []string{"claim_id"},
			"additionalProperties": false,
		},
		Handle: proposeRefundLinkGuardian,
	})
	executors["refund_link_guardian"] = execRefundLinkGuardian

	s.Register(&Tool{
		Name:        "send_partner_wa_reminder",
		Description: "Kirim pesan WhatsApp pengingat ke mitra lewat template admin (mis. melengkapi data WA, dokumen, atau menanggapi tiket).",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"partner_id": map[string]any{"type": "string"},
				"template":   map[string]any{"type": "string", "description": "Key template di platform_wa / default bila kosong."},
				"note":       map[string]any{"type": "string"},
			},
			"required":             []string{"partner_id"},
			"additionalProperties": false,
		},
		Handle: proposeSendPartnerWAReminder,
	})
	executors["send_partner_wa_reminder"] = execSendPartnerWAReminder
}

// ---------- helper umum ----------

// sendCard: dipanggil setiap handler Propose setelah baris agent_actions
// terbuat. Mengirim kartu Telegram (bila bot aktif) dan mengaitkan
// message_id. Kegagalan kirim BUKAN kegagalan propose — agent_actions tetap
// hidup; admin dapat membuka /admin/agent-actions (fase berikut) untuk
// menyetujui.
func (s *Server) sendCard(ctx context.Context, a *Action) string {
	if s.Telegram == nil {
		return "tidak ada bot Telegram terpasang; buka /admin/agent-actions untuk menyetujui"
	}
	chatID, msgID, err := s.Telegram.SendApprovalCard(ctx, s.MCPConfig.Snapshot().TelegramChatIDs, a)
	if err != nil {
		return "kartu Telegram gagal terkirim: " + err.Error()
	}
	if err := s.BindTelegramMessage(ctx, a.ID, chatID, msgID); err != nil {
		return "message_id Telegram gagal dicatat: " + err.Error()
	}
	return ""
}

func proposeResult(a *Action, note string) (any, error) {
	payload := map[string]any{
		"status":     "awaiting_approval",
		"code":       a.Code,
		"action_id":  a.ID,
		"tool":       a.Tool,
		"summary":    a.Summary,
		"expires_at": a.ExpiresAt,
	}
	if note != "" {
		payload["note"] = note
	}
	return textResult(payload)
}

// strFromRaw: ekstrak field string non-kosong (tanpa trim panjang).
func strFromRaw(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return strings.TrimSpace(v)
}

// intFromRaw: ekstrak field numerik ke int64.
func intFromRaw(m map[string]any, k string) (int64, bool) {
	switch n := m[k].(type) {
	case float64:
		return int64(n), true
	case int:
		return int64(n), true
	case int64:
		return n, true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	}
	return 0, false
}

// ---------- review_media ----------

type reviewMediaParams struct {
	MediaID  string `json:"media_id"`
	Decision string `json:"decision"`
	Reason   string `json:"reason"`
}

func proposeReviewMedia(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var p reviewMediaParams
	if err := decodeArgs(raw, &p); err != nil {
		return nil, err
	}
	if p.MediaID == "" {
		return nil, errors.New("media_id wajib")
	}
	if p.Decision != "approve" && p.Decision != "reject" {
		return nil, errors.New("decision harus approve atau reject")
	}
	if p.Decision == "reject" && strings.TrimSpace(p.Reason) == "" {
		return nil, errors.New("reason wajib untuk reject")
	}

	// Kembangkan ringkasan dengan metadata media (nama + domain).
	label := p.MediaID
	if resp, err := s.API.Call(ctx, "GET", "/v1/platform/media/"+p.MediaID, nil, nil, client.ReadAsUserID, nil); err == nil {
		var row map[string]any
		if err := resp.JSON(&row); err == nil {
			parts := []string{}
			if name, _ := row["name"].(string); name != "" {
				parts = append(parts, name)
			}
			if dom, _ := row["domain"].(string); dom != "" {
				parts = append(parts, dom)
			}
			if len(parts) > 0 {
				label = strings.Join(parts, " / ") + " (" + p.MediaID + ")"
			}
		}
	}

	verb := "Setujui"
	if p.Decision == "reject" {
		verb = "Tolak"
	}
	summary := verb + " media: " + label
	if p.Reason != "" {
		summary += "\nAlasan: " + p.Reason
	}

	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:    "review_media",
		Summary: summary,
		Params:  p,
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execReviewMedia(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p reviewMediaParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	body := map[string]any{"decision": p.Decision}
	if p.Reason != "" {
		body["reason"] = p.Reason
	}
	resp, err := s.API.Call(ctx, "POST", "/v1/platform/media/"+p.MediaID+"/review", nil, body, actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := resp.JSON(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// ---------- suspend / unsuspend media ----------

type suspendMediaParams struct {
	MediaID string `json:"media_id"`
	Reason  string `json:"reason"`
}

func proposeSuspendMedia(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var p suspendMediaParams
	if err := decodeArgs(raw, &p); err != nil {
		return nil, err
	}
	if p.MediaID == "" || p.Reason == "" {
		return nil, errors.New("media_id dan reason wajib")
	}
	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:    "suspend_media",
		Summary: fmt.Sprintf("Tangguhkan media %s\nAlasan: %s", p.MediaID, p.Reason),
		Params:  p,
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execSuspendMedia(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p suspendMediaParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	resp, err := s.API.Call(ctx, "POST", "/v1/platform/media/"+p.MediaID+"/suspend",
		nil, map[string]any{"reason": p.Reason}, actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var out any
	_ = resp.JSON(&out)
	return out, nil
}

func proposeUnsuspendMedia(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var p suspendMediaParams
	if err := decodeArgs(raw, &p); err != nil {
		return nil, err
	}
	if p.MediaID == "" {
		return nil, errors.New("media_id wajib")
	}
	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:    "unsuspend_media",
		Summary: fmt.Sprintf("Cabut penangguhan media %s\nCatatan: %s", p.MediaID, p.Reason),
		Params:  p,
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execUnsuspendMedia(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p suspendMediaParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	body := map[string]any{}
	if p.Reason != "" {
		body["reason"] = p.Reason
	}
	resp, err := s.API.Call(ctx, "POST", "/v1/platform/media/"+p.MediaID+"/unsuspend",
		nil, body, actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var out any
	_ = resp.JSON(&out)
	return out, nil
}

// ---------- update_settlement_status ----------

type updateSettlementParams struct {
	SettlementID     string `json:"settlement_id"`
	Status           string `json:"status"`
	PaymentReference string `json:"payment_reference"`
	Note             string `json:"note"`
}

func proposeUpdateSettlementStatus(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var p updateSettlementParams
	if err := decodeArgs(raw, &p); err != nil {
		return nil, err
	}
	if p.SettlementID == "" {
		return nil, errors.New("settlement_id wajib")
	}
	switch p.Status {
	case "approved", "processing", "paid", "failed":
	default:
		return nil, errors.New("status harus salah satu: approved, processing, paid, failed")
	}
	if p.Status == "paid" && strings.TrimSpace(p.PaymentReference) == "" {
		return nil, errors.New("payment_reference wajib bila status=paid")
	}

	// Perkaya ringkasan dengan jumlah & mitra.
	label := p.SettlementID
	if resp, err := s.API.Call(ctx, "GET", "/v1/platform/settlements/"+p.SettlementID, nil, nil, client.ReadAsUserID, nil); err == nil {
		var row map[string]any
		if err := resp.JSON(&row); err == nil {
			parts := []string{p.SettlementID}
			if n, ok := row["total_amount"].(float64); ok {
				parts = append(parts, fmt.Sprintf("Rp%s", formatRupiah(int64(n))))
			}
			if pName, _ := row["partner_name"].(string); pName != "" {
				parts = append(parts, pName)
			}
			label = strings.Join(parts, " · ")
		}
	}

	summary := fmt.Sprintf("Set penarikan %s → <b>%s</b>", label, p.Status)
	if p.PaymentReference != "" {
		summary += "\nRef: " + p.PaymentReference
	}
	if p.Note != "" {
		summary += "\nCatatan: " + p.Note
	}

	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:               "update_settlement_status",
		Summary:            summary,
		Params:             p,
		RequiredPermission: "settlements",
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execUpdateSettlementStatus(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p updateSettlementParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	body := map[string]any{"status": p.Status}
	if p.PaymentReference != "" {
		body["payment_reference"] = p.PaymentReference
	}
	if p.Note != "" {
		body["note"] = p.Note
	}
	resp, err := s.API.Call(ctx, "PUT", "/v1/platform/settlements/"+p.SettlementID,
		nil, body, actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var out any
	_ = resp.JSON(&out)
	return out, nil
}

// ---------- adjust_credit ----------

type adjustCreditParams struct {
	BrandID string `json:"brand_id"`
	TC      int64  `json:"tc"`
	Reason  string `json:"reason"`
}

func proposeAdjustCredit(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	// Parse manual untuk memaksa tc sebagai integer (JSON number).
	var m map[string]any
	if err := decodeArgs(raw, &m); err != nil {
		return nil, err
	}
	p := adjustCreditParams{BrandID: strFromRaw(m, "brand_id"), Reason: strFromRaw(m, "reason")}
	if tc, ok := intFromRaw(m, "tc"); ok {
		p.TC = tc
	} else {
		return nil, errors.New("tc wajib berupa bilangan bulat")
	}
	if p.BrandID == "" || p.TC == 0 || p.Reason == "" {
		return nil, errors.New("brand_id, tc (non-nol), dan reason wajib")
	}
	max := s.MCPConfig.Snapshot().MaxCreditAdjustTC
	if max <= 0 {
		max = 50_000_000
	}
	if p.TC > max || p.TC < -max {
		return nil, fmt.Errorf("tc di luar batas MCP_MAX_CREDIT_ADJUST_TC (±%d)", max)
	}

	brandLabel := p.BrandID
	if resp, err := s.API.Call(ctx, "GET", "/v1/platform/operations/users", nil, nil, client.ReadAsUserID, nil); err == nil {
		// Tidak ada endpoint brand-by-id yang admin-only; label apa adanya cukup.
		_ = resp
	}

	sign := "+"
	if p.TC < 0 {
		sign = ""
	}
	summary := fmt.Sprintf(
		"Sesuaikan saldo brand <code>%s</code>\nJumlah: %s%d TC (Rp%s)\nAlasan: %s",
		brandLabel, sign, p.TC, formatRupiah(p.TC), p.Reason)

	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:               "adjust_credit",
		Summary:            summary,
		Params:             p,
		RequiredPermission: "topup",
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execAdjustCredit(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p adjustCreditParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	resp, err := s.API.Call(ctx, "POST", "/v1/platform/operations/credit-adjustment", nil,
		map[string]any{"brand_id": p.BrandID, "tc": p.TC, "reason": p.Reason},
		actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var out any
	_ = resp.JSON(&out)
	return out, nil
}

// ---------- review_credit_report ----------

type reviewCreditReportParams struct {
	ReportID string `json:"report_id"`
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

func proposeReviewCreditReport(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var p reviewCreditReportParams
	if err := decodeArgs(raw, &p); err != nil {
		return nil, err
	}
	if p.ReportID == "" {
		return nil, errors.New("report_id wajib")
	}
	if p.Decision != "approve" && p.Decision != "reject" {
		return nil, errors.New("decision harus approve atau reject")
	}
	summary := fmt.Sprintf("Review klaim top-up %s → %s", p.ReportID, p.Decision)
	if p.Note != "" {
		summary += "\nCatatan: " + p.Note
	}
	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:               "review_credit_report",
		Summary:            summary,
		Params:             p,
		RequiredPermission: "topup",
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execReviewCreditReport(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p reviewCreditReportParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	body := map[string]any{"decision": p.Decision}
	if p.Note != "" {
		body["note"] = p.Note
	}
	resp, err := s.API.Call(ctx, "POST", "/v1/platform/operations/credit-reports/"+p.ReportID+"/review",
		nil, body, actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var out any
	_ = resp.JSON(&out)
	return out, nil
}

// ---------- set_user_status ----------

type setUserStatusParams struct {
	UserID   string `json:"user_id"`
	IsActive bool   `json:"is_active"`
	Approve  bool   `json:"approve"`
	Note     string `json:"note"`
}

func proposeSetUserStatus(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var p setUserStatusParams
	if err := decodeArgs(raw, &p); err != nil {
		return nil, err
	}
	if p.UserID == "" {
		return nil, errors.New("user_id wajib")
	}
	verb := "Nonaktifkan"
	if p.IsActive {
		verb = "Aktifkan"
		if p.Approve {
			verb = "Approve & aktifkan"
		}
	}
	summary := fmt.Sprintf("%s user <code>%s</code>", verb, p.UserID)
	if p.Note != "" {
		summary += "\nCatatan: " + p.Note
	}
	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:               "set_user_status",
		Summary:            summary,
		Params:             p,
		RequiredPermission: "users",
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execSetUserStatus(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p setUserStatusParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	body := map[string]any{"is_active": p.IsActive}
	if p.Approve {
		body["approve"] = true
	}
	resp, err := s.API.Call(ctx, "PUT", "/v1/platform/operations/users/"+p.UserID+"/status",
		nil, body, actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var out any
	_ = resp.JSON(&out)
	return out, nil
}

// ---------- set_partner_status ----------

type setPartnerStatusParams struct {
	PartnerID string `json:"partner_id"`
	Status    string `json:"status"`
	Note      string `json:"note"`
}

func proposeSetPartnerStatus(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var p setPartnerStatusParams
	if err := decodeArgs(raw, &p); err != nil {
		return nil, err
	}
	if p.PartnerID == "" {
		return nil, errors.New("partner_id wajib")
	}
	if p.Status != "active" && p.Status != "inactive" {
		return nil, errors.New("status harus active atau inactive")
	}
	summary := fmt.Sprintf("Set mitra <code>%s</code> → <b>%s</b>", p.PartnerID, p.Status)
	if p.Note != "" {
		summary += "\nCatatan: " + p.Note
	}
	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:               "set_partner_status",
		Summary:            summary,
		Params:             p,
		RequiredPermission: "partners",
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execSetPartnerStatus(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p setPartnerStatusParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	// PartnerUpdate menerima PATCH-style partial update; status cukup dikirim.
	resp, err := s.API.Call(ctx, "PUT", "/v1/platform/partners/"+p.PartnerID,
		nil, map[string]any{"status": p.Status}, actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var out any
	_ = resp.JSON(&out)
	return out, nil
}

// ---------- cancel / retry publication ----------

type publicationIDParams struct {
	PublicationID string `json:"publication_id"`
	Reason        string `json:"reason"`
	Note          string `json:"note"`
}

func proposeCancelPublication(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var p publicationIDParams
	if err := decodeArgs(raw, &p); err != nil {
		return nil, err
	}
	if p.PublicationID == "" {
		return nil, errors.New("publication_id wajib")
	}
	if p.Reason == "" {
		return nil, errors.New("reason wajib untuk pembatalan")
	}
	summary := fmt.Sprintf("Batalkan publikasi <code>%s</code>\nAlasan: %s", p.PublicationID, p.Reason)
	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:    "cancel_publication",
		Summary: summary,
		Params:  p,
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execCancelPublication(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p publicationIDParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	body := map[string]any{"reason": p.Reason}
	resp, err := s.API.Call(ctx, "POST", "/v1/platform/operations/publications/"+p.PublicationID+"/cancel",
		nil, body, actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var out any
	_ = resp.JSON(&out)
	return out, nil
}

func proposeRetryPublication(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var p publicationIDParams
	if err := decodeArgs(raw, &p); err != nil {
		return nil, err
	}
	if p.PublicationID == "" {
		return nil, errors.New("publication_id wajib")
	}
	summary := fmt.Sprintf("Jalankan ulang publikasi <code>%s</code>", p.PublicationID)
	if p.Note != "" {
		summary += "\nCatatan: " + p.Note
	}
	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:    "retry_publication",
		Summary: summary,
		Params:  p,
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execRetryPublication(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p publicationIDParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	resp, err := s.API.Call(ctx, "POST", "/v1/platform/operations/publications/"+p.PublicationID+"/retry",
		nil, map[string]any{}, actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var out any
	_ = resp.JSON(&out)
	return out, nil
}

// ---------- reply_ticket ----------

type replyTicketParams struct {
	TicketID string `json:"ticket_id"`
	Message  string `json:"message"`
	Internal bool   `json:"internal"`
}

func proposeReplyTicket(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var p replyTicketParams
	if err := decodeArgs(raw, &p); err != nil {
		return nil, err
	}
	if p.TicketID == "" || strings.TrimSpace(p.Message) == "" {
		return nil, errors.New("ticket_id dan message wajib")
	}
	preview := p.Message
	if len(preview) > 200 {
		preview = preview[:200] + "…"
	}
	tag := ""
	if p.Internal {
		tag = " (internal)"
	}
	summary := fmt.Sprintf("Balas tiket <code>%s</code>%s\n%s", p.TicketID, tag, preview)
	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:    "reply_ticket",
		Summary: summary,
		Params:  p,
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execReplyTicket(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p replyTicketParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	body := map[string]any{"message": p.Message}
	if p.Internal {
		body["internal"] = true
	}
	resp, err := s.API.Call(ctx, "POST", "/v1/platform/tickets/"+p.TicketID+"/messages",
		nil, body, actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var out any
	_ = resp.JSON(&out)
	return out, nil
}

// ---------- refund_link_guardian ----------

type refundLinkGuardianParams struct {
	ClaimID string `json:"claim_id"`
	Note    string `json:"note"`
}

func proposeRefundLinkGuardian(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var p refundLinkGuardianParams
	if err := decodeArgs(raw, &p); err != nil {
		return nil, err
	}
	if p.ClaimID == "" {
		return nil, errors.New("claim_id wajib")
	}
	summary := fmt.Sprintf("Refund klaim Link Guardian <code>%s</code>", p.ClaimID)
	if p.Note != "" {
		summary += "\nCatatan: " + p.Note
	}
	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:               "refund_link_guardian",
		Summary:            summary,
		Params:             p,
		RequiredPermission: "payments",
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execRefundLinkGuardian(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p refundLinkGuardianParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	body := map[string]any{}
	if p.Note != "" {
		body["note"] = p.Note
	}
	resp, err := s.API.Call(ctx, "POST", "/v1/platform/link-guardian/claims/"+p.ClaimID+"/refund",
		nil, body, actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var out any
	_ = resp.JSON(&out)
	return out, nil
}

// ---------- send_partner_wa_reminder ----------

type sendPartnerWAReminderParams struct {
	PartnerID string `json:"partner_id"`
	Template  string `json:"template"`
	Note      string `json:"note"`
}

func proposeSendPartnerWAReminder(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var p sendPartnerWAReminderParams
	if err := decodeArgs(raw, &p); err != nil {
		return nil, err
	}
	if p.PartnerID == "" {
		return nil, errors.New("partner_id wajib")
	}
	summary := fmt.Sprintf("Kirim WA reminder ke mitra <code>%s</code>", p.PartnerID)
	if p.Template != "" {
		summary += "\nTemplate: " + p.Template
	}
	if p.Note != "" {
		summary += "\nCatatan: " + p.Note
	}
	action, err := s.Propose(ctx, client, ActionDraft{
		Tool:    "send_partner_wa_reminder",
		Summary: summary,
		Params:  p,
	})
	if err != nil {
		return nil, err
	}
	return proposeResult(action, s.sendCard(ctx, action))
}

func execSendPartnerWAReminder(ctx context.Context, s *Server, actAsUser string, agent *AgentHeader, raw json.RawMessage) (any, error) {
	var p sendPartnerWAReminderParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	body := map[string]any{}
	if p.Template != "" {
		body["template"] = p.Template
	}
	resp, err := s.API.Call(ctx, "POST", "/v1/platform/partners/"+p.PartnerID+"/wa-reminder",
		nil, body, actAsUser, agent)
	if err != nil {
		return nil, err
	}
	var out any
	_ = resp.JSON(&out)
	return out, nil
}

// ---------- utilitas ----------

// formatRupiah: 1500000 → "1.500.000".
func formatRupiah(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	// Sisipkan titik tiap 3 digit dari belakang.
	out := []byte{}
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, c)
	}
	return sign + string(out)
}
