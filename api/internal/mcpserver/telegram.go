package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Telegram: bot Persetujuan. Minimal, berbasis long polling (getUpdates) —
// cara ini tidak butuh webhook publik, jadi aman dipasang di balik firewall
// VPS. Satu instance saja boleh polling pada satu waktu; tidak masalah
// karena cmd/mcp hanya jalan di VPS (Mac mini terhubung ke MCP via HTTPS,
// tidak menjalankan botnya).
//
// API Telegram yang dipakai:
//   - sendMessage          (kirim kartu persetujuan)
//   - editMessageText      (update isi kartu setelah diputuskan)
//   - getUpdates           (long poll dengan allowed_updates=["callback_query"])
//   - answerCallbackQuery  (konfirmasi klik supaya tombol tidak loading)
type Telegram struct {
	Token  string
	HTTP   *http.Client
	server *Server
	offset int64
}

// NewTelegram: siapkan bot. Server butuh *Server karena bot memeriksa
// permission dan memanggil executor saat tombol ditekan.
func NewTelegram(token string, server *Server) *Telegram {
	return &Telegram{
		Token:  token,
		HTTP:   &http.Client{Timeout: 45 * time.Second},
		server: server,
	}
}

// Run: loop long polling. Dihentikan saat ctx.Done.
func (t *Telegram) Run(ctx context.Context) {
	slog.Info("[telegram] bot Persetujuan mulai")
	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := t.getUpdates(ctx, 25)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			slog.Warn("[telegram] getUpdates gagal", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}
		for _, u := range updates {
			if u.UpdateID >= t.offset {
				t.offset = u.UpdateID + 1
			}
			if u.CallbackQuery != nil {
				t.handleCallback(ctx, u.CallbackQuery)
			}
		}
	}
}

// ---------- model minimal Telegram ----------

type tgUpdate struct {
	UpdateID      int64            `json:"update_id"`
	CallbackQuery *tgCallbackQuery `json:"callback_query"`
}

type tgCallbackQuery struct {
	ID      string     `json:"id"`
	From    tgUser     `json:"from"`
	Message *tgMessage `json:"message"`
	Data    string     `json:"data"`
}

type tgUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type tgMessage struct {
	MessageID int64  `json:"message_id"`
	Chat      tgChat `json:"chat"`
}

type tgChat struct {
	ID int64 `json:"id"`
}

type tgResp struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description"`
	Result      json.RawMessage `json:"result"`
}

// ---------- API Telegram ----------

func (t *Telegram) call(ctx context.Context, method string, form url.Values) (json.RawMessage, error) {
	u := "https://api.telegram.org/bot" + t.Token + "/" + method
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := t.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var out tgResp
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("telegram parse: %w (status %d)", err, resp.StatusCode)
	}
	if !out.OK {
		return nil, fmt.Errorf("telegram %s: %s", method, out.Description)
	}
	return out.Result, nil
}

