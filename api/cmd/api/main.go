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
	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	authdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/auth"
	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	clientreaddomain "github.com/rahmaangriawan/tauthub-go/internal/domains/clientread"
	contentdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/content"
	knowledgedomain "github.com/rahmaangriawan/tauthub-go/internal/domains/knowledge"
	partnerdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/partner"
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
		contentDeps := &contentdomain.Deps{Pool: database.Pool, JobInline: cfg.JobInline, WriteRateLimitPerMinute: cfg.WriteRateLimitPerMinute}
		clientRead := &clientreaddomain.Deps{Pool: database.Pool}
		partnerDeps := &partnerdomain.Deps{Pool: database.Pool, Cfg: cfg}
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

			// Content pipeline (fase 4).
			authed.Get("/brands/{brandId}/content-jobs", contentDeps.JobsList)
			authed.Post("/brands/{brandId}/content-jobs", contentDeps.JobsCreate)
			authed.Get("/brands/{brandId}/content-jobs/{jobId}", contentDeps.JobGet)
			authed.Post("/brands/{brandId}/content-jobs/{jobId}/approve-outline", contentDeps.JobApproveOutline)
			authed.Put("/brands/{brandId}/content-jobs/{jobId}/outline", contentDeps.JobUpdateOutline)
			authed.Post("/brands/{brandId}/content-jobs/{jobId}/cancel", contentDeps.JobCancel)
			authed.Get("/brands/{brandId}/documents", contentDeps.DocumentsList)
			authed.Post("/brands/{brandId}/documents", contentDeps.DocumentsCreate)
			authed.Get("/brands/{brandId}/documents/{documentId}", contentDeps.DocumentGet)
			authed.Put("/brands/{brandId}/documents/{documentId}", contentDeps.DocumentUpdate)
			authed.Get("/brands/{brandId}/documents/{documentId}/html", contentDeps.DocumentHTML)

			// Content intakes (fase 4 slice B).
			authed.Get("/brands/{brandId}/content-intake-types", contentDeps.IntakeTypesList)
			authed.Get("/brands/{brandId}/content-intakes", contentDeps.IntakeList)
			authed.Post("/brands/{brandId}/content-intakes", contentDeps.IntakeCreate)
			authed.Get("/brands/{brandId}/content-intakes/{intakeId}", contentDeps.IntakeGet)
			authed.Put("/brands/{brandId}/content-intakes/{intakeId}", contentDeps.IntakeUpdate)
			authed.Post("/brands/{brandId}/content-intakes/{intakeId}/analyze", contentDeps.IntakeAnalyze)
			authed.Post("/brands/{brandId}/content-intakes/{intakeId}/create-job", contentDeps.IntakeCreateJob)
			authed.Post("/brands/{brandId}/chat", contentDeps.Chat)

			// Section BACA klien (katalog media, kampanye, pelacak 404, analitik, kredit, invoice).
			authed.Get("/brands/{brandId}/media", clientRead.MediaCatalog)
			authed.Get("/brands/{brandId}/media/categories", clientRead.MediaCategories)
			authed.Get("/brands/{brandId}/media/{mediaId}", clientRead.MediaDetail)
			authed.Get("/brands/{brandId}/campaigns", clientRead.Campaigns)
			authed.Get("/brands/{brandId}/link-monitor", clientRead.LinkMonitor)
			authed.Get("/brands/{brandId}/analytics", clientRead.Analytics)
			authed.Get("/brands/{brandId}/credits", clientRead.Credits)
			authed.Get("/brands/{brandId}/invoices", clientRead.Invoices)

			// ---------- Portal mitra (/mitra) — port app/v1/partner/* ----------
			authed.Get("/partner/me", partnerDeps.Me)
			authed.Patch("/partner/me", partnerDeps.MeUpdate)
			authed.Get("/partner/me/vip", partnerDeps.MeVip)

			authed.Get("/partner/media", partnerDeps.MediaList)
			authed.Post("/partner/media", partnerDeps.MediaCreate)
			authed.Post("/partner/media/import", partnerDeps.MediaImport)
			authed.Get("/partner/media/categories", partnerDeps.MediaCategories)
			authed.Get("/partner/media/template-url", partnerDeps.MediaTemplateURL)
			authed.Get("/partner/media/{mediaId}", partnerDeps.MediaGet)
			authed.Put("/partner/media/{mediaId}", partnerDeps.MediaUpdate)
			authed.Delete("/partner/media/{mediaId}", partnerDeps.MediaDelete)
			authed.Post("/partner/media/{mediaId}/submit", partnerDeps.MediaSubmit)
			authed.Patch("/partner/media/{mediaId}/category", partnerDeps.MediaCategory)
			authed.Get("/partner/media/{mediaId}/negotiation", partnerDeps.MediaNegotiation)
			authed.Post("/partner/media/{mediaId}/negotiation", partnerDeps.MediaNegotiation)
			authed.Get("/partner/media/{mediaId}/ai-summary", partnerDeps.MediaAISummary)
			authed.Get("/partner/media/{mediaId}/similarweb", partnerDeps.MediaSimilarWeb)
			authed.Post("/partner/media/{mediaId}/similarweb", partnerDeps.MediaSimilarWeb)
			authed.Get("/partner/media/{mediaId}/domain-verification", partnerDeps.MediaDomainVerification)
			authed.Post("/partner/media/{mediaId}/domain-verification", partnerDeps.MediaDomainVerification)
			authed.Post("/partner/media/{mediaId}/dewan-pers-document", partnerDeps.MediaDewanPersDocument)
			authed.Post("/partner/media/{mediaId}/dewan-pers-verification", partnerDeps.MediaDewanPersVerification)
			authed.Get("/partner/media/{mediaId}/seo", partnerDeps.MediaSEO)
			authed.Post("/partner/media/{mediaId}/seo", partnerDeps.MediaSEO)
			authed.Post("/partner/media/{mediaId}/test-publishing-api", partnerDeps.MediaTestPublishingAPI)
			authed.Patch("/partner/media/{mediaId}/custom-web", partnerDeps.MediaCustomWeb)
			authed.Post("/partner/media/{mediaId}/custom-web", partnerDeps.MediaCustomWeb)
			authed.Post("/partner/media/{mediaId}/custom-web/test-connection", partnerDeps.MediaCustomWebTest)
			authed.Post("/partner/media/{mediaId}/blogger", partnerDeps.MediaBlogger)
			authed.Delete("/partner/media/{mediaId}/blogger", partnerDeps.MediaBlogger)
			authed.Get("/partner/media/{mediaId}/blogger/blogs", partnerDeps.MediaBloggerBlogs)
			authed.Get("/partner/media/{mediaId}/blogger/connect", partnerDeps.MediaBloggerConnect)
			authed.Post("/partner/media/{mediaId}/blogger/test-connection", partnerDeps.MediaBloggerTest)

			authed.Get("/partner/publications", partnerDeps.PublicationsList)
			authed.Get("/partner/publications/{publicationId}", partnerDeps.PublicationGet)
			authed.Post("/partner/publications/{publicationId}/approve", partnerDeps.PublicationApprove)
			authed.Post("/partner/publications/{publicationId}/reject", partnerDeps.PublicationReject)
			authed.Post("/partner/publications/{publicationId}/url", partnerDeps.PublicationURL)
			authed.Get("/partner/publications/{publicationId}/draft", partnerDeps.PublicationDraft)
			authed.Put("/partner/publications/{publicationId}/draft", partnerDeps.PublicationDraft)
			authed.Post("/partner/publications/{publicationId}/draft/image", partnerDeps.PublicationDraftImage)
			authed.Post("/partner/publications/{publicationId}/rewrite", partnerDeps.PublicationRewrite)

			authed.Get("/partner/link-monitor", partnerDeps.LinkMonitor)
			authed.Get("/partner/crawler", partnerDeps.Crawler)
			authed.Post("/partner/crawler", partnerDeps.Crawler)

			authed.Get("/partner/earnings", partnerDeps.Earnings)
			authed.Get("/partner/settlements", partnerDeps.Settlements)
			authed.Post("/partner/settlements", partnerDeps.Settlements)
			authed.Get("/partner/referrals", partnerDeps.Referrals)

			authed.Get("/partner/documents", partnerDeps.Documents)
			authed.Post("/partner/documents", partnerDeps.Documents)
			authed.Delete("/partner/documents/{id}", partnerDeps.DocumentDelete)

			authed.Get("/partner/notifications", partnerDeps.Notifications)
			authed.Post("/partner/notifications", partnerDeps.Notifications)
			authed.Post("/partner/notifications/{notificationId}/read", partnerDeps.NotificationRead)
			authed.Get("/partner/notification-preferences", partnerDeps.NotificationPreferences)
			authed.Put("/partner/notification-preferences", partnerDeps.NotificationPreferences)
			authed.Get("/partner/notification-webhooks", partnerDeps.NotificationWebhooks)
			authed.Post("/partner/notification-webhooks", partnerDeps.NotificationWebhooks)
			authed.Put("/partner/notification-webhooks/{webhookId}", partnerDeps.NotificationWebhookUpdate)
			authed.Delete("/partner/notification-webhooks/{webhookId}", partnerDeps.NotificationWebhookUpdate)

			authed.Get("/partner/rss-feeds", partnerDeps.RSSFeeds)
			authed.Delete("/partner/rss-feeds/{feedId}", partnerDeps.RSSFeedDelete)
			authed.Post("/partner/rss-feeds/{feedId}/rotate", partnerDeps.RSSFeedRotate)
			authed.Post("/partner/rss-feeds/{feedId}/test", partnerDeps.RSSFeedTest)
			authed.Post("/partner/rss-feeds/{feedId}/verify", partnerDeps.RSSFeedVerify)

			authed.Get("/partner/publishing-api-keys", partnerDeps.PublishingAPIKeys)
			authed.Post("/partner/publishing-api-keys", partnerDeps.PublishingAPIKeys)
			authed.Post("/partner/publishing-api-keys/{keyId}", partnerDeps.PublishingAPIKeyAction)
			authed.Delete("/partner/publishing-api-keys/{keyId}", partnerDeps.PublishingAPIKeyAction)
			authed.Get("/partner/publishing-api/docs", partnerDeps.PublishingAPIDocs)

			authed.Get("/partner/seo/check", partnerDeps.SEOCheck)
			authed.Post("/partner/seo/check", partnerDeps.SEOCheck)
			authed.Get("/partner/seo/speed-audit", partnerDeps.SpeedAudit)
			authed.Post("/partner/seo/speed-audit", partnerDeps.SpeedAudit)

			authed.Get("/partner/gsc", partnerDeps.GSC)
			authed.Post("/partner/gsc", partnerDeps.GSC)
			authed.Get("/partner/gsc/connect", partnerDeps.GSCConnect)
			authed.Get("/partner/gsc/properties", partnerDeps.GSCProperties)
			authed.Post("/partner/gsc/properties", partnerDeps.GSCProperties)
			authed.Post("/partner/gsc/snapshots", partnerDeps.GSCSnapshots)

			authed.Get("/partner/cms-connections", partnerDeps.CMSConnections)
			authed.Post("/partner/cms-connections", partnerDeps.CMSConnections)
			authed.Put("/partner/cms-connections/{connectionId}", partnerDeps.CMSConnectionUpdate)
			authed.Delete("/partner/cms-connections/{connectionId}", partnerDeps.CMSConnectionUpdate)
			authed.Post("/partner/cms-connections/{connectionId}/test", partnerDeps.CMSConnectionTest)

			authed.Get("/partner/vip/brand", partnerDeps.VipBrand)
			authed.Post("/partner/vip/brand", partnerDeps.VipBrand)
			authed.Get("/partner/vip/domains", partnerDeps.VipDomains)
			authed.Get("/partner/vip/wallet", partnerDeps.VipWallet)
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
