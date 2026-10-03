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

	"github.com/rahmaangriawan/tauthub-go/internal/adminshared"
	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/config"
	"github.com/rahmaangriawan/tauthub-go/internal/db"
	authdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/auth"
	brandflowdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brandflow"
	brandsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brands"
	campaignsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/campaigns"
	clientreaddomain "github.com/rahmaangriawan/tauthub-go/internal/domains/clientread"
	clienttautdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/clienttaut"
	contentdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/content"
	knowledgedomain "github.com/rahmaangriawan/tauthub-go/internal/domains/knowledge"
	partnerdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/partner"
	platformdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/platform"
	pushdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/push"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
	"github.com/rahmaangriawan/tauthub-go/internal/jobwire"
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

	// Google service dipakai bersama auth (alur publik) dan platform (diagnostik
	// config admin) supaya statusnya konsisten.
	googleSvc := &auth.Google{
		Pool:            database.Pool,
		AppSecret:       cfg.Secret,
		CmsSecret:       cfg.CmsEncryptionSecret,
		EnvClientID:     cfg.GoogleClientID,
		EnvClientSecret: cfg.GoogleClientSecret,
		EnvRedirectURI:  cfg.GoogleRedirectURI,
		IsProduction:    cfg.IsProduction(),
	}

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
		Google: googleSvc,
	}

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(httpx.Recover(cfg.AppEnv))
	// Header keamanan untuk semua respons (nosniff, referrer, frame, subset CSP,
	// HSTS saat produksi) — murni aditif.
	r.Use(httpx.SecurityHeaders(cfg.IsProduction()))

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

	// Deps platform dipakai juga oleh route root (/t/{code} short link), jadi
	// dibuat di level ini lalu dipakai ulang di dalam grup /v1.
	platformDeps := &platformdomain.Deps{Pool: database.Pool, Cfg: cfg, Admin: &adminshared.Deps{Pool: database.Pool, Cfg: cfg}, Google: googleSvc}
	partnerDeps := &partnerdomain.Deps{Pool: database.Pool, Cfg: cfg}

	// Short link distribusi: /t/{code} → 301 ke URL tujuan sambil mencatat
	// tracking_link_click. Parity app/t/[code]/route.ts (go.tulisio.id/{code}).
	r.Get("/t/{code}", platformDeps.TrackingRedirect)

	// Berkas unggahan (sampul artikel, bukti transfer, dokumen dewan pers).
	// Wajib login via cookie refresh_token; nama berkas dibatasi. Parity
	// app/uploads/[key]/route.ts.
	r.Get("/uploads/{key}", platformDeps.UploadsServe)

	// Callback OAuth Google di root (redirect URI terdaftar di Google Console):
	// /api/blogger/callback (Blogger) dan /api/gsc/callback (Search Console).
	r.Get("/api/blogger/callback", partnerDeps.BloggerCallback)
	r.Get("/api/gsc/callback", partnerDeps.GSCCallback)

	// Feed RSS 2.0 mitra (publik; token per feed jadi rahasianya) + callback
	// status importer mitra — port app/feed/partner/[token]/*.
	r.Get("/feed/partner/{token}", platformDeps.RSSFeedServe)
	r.Post("/feed/partner/{token}/import", platformDeps.RSSFeedImport)

	// /v1 — CORS allowlist (port proxy.ts) + route per domain.
	r.Route("/v1", func(v1 chi.Router) {
		v1.Use(httpx.CORSMiddleware)

		// Auth publik (fase 1).
		v1.Post("/auth/login", authDeps.Login)
		v1.Post("/auth/refresh", authDeps.Refresh)
		v1.Post("/auth/logout", authDeps.Logout)
		v1.Post("/auth/register", authDeps.Register)
		v1.Post("/auth/forgot-password", authDeps.ForgotPassword)
		// Gate SSR non-rotasi (SvelteKit +layout.server): verifikasi refresh_token
		// tanpa rotasi sesi, kembalikan role/admin/partner/membership.
		v1.Post("/auth/ssr-gate", authDeps.SSRGate)
		v1.Post("/auth/reset-password", authDeps.ResetPassword)
		v1.Post("/auth/2fa/verify", authDeps.TwoFactorVerify)
		v1.Get("/auth/turnstile", authDeps.TurnstileConfig)
		// Landing link referral (/register?ref=KODE) — mencatat klik + audit,
		// dipanggil server-side oleh route SvelteKit /register.
		v1.Post("/auth/referral/click", authDeps.ReferralClick)
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
		tautDeps := &clienttautdomain.Deps{Pool: database.Pool, Cfg: cfg}
		brandFlowDeps := &brandflowdomain.Deps{Pool: database.Pool, Cfg: cfg}
		pushDeps := pushdomain.New(database.Pool, cfg)

		// Kanonikalisasi {campaignId}: URL boleh memakai public_id 6 karakter,
		// handler tetap menerima UUID. Diterapkan per rute karena middleware
		// grup jalan sebelum chi mengisi URLParams, jadi parameternya belum ada.
		campaignResolver := campaignsdomain.NewResolver(database.Pool)
		resolveCampaign := campaignResolver.Wrap

		// Dispatcher job bersama (dipakai /v1/internal/jobs dan cmd/worker).
		jobwire.Register(database.Pool, cfg)
		jobRunner := jobs.NewRunner(database.Pool)
		jobwire.WirePlatform(platformDeps, database.Pool, cfg, jobRunner)
		v1.Group(func(authed chi.Router) {
			authed.Use(authDeps.Auth.RequireAuth)
			authed.Get("/auth/me", authDeps.Me)
			authed.Patch("/auth/me", authDeps.MeUpdate)

			// Pengumuman (broadcast admin → seluruh pengguna klien/mitra).
			// Notifikasi perangkat (Web Push) — kunci publik VAPID & langganan.
			authed.Get("/push/config", pushDeps.Config)
			authed.Post("/push/subscriptions", pushDeps.Subscribe)
			authed.Delete("/push/subscriptions", pushDeps.Unsubscribe)
			authed.Post("/push/test", pushDeps.Test)

			authed.Get("/announcements", platformDeps.AnnouncementsList)
			authed.Post("/announcements/{announcementId}/read", platformDeps.AnnouncementMarkRead)
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
			authed.Get("/brands/{brandId}/credits/topup", brandFlowDeps.CreditsTopupLookup)
			authed.Post("/brands/{brandId}/credits/topup", brandFlowDeps.CreditsTopupCreate)
			authed.Post("/brands/{brandId}/link-monitor/request-replacement", brandFlowDeps.LinkMonitorRequestReplacement)
			authed.Get("/brands/{brandId}/reports/link-clicks", brandFlowDeps.ReportsLinkClicks)
			authed.Post("/brands/{brandId}/credit-reports", brandFlowDeps.CreditReportsCreate)
			authed.Get("/brands/{brandId}/pr-calculator", brandFlowDeps.PrCalculator)
			authed.Get("/brands/{brandId}/reports", brandFlowDeps.ReportsGet)

			// ---------- Alur brand lanjutan (/v1/brands/{brandId}) — fase 6–11 ----------
			// Kampanye, publikasi, rencana publikasi, batch/varian, integrasi CMS,
			// dokumen, laporan, dan Taut Pintar pada tingkat brand.
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}/angle", resolveCampaign(brandFlowDeps.CampaignMediaItemAngle))
			authed.Get("/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}/angle-options", resolveCampaign(brandFlowDeps.CampaignMediaItemAngleOptions))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}/paraphrase", resolveCampaign(brandFlowDeps.CampaignMediaItemParaphrase))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/articles/{mediaId}/upload", resolveCampaign(brandFlowDeps.TautArticleUpload))
			authed.Patch("/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}", resolveCampaign(brandFlowDeps.CampaignMediaItemPatch))
			authed.Delete("/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}", resolveCampaign(brandFlowDeps.CampaignMediaItemDelete))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/articles/{mediaId}", resolveCampaign(brandFlowDeps.TautArticleGet))
			authed.Put("/brands/{brandId}/taut/campaigns/{campaignId}/articles/{mediaId}", resolveCampaign(brandFlowDeps.TautArticlePut))
			authed.Delete("/brands/{brandId}/taut/campaigns/{campaignId}/files/{fileId}", resolveCampaign(brandFlowDeps.TautFileDelete))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/gatekeeper/audit-all", resolveCampaign(brandFlowDeps.TautGatekeeperAuditAll))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/gatekeeper/audits", resolveCampaign(brandFlowDeps.TautGatekeeperAudits))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/gatekeeper/distribute", resolveCampaign(brandFlowDeps.TautGatekeeperDistribute))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/produce/{mediaId}", resolveCampaign(brandFlowDeps.TautProduceOne))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/produce/{mediaId}/stream", resolveCampaign(brandFlowDeps.TautProduceOneStream))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow/analyze", resolveCampaign(brandFlowDeps.CampaignFlowAnalyze))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow/article-regen", resolveCampaign(brandFlowDeps.CampaignFlowArticleRegen))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow/checkout", resolveCampaign(brandFlowDeps.CampaignFlowCheckout))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow/media", resolveCampaign(brandFlowDeps.CampaignFlowMedia))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow/media-type", resolveCampaign(brandFlowDeps.CampaignFlowMediaType))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/report/share", resolveCampaign(brandFlowDeps.CampaignReportShare))
			authed.Post("/brands/{brandId}/publication-plans/occurrences/{occurrenceId}/skip", brandFlowDeps.PublicationPlanOccurrenceSkip)
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/crawl", resolveCampaign(brandFlowDeps.TautCrawl))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/files", resolveCampaign(brandFlowDeps.TautFilesList))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/files", resolveCampaign(brandFlowDeps.TautFilesUpload))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/documents/cleanup", resolveCampaign(brandFlowDeps.TautDocCleanup))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/produce", resolveCampaign(brandFlowDeps.TautProduce))
			authed.Post("/brands/{brandId}/batches/{batchId}/cancel", brandFlowDeps.BatchCancel)
			authed.Post("/brands/{brandId}/batches/{batchId}/pause", brandFlowDeps.BatchPause)
			authed.Post("/brands/{brandId}/batches/{batchId}/retry", brandFlowDeps.BatchRetry)
			authed.Post("/brands/{brandId}/batches/{batchId}/start", brandFlowDeps.BatchStart)
			authed.Get("/brands/{brandId}/campaigns/{campaignId}/flow", resolveCampaign(brandFlowDeps.CampaignFlowGet))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow", resolveCampaign(brandFlowDeps.CampaignFlowPost))
			authed.Get("/brands/{brandId}/campaigns/{campaignId}/publications", resolveCampaign(brandFlowDeps.CampaignPublicationsList))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/publications", resolveCampaign(brandFlowDeps.CampaignPublicationsCreate))
			authed.Post("/brands/{brandId}/cms-connections/{connectionId}/test", brandFlowDeps.CmsConnectionTest)
			authed.Post("/brands/{brandId}/content-intakes/{intakeId}/ai-fill", brandFlowDeps.ContentIntakeAIFill)
			authed.Post("/brands/{brandId}/content-intakes/{intakeId}/auto-key-messages", brandFlowDeps.ContentIntakeAutoKeyMessages)
			authed.Post("/brands/{brandId}/content-intakes/{intakeId}/recommend", brandFlowDeps.ContentIntakeRecommend)
			authed.Post("/brands/{brandId}/documents/{documentId}/publish", brandFlowDeps.ContentDocumentPublish)
			authed.Post("/brands/{brandId}/documents/{documentId}/standard-check", brandFlowDeps.ContentDocumentStandardCheck)
			authed.Get("/brands/{brandId}/media/{mediaId}/crawl-status", brandFlowDeps.MediaCrawlStatus)
			authed.Get("/brands/{brandId}/media/{mediaId}/seo", brandFlowDeps.MediaSEO)
			authed.Post("/brands/{brandId}/publication-plans/{planId}/cancel", brandFlowDeps.PublicationPlanCancel)
			authed.Post("/brands/{brandId}/publication-plans/{planId}/pause", brandFlowDeps.PublicationPlanPause)
			authed.Post("/brands/{brandId}/publication-plans/{planId}/resume", brandFlowDeps.PublicationPlanResume)
			authed.Get("/brands/{brandId}/publications/variants/{batchId}", brandFlowDeps.PublicationVariantBatchGet)
			authed.Post("/brands/{brandId}/publications/{publicationId}/accept", brandFlowDeps.PublicationAccept)
			authed.Post("/brands/{brandId}/publications/{publicationId}/auto-replace", brandFlowDeps.PublicationAutoReplace)
			authed.Post("/brands/{brandId}/publications/{publicationId}/cancel", brandFlowDeps.PublicationCancel)
			authed.Post("/brands/{brandId}/publications/{publicationId}/request-revision", brandFlowDeps.PublicationRequestRevision)
			authed.Post("/brands/{brandId}/publications/{publicationId}/request-url", brandFlowDeps.PublicationRequestURL)
			authed.Post("/brands/{brandId}/publications/{publicationId}/reset-approval", brandFlowDeps.PublicationResetApproval)
			authed.Post("/brands/{brandId}/publications/{publicationId}/retry", brandFlowDeps.PublicationRetry)
			authed.Get("/brands/{brandId}/publications/{publicationId}/tracking", brandFlowDeps.PublicationTrackingGet)
			authed.Post("/brands/{brandId}/publications/{publicationId}/tracking", brandFlowDeps.PublicationTrackingPost)
			authed.Get("/brands/{brandId}/analyze/{topic}", brandFlowDeps.TautAnalyzeTopic)
			authed.Get("/brands/{brandId}/batches/{batchId}", brandFlowDeps.BatchGet)
			authed.Get("/brands/{brandId}/campaigns/{campaignId}", resolveCampaign(brandFlowDeps.CampaignGet))
			authed.Put("/brands/{brandId}/campaigns/{campaignId}", resolveCampaign(brandFlowDeps.CampaignUpdate))
			authed.Delete("/brands/{brandId}/campaigns/{campaignId}", resolveCampaign(brandFlowDeps.CampaignCancel))
			authed.Get("/brands/{brandId}/cms-connections/{connectionId}", brandFlowDeps.CmsConnectionGet)
			authed.Put("/brands/{brandId}/cms-connections/{connectionId}", brandFlowDeps.CmsConnectionUpdate)
			authed.Delete("/brands/{brandId}/cms-connections/{connectionId}", brandFlowDeps.CmsConnectionDelete)
			authed.Patch("/brands/{brandId}/documents/{documentId}", brandFlowDeps.ContentDocumentPatch)
			authed.Get("/brands/{brandId}/media/recommendations", brandFlowDeps.MediaRecommendations)
			authed.Put("/brands/{brandId}/notification-webhooks/{webhookId}", brandFlowDeps.NotificationWebhookUpdate)
			authed.Delete("/brands/{brandId}/notification-webhooks/{webhookId}", brandFlowDeps.NotificationWebhookDelete)
			authed.Get("/brands/{brandId}/publication-plans/{planId}", brandFlowDeps.PublicationPlanGet)
			authed.Post("/brands/{brandId}/publications/variants", brandFlowDeps.PublicationVariantsCreate)
			authed.Get("/brands/{brandId}/publications/variants", brandFlowDeps.PublicationVariantsList)
			authed.Get("/brands/{brandId}/publications/{publicationId}", brandFlowDeps.PublicationGet)
			authed.Get("/brands/{brandId}/publications/{publicationId}/replacement", brandFlowDeps.PublicationReplacementGet)
			authed.Get("/brands/{brandId}/taut/agent-status", brandFlowDeps.TautAgentStatus)
			authed.Post("/brands/{brandId}/assets", brandFlowDeps.AssetsUpload)
			authed.Get("/brands/{brandId}/batches", brandFlowDeps.BatchesList)
			authed.Post("/brands/{brandId}/batches", brandFlowDeps.BatchesCreate)
			authed.Post("/brands/{brandId}/campaigns", brandFlowDeps.CampaignsCreate)
			authed.Get("/brands/{brandId}/cms-connections", brandFlowDeps.CmsConnectionsList)
			authed.Post("/brands/{brandId}/cms-connections", brandFlowDeps.CmsConnectionCreate)
			authed.Get("/brands/{brandId}/ga4", brandFlowDeps.Ga4Get)
			authed.Get("/brands/{brandId}/media-packages", brandFlowDeps.MediaPackagesList)
			authed.Get("/brands/{brandId}/notification-webhooks", brandFlowDeps.NotificationWebhookList)
			authed.Post("/brands/{brandId}/notification-webhooks", brandFlowDeps.NotificationWebhookCreate)
			authed.Get("/brands/{brandId}/packages", brandFlowDeps.PackagesList)
			authed.Get("/brands/{brandId}/publication-plans", brandFlowDeps.PublicationPlansList)
			authed.Post("/brands/{brandId}/publication-plans", brandFlowDeps.PublicationPlansCreate)
			authed.Get("/brands/{brandId}/publications", brandFlowDeps.PublicationsList)
			authed.Post("/brands/{brandId}/publications", brandFlowDeps.PublicationsCreate)
			authed.Get("/brands/{brandId}/usage", brandFlowDeps.UsageGet)
			authed.Get("/brands/{brandId}/notifications", clientRead.BrandNotifications)
			authed.Post("/brands/{brandId}/notifications", clientRead.BrandNotifications)
			authed.Post("/brands/{brandId}/notifications/{notificationId}/read", clientRead.BrandNotificationRead)
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/writer-links", resolveCampaign(brandFlowDeps.TautWriterLinks))
			authed.Put("/brands/{brandId}/taut/campaigns/{campaignId}/writer-links", resolveCampaign(brandFlowDeps.TautWriterLinks))
				// Pustaka gambar AI per artikel (isolasi brand+campaign+media).
				authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/images", resolveCampaign(brandFlowDeps.ArticleImagesList))
				authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/images/generate", resolveCampaign(brandFlowDeps.ArticleImageGenerate))
				authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/images/upload", resolveCampaign(brandFlowDeps.ArticleImageUpload))
				authed.Delete("/brands/{brandId}/taut/campaigns/{campaignId}/images/{imageId}", resolveCampaign(brandFlowDeps.ArticleImageDelete))

			// ---------- Taut Pintar klien (/v1/brands/{brandId}/taut/*) ----------
			authed.Get("/brands/{brandId}/taut/campaigns", tautDeps.TautCampaignsList)
			authed.Post("/brands/{brandId}/taut/campaigns", tautDeps.TautCampaignCreate)
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}", resolveCampaign(tautDeps.TautCampaignGet))
			authed.Put("/brands/{brandId}/taut/campaigns/{campaignId}", resolveCampaign(tautDeps.TautCampaignUpdate))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}", resolveCampaign(tautDeps.TautCampaignAction))
			authed.Delete("/brands/{brandId}/taut/campaigns/{campaignId}", resolveCampaign(tautDeps.TautCampaignDelete))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/advance", resolveCampaign(tautDeps.TautCampaignAdvance))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/validate", resolveCampaign(tautDeps.TautCampaignValidateGet))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/validate", resolveCampaign(tautDeps.TautCampaignValidatePost))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/schedule", resolveCampaign(tautDeps.TautCampaignSchedule))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/budget-estimate", resolveCampaign(tautDeps.TautBudgetEstimate))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/placement", resolveCampaign(tautDeps.TautPlacementGet))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/placement", resolveCampaign(tautDeps.TautPlacementPost))
			authed.Put("/brands/{brandId}/taut/campaigns/{campaignId}/placement", resolveCampaign(tautDeps.TautPlacementPut))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/finish", resolveCampaign(tautDeps.TautCampaignFinish))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/profile", resolveCampaign(tautDeps.TautProfileGet))
			authed.Put("/brands/{brandId}/taut/campaigns/{campaignId}/profile", resolveCampaign(tautDeps.TautProfilePut))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/profile", resolveCampaign(tautDeps.TautProfileAnalyzeLegacy))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/profile/analyze", resolveCampaign(tautDeps.TautProfileAnalyze))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/profile/draft", resolveCampaign(tautDeps.TautProfileDraftGet))
			authed.Put("/brands/{brandId}/taut/campaigns/{campaignId}/profile/draft", resolveCampaign(tautDeps.TautProfileDraftPut))
			authed.Delete("/brands/{brandId}/taut/campaigns/{campaignId}/profile/result", resolveCampaign(tautDeps.TautProfileResultDelete))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/profile/results/{resultId}", resolveCampaign(tautDeps.TautProfileResultGet))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/profile/verify", resolveCampaign(tautDeps.TautProfileVerify))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/profiler/options", resolveCampaign(tautDeps.TautProfilerOptions))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/submit-final", resolveCampaign(tautDeps.TautSubmitFinal))

		})

		// Berita acara kampanye by token share — PUBLIK (tautan dibagikan ke
		// pihak ketiga), parity app/v1/campaign-reports/[token].
		v1.Get("/campaign-reports/{token}", clientRead.CampaignReportGet)
		// Endpoint publik & infra (§7): feed JSON mitra, tracking event, callback
		// publishing, laporan bug (butuh login), trigger worker/scheduler.
		v1.Get("/publishing/feed.json", platformDeps.PublishingFeedJSON)
		v1.Post("/publishing/callbacks", platformDeps.PublishingCallback)
		v1.Post("/tracking/events", platformDeps.TrackingEventCreate)
		v1.Post("/internal/jobs", platformDeps.InternalJobPull)
		v1.Get("/internal/cron/scheduler", platformDeps.InternalCronScheduler)
		v1.Post("/internal/cron/scheduler", platformDeps.InternalCronScheduler)

		// ---------- Pembayaran PUBLIK (/v1/payments/*) — port app/v1/payments ----------
		// Webhook (verifikasi signature provider) + status polling pasca-redirect
		// gateway: PUBLIK. gateway/active butuh Bearer user biasa.
		v1.Post("/payments/webhooks/{provider}", platformDeps.PaymentWebhook)
		v1.Get("/payments/status/{transactionId}", platformDeps.PublicPaymentStatus)

		v1.Group(func(authed chi.Router) {
			authed.Use(authDeps.Auth.RequireAuth)

			authed.Get("/payments", platformDeps.PublicPaymentsList)
			authed.Post("/payments", platformDeps.PublicPaymentsCreate)
			authed.Get("/payments/gateway/active", platformDeps.PublicPaymentGatewayActive)

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
			authed.Post("/partner/publications/{publicationId}/revision/accept", partnerDeps.PublicationRevisionAccept)
			authed.Post("/partner/publications/{publicationId}/revision/reject", partnerDeps.PublicationRevisionReject)
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

			// ---------- Kontrol-plane platform (/v1/platform/*) — port app/v1/platform ----------

			// Admins.
			// Konfigurasi admin (link clicks, OAuth Google, server config, short link).
			authed.Get("/platform/link-clicks", platformDeps.LinkClicksGet)
			authed.Get("/platform/oauth/google", platformDeps.OAuthGoogleGet)
			authed.Put("/platform/oauth/google", platformDeps.OAuthGooglePut)
			authed.Get("/platform/server-config", platformDeps.ServerConfigGet)
			authed.Put("/platform/server-config", platformDeps.ServerConfigPut)
			authed.Get("/platform/short-link-config", platformDeps.ShortLinkConfigGet)
			authed.Put("/platform/short-link-config", platformDeps.ShortLinkConfigPut)
			authed.Get("/platform/admins", platformDeps.AdminsList)
			authed.Post("/platform/admins", platformDeps.AdminsCreate)
			authed.Get("/platform/admins/me", platformDeps.AdminsMe)
			authed.Patch("/platform/admins/{adminId}", platformDeps.AdminsUpdate)
			authed.Delete("/platform/admins/{adminId}", platformDeps.AdminsDelete)

			// Riwayat notifikasi admin (per user admin yang login).
			authed.Get("/platform/notifications", platformDeps.AdminNotifications)
			authed.Post("/platform/notifications", platformDeps.AdminNotifications)
			authed.Post("/platform/notifications/{notificationId}/read", platformDeps.AdminNotificationRead)

			// Pengumuman (sisi admin).
			authed.Get("/platform/announcements", platformDeps.AdminAnnouncementsList)
			authed.Post("/platform/announcements", platformDeps.AdminAnnouncementCreate)
			authed.Delete("/platform/announcements/{announcementId}", platformDeps.AdminAnnouncementDelete)

			// AI.
			authed.Get("/platform/ai/configuration", platformDeps.AIConfigurationGet)
			authed.Put("/platform/ai/configuration", platformDeps.AIConfigurationPut)
			authed.Get("/platform/ai/models", platformDeps.AIModelsList)
			authed.Get("/platform/ai/models/{modelId}/knowledge", platformDeps.AIModelKnowledgeGet)
			authed.Put("/platform/ai/models/{modelId}/knowledge", platformDeps.AIModelKnowledgePut)
			authed.Delete("/platform/ai/models/{modelId}/knowledge", platformDeps.AIModelKnowledgeDelete)
			authed.Post("/platform/ai/probe", platformDeps.AIProbe)
			authed.Get("/platform/ai/status", platformDeps.AIStatus)
			authed.Post("/platform/ai/test-generation", platformDeps.AITestGeneration)
			authed.Post("/platform/ai/image-test", platformDeps.AIImageTest)
			authed.Get("/platform/ai/usage", platformDeps.AIUsage)

			// Media (segmen literal sebelum {mediaId}).
			authed.Get("/platform/media", platformDeps.MediaList)
			authed.Post("/platform/media", platformDeps.MediaCreate)
			authed.Get("/platform/media/archive-impact", platformDeps.MediaArchiveImpact)
			authed.Post("/platform/media/archive-all", platformDeps.MediaArchiveAll)
			authed.Post("/platform/media/bulk", platformDeps.MediaBulkUpsert)
			authed.Delete("/platform/media/bulk", platformDeps.MediaBulkDelete)
			authed.Get("/platform/media/dewan-pers-reviews", platformDeps.MediaDewanPersReviews)
			authed.Post("/platform/media/dewan-pers-reviews/{reviewId}", platformDeps.MediaDewanPersReviewDecide)
			authed.Get("/platform/media/intelligence", platformDeps.MediaIntelligence)
			authed.Post("/platform/media/recompute-health", platformDeps.MediaRecomputeHealthBulk)
			authed.Post("/platform/media/restore-archived", platformDeps.MediaRestoreArchived)
			authed.Post("/platform/media/seo-bulk", platformDeps.MediaSEOBulk)
			authed.Post("/platform/media/similarweb-bulk", platformDeps.MediaSimilarWebBulk)
			authed.Post("/platform/media/sync-credit-cost", platformDeps.MediaSyncCreditCost)
			authed.Get("/platform/media/{mediaId}", platformDeps.MediaGet)
			authed.Put("/platform/media/{mediaId}", platformDeps.MediaUpdate)
			authed.Delete("/platform/media/{mediaId}", platformDeps.MediaDelete)
			authed.Get("/platform/media/{mediaId}/audit", platformDeps.MediaAudit)
			authed.Post("/platform/media/{mediaId}/recompute-health", platformDeps.MediaRecomputeHealth)
			authed.Post("/platform/media/{mediaId}/review", platformDeps.MediaReview)
			authed.Get("/platform/media/{mediaId}/seo", platformDeps.MediaSEO)
			authed.Post("/platform/media/{mediaId}/seo", platformDeps.MediaSEO)
			authed.Get("/platform/media/{mediaId}/similarweb", platformDeps.MediaSimilarWeb)
			authed.Post("/platform/media/{mediaId}/similarweb", platformDeps.MediaSimilarWeb)

			// Media packages.
			authed.Get("/platform/media-packages", platformDeps.MediaPackagesList)
			authed.Post("/platform/media-packages", platformDeps.MediaPackageCreate)
			authed.Put("/platform/media-packages/{packageId}", platformDeps.MediaPackageUpdate)
			authed.Delete("/platform/media-packages/{packageId}", platformDeps.MediaPackageDelete)

			// Media standards.
			authed.Get("/platform/media-standards", platformDeps.MediaStandardsList)
			authed.Post("/platform/media-standards", platformDeps.MediaStandardCreate)
			authed.Get("/platform/media-standards/{mediaId}", platformDeps.MediaStandardGet)
			authed.Put("/platform/media-standards/{mediaId}", platformDeps.MediaStandardUpdate)
			authed.Delete("/platform/media-standards/{mediaId}", platformDeps.MediaStandardDelete)

			// Media summaries.
			authed.Get("/platform/media-summaries", platformDeps.MediaSummariesList)
			authed.Post("/platform/media-summaries/{mediaId}/regenerate", platformDeps.MediaSummaryRegenerate)

			// Operations.
			authed.Get("/platform/operations/access", platformDeps.OperationsAccess)
			authed.Get("/platform/operations/summary", platformDeps.OperationsSummary)
			// audit = alias audit-log di app lama (routes.json: metode kosong).
			authed.Get("/platform/operations/audit", platformDeps.OperationsAudit)
			authed.Get("/platform/operations/audit-log", platformDeps.OperationsAuditLog)
			authed.Get("/platform/operations/brands", platformDeps.OperationsBrands)
			authed.Get("/platform/operations/brands/{brandId}/campaigns", platformDeps.OperationsBrandCampaigns)
			authed.Get("/platform/operations/brands/{brandId}/connections", platformDeps.OperationsBrandConnections)
			authed.Post("/platform/operations/brands/{brandId}/connections", platformDeps.OperationsBrandConnections)
			authed.Delete("/platform/operations/brands/{brandId}/connections/{connectionId}", platformDeps.OperationsBrandConnectionDelete)
			authed.Post("/platform/operations/brands/{brandId}/connections/{connectionId}/test", platformDeps.OperationsBrandConnectionTest)
			authed.Get("/platform/operations/brands/{brandId}/content-settings", platformDeps.OperationsBrandContentSettings)
			authed.Put("/platform/operations/brands/{brandId}/content-settings", platformDeps.OperationsBrandContentSettings)
			authed.Post("/platform/operations/credit-adjustment", platformDeps.OperationsCreditAdjustment)
			authed.Get("/platform/operations/credit-reports", platformDeps.OperationsCreditReports)
			authed.Post("/platform/operations/credit-reports/{reportId}/review", platformDeps.OperationsCreditReportReview)
			authed.Get("/platform/operations/dead-letters", platformDeps.OperationsDeadLetters)
			authed.Post("/platform/operations/dead-letters/{id}/retry", platformDeps.OperationsDeadLetterRetry)
			authed.Get("/platform/operations/errors", platformDeps.OperationsErrors)
			authed.Put("/platform/operations/generation-switch", platformDeps.OperationsGenerationSwitch)
			authed.Get("/platform/operations/notifications", platformDeps.OperationsNotifications)
			authed.Get("/platform/operations/publications", platformDeps.OperationsPublications)
			authed.Get("/platform/operations/publications/{publicationId}", platformDeps.OperationsPublicationDetail)
			authed.Post("/platform/operations/publications/{publicationId}/retry", platformDeps.OperationsPublicationRetry)
			authed.Get("/platform/operations/running-batches", platformDeps.OperationsRunningBatches)
			authed.Get("/platform/operations/users", platformDeps.OperationsUsers)
			authed.Post("/platform/operations/users", platformDeps.OperationsUsers)
			authed.Get("/platform/operations/users/{userId}/brands", platformDeps.OperationsUserBrands)
			authed.Get("/platform/operations/users/{userId}/daily-quota", platformDeps.OperationsUserDailyQuota)
			authed.Put("/platform/operations/users/{userId}/daily-quota", platformDeps.OperationsUserDailyQuota)
			authed.Get("/platform/operations/users/{userId}/generate-quota", platformDeps.OperationsUserGenerateQuota)
			authed.Put("/platform/operations/users/{userId}/generate-quota", platformDeps.OperationsUserGenerateQuota)
			authed.Put("/platform/operations/users/{userId}/password", platformDeps.OperationsUserPassword)
			authed.Put("/platform/operations/users/{userId}/role", platformDeps.OperationsUserRole)
			authed.Put("/platform/operations/users/{userId}/status", platformDeps.OperationsUserStatus)
			authed.Get("/platform/operations/variant-batches", platformDeps.OperationsVariantBatches)
			authed.Get("/platform/operations/variant-batches/{batchId}", platformDeps.OperationsVariantBatchDetail)
			authed.Post("/platform/operations/variant-batches/{batchId}/retry", platformDeps.OperationsVariantBatchRetry)

			// Partners.
			authed.Get("/platform/partners", platformDeps.PartnersList)
			authed.Post("/platform/partners", platformDeps.PartnerCreate)
			authed.Get("/platform/partners/{partnerId}", platformDeps.PartnerGet)
			authed.Put("/platform/partners/{partnerId}", platformDeps.PartnerUpdate)
			authed.Delete("/platform/partners/{partnerId}", platformDeps.PartnerDelete)
			authed.Get("/platform/partners/{partnerId}/connections", platformDeps.PartnerConnections)
			authed.Post("/platform/partners/{partnerId}/connections", platformDeps.PartnerConnections)
			authed.Put("/platform/partners/{partnerId}/connections/{connectionId}", platformDeps.PartnerConnectionUpdate)
			authed.Delete("/platform/partners/{partnerId}/connections/{connectionId}", platformDeps.PartnerConnectionDelete)
			authed.Post("/platform/partners/{partnerId}/connections/{connectionId}/test", platformDeps.PartnerConnectionTest)
			authed.Get("/platform/partners/{partnerId}/documents", platformDeps.PartnerDocuments)
			authed.Put("/platform/partners/{partnerId}/documents/{id}", platformDeps.PartnerDocumentUpdate)
			authed.Get("/platform/partners/{partnerId}/vip", platformDeps.PartnerVipGet)
			authed.Put("/platform/partners/{partnerId}/vip", platformDeps.PartnerVipUpdate)

			// Payments (admin).
			authed.Get("/platform/payments/gateways", platformDeps.PaymentsGateways)
			authed.Post("/platform/payments/gateways", platformDeps.PaymentGatewaySave)
			authed.Post("/platform/payments/gateways/{gatewayId}/activate", platformDeps.PaymentGatewayActivate)
			authed.Post("/platform/payments/gateways/{gatewayId}/test", platformDeps.PaymentGatewayTest)
			authed.Get("/platform/payments/transactions", platformDeps.PaymentTransactions)

			// Settings.
			authed.Get("/platform/settings/email", platformDeps.SettingsEmailGet)
			authed.Put("/platform/settings/email", platformDeps.SettingsEmailPut)
			authed.Get("/platform/settings/email/deliveries", platformDeps.SettingsEmailDeliveries)
			authed.Post("/platform/settings/email/test-connection", platformDeps.SettingsEmailTestConnection)
			authed.Post("/platform/settings/email/test-send", platformDeps.SettingsEmailTestSend)
			authed.Get("/platform/settings/pagespeed", platformDeps.SettingsPageSpeedGet)
			authed.Put("/platform/settings/pagespeed", platformDeps.SettingsPageSpeedPut)
			authed.Post("/platform/settings/pagespeed/test", platformDeps.SettingsPageSpeedTest)
			authed.Get("/platform/settings/turnstile", platformDeps.SettingsTurnstileGet)
			authed.Put("/platform/settings/turnstile", platformDeps.SettingsTurnstilePut)
			authed.Post("/platform/settings/turnstile/test", platformDeps.SettingsTurnstileTest)

			// PR.
			authed.Get("/platform/pr/analysis-history", platformDeps.PRAnalysisHistory)
			authed.Get("/platform/pr/brand-standards", platformDeps.PRBrandStandards)
			authed.Put("/platform/pr/brand-standards/{brandId}", platformDeps.PRBrandStandardUpdate)
			authed.Get("/platform/pr/config", platformDeps.PRConfigGet)
			authed.Put("/platform/pr/config", platformDeps.PRConfigPut)
			authed.Get("/platform/pr/media-rules", platformDeps.PRMediaRulesGet)
			authed.Put("/platform/pr/media-rules", platformDeps.PRMediaRulesPut)
			authed.Post("/platform/pr/recompute-scores", platformDeps.PRRecomputeScores)
			authed.Post("/platform/pr/rerun-analysis", platformDeps.PRRerunAnalysis)
			authed.Get("/platform/pr/scoring-profiles", platformDeps.PRScoringProfiles)
			authed.Post("/platform/pr/scoring-profiles", platformDeps.PRScoringProfiles)
			authed.Post("/platform/pr/scoring-profiles/{profileId}/activate", platformDeps.PRScoringProfileActivate)

			// Taut.
			authed.Get("/platform/taut-agent-logs", platformDeps.TautAgentLogs)
			authed.Get("/platform/taut-agents", platformDeps.TautAgentsList)
			authed.Get("/platform/taut-agents/{agent}", platformDeps.TautAgentGet)
			authed.Put("/platform/taut-agents/{agent}", platformDeps.TautAgentUpdate)
			authed.Post("/platform/taut-agents/{agent}/test", platformDeps.TautAgentTest)
			authed.Get("/platform/taut/ai-usage", platformDeps.TautAIUsage)
			authed.Get("/platform/taut/angles/{angleId}", platformDeps.TautAngleGet)
			authed.Patch("/platform/taut/angles/{angleId}", platformDeps.TautAngleUpdate)
			authed.Delete("/platform/taut/angles/{angleId}", platformDeps.TautAngleDelete)
			authed.Post("/platform/taut/angles/{angleId}/test", platformDeps.TautAngleTest)
			authed.Get("/platform/taut/angles/{angleId}/versions", platformDeps.TautAngleVersions)
			authed.Post("/platform/taut/angles/{angleId}/versions", platformDeps.TautAngleVersionCreate)
			authed.Get("/platform/taut/angles/{angleId}/versions/{versionId}", platformDeps.TautAngleVersionGet)
			authed.Patch("/platform/taut/angles/{angleId}/versions/{versionId}", platformDeps.TautAngleVersionUpdate)
			authed.Delete("/platform/taut/angles/{angleId}/versions/{versionId}", platformDeps.TautAngleVersionDelete)
			authed.Post("/platform/taut/angles/{angleId}/versions/{versionId}/activate", platformDeps.TautAngleVersionActivate)
			authed.Get("/platform/taut/base-prompt", platformDeps.TautBasePromptGet)
			authed.Post("/platform/taut/base-prompt", platformDeps.TautBasePromptCreate)
			authed.Get("/platform/taut/base-prompt/{versionId}", platformDeps.TautBasePromptVersionGet)
			authed.Patch("/platform/taut/base-prompt/{versionId}", platformDeps.TautBasePromptVersionUpdate)
			authed.Delete("/platform/taut/base-prompt/{versionId}", platformDeps.TautBasePromptVersionDelete)
			authed.Post("/platform/taut/base-prompt/{versionId}/activate", platformDeps.TautBasePromptVersionActivate)
			authed.Get("/platform/taut/channels", platformDeps.TautChannelsList)
			authed.Post("/platform/taut/channels", platformDeps.TautChannelCreate)
			authed.Patch("/platform/taut/channels/{channelId}", platformDeps.TautChannelUpdate)
			authed.Delete("/platform/taut/channels/{channelId}", platformDeps.TautChannelDelete)
			authed.Get("/platform/taut/channels/{channelId}/angles", platformDeps.TautChannelAngles)
			authed.Post("/platform/taut/channels/{channelId}/angles", platformDeps.TautAngleCreate)
			authed.Get("/platform/taut/runs", platformDeps.TautRunsList)
			authed.Post("/platform/taut/runs/cleanup", platformDeps.TautRunsCleanup)
			authed.Post("/platform/taut/runs/retry/{runId}", platformDeps.TautRunRetry)
			authed.Get("/platform/taut/runs/{campaignId}", resolveCampaign(platformDeps.TautRunsByCampaign))

			// Knowledge / content.
			authed.Get("/platform/knowledge-bases", platformDeps.KnowledgeBasesList)
			authed.Post("/platform/knowledge-bases", platformDeps.KnowledgeBaseCreate)
			authed.Get("/platform/knowledge-bases/{knowledgeBaseId}", platformDeps.KnowledgeBaseGet)
			authed.Patch("/platform/knowledge-bases/{knowledgeBaseId}", platformDeps.KnowledgeBaseUpdate)
			authed.Get("/platform/knowledge-bases/{knowledgeBaseId}/versions", platformDeps.KnowledgeBaseVersionsList)
			authed.Post("/platform/knowledge-bases/{knowledgeBaseId}/versions", platformDeps.KnowledgeBaseVersionCreate)
			authed.Get("/platform/knowledge-bases/{knowledgeBaseId}/versions/{versionId}", platformDeps.KnowledgeBaseVersionGet)
			authed.Patch("/platform/knowledge-bases/{knowledgeBaseId}/versions/{versionId}", platformDeps.KnowledgeBaseVersionUpdate)
			authed.Delete("/platform/knowledge-bases/{knowledgeBaseId}/versions/{versionId}", platformDeps.KnowledgeBaseVersionDelete)
			authed.Post("/platform/knowledge-bases/{knowledgeBaseId}/versions/{versionId}/activate", platformDeps.KnowledgeBaseVersionActivate)
			authed.Post("/platform/knowledge-bases/{knowledgeBaseId}/versions/{versionId}/rules", platformDeps.KnowledgeBaseVersionRules)
			authed.Get("/platform/content-intake-types", platformDeps.ContentIntakeTypesList)
			authed.Post("/platform/content-intake-types", platformDeps.ContentIntakeTypeCreate)
			authed.Get("/platform/content-intake-types/{code}", platformDeps.ContentIntakeTypeGet)
			authed.Put("/platform/content-intake-types/{code}", platformDeps.ContentIntakeTypeUpdate)
			authed.Delete("/platform/content-intake-types/{code}", platformDeps.ContentIntakeTypeDelete)
			authed.Get("/platform/content-intake-types/{code}/standards", platformDeps.ContentIntakeTypeStandardsGet)
			authed.Put("/platform/content-intake-types/{code}/standards", platformDeps.ContentIntakeTypeStandardsPut)
			authed.Post("/platform/content-intake-types/{code}/standards/versions/{versionId}/activate", platformDeps.ContentIntakeTypeStandardVersionActivate)
			authed.Get("/platform/moderations", platformDeps.ModerationsList)
			authed.Post("/platform/moderations/{auditId}/review", platformDeps.ModerationReview)

			// Crawler / GSC / WA / RSS / SEO / GA4.
			authed.Get("/platform/crawlhub", platformDeps.CrawlhubConfig)
			authed.Post("/platform/crawlhub", platformDeps.CrawlhubAction)
			authed.Put("/platform/crawlhub", platformDeps.CrawlhubConfigUpdate)
			authed.Get("/platform/crawlhub/targets", platformDeps.CrawlhubTargets)
			authed.Post("/platform/crawlhub/targets", platformDeps.CrawlhubTargetsSync)
			authed.Get("/platform/gsc-config", platformDeps.GSCConfigGet)
			authed.Put("/platform/gsc-config", platformDeps.GSCConfigPut)
			authed.Get("/platform/ga4-configs", platformDeps.GA4ConfigsList)
			authed.Post("/platform/ga4-configs", platformDeps.GA4ConfigCreate)
			authed.Get("/platform/ga4-configs/{configId}", platformDeps.GA4ConfigGet)
			authed.Put("/platform/ga4-configs/{configId}", platformDeps.GA4ConfigUpdate)
			authed.Delete("/platform/ga4-configs/{configId}", platformDeps.GA4ConfigDelete)
			authed.Get("/platform/wa/config", platformDeps.WAGatewayConfig)
			authed.Put("/platform/wa/config", platformDeps.WAGatewayConfigUpdate)
			authed.Get("/platform/wa/deliveries", platformDeps.WADeliveries)
			authed.Get("/platform/wa/templates", platformDeps.WATemplates)
			authed.Put("/platform/wa/templates", platformDeps.WATemplateSave)
			authed.Delete("/platform/wa/templates", platformDeps.WATemplateReset)
			authed.Post("/platform/wa/test", platformDeps.WATest)
			authed.Get("/platform/rss-feeds", platformDeps.RSSFeedsList)
			authed.Post("/platform/rss-feeds", platformDeps.RSSFeedCreate)
			authed.Get("/platform/rss-feeds/{feedId}", platformDeps.RSSFeedGet)
			authed.Put("/platform/rss-feeds/{feedId}", platformDeps.RSSFeedUpdate)
			authed.Delete("/platform/rss-feeds/{feedId}", platformDeps.RSSFeedDelete)
			authed.Post("/platform/rss-feeds/{feedId}", platformDeps.RSSFeedRotateToken)
			authed.Get("/platform/seo/config", platformDeps.SEOConfigGet)
			authed.Put("/platform/seo/config", platformDeps.SEOConfigPut)
			authed.Get("/platform/seo/diagnose", platformDeps.SEODiagnose)

			// Credits / invoices / packages / orders / revenue / settlements / referrals / VIP.
			authed.Get("/platform/credits", platformDeps.CreditsConfigGet)
			authed.Post("/platform/credits", platformDeps.CreditsAdjust)
			authed.Put("/platform/credits", platformDeps.CreditsConfigPut)
			authed.Get("/platform/invoices", platformDeps.InvoicesList)
			authed.Post("/platform/invoices", platformDeps.InvoiceCreate)
			authed.Get("/platform/invoices/{invoiceId}", platformDeps.InvoiceGet)
			authed.Put("/platform/invoices/{invoiceId}", platformDeps.InvoiceUpdate)
			authed.Delete("/platform/invoices/{invoiceId}", platformDeps.InvoiceVoid)
			authed.Get("/platform/packages", platformDeps.CreditPackagesList)
			authed.Post("/platform/packages", platformDeps.CreditPackageCreate)
			authed.Get("/platform/packages/{packageId}", platformDeps.CreditPackageGet)
			authed.Put("/platform/packages/{packageId}", platformDeps.CreditPackageUpdate)
			authed.Delete("/platform/packages/{packageId}", platformDeps.CreditPackageRevoke)
			authed.Get("/platform/orders", platformDeps.OrdersList)
			authed.Get("/platform/revenue-shares", platformDeps.RevenueSharesList)
			authed.Post("/platform/revenue-shares", platformDeps.RevenueShareCreate)
			authed.Get("/platform/revenue-shares/{ruleId}", platformDeps.RevenueShareGet)
			authed.Put("/platform/revenue-shares/{ruleId}", platformDeps.RevenueShareUpdate)
			authed.Delete("/platform/revenue-shares/{ruleId}", platformDeps.RevenueShareArchive)
			authed.Get("/platform/settlements", platformDeps.SettlementsList)
			authed.Post("/platform/settlements", platformDeps.SettlementCreate)
			authed.Get("/platform/settlements/config", platformDeps.SettlementConfigGet)
			authed.Put("/platform/settlements/config", platformDeps.SettlementConfigPut)
			authed.Get("/platform/settlements/{settlementId}", platformDeps.SettlementGet)
			authed.Put("/platform/settlements/{settlementId}", platformDeps.SettlementUpdate)
			authed.Delete("/platform/settlements/{settlementId}", platformDeps.SettlementVoid)
			authed.Post("/platform/settlements/{settlementId}/receipt", platformDeps.SettlementReceiptUpload)
			authed.Get("/platform/referrals", platformDeps.ReferralsList)
			authed.Get("/platform/referral-audit", platformDeps.ReferralAuditList)
			authed.Get("/platform/referred-domains", platformDeps.ReferredDomainsList)
			authed.Put("/platform/referred-domains", platformDeps.ReferredDomainsReview)
			authed.Get("/platform/vip-partners", platformDeps.VipPartnersList)
			authed.Get("/platform/vip-partners/{partnerId}/domains", platformDeps.VipPartnerDomains)
			authed.Get("/platform/pricing", platformDeps.PricingGet)
			authed.Put("/platform/pricing", platformDeps.PricingPut)

			// Bug reports / auto-replace / config / gatekeeper / diagnostics / link-monitor.
			authed.Post("/bug-reports", platformDeps.BugReportSubmit)
			authed.Get("/platform/bug-reports", platformDeps.BugReportsList)
			authed.Get("/platform/bug-reports/{reportId}", platformDeps.BugReportGet)
			authed.Patch("/platform/bug-reports/{reportId}", platformDeps.BugReportUpdate)
			authed.Get("/platform/auto-replace", platformDeps.AutoReplaceGet)
			authed.Put("/platform/auto-replace", platformDeps.AutoReplacePut)
			authed.Get("/platform/config/template-url", platformDeps.ConfigTemplateURLGet)
			authed.Put("/platform/config/template-url", platformDeps.ConfigTemplateURLPut)
			authed.Get("/platform/gatekeeper/logs", platformDeps.GatekeeperLogs)
			authed.Get("/platform/gatekeeper/stats", platformDeps.GatekeeperStats)
			authed.Get("/platform/domain-diagnostics", platformDeps.DomainDiagnosticsList)
			authed.Post("/platform/domain-diagnostics/recheck", platformDeps.DomainDiagnosticsRecheck)
			authed.Post("/platform/domain-diagnostics/recheck-all", platformDeps.DomainDiagnosticsRecheckAll)
			authed.Get("/platform/link-monitor", platformDeps.LinkMonitorGlobal)
			authed.Post("/platform/link-monitor/check", platformDeps.LinkMonitorCheck)
			authed.Post("/platform/link-monitor/replace", platformDeps.LinkMonitorReplace)
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
