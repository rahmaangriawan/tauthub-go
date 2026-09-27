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
	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	knowledgedomain "github.com/rahmaangriawan/tauthub-go/internal/domains/knowledge"
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
		Pool: database.Pool,
		Cfg:  cfg,
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
		Google: &auth.Google{
			Pool:            database.Pool,
			AppSecret:       cfg.Secret,
			CmsSecret:       cfg.CmsEncryptionSecret,
			EnvClientID:     cfg.GoogleClientID,
			EnvClientSecret: cfg.GoogleClientSecret,
			EnvRedirectURI:  cfg.GoogleRedirectURI,
			IsProduction:    cfg.IsProduction(),
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
		v1.Post("/auth/register", authDeps.Register)
		v1.Post("/auth/forgot-password", authDeps.ForgotPassword)
		v1.Post("/auth/reset-password", authDeps.ResetPassword)
		v1.Post("/auth/2fa/verify", authDeps.TwoFactorVerify)
		v1.Get("/auth/turnstile", authDeps.TurnstileConfig)
		v1.Get("/auth/google/url", authDeps.GoogleURL)
		v1.Get("/auth/google/callback", authDeps.GoogleCallback)
		v1.Post("/auth/google/complete", authDeps.GoogleComplete)
		v1.Post("/auth/google/link", authDeps.GoogleLink)
		v1.Post("/auth/google/exchange-session", authDeps.GoogleExchangeSession)

		// Brands (fase 2) — butuh Bearer.
		brands := &brandsdomain.Deps{Pool: database.Pool}
		knowledge := &knowledgedomain.Deps{Pool: database.Pool}
		v1.Group(func(authed chi.Router) {
			authed.Use(authDeps.Auth.RequireAuth)
			authed.Get("/auth/me", authDeps.Me)
			authed.Patch("/auth/me", authDeps.MeUpdate)
			authed.Post("/auth/2fa/setup", authDeps.TwoFactorSetup)
			authed.Post("/auth/2fa/enable", authDeps.TwoFactorEnable)
			authed.Post("/auth/2fa/disable", authDeps.TwoFactorDisable)
			authed.Post("/auth/change-password", authDeps.ChangePassword)
			authed.Get("/me/notification-preferences", authDeps.NotificationPreferencesGet)
			authed.Put("/me/notification-preferences", authDeps.NotificationPreferencesPut)

			authed.Get("/brands", brands.List)
			authed.Post("/brands", brands.Create)
			authed.Get("/brands/{brandId}", brands.Get)
			authed.Post("/brands/{brandId}/activate", brands.Activate)
			authed.Put("/brands/{brandId}/profile", brands.UpdateProfile)
			authed.Put("/brands/{brandId}/voice", brands.UpdateVoice)
			authed.Put("/brands/{brandId}/editorial-rules", brands.UpdateRules)
			authed.Get("/brands/{brandId}/configuration", brands.Configuration)
			authed.Get("/brands/{brandId}/dashboard", brands.Dashboard)
			authed.Get("/brands/{brandId}/overview", brands.Overview)
			authed.Post("/brands/{brandId}/prompts", brands.CreatePrompt)
			authed.Post("/brands/{brandId}/prompts/{promptId}/activate", brands.ActivatePrompt)
			authed.Get("/brands/{brandId}/members", brands.MembersList)
			authed.Post("/brands/{brandId}/members", brands.MembersCreate)
			authed.Put("/brands/{brandId}/members/{membershipId}", brands.MemberUpdate)
			authed.Delete("/brands/{brandId}/members/{membershipId}", brands.MemberDelete)

			// Knowledge (fase 3).
			authed.Get("/brands/{brandId}/knowledge/collections", knowledge.CollectionsList)
			authed.Post("/brands/{brandId}/knowledge/collections", knowledge.CollectionsCreate)
			authed.Get("/brands/{brandId}/knowledge/collections/{collectionId}", knowledge.CollectionGet)
			authed.Put("/brands/{brandId}/knowledge/collections/{collectionId}", knowledge.CollectionUpdate)
			authed.Delete("/brands/{brandId}/knowledge/collections/{collectionId}", knowledge.CollectionDelete)
			authed.Get("/brands/{brandId}/knowledge/collections/{collectionId}/sources", knowledge.SourcesList)
			authed.Post("/brands/{brandId}/knowledge/collections/{collectionId}/sources", knowledge.SourcesCreate)
			authed.Get("/brands/{brandId}/knowledge/sources/{sourceId}", knowledge.SourceGet)
			authed.Delete("/brands/{brandId}/knowledge/sources/{sourceId}", knowledge.SourceDelete)
			authed.Get("/brands/{brandId}/knowledge/sources/{sourceId}/content", knowledge.SourceContent)
			authed.Post("/brands/{brandId}/knowledge/sitemap", knowledge.SitemapCrawl)
			authed.Get("/brands/{brandId}/knowledge/sitemap", knowledge.SitemapStatus)
			authed.Get("/brands/{brandId}/chat-learning", knowledge.ChatLearningList)
			authed.Delete("/brands/{brandId}/chat-learning", knowledge.ChatLearningDelete)
		})

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
