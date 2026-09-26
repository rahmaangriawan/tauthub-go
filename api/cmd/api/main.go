package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	authdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

func main() {
	if err := run(); err != nil {
		slog.Error("api berhenti dengan error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	database, err := db.Open(cfg.DBDSN(), cfg.PoolSize)
	if err != nil {
		return err
	}
	defer database.Close()
	slog.Info("db terhubung", "host", cfg.DBHost, "name", cfg.DBName)

	authDeps := &authdomain.Deps{
		Pool:      database.Pool,
		Cfg:       cfg,
		Turnstile: &auth.Turnstile{
			Pool:                  database.Pool,
			TurnstileSiteKeyEnv:   os.Getenv("TURNSTILE_SITE_KEY"),
			TurnstileSecretKeyEnv: os.Getenv("TURNSTILE_SECRET_KEY"),
			CmsEncryptionSecret:   cfg.CmsEncryptionSecret,
		},
		Auth: &httpx.Authenticator{
			Pool:   database.Pool,
			Secret: cfg.Secret,
			Issuer: cfg.AppName,
		},
	}

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(httpx.Recover(cfg.AppEnv))

	// healthz — cek hidup + DB.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := database.Ping(ctx); err != nil {
			slog.Error("healthz: db ping gagal", "err", err)
			httpx.Error(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "db": "ok", "app": cfg.AppName, "version": cfg.Version})
	})

	// /v1 — CORS allowlist (port proxy.ts) + route per domain.
	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(httpx.CORSMiddleware)

		// Auth publik (fase 1).
		v1.Post("/auth/login", authDeps.Login)
		v1.Post("/auth/refresh", authDeps.Refresh)
		v1.Post("/auth/logout", authDeps.Logout)
		v1.Post("/auth/2fa/verify", authDeps.TwoFactorVerify)
		v1.Get("/auth/turnstile", authDeps.TurnstileConfig)

		// Butuh Bearer access token.
		v1.Group(func(authed chi.Router) {
			authed.Use(authDeps.Auth.RequireAuth)
			authed.Get("/auth/me", authDeps.Me)
			authed.Patch("/auth/me", authDeps.MeUpdate)
		})

		// TODO fase 1 lanjutan: register, 2fa setup/enable/disable,
		// change/forgot/reset password, google oauth, me/notification-preferences.
		// TODO fase 2+: peta lengkap di REWRITE/04_API_ENDPOINTS.md.
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("api listening", "port", cfg.Port, "env", cfg.AppEnv)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen gagal", "err", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	slog.Info("shutdown dimulai")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
