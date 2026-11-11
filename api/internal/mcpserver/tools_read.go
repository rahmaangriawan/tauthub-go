package mcpserver

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
)

// registerReadTools: daftar tool baca awal. Semua memanggil endpoint
// /v1/platform/* lewat APIClient dengan actAsUser = agent_clients.read_as_user_id.
// Fase 1 — sengaja terbatas supaya cepat diuji; tambah tool baca berikut di
// fase berikutnya: list_clients, list_partners, list_media, list_orders,
// list_settlements, list_tickets, activity_feed, audit_log.
func (s *Server) registerReadTools() {
	s.Register(&Tool{
		Name:        "operations_summary",
		Description: "Ringkasan ekosistem: jumlah klien, mitra, media, publikasi, dan peringatan sistem. Satu panggilan untuk gambaran menyeluruh.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
		Handle:      handleOperationsSummary,
	})
	s.Register(&Tool{
		Name:        "attention_queue",
		Description: "Antrian yang butuh perhatian admin: media menunggu review, penarikan mitra pending, klaim top-up pending, tiket belum dibaca, item dead-letter. Hasilnya menggabungkan beberapa endpoint admin.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
		Handle:      handleAttentionQueue,
	})
	s.Register(&Tool{
		Name:        "finance_report",
		Description: "Ringkasan keuangan platform untuk rentang tanggal (saldo klien, pendapatan, komisi mitra). Butuh izin admin 'payments'.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"from": map[string]any{"type": "string", "format": "date", "description": "YYYY-MM-DD; default 30 hari lalu."},
				"to":   map[string]any{"type": "string", "format": "date", "description": "YYYY-MM-DD; default hari ini."},
			},
			"additionalProperties": false,
		},
		Handle: handleFinanceReport,
	})
	s.Register(&Tool{
		Name:        "list_media",
		Description: "Daftar media (situs mitra). Dapat difilter per status (pending_review, active, rejected, …), mitra, atau pencarian nama/domain.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":     map[string]any{"type": "string", "description": "Filter status: pending_review|active|inactive|archived|hidden|rejected."},
				"partner_id": map[string]any{"type": "string"},
				"search":     map[string]any{"type": "string"},
				"limit":      map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "default": 50},
				"offset":     map[string]any{"type": "integer", "minimum": 0, "default": 0},
			},
			"additionalProperties": false,
		},
		Handle: handleListMedia,
	})
	s.Register(&Tool{
		Name:        "list_settlements",
		Description: "Daftar penarikan mitra. Difilter per status (pending, approved, processing, paid, failed) dan rentang tanggal.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status": map[string]any{"type": "string"},
				"from":   map[string]any{"type": "string", "format": "date"},
				"to":     map[string]any{"type": "string", "format": "date"},
				"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "default": 50},
				"offset": map[string]any{"type": "integer", "minimum": 0, "default": 0},
			},
			"additionalProperties": false,
		},
		Handle: handleListSettlements,
	})
	s.Register(&Tool{
		Name:        "audit_log",
		Description: "Catatan aksi admin (audit_logs). Difilter per aktor, action, atau rentang tanggal.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"actor":  map[string]any{"type": "string"},
				"action": map[string]any{"type": "string"},
				"from":   map[string]any{"type": "string", "format": "date"},
				"to":     map[string]any{"type": "string", "format": "date"},
				"limit":  map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "default": 50},
				"offset": map[string]any{"type": "integer", "minimum": 0, "default": 0},
			},
			"additionalProperties": false,
		},
		Handle: handleAuditLog,
	})
}

// ---------- handler ----------

func handleOperationsSummary(ctx context.Context, s *Server, client *AgentClient, _ json.RawMessage) (any, error) {
	resp, err := s.API.Call(ctx, "GET", "/v1/platform/operations/summary", nil, nil, client.ReadAsUserID, nil)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := resp.JSON(&payload); err != nil {
		return nil, err
	}
	return textResult(payload)
}

type attentionCounts struct {
	MediaPendingReview int `json:"media_pending_review"`
	SettlementsPending int `json:"settlements_pending"`
	CreditReports      int `json:"credit_reports_pending"`
	UnreadTickets      int `json:"tickets_unread"`
	DeadLetters        int `json:"dead_letter_items"`
}

type attentionResult struct {
	Counts attentionCounts `json:"counts"`
	Media  any             `json:"media_pending_review_top,omitempty"`
	Settle any             `json:"settlements_pending_top,omitempty"`
	Credit any             `json:"credit_reports_pending_top,omitempty"`
	Errors []string        `json:"errors,omitempty"`
}