func (t *Telegram) getUpdates(ctx context.Context, timeoutSec int) ([]tgUpdate, error) {
	form := url.Values{}
	form.Set("timeout", strconv.Itoa(timeoutSec))
	if t.offset > 0 {
		form.Set("offset", strconv.FormatInt(t.offset, 10))
	}
	form.Set("allowed_updates", `["callback_query"]`)
	raw, err := t.call(ctx, "getUpdates", form)
	if err != nil {
		return nil, err
	}
	var updates []tgUpdate
	if err := json.Unmarshal(raw, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

// SendApprovalCard: kirim kartu Persetujuan ke setiap chat di allowlist.
// Kembalikan (chatID, messageID) pengiriman pertama yang sukses — supaya
// agent_actions.telegram_message_id bisa dicatat untuk menyunting nanti.
// Pengiriman ke chat lain tetap dicoba (best-effort), hanya saja mereka
// tidak ter-track untuk edit; itu diterima karena hanya satu admin yang
// akan menekan.
func (t *Telegram) SendApprovalCard(ctx context.Context, chatIDs []int64, action *Action) (int64, int64, error) {
	if len(chatIDs) == 0 {
		return 0, 0, errors.New("tidak ada chat_id Telegram yang diizinkan (TELEGRAM_APPROVAL_CHAT_IDS kosong)")
	}
	text := formatApprovalText(action)
	keyboard := map[string]any{
		"inline_keyboard": [][]map[string]any{
			{
				{"text": "✅ Setujui", "callback_data": "ok:" + action.Code},
				{"text": "⛔️ Tolak", "callback_data": "no:" + action.Code},
			},
		},
	}
	kbJSON, _ := json.Marshal(keyboard)

	var firstChatID, firstMsgID int64
	var firstErr error
	for i, chatID := range chatIDs {
		form := url.Values{}
		form.Set("chat_id", strconv.FormatInt(chatID, 10))
		form.Set("text", text)
		form.Set("parse_mode", "HTML")
		form.Set("reply_markup", string(kbJSON))
		raw, err := t.call(ctx, "sendMessage", form)
		if err != nil {
			slog.Warn("[telegram] sendMessage gagal", "chat", chatID, "err", err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		var msg tgMessage
		if err := json.Unmarshal(raw, &msg); err == nil && i == 0 {
			firstChatID = chatID
			firstMsgID = msg.MessageID
		}
		if firstChatID == 0 {
			firstChatID = chatID
			if err := json.Unmarshal(raw, &msg); err == nil {
				firstMsgID = msg.MessageID
			}
		}
	}
	if firstChatID == 0 {
		return 0, 0, firstErr
	}
	return firstChatID, firstMsgID, nil
}

// ---------- handler callback ----------

func (t *Telegram) handleCallback(ctx context.Context, cq *tgCallbackQuery) {
	_ = t.answerCallback(ctx, cq.ID, "") // kecilkan loading tombol lebih dulu

	data := cq.Data
	if len(data) < 4 {
		return
	}
	verb := data[:2]
	code := data[3:]
	if verb != "ok" && verb != "no" {
		return
	}

	// Allowlist chat.
	if !t.chatAllowed(cq.Message) {
		slog.Warn("[telegram] callback dari chat di luar allowlist", "chat", cq.Message.Chat.ID)
		t.replyAlert(ctx, cq.ID, "Chat ini tidak diizinkan.")
		return
	}

	// Siapa yang menekan? → admin.
	adminID, _, role, perms, err := t.server.AdminForTelegramUser(ctx, cq.From.ID)
	if err != nil {
		t.replyAlert(ctx, cq.ID, "Akun Telegram Anda belum terdaftar sebagai admin.")
		return
	}

	// Lookup aksi.
	action, err := t.server.LookupByCode(ctx, code)
	if err != nil {
		t.replyAlert(ctx, cq.ID, "Aksi tidak ditemukan atau sudah hilang.")
		return
	}
	if action.Status != "proposed" {
		t.replyAlert(ctx, cq.ID, fmt.Sprintf("Aksi sudah %s, tidak bisa diubah.", action.Status))
		t.updateCardAfter(ctx, action)
		return
	}
	if time.Now().After(action.ExpiresAt) {
		_, _ = t.server.Pool.Exec(ctx,
			`UPDATE agent_actions SET status='expired' WHERE id=$1 AND status='proposed'`, action.ID)
		t.replyAlert(ctx, cq.ID, "Aksi kedaluwarsa.")
		return
	}

	if !CanApprove(role, perms, action.RequiredPermission) {
		t.replyAlert(ctx, cq.ID, "Anda tidak punya izin untuk aksi ini.")
		return
	}

	switch verb {
	case "no":
		if err := t.server.Reject(ctx, action.ID, adminID, "ditolak via Telegram"); err != nil {
			t.replyAlert(ctx, cq.ID, "Gagal menolak: "+err.Error())
			return
		}
		action.Status = "rejected"
		action.ApprovedBy = &adminID
		t.updateCardAfter(ctx, action)
	case "ok":
		if err := t.server.ClaimForExecution(ctx, action.ID, adminID); err != nil {
			t.replyAlert(ctx, cq.ID, "Gagal menyetujui: "+err.Error())
			return
		}
		// Eksekutor berjalan di goroutine singkat supaya callback cepat selesai.
		go t.execute(context.Background(), action, adminID)
	}
}

// execute: jalankan executor sesuai nama tool, lalu update kartu.
func (t *Telegram) execute(ctx context.Context, action *Action, approverID string) {
	ex, ok := executors[action.Tool]
	if !ok {
		_ = t.server.MarkFailed(ctx, action.ID, "executor tidak ditemukan untuk tool "+action.Tool)
		action.Status = "failed"
		t.updateCardAfter(ctx, action)
		return
	}
	agentCtx := &AgentHeader{
		ClientID:   action.ClientID,
		ActionID:   action.ID,
		ActionCode: action.Code,
	}
	result, err := ex(ctx, t.server, approverID, agentCtx, action.Params)
	if err != nil {
		_ = t.server.MarkFailed(ctx, action.ID, err.Error())
		action.Status = "failed"
		action.ErrorMsg = err.Error()
	} else {
		_ = t.server.MarkExecuted(ctx, action.ID, result)
		action.Status = "executed"
	}
	t.updateCardAfter(ctx, action)
}

// updateCardAfter: edit teks kartu dengan status akhir, tanpa tombol.
func (t *Telegram) updateCardAfter(ctx context.Context, action *Action) {
	if action.TelegramChatID == nil || action.TelegramMessageID == nil {
		return
	}
	form := url.Values{}
	form.Set("chat_id", strconv.FormatInt(*action.TelegramChatID, 10))
	form.Set("message_id", strconv.FormatInt(*action.TelegramMessageID, 10))
	form.Set("parse_mode", "HTML")
	form.Set("text", formatApprovalText(action))
	form.Set("reply_markup", `{"inline_keyboard":[]}`)
	if _, err := t.call(ctx, "editMessageText", form); err != nil {
		slog.Warn("[telegram] editMessageText gagal", "err", err)
	}
}

func (t *Telegram) answerCallback(ctx context.Context, cqID, alertText string) error {
	form := url.Values{}
	form.Set("callback_query_id", cqID)
	if alertText != "" {
		form.Set("text", alertText)
		form.Set("show_alert", "true")
	}
	_, err := t.call(ctx, "answerCallbackQuery", form)
	return err
}

func (t *Telegram) replyAlert(ctx context.Context, cqID, text string) {
	_ = t.answerCallback(ctx, cqID, text)
}

func (t *Telegram) chatAllowed(m *tgMessage) bool {
	if m == nil {
		return false
	}
	for _, id := range t.server.MCPConfig.Snapshot().TelegramChatIDs {
		if id == m.Chat.ID {
			return true
		}
	}
	return false
}

// ---------- rendering kartu ----------

func formatApprovalText(a *Action) string {
	var b strings.Builder
	switch a.Status {
	case "proposed":
		b.WriteString("🔔 <b>Butuh persetujuan</b>\n")
	case "executed":
		b.WriteString("✅ <b>Dieksekusi</b>\n")
	case "failed":
		b.WriteString("⚠️ <b>Gagal</b>\n")
	case "rejected":
		b.WriteString("⛔️ <b>Ditolak</b>\n")
	case "expired":
		b.WriteString("⌛️ <b>Kedaluwarsa</b>\n")
	default:
		b.WriteString("• <b>" + a.Status + "</b>\n")
	}
	b.WriteString("<b>Tool:</b> ")
	b.WriteString(htmlEscape(a.Tool))
	b.WriteString("\n<b>Kode:</b> <code>")
	b.WriteString(htmlEscape(a.Code))
	b.WriteString("</code>\n\n")
	b.WriteString(htmlEscape(a.Summary))
	if a.ErrorMsg != "" {
		b.WriteString("\n\n<b>Error:</b> ")
		b.WriteString(htmlEscape(a.ErrorMsg))
	}
	if a.ApprovedBy != nil && *a.ApprovedBy != "" {
		b.WriteString("\n<b>Oleh admin:</b> <code>")
		b.WriteString(htmlEscape(*a.ApprovedBy))
		b.WriteString("</code>")
	}
	return b.String()
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
