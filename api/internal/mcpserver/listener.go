package mcpserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"strconv"
	"time"
)

// Set event yang diteruskan ke Telegram. Semua event lain tetap muncul di
// notifications tapi tidak membangunkan bot — mencegah banjir pesan.
var agentEventAllowlist = map[string]bool{
	"media.submitted":             true,
	"media.resubmitted":           true,
	"media.dewan_pers.submitted":  true,
	"withdrawal.requested":        true,
	"publication.link_lost":       true,
	"variant_batch.failed":        true,
	"platform_admin.credit_claim": true, // nama hipotetis; aman walau tidak ada
}

type agentEvent struct {
	Type  string `json:"type"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Link  string `json:"link"`
}

// RunListener: subscribe LISTEN agent_events dan forward ke Telegram. Dipanggil
// cmd/mcp di goroutine. Reconnect otomatis bila koneksi putus.
func (s *Server) RunListener(ctx context.Context) {
	if s.Telegram == nil {
		slog.Info("[listener] bot Telegram tidak aktif; listener agent_events tidak dijalankan")
		return
	}
	slog.Info("[listener] mulai subscribe agent_events (pg_notify)")
	for {
		if ctx.Err() != nil {
			return
		}
		if err := s.listenOnce(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("[listener] gagal, reconnect 5s", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (s *Server) listenOnce(ctx context.Context) error {
	conn, err := s.Pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN agent_events"); err != nil {
		return err
	}
	for {
		notif, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		var ev agentEvent
		if err := json.Unmarshal([]byte(notif.Payload), &ev); err != nil {
			slog.Warn("[listener] payload tidak valid", "err", err)
			continue
		}
		if !agentEventAllowlist[ev.Type] {
			continue
		}
		go s.deliverEvent(context.Background(), ev)
	}
}

// deliverEvent: kirim pesan Telegram sederhana ke setiap chat allowlist.
// Tidak membuat agent_actions — ini murni notifikasi yang mengundang admin
// memeriksa dashboard, bukan aksi yang menunggu persetujuan.
func (s *Server) deliverEvent(ctx context.Context, ev agentEvent) {
	chatIDs := s.MCPConfig.Snapshot().TelegramChatIDs
	if len(chatIDs) == 0 {
		return
	}
	text := "🔔 <b>" + htmlEscape(ev.Title) + "</b>\n" + htmlEscape(ev.Body)
	if ev.Link != "" {
		full := ev.Link
		if s.Cfg.PublicAppURL != "" && len(ev.Link) > 0 && ev.Link[0] == '/' {
			full = s.Cfg.PublicAppURL + ev.Link
		}
		text += "\n\n<a href=\"" + htmlEscape(full) + "\">Buka di dashboard</a>"
	}
	for _, chat := range chatIDs {
		form := url.Values{}
		form.Set("chat_id", strconv.FormatInt(chat, 10))
		form.Set("text", text)
		form.Set("parse_mode", "HTML")
		form.Set("disable_web_page_preview", "true")
		if _, err := s.Telegram.call(ctx, "sendMessage", form); err != nil {
			slog.Warn("[listener] kirim Telegram gagal", "chat", chat, "err", err)
		}
	}
}
