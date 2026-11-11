// Binary tauthub-mcp: MCP (Model Context Protocol) server untuk admin
// dashboard Tauthub. Dijalankan sebagai service systemd terpisah (lihat
// deploy/tauthub-mcp.service) dan memanggil /v1/platform/* via loopback.
//
// Lihat plans/ini-aku-baru-ngide-gentle-bunny.md untuk arsitektur lengkap.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/mcpconfig"
	"github.com/rahmaangriawan/tauthub-go/internal/mcpserver"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config: load gagal", "err", err)
		os.Exit(1)
	}
	if cfg.APIInternalURL == "" {
		slog.Error("API_INTERNAL_URL kosong (butuh alamat internal cmd/api, mis. http://127.0.0.1:8080)")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	database, err := db.Open(cfg.DBDSN(), cfg.PoolSize)
	if err != nil {
		slog.Error("db: open gagal", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	bootCtx, bootCancel := context.WithTimeout(ctx, 5*time.Second)
	store, err := mcpconfig.New(bootCtx, database.Pool, cfg.Secret, cfg.CmsEncryptionSecret)
	bootCancel()
	if err != nil {
		slog.Error("mcpconfig: load gagal", "err", err)
		os.Exit(1)
	}
	snap := store.Snapshot()
	if snap.ServiceKey == "" {
		slog.Warn("service key MCP belum di-set. Server tetap start, tapi semua panggilan dari agent akan ditolak sampai admin mengisi di /admin/konfigurasi/mcp")
	}

	// Dengarkan rotasi di DB — getter per-request mengambil snapshot terbaru.
	go store.ListenForChanges(ctx)

	srv := mcpserver.New(cfg, database.Pool, store)
	// Bot Telegram & listener butuh token saat start. Jika token di-set
	// belakangan, admin harus `systemctl restart tauthub-mcp` supaya loop
	// bot aktif. Rotasi key, chat IDs, writes toggle, dan batas kredit
	// tetap live tanpa restart.
	if snap.TelegramBotToken != "" {
		tg := mcpserver.NewTelegram(snap.TelegramBotToken, srv)
		srv.AttachTelegram(tg)
		go tg.Run(ctx)
		go srv.RunListener(ctx)
	} else {
		slog.Warn("bot token Telegram kosong — bot Persetujuan tidak aktif; tool tulis tetap menyimpan draft")
	}

	httpSrv := &http.Server{
		Addr:              ":" + cfg.MCPPort,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	slog.Info("tauthub-mcp dengar", "addr", httpSrv.Addr, "api", cfg.APIInternalURL,
		"writes", snap.WritesEnabled, "bot", snap.TelegramBotToken != "")
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("mcp: listen gagal", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("tauthub-mcp: shutdown")
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(sctx)
}