func handleAttentionQueue(ctx context.Context, s *Server, client *AgentClient, _ json.RawMessage) (any, error) {
	out := attentionResult{}
	// Ringkasan: satu panggilan /operations/summary untuk angka global.
	if resp, err := s.API.Call(ctx, "GET", "/v1/platform/operations/summary", nil, nil, client.ReadAsUserID, nil); err == nil {
		var sum map[string]any
		if err := resp.JSON(&sum); err == nil {
			out.Counts.DeadLetters = asInt(sum["dead_letter_items"])
		}
	} else {
		out.Errors = append(out.Errors, "summary: "+err.Error())
	}
	// Media pending review.
	{
		q := url.Values{}
		q.Set("status", "pending_review")
		q.Set("limit", "10")
		if resp, err := s.API.Call(ctx, "GET", "/v1/platform/media", q, nil, client.ReadAsUserID, nil); err == nil {
			var r map[string]any
			if err := resp.JSON(&r); err == nil {
				if items, ok := r["items"].([]any); ok {
					out.Media = items
					out.Counts.MediaPendingReview = len(items)
				}
				if total, ok := r["total"].(float64); ok {
					out.Counts.MediaPendingReview = int(total)
				}
			}
		} else {
			out.Errors = append(out.Errors, "media: "+err.Error())
		}
	}
	// Penarikan pending.
	{
		q := url.Values{}
		q.Set("status", "pending")
		q.Set("limit", "10")
		if resp, err := s.API.Call(ctx, "GET", "/v1/platform/settlements", q, nil, client.ReadAsUserID, nil); err == nil {
			var r map[string]any
			if err := resp.JSON(&r); err == nil {
				if items, ok := r["items"].([]any); ok {
					out.Settle = items
					out.Counts.SettlementsPending = len(items)
				}
				if total, ok := r["total"].(float64); ok {
					out.Counts.SettlementsPending = int(total)
				}
			}
		} else {
			out.Errors = append(out.Errors, "settlements: "+err.Error())
		}
	}
	// Klaim top-up pending.
	{
		q := url.Values{}
		q.Set("status", "pending")
		q.Set("limit", "10")
		if resp, err := s.API.Call(ctx, "GET", "/v1/platform/operations/credit-reports", q, nil, client.ReadAsUserID, nil); err == nil {
			var r map[string]any
			if err := resp.JSON(&r); err == nil {
				if items, ok := r["items"].([]any); ok {
					out.Credit = items
					out.Counts.CreditReports = len(items)
				}
				if total, ok := r["total"].(float64); ok {
					out.Counts.CreditReports = int(total)
				}
			}
		} else {
			out.Errors = append(out.Errors, "credit_reports: "+err.Error())
		}
	}
	// Tiket belum dibaca — endpoint khusus.
	{
		if resp, err := s.API.Call(ctx, "GET", "/v1/platform/tickets/unread-count", nil, nil, client.ReadAsUserID, nil); err == nil {
			var r map[string]any
			if err := resp.JSON(&r); err == nil {
				out.Counts.UnreadTickets = asInt(r["unread_count"])
				if out.Counts.UnreadTickets == 0 {
					out.Counts.UnreadTickets = asInt(r["count"])
				}
			}
		} else {
			out.Errors = append(out.Errors, "tickets: "+err.Error())
		}
	}
	return textResult(out)
}

type dateRangeArgs struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func handleFinanceReport(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var args dateRangeArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	q := url.Values{}
	if args.From != "" {
		q.Set("from", args.From)
	}
	if args.To != "" {
		q.Set("to", args.To)
	}
	resp, err := s.API.Call(ctx, "GET", "/v1/platform/finance/report", q, nil, client.ReadAsUserID, nil)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := resp.JSON(&payload); err != nil {
		return nil, err
	}
	return textResult(payload)
}

type listMediaArgs struct {
	Status    string `json:"status"`
	PartnerID string `json:"partner_id"`
	Search    string `json:"search"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
}

func handleListMedia(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var args listMediaArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	q := url.Values{}
	if args.Status != "" {
		q.Set("status", args.Status)
	}
	if args.PartnerID != "" {
		q.Set("partner_id", args.PartnerID)
	}
	if args.Search != "" {
		q.Set("search", args.Search)
	}
	if args.Limit <= 0 {
		args.Limit = 50
	}
	q.Set("limit", strconv.Itoa(args.Limit))
	q.Set("offset", strconv.Itoa(args.Offset))
	resp, err := s.API.Call(ctx, "GET", "/v1/platform/media", q, nil, client.ReadAsUserID, nil)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := resp.JSON(&payload); err != nil {
		return nil, err
	}
	return textResult(payload)
}

type listSettlementsArgs struct {
	Status string `json:"status"`
	From   string `json:"from"`
	To     string `json:"to"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func handleListSettlements(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var args listSettlementsArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	q := url.Values{}
	if args.Status != "" {
		q.Set("status", args.Status)
	}
	if args.From != "" {
		q.Set("from", args.From)
	}
	if args.To != "" {
		q.Set("to", args.To)
	}
	if args.Limit <= 0 {
		args.Limit = 50
	}
	q.Set("limit", strconv.Itoa(args.Limit))
	q.Set("offset", strconv.Itoa(args.Offset))
	resp, err := s.API.Call(ctx, "GET", "/v1/platform/settlements", q, nil, client.ReadAsUserID, nil)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := resp.JSON(&payload); err != nil {
		return nil, err
	}
	return textResult(payload)
}

type auditLogArgs struct {
	Actor  string `json:"actor"`
	Action string `json:"action"`
	From   string `json:"from"`
	To     string `json:"to"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

func handleAuditLog(ctx context.Context, s *Server, client *AgentClient, raw json.RawMessage) (any, error) {
	var args auditLogArgs
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	q := url.Values{}
	if args.Actor != "" {
		q.Set("actor", args.Actor)
	}
	if args.Action != "" {
		q.Set("action", args.Action)
	}
	if args.From != "" {
		q.Set("from", args.From)
	}
	if args.To != "" {
		q.Set("to", args.To)
	}
	if args.Limit <= 0 {
		args.Limit = 50
	}
	q.Set("limit", strconv.Itoa(args.Limit))
	q.Set("offset", strconv.Itoa(args.Offset))
	resp, err := s.API.Call(ctx, "GET", "/v1/platform/operations/audit-log", q, nil, client.ReadAsUserID, nil)
	if err != nil {
		return nil, err
	}
	var payload any
	if err := resp.JSON(&payload); err != nil {
		return nil, err
	}
	return textResult(payload)
}

// asInt: konversi best-effort any → int (menangani float64 dari JSON).
func asInt(v any) int {
	switch n := v.(type) {
	case nil:
		return 0
	case int:
		return n
	case float64:
		return int(n)
	case string:
		i, err := strconv.Atoi(n)
		if err != nil {
			return 0
		}
		return i
	}
	return 0
}
