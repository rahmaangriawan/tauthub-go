package platform

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// notifyAgentEvent: kirim event lewat pg_notify('agent_events', json). Dipakai
// notifyPlatformAdmins (dan padanannya di brandflow) sebagai broadcast ke
// cmd/mcp supaya bot Telegram dapat memberi notifikasi proaktif.
//
// Payload JSON: {type, title, body, link}. Konsumer (mcpserver.listener)
// memfilter type yang boleh diteruskan ke Telegram.
//
// Fungsi ini best-effort; kegagalan pg_notify tidak boleh membatalkan alur
// utama (insert notifications sudah terjadi di caller).
func notifyAgentEvent(ctx context.Context, pool *pgxpool.Pool, notifType, title, body string, link *string) {
	linkStr := ""
	if link != nil {
		linkStr = *link
	}
	buf, err := json.Marshal(map[string]any{
		"type": notifType, "title": title, "body": body, "link": linkStr,
	})
	if err != nil {
		slog.Warn("[platform] pg_notify agent_events encode gagal", "err", err)
		return
	}
	// pg_notify menerima payload teks sampai 8000 byte; kita sudah aman.
	if _, err := pool.Exec(ctx, "SELECT pg_notify('agent_events', $1)", string(buf)); err != nil {
		slog.Warn("[platform] pg_notify agent_events gagal", "err", err)
	}
}
