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
	briefdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/brief"
	campaignsdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/campaigns"
	chatdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/chat"
	clientreaddomain "github.com/rahmaangriawan/tauthub-go/internal/domains/clientread"
	clienttautdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/clienttaut"
	contentdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/content"
	knowledgedomain "github.com/rahmaangriawan/tauthub-go/internal/domains/knowledge"
	partnerdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/partner"
	platformdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/platform"
	profilerdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/profiler"
	publiccatalogdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/publiccatalog"
	pushdomain "github.com/rahmaangriawan/tauthub-go/internal/domains/push"
	workspacedomain "github.com/rahmaangriawan/tauthub-go/internal/domains/workspace"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
	"github.com/rahmaangriawan/tauthub-go/internal/jobs"
	"github.com/rahmaangriawan/tauthub-go/internal/jobwire"
	"github.com/rahmaangriawan/tauthub-go/internal/mcpconfig"
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

	// Konfigurasi MCP admin tersimpan terenkripsi di tabel mcp_config dan
	// dibaca lewat *mcpconfig.Store. Authenticator mendapat getter yang
	// mengambil service key terbaru per request supaya rotasi berlaku tanpa
	// restart.
	bootCtx, bootCancel := context.WithTimeout(context.Background(), 5*time.Second)
	mcpStore, err := mcpconfig.New(bootCtx, database.Pool, cfg.Secret, cfg.CmsEncryptionSecret)
	bootCancel()
	if err != nil {
		return err
	}

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
			Pool:               database.Pool,
			Secret:             cfg.Secret,
			Issuer:             cfg.AppName,
			ServiceKeyProvider: mcpStore.ServiceKey,
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
	// ID grup WA admin yang masih plaintext dari versi lama dienkripsi sekali
	// saat start (idempoten). Tidak menggagalkan start bila tabel belum ada.
	platformDeps.BackfillWAGroupTarget(context.Background())
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
		// OTP email sebelum akun dibuat (wajib untuk pendaftaran email).
		v1.Post("/auth/register/request-code", authDeps.RequestRegisterCode)
		v1.Post("/auth/register/verify-code", authDeps.VerifyRegisterCode)
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

		// Local Retrieval Worker — auth token khusus di handler (bukan JWT),
		// di-polling komputer lokal admin untuk domain yang memblokir IP VPS.
		v1.Get("/platform/local-worker/claim", platformDeps.LocalWorkerClaim)
		v1.Post("/platform/local-worker/requests/{id}/result", platformDeps.LocalWorkerResult)

		// Brands (fase 2) — butuh Bearer.
		brands := &brandsdomain.Deps{Pool: database.Pool, PublicAppURL: cfg.PublicAppURL}
		knowledge := &knowledgedomain.Deps{Pool: database.Pool}
		profilerDeps := &profilerdomain.Deps{Pool: database.Pool}
		workspaceDeps := &workspacedomain.Deps{Pool: database.Pool}
		briefDeps := &briefdomain.Deps{Pool: database.Pool, Cfg: cfg, Admin: &adminshared.Deps{Pool: database.Pool, Cfg: cfg}}
		contentDeps := &contentdomain.Deps{Pool: database.Pool, JobInline: cfg.JobInline, WriteRateLimitPerMinute: cfg.WriteRateLimitPerMinute}
		clientRead := &clientreaddomain.Deps{Pool: database.Pool}
		tautDeps := &clienttautdomain.Deps{Pool: database.Pool, Cfg: cfg}
		brandFlowDeps := &brandflowdomain.Deps{Pool: database.Pool, Cfg: cfg}
		chatDeps := &chatdomain.Deps{Pool: database.Pool, Cfg: cfg}
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

			// Changelog ("Apa yang Baru") — entri rilis hasil impor admin dari
			// web/static/changelog.md (dashboard klien & mitra).
			authed.Get("/changelog", platformDeps.ChangelogList)
			authed.Post("/changelog/{entryId}/read", platformDeps.ChangelogMarkRead)
			authed.Post("/auth/2fa/setup", authDeps.TwoFactorSetup)
			authed.Post("/auth/2fa/enable", authDeps.TwoFactorEnable)
			authed.Post("/auth/2fa/disable", authDeps.TwoFactorDisable)
			authed.Post("/auth/change-password", authDeps.ChangePassword)
			authed.Post("/auth/verify-password", authDeps.VerifyPassword)
			authed.Get("/me/notification-preferences", authDeps.NotificationPreferencesGet)
			authed.Put("/me/notification-preferences", authDeps.NotificationPreferencesPut)

			authed.Get("/brands", brands.List)
			authed.Post("/brands", brands.Create)
			authed.Get("/brands/{brandId}", brands.Get)
			authed.Post("/brands/{brandId}/activate", brands.Activate)
			authed.Put("/brands/{brandId}/profile", brands.UpdateProfile)
			authed.Put("/brands/{brandId}/voice", brands.UpdateVoice)
			authed.Put("/brands/{brandId}/editorial-rules", brands.UpdateRules)

			// Workspace agensi: pengelompokan brand, anggota, dan aturan induk.
			// Otorisasi tetap per brand lewat BrandAccess; workspace menjadi
			// fallback peran untuk brand yang punya workspace_id.
			authed.Get("/workspaces", workspaceDeps.List)
			authed.Post("/workspaces", workspaceDeps.Create)
			authed.Get("/workspaces/{workspaceId}/members", workspaceDeps.MembersList)
			authed.Post("/workspaces/{workspaceId}/members", workspaceDeps.MembersCreate)
			authed.Delete("/workspaces/{workspaceId}/members/{userId}", workspaceDeps.MemberDelete)
			authed.Get("/workspaces/{workspaceId}/rules", workspaceDeps.RulesGet)
			authed.Put("/workspaces/{workspaceId}/rules", workspaceDeps.RulesUpdate)
			authed.Get("/workspaces/{workspaceId}/brands", workspaceDeps.BrandsList)
			authed.Post("/workspaces/{workspaceId}/brands", workspaceDeps.BrandCreate)
			authed.Post("/workspaces/{workspaceId}/brands/{brandId}/attach", workspaceDeps.BrandAttach)
			authed.Delete("/workspaces/{workspaceId}/brands/{brandId}/attach", workspaceDeps.BrandDetach)
			authed.Get("/brands/{brandId}/configuration", brands.Configuration)
			authed.Get("/brands/{brandId}/dashboard", brands.Dashboard)
			authed.Get("/brands/{brandId}/overview", brands.Overview)
			authed.Post("/brands/{brandId}/prompts", brands.CreatePrompt)
			authed.Post("/brands/{brandId}/prompts/{promptId}/activate", brands.ActivatePrompt)
			authed.Get("/brands/{brandId}/members", brands.MembersList)
			authed.Post("/brands/{brandId}/members", brands.MembersCreate)
			authed.Put("/brands/{brandId}/members/{membershipId}", brands.MemberUpdate)
			authed.Get("/brands/{brandId}/invitations", brands.InvitationsList)
			authed.Delete("/brands/{brandId}/invitations/{invitationId}", brands.InvitationRevoke)
			authed.Post("/invitations/{token}/accept", brands.InvitationAccept)
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

			// Profiler (Structured Knowledge Workspace) tingkat klien.
			authed.Get("/brands/{brandId}/profiler/profiles", profilerDeps.ProfilesList)
			authed.Post("/brands/{brandId}/profiler/profiles", profilerDeps.ProfileCreate)
			authed.Get("/brands/{brandId}/profiler/profiles/{profileId}", profilerDeps.ProfileGet)
			authed.Get("/brands/{brandId}/profiler/profiles/{profileId}/export", profilerDeps.ProfileExport)
			authed.Get("/brands/{brandId}/profiler/profiles/{profileId}/history", profilerDeps.ProfileHistory)
			authed.Put("/brands/{brandId}/profiler/profiles/{profileId}", profilerDeps.ProfileUpdate)
			authed.Delete("/brands/{brandId}/profiler/profiles/{profileId}", profilerDeps.ProfileDelete)
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/duplicate", profilerDeps.ProfileDuplicate)
			// Pustaka template brief: simpan brief sebagai template, lalu buat
			// brief baru dari template itu (lintas brand dalam workspace).
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/save-as-template", profilerDeps.SaveAsTemplate)
			authed.Get("/templates", profilerDeps.ListTemplates)
			authed.Post("/templates/{templateId}/apply", profilerDeps.ApplyTemplate)
			authed.Delete("/templates/{templateId}", profilerDeps.DeleteTemplate)
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/blocks", profilerDeps.BlockCreate)
			authed.Put("/brands/{brandId}/profiler/blocks/{blockId}", profilerDeps.BlockUpdate)
			authed.Delete("/brands/{brandId}/profiler/blocks/{blockId}", profilerDeps.BlockDelete)
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/documents", profilerDeps.DocumentUpload)
			authed.Delete("/brands/{brandId}/profiler/documents/{documentId}", profilerDeps.DocumentDelete)
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/images", profilerDeps.ImageUpload)
			authed.Put("/brands/{brandId}/profiler/images/{imageId}", profilerDeps.ImageUpdate)
			authed.Delete("/brands/{brandId}/profiler/images/{imageId}", profilerDeps.ImageDelete)
			authed.Get("/brands/{brandId}/profiler/profiles/{profileId}/timeline", profilerDeps.TimelineList)
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/timeline", profilerDeps.TimelineCreate)
			authed.Delete("/brands/{brandId}/profiler/timeline/{postId}", profilerDeps.TimelineDelete)
			authed.Get("/brands/{brandId}/profiler/link-preview", profilerDeps.TimelineLinkPreview)
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/urls", profilerDeps.URLAdd)
			authed.Post("/brands/{brandId}/profiler/urls/{urlId}/sync", profilerDeps.URLSync)
			authed.Delete("/brands/{brandId}/profiler/urls/{urlId}", profilerDeps.URLDelete)

			// Taut Brief — chatbot penyusun Profiler brand (terisolasi dari agent Taut Pintar).
			authed.Get("/brands/{brandId}/profiler/profiles/{profileId}/brief/chat", briefDeps.BriefChatGet)
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/brief/chat", briefDeps.BriefChatPost)
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/brief/chat/stream", briefDeps.BriefChatStream)
			authed.Delete("/brands/{brandId}/profiler/profiles/{profileId}/brief/chat", briefDeps.BriefChatReset)
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/brief/fields", briefDeps.BriefFieldsPost)
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/brief/fields/batch", briefDeps.BriefFieldsBatchPost)
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/brief/actions/consume", briefDeps.BriefActionConsume)
			authed.Get("/brands/{brandId}/profiler/profiles/{profileId}/brief/summaries", briefDeps.BriefSummariesGet)
			authed.Post("/brands/{brandId}/profiler/profiles/{profileId}/brief/summaries/{kind}/regenerate", briefDeps.BriefSummaryRegenerate)

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
			authed.Get("/brands/{brandId}/performance", brandFlowDeps.PerformanceOverview)
			authed.Get("/brands/{brandId}/audit", brandFlowDeps.AuditPublications)
			authed.Get("/brands/{brandId}/calendar", brandFlowDeps.CalendarMonth)
			authed.Get("/brands/{brandId}/campaigns/{campaignId}/performance", resolveCampaign(brandFlowDeps.CampaignPerformance))
			authed.Get("/brands/{brandId}/analytics", clientRead.Analytics)
			authed.Get("/brands/{brandId}/credits", clientRead.Credits)
			authed.Get("/brands/{brandId}/invoices", clientRead.Invoices)
			authed.Get("/brands/{brandId}/credits/topup", brandFlowDeps.CreditsTopupLookup)
			authed.Post("/brands/{brandId}/credits/topup", brandFlowDeps.CreditsTopupCreate)
			authed.Post("/brands/{brandId}/link-monitor/request-replacement", brandFlowDeps.LinkMonitorRequestReplacement)
			authed.Get("/brands/{brandId}/credits/promo", brandFlowDeps.CreditsPromoCheck)
			authed.Get("/brands/{brandId}/billing-profile", brandFlowDeps.BillingProfileGet)
			authed.Put("/brands/{brandId}/billing-profile", brandFlowDeps.BillingProfilePut)
			authed.Post("/brands/{brandId}/link-monitor/guarantee/choose", brandFlowDeps.LinkGuaranteeChoose)
			authed.Post("/brands/{brandId}/link-monitor/check", brandFlowDeps.LinkMonitorCheckBrand)
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
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/produce/topic-instruction", resolveCampaign(brandFlowDeps.TautTopicInstruction))
			// Usulan topik untuk dialog "Buat N artikel" (satu panggilan untuk
			// seluruh artikel pada satu media).
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/produce/topics", resolveCampaign(brandFlowDeps.TautSuggestTopics))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/produce/{mediaId}/stream", resolveCampaign(brandFlowDeps.TautProduceOneStream))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow/analyze", resolveCampaign(brandFlowDeps.CampaignFlowAnalyze))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow/article-regen", resolveCampaign(brandFlowDeps.CampaignFlowArticleRegen))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow/checkout", resolveCampaign(brandFlowDeps.CampaignFlowCheckout))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow/media", resolveCampaign(brandFlowDeps.CampaignFlowMedia))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/flow/media-type", resolveCampaign(brandFlowDeps.CampaignFlowMediaType))
			authed.Post("/brands/{brandId}/campaigns/{campaignId}/report/share", resolveCampaign(brandFlowDeps.CampaignReportShare))
			authed.Delete("/brands/{brandId}/campaigns/{campaignId}/report/share", resolveCampaign(brandFlowDeps.CampaignReportRevoke))
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
			authed.Post("/brands/{brandId}/publications/{publicationId}/schedule", brandFlowDeps.PublicationSchedule)
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
			authed.Get("/brands/{brandId}/media/similar", brandFlowDeps.MediaSimilar)
			authed.Get("/brands/{brandId}/loyalty", brandFlowDeps.Loyalty)
			authed.Post("/brands/{brandId}/media/matchmaker", brandFlowDeps.MediaMatchmaker)
			authed.Put("/brands/{brandId}/notification-webhooks/{webhookId}", brandFlowDeps.NotificationWebhookUpdate)
			authed.Delete("/brands/{brandId}/notification-webhooks/{webhookId}", brandFlowDeps.NotificationWebhookDelete)
			authed.Get("/brands/{brandId}/publication-plans/{planId}", brandFlowDeps.PublicationPlanGet)
			authed.Post("/brands/{brandId}/publications/variants", brandFlowDeps.PublicationVariantsCreate)
			authed.Get("/brands/{brandId}/publications/variants", brandFlowDeps.PublicationVariantsList)
			authed.Get("/brands/{brandId}/publications/{publicationId}", brandFlowDeps.PublicationGet)
			authed.Get("/brands/{brandId}/publications/{publicationId}/replacement", brandFlowDeps.PublicationReplacementGet)
			authed.Get("/brands/{brandId}/taut/agent-status", brandFlowDeps.TautAgentStatus)
			authed.Get("/brands/{brandId}/taut/article-types", brandFlowDeps.TautArticleTypesList)
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
			authed.Get("/brands/{brandId}/notifications/publication-unread", clientRead.BrandPublicationUnread)
			authed.Post("/brands/{brandId}/notifications/publication-unread", clientRead.BrandPublicationUnread)
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/writer-links", resolveCampaign(brandFlowDeps.TautWriterLinks))
			authed.Put("/brands/{brandId}/taut/campaigns/{campaignId}/writer-links", resolveCampaign(brandFlowDeps.TautWriterLinks))
			// Pustaka gambar AI per artikel (isolasi brand+campaign+media).
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/images", resolveCampaign(brandFlowDeps.ArticleImagesList))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/images/generate", resolveCampaign(brandFlowDeps.ArticleImageGenerate))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/images/upload", resolveCampaign(brandFlowDeps.ArticleImageUpload))
			authed.Delete("/brands/{brandId}/taut/campaigns/{campaignId}/images/{imageId}", resolveCampaign(brandFlowDeps.ArticleImageDelete))

			// ---------- Taut Pintar klien (/v1/brands/{brandId}/taut/*) ----------
			// ---------- Obrolan klien ↔ mitra (forum per publikasi, 3×24 jam) ----------
			authed.Get("/brands/{brandId}/chat/threads", chatDeps.ClientThreadsList)
			authed.Post("/brands/{brandId}/chat/threads", chatDeps.ClientThreadCreate)
			// Rating kecepatan terbit per media (tombol "Beri Rating" di ringkasan).
			authed.Get("/brands/{brandId}/media/{mediaId}/rating", chatDeps.MediaRatingGet)
			authed.Put("/brands/{brandId}/media/{mediaId}/rating", chatDeps.MediaRatingPut)
			authed.Get("/brands/{brandId}/chat/threads/{threadId}", chatDeps.ClientThreadGet)
			authed.Post("/brands/{brandId}/chat/threads/{threadId}/messages", chatDeps.ClientMessageSend)

			authed.Get("/brands/{brandId}/taut/campaigns", tautDeps.TautCampaignsList)
			authed.Post("/brands/{brandId}/taut/campaigns", tautDeps.TautCampaignCreate)
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}", resolveCampaign(tautDeps.TautCampaignGet))
			authed.Put("/brands/{brandId}/taut/campaigns/{campaignId}", resolveCampaign(tautDeps.TautCampaignUpdate))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}", resolveCampaign(tautDeps.TautCampaignAction))
			authed.Delete("/brands/{brandId}/taut/campaigns/{campaignId}", resolveCampaign(tautDeps.TautCampaignDelete))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/advance", resolveCampaign(tautDeps.TautCampaignAdvance))
			// Putaran baru: pakai ulang kampanye yang sudah terdistribusi untuk
			// postingan berikutnya di waktu lain.
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/rounds", resolveCampaign(tautDeps.TautCampaignNewRound))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/duplicate", resolveCampaign(tautDeps.TautCampaignDuplicate))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/validate", resolveCampaign(tautDeps.TautCampaignValidateGet))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/validate", resolveCampaign(tautDeps.TautCampaignValidatePost))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/schedule", resolveCampaign(tautDeps.TautCampaignSchedule))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/budget-estimate", resolveCampaign(tautDeps.TautBudgetEstimate))
			authed.Get("/brands/{brandId}/taut/campaigns/{campaignId}/placement", resolveCampaign(tautDeps.TautPlacementGet))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/placement", resolveCampaign(tautDeps.TautPlacementPost))
			authed.Put("/brands/{brandId}/taut/campaigns/{campaignId}/placement", resolveCampaign(tautDeps.TautPlacementPut))
			// Jumlah artikel per media (satu domain boleh beberapa artikel).
			authed.Put("/brands/{brandId}/taut/campaigns/{campaignId}/placement/{mediaId}/quantity", resolveCampaign(tautDeps.TautPlacementQuantity))
			// Hapus satu artikel tertentu pada satu media (tombol tong sampah).
			authed.Delete("/brands/{brandId}/taut/campaigns/{campaignId}/placement/{mediaId}/article/{slot}", resolveCampaign(tautDeps.TautPlacementDeleteArticle))
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
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/angles/{slug}/pick", resolveCampaign(tautDeps.TautAnglePick))
			authed.Post("/brands/{brandId}/taut/campaigns/{campaignId}/submit-final", resolveCampaign(tautDeps.TautSubmitFinal))

		})

		// Berita acara kampanye by token share — PUBLIK (tautan dibagikan ke
		// pihak ketiga), parity app/v1/campaign-reports/[token].
		v1.Get("/campaign-reports/{token}", clientRead.CampaignReportGet)
		// Pratinjau undangan anggota brand (tanpa login; token acak sekali pakai).
		v1.Get("/invitations/{token}", brands.InvitationPreview)

		// ---------- Katalog media PUBLIK (/v1/public/*) — data.tauthub.com ----------
		// Tanpa autentikasi; hanya media active + public_listed yang tampil.
		// Kill switch: PUBLIC_CATALOG_ENABLED=false → semua balas 503.
		publicCatalog := &publiccatalogdomain.Deps{Pool: database.Pool, Enabled: cfg.PublicCatalogEnabled}
		v1.Get("/public/media", publicCatalog.MediaList)
		v1.Get("/public/media/{publicSlug}", publicCatalog.MediaDetail)
		v1.Get("/public/kategori", publicCatalog.Categories)
		v1.Get("/public/statistik", publicCatalog.Stats)
		v1.Get("/public/sitemap", publicCatalog.Sitemap)
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
		// Webhook bounce/komplain email dari Kirim.id (publik, HMAC X-Kirim-Signature).
		v1.Post("/webhooks/email/kirim", platformDeps.KirimEmailWebhook)
		v1.Get("/payments/status/{transactionId}", platformDeps.PublicPaymentStatus)

		v1.Group(func(authed chi.Router) {
			authed.Use(authDeps.Auth.RequireAuth)

			authed.Get("/payments", platformDeps.PublicPaymentsList)
			authed.Post("/payments", platformDeps.PublicPaymentsCreate)
			authed.Get("/payments/gateway/active", platformDeps.PublicPaymentGatewayActive)

			// ---------- Portal mitra (/mitra) — port app/v1/partner/* ----------
			authed.Get("/partner/chat/threads", chatDeps.PartnerThreadsList)
			authed.Post("/partner/chat/threads", chatDeps.PartnerThreadCreate)
			authed.Get("/partner/chat/threads/{threadId}", chatDeps.PartnerThreadGet)
			authed.Post("/partner/chat/threads/{threadId}/messages", chatDeps.PartnerMessageSend)
			authed.Get("/partner/me", partnerDeps.Me)
			authed.Patch("/partner/me", partnerDeps.MeUpdate)
			authed.Get("/partner/me/vip", partnerDeps.MeVip)
			authed.Get("/partner/seller-center", partnerDeps.SellerCenter)
			authed.Get("/partner/traffic", partnerDeps.Traffic)
			authed.Post("/partner/media/{mediaId}/gsc-snapshot", partnerDeps.GSCMediaSnapshot)

			authed.Get("/partner/media", partnerDeps.MediaList)
			authed.Post("/partner/media", partnerDeps.MediaCreate)
			authed.Post("/partner/media/import", partnerDeps.MediaImport)
			authed.Get("/partner/secure-direct/request", partnerDeps.SecureDirectStatus)
			authed.Post("/partner/secure-direct/request", partnerDeps.SecureDirectRequestCreate)
			authed.Get("/partner/media/categories", partnerDeps.MediaCategories)
			authed.Get("/partner/media/template-url", partnerDeps.MediaTemplateURL)
			authed.Get("/partner/media/{mediaId}", partnerDeps.MediaGet)
			authed.Put("/partner/media/{mediaId}", partnerDeps.MediaUpdate)
			authed.Delete("/partner/media/{mediaId}", partnerDeps.MediaDelete)
			authed.Post("/partner/media/{mediaId}/submit", partnerDeps.MediaSubmit)
			authed.Post("/partner/media/{mediaId}/catalog-visibility", partnerDeps.MediaCatalogVisibility)
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
			authed.Delete("/partner/media/{mediaId}/dewan-pers-verification", partnerDeps.MediaDewanPersVerificationDelete)
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
			authed.Get("/partner/publications/unread-count", partnerDeps.PublicationsUnread)
			authed.Post("/partner/publications/mark-seen", partnerDeps.PublicationsMarkSeen)
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
			// Tiket dukungan mitra: tiket + percakapan dengan admin.
			authed.Get("/partner/tickets", partnerDeps.Tickets)
			authed.Post("/partner/tickets", partnerDeps.Tickets)
			authed.Get("/partner/tickets/unread-count", partnerDeps.TicketUnreadCount)
			authed.Get("/partner/tickets/{ticketId}", partnerDeps.TicketDetail)
			authed.Post("/partner/tickets/{ticketId}/messages", partnerDeps.TicketReply)
			authed.Post("/partner/tickets/{ticketId}/status", partnerDeps.TicketStatus)

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
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/oauth/google", platformDeps.OAuthGoogleGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/oauth/google", platformDeps.OAuthGooglePut)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/server-config", platformDeps.ServerConfigGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/server-config", platformDeps.ServerConfigPut)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/short-link-config", platformDeps.ShortLinkConfigGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/short-link-config", platformDeps.ShortLinkConfigPut)
			authed.Get("/platform/admins", platformDeps.AdminsList)
			authed.Post("/platform/admins", platformDeps.AdminsCreate)
			authed.Get("/platform/admins/me", platformDeps.AdminsMe)
			authed.Patch("/platform/admins/{adminId}", platformDeps.AdminsUpdate)
			authed.Put("/platform/admins/{adminId}/wa-number", platformDeps.AdminsWANumber)
			authed.Delete("/platform/admins/{adminId}", platformDeps.AdminsDelete)

			// Aksi yang diusulkan agent MCP (Hermes / Claude Code). Hanya baca —
			// persetujuan dilakukan di bot Telegram, bukan lewat endpoint ini.
			authed.Get("/platform/agent-actions", platformDeps.AgentActionsList)
			authed.Get("/platform/agent-actions/{id}", platformDeps.AgentActionGet)

			// Konfigurasi MCP admin (service key, bot Telegram, allowlist, kill
			// switch tulis). Super Admin only karena menyentuh secret.
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/mcp-config", platformDeps.McpConfigGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/mcp-config", platformDeps.McpConfigPut)

			// Riwayat notifikasi admin (per user admin yang login).
			authed.Get("/platform/notifications", platformDeps.AdminNotifications)
			authed.Post("/platform/notifications", platformDeps.AdminNotifications)
			authed.Post("/platform/notifications/{notificationId}/read", platformDeps.AdminNotificationRead)

			// Pengumuman (sisi admin).
			authed.Get("/platform/announcements", platformDeps.AdminAnnouncementsList)
			authed.Post("/platform/announcements", platformDeps.AdminAnnouncementCreate)
			authed.Delete("/platform/announcements/{announcementId}", platformDeps.AdminAnnouncementDelete)

			// Changelog (sisi admin): impor entri dari web/static/changelog.md,
			// ubah status terbit, hapus entri.
			authed.Get("/platform/chat/threads", platformDeps.AdminChatThreads)
			authed.Get("/platform/chat/threads/{threadId}", platformDeps.AdminChatThreadDetail)
			// Tiket dukungan: semua admin boleh membalas.
			authed.Get("/platform/tickets", platformDeps.AdminTickets)
			// Admin memulai percakapan dengan mitra (tombol "Chat partner").
			authed.Post("/platform/tickets", platformDeps.AdminTicketStart)
			authed.Get("/platform/tickets/unread-count", platformDeps.AdminTicketsUnreadCount)
			authed.Get("/platform/tickets/{ticketId}", platformDeps.AdminTicketDetail)
			authed.Post("/platform/tickets/{ticketId}/messages", platformDeps.AdminTicketReply)
			authed.Patch("/platform/tickets/{ticketId}", platformDeps.AdminTicketStatus)
			authed.Get("/platform/changelog", platformDeps.AdminChangelogList)
			authed.Post("/platform/changelog/import", platformDeps.AdminChangelogImport)
			authed.Patch("/platform/changelog/{entryId}", platformDeps.AdminChangelogUpdate)
			authed.Delete("/platform/changelog/{entryId}", platformDeps.AdminChangelogDelete)

			// AI.
			authed.Get("/platform/ai/configuration", platformDeps.AIConfigurationGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/ai/configuration", platformDeps.AIConfigurationPut)
			authed.Get("/platform/ai/models", platformDeps.AIModelsList)
			authed.Get("/platform/ai/models/{modelId}/knowledge", platformDeps.AIModelKnowledgeGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/ai/models/{modelId}/knowledge", platformDeps.AIModelKnowledgePut)
			authed.With(platformDeps.SuperAdminOnly).Delete("/platform/ai/models/{modelId}/knowledge", platformDeps.AIModelKnowledgeDelete)
			authed.Post("/platform/ai/probe", platformDeps.AIProbe)
			authed.Get("/platform/ai/status", platformDeps.AIStatus)
			authed.Post("/platform/ai/test-generation", platformDeps.AITestGeneration)
			authed.Post("/platform/ai/image-test", platformDeps.AIImageTest)
			authed.Get("/platform/ai/image-logs", platformDeps.AIImageLogs)
			authed.Get("/platform/ai/usage", platformDeps.AIUsage)

			// Media (segmen literal sebelum {mediaId}).
			authed.Get("/platform/media", platformDeps.MediaList)
			authed.Post("/platform/media", platformDeps.MediaCreate)
			authed.Get("/platform/media/archive-impact", platformDeps.MediaArchiveImpact)
			authed.Post("/platform/media/archive-all", platformDeps.MediaArchiveAll)
			authed.Post("/platform/media/bulk", platformDeps.MediaBulkUpsert)
			authed.Delete("/platform/media/bulk", platformDeps.MediaBulkDelete)
			authed.Post("/platform/media/bulk-link-kind", platformDeps.MediaBulkLinkKind)
			authed.Get("/platform/media/dewan-pers-reviews", platformDeps.MediaDewanPersReviews)
			authed.Post("/platform/media/dewan-pers-reviews/{reviewId}", platformDeps.MediaDewanPersReviewDecide)
			authed.Get("/platform/media/intelligence", platformDeps.MediaIntelligence)
			authed.Post("/platform/media/recompute-health", platformDeps.MediaRecomputeHealthBulk)
			authed.Post("/platform/media/restore-archived", platformDeps.MediaRestoreArchived)
			authed.Post("/platform/media/seo-bulk", platformDeps.MediaSEOBulk)
			// Cek DA semua: antrekan media_seo_refresh untuk seluruh media yang
			// metriknya belum segar, plus indikator antreannya.
			authed.Post("/platform/media/seo-refresh-all", platformDeps.MediaSEORefreshAll)
			authed.Get("/platform/media/seo-refresh-status", platformDeps.MediaSEORefreshStatus)
			// Riwayat job pemeriksaan media (tab "Log" di /admin/media).
			authed.Get("/platform/media/job-log", platformDeps.MediaJobLog)
			authed.Post("/platform/media/similarweb-bulk", platformDeps.MediaSimilarWebBulk)
			authed.Get("/platform/media/similarweb-status", platformDeps.MediaSimilarWebStatus)
			authed.Post("/platform/media/sync-credit-cost", platformDeps.MediaSyncCreditCost)
			authed.Get("/platform/media/{mediaId}", platformDeps.MediaGet)
			authed.Put("/platform/media/{mediaId}", platformDeps.MediaUpdate)
			authed.Delete("/platform/media/{mediaId}", platformDeps.MediaDelete)
			authed.Get("/platform/media/{mediaId}/audit", platformDeps.MediaAudit)
			authed.Post("/platform/media/{mediaId}/recompute-health", platformDeps.MediaRecomputeHealth)
			authed.Post("/platform/media/{mediaId}/review", platformDeps.MediaReview)
			// Pelanggaran + penangguhan domain (panel detail media admin).
			authed.Get("/platform/media/{mediaId}/violations", platformDeps.MediaViolationsList)
			authed.Post("/platform/media/{mediaId}/violations", platformDeps.MediaViolationCreate)
			authed.Post("/platform/media/{mediaId}/suspend", platformDeps.MediaSuspend)
			authed.Post("/platform/media/{mediaId}/unsuspend", platformDeps.MediaUnsuspend)
			authed.Get("/platform/media/{mediaId}/seo", platformDeps.MediaSEO)
			authed.Post("/platform/media/{mediaId}/seo", platformDeps.MediaSEO)
			// Antrekan, bukan jalankan langsung: satu pemeriksaan butuh ~25-30
			// detik (kecepatan situs + penyedia DA + SERPens) sehingga selalu
			// melewati batas 12 detik di klien bila dijalankan sinkron.
			authed.Post("/platform/media/{mediaId}/seo-refresh", platformDeps.MediaSEORefreshOne)
			// Cek GEO satuan: hanya AI Trust + sitasi, tanpa penyedia DA/PA.
			authed.Post("/platform/media/{mediaId}/geo-refresh", platformDeps.MediaGEORefreshOne)
			authed.Post("/platform/media/seo-refresh-cancel", platformDeps.MediaSEORefreshCancel)
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
			authed.Post("/platform/media-summaries/{mediaId}/draft", platformDeps.MediaSummaryAIDraft)
			authed.Post("/platform/media-summaries/{mediaId}/manual", platformDeps.MediaSummaryManualSet)

			// Operations.
			authed.Get("/platform/operations/access", platformDeps.OperationsAccess)
			authed.Get("/platform/operations/summary", platformDeps.OperationsSummary)
			// audit = alias audit-log di app lama (routes.json: metode kosong).
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/operations/audit", platformDeps.OperationsAudit)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/operations/audit-log", platformDeps.OperationsAuditLog)
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
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/operations/errors", platformDeps.OperationsErrors)
			authed.Put("/platform/operations/generation-switch", platformDeps.OperationsGenerationSwitch)
			authed.Get("/platform/operations/queue-alerts", platformDeps.OperationsQueueAlerts)
			authed.Get("/platform/operations/notifications", platformDeps.OperationsNotifications)
			authed.Get("/platform/operations/publications", platformDeps.OperationsPublications)
			authed.Get("/platform/operations/publications/{publicationId}", platformDeps.OperationsPublicationDetail)
			authed.Post("/platform/operations/publications/{publicationId}/retry", platformDeps.OperationsPublicationRetry)
			authed.Post("/platform/operations/publications/{publicationId}/cancel", platformDeps.OperationsPublicationCancel)
			authed.Post("/platform/operations/publications/{publicationId}/repair-images", platformDeps.OperationsPublicationRepairImages)
			authed.Post("/platform/operations/publications/{publicationId}/resync-wordpress", platformDeps.OperationsPublicationResyncWordpress)
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
			authed.Get("/platform/operations/registration-blocks", platformDeps.RegistrationBlocksList)
			authed.Delete("/platform/operations/registration-blocks", platformDeps.RegistrationBlockRemove)
			authed.Get("/platform/operations/email-suppressions", platformDeps.EmailSuppressionsList)
			authed.Delete("/platform/operations/email-suppressions", platformDeps.EmailSuppressionRemove)
			authed.Get("/platform/operations/blocked-domains", platformDeps.BlockedDomainsList)
			authed.Get("/platform/operations/blocked-domains/builtin", platformDeps.BuiltinDisposableDomains)
			authed.Post("/platform/operations/blocked-domains", platformDeps.BlockedDomainAdd)
			authed.Delete("/platform/operations/blocked-domains", platformDeps.BlockedDomainRemove)
			authed.Get("/platform/operations/registration-guard", platformDeps.RegistrationGuardLimits)
			authed.Put("/platform/operations/registration-guard", platformDeps.RegistrationGuardLimits)
			authed.Get("/platform/operations/variant-batches", platformDeps.OperationsVariantBatches)
			authed.Get("/platform/operations/variant-batches/{batchId}", platformDeps.OperationsVariantBatchDetail)
			authed.Post("/platform/operations/variant-batches/{batchId}/retry", platformDeps.OperationsVariantBatchRetry)

			// Partners.
			authed.Get("/platform/partners", platformDeps.PartnersList)
			authed.Post("/platform/partners", platformDeps.PartnerCreate)
			authed.Get("/platform/partners/{partnerId}", platformDeps.PartnerGet)
			authed.Put("/platform/partners/{partnerId}", platformDeps.PartnerUpdate)
			authed.Post("/platform/partners/{partnerId}/wa-reminder", platformDeps.AdminPartnerWAReminder)
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

			// Payments (admin). Super Admin saja: konfigurasi gateway berisi
			// kredensial payment provider (server key, secret), jadi tidak
			// dibuka untuk admin biasa meski punya permission "payments".
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/payments/gateways", platformDeps.PaymentsGateways)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/payments/gateways", platformDeps.PaymentGatewaySave)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/payments/gateways/{gatewayId}/activate", platformDeps.PaymentGatewayActivate)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/payments/gateways/{gatewayId}/test", platformDeps.PaymentGatewayTest)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/payments/transactions", platformDeps.PaymentTransactions)

			// Settings.
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/settings/email", platformDeps.SettingsEmailGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/settings/email", platformDeps.SettingsEmailPut)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/settings/email/webhook", platformDeps.KirimWebhookSettingsGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/settings/email/webhook", platformDeps.KirimWebhookSettingsPut)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/settings/email/deliveries", platformDeps.SettingsEmailDeliveries)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/settings/email/test-connection", platformDeps.SettingsEmailTestConnection)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/settings/email/test-send", platformDeps.SettingsEmailTestSend)
			// Panel Template email: gaya + copy per audiens + saklar kanal.
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/email/style", platformDeps.EmailStyleGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/email/style", platformDeps.EmailStylePut)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/email/templates", platformDeps.EmailTemplatesList)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/email/templates", platformDeps.EmailTemplateSave)
			authed.With(platformDeps.SuperAdminOnly).Delete("/platform/email/templates", platformDeps.EmailTemplateReset)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/email/channels", platformDeps.EmailChannelSet)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/email/preview", platformDeps.EmailPreview)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/email/test-send", platformDeps.EmailTemplateTestSend)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/settings/pagespeed", platformDeps.SettingsPageSpeedGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/settings/pagespeed", platformDeps.SettingsPageSpeedPut)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/settings/pagespeed/test", platformDeps.SettingsPageSpeedTest)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/settings/turnstile", platformDeps.SettingsTurnstileGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/settings/turnstile", platformDeps.SettingsTurnstilePut)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/settings/turnstile/test", platformDeps.SettingsTurnstileTest)

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

			// Taut Brief — console admin asisten Profiler brand.
			authed.Get("/platform/taut-brief/config", briefDeps.TautBriefConfigGet)
			authed.Put("/platform/taut-brief/config", briefDeps.TautBriefConfigUpdate)
			authed.Post("/platform/taut-brief/config/test", briefDeps.TautBriefConfigTest)
			authed.Get("/platform/taut-brief/prompts", briefDeps.TautBriefPromptsList)
			authed.Post("/platform/taut-brief/prompts", briefDeps.TautBriefPromptCreate)
			authed.Get("/platform/taut-brief/prompts/{versionId}", briefDeps.TautBriefPromptGet)
			authed.Patch("/platform/taut-brief/prompts/{versionId}", briefDeps.TautBriefPromptUpdate)
			authed.Delete("/platform/taut-brief/prompts/{versionId}", briefDeps.TautBriefPromptDelete)
			authed.Post("/platform/taut-brief/prompts/{versionId}/activate", briefDeps.TautBriefPromptActivate)
			authed.Get("/platform/taut-brief/fields", briefDeps.TautBriefFieldsList)
			authed.Post("/platform/taut-brief/fields", briefDeps.TautBriefFieldCreate)
			authed.Get("/platform/taut-brief/fields/{versionId}", briefDeps.TautBriefFieldGet)
			authed.Patch("/platform/taut-brief/fields/{versionId}", briefDeps.TautBriefFieldUpdate)
			authed.Delete("/platform/taut-brief/fields/{versionId}", briefDeps.TautBriefFieldDelete)
			authed.Post("/platform/taut-brief/fields/{versionId}/activate", briefDeps.TautBriefFieldActivate)
			authed.Post("/platform/taut-brief/test", briefDeps.TautBriefTest)
			authed.Get("/platform/taut-brief/logs", briefDeps.TautBriefLogs)
			// Agent ringkasan brief (entitas & file) — config + prompt berversi.
			authed.Get("/platform/taut-brief/agents/{agent}/config", briefDeps.TautBriefAgentConfigGet)
			authed.Put("/platform/taut-brief/agents/{agent}/config", briefDeps.TautBriefAgentConfigUpdate)
			authed.Post("/platform/taut-brief/agents/{agent}/config/test", briefDeps.TautBriefAgentConfigTest)
			authed.Get("/platform/taut-brief/agents/{agent}/prompts", briefDeps.TautBriefAgentPromptsList)
			authed.Post("/platform/taut-brief/agents/{agent}/prompts", briefDeps.TautBriefAgentPromptCreate)
			authed.Get("/platform/taut-brief/agents/{agent}/prompts/{versionId}", briefDeps.TautBriefAgentPromptGet)
			authed.Patch("/platform/taut-brief/agents/{agent}/prompts/{versionId}", briefDeps.TautBriefAgentPromptUpdate)
			authed.Delete("/platform/taut-brief/agents/{agent}/prompts/{versionId}", briefDeps.TautBriefAgentPromptDelete)
			authed.Post("/platform/taut-brief/agents/{agent}/prompts/{versionId}/activate", briefDeps.TautBriefAgentPromptActivate)
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
			authed.Get("/platform/taut/article-types", platformDeps.TautArticleTypesList)
			authed.Post("/platform/taut/article-types", platformDeps.TautArticleTypeCreate)
			authed.Get("/platform/taut/article-types/{typeId}", platformDeps.TautArticleTypeGet)
			authed.Patch("/platform/taut/article-types/{typeId}", platformDeps.TautArticleTypeUpdate)
			authed.Delete("/platform/taut/article-types/{typeId}", platformDeps.TautArticleTypeDelete)
			authed.Post("/platform/taut/article-types/{typeId}/test", platformDeps.TautArticleTypeTest)
			authed.Get("/platform/taut/article-types/{typeId}/versions", platformDeps.TautArticleTypeVersions)
			authed.Post("/platform/taut/article-types/{typeId}/versions", platformDeps.TautArticleTypeVersionCreate)
			authed.Get("/platform/taut/article-types/{typeId}/versions/{versionId}", platformDeps.TautArticleTypeVersionGet)
			authed.Patch("/platform/taut/article-types/{typeId}/versions/{versionId}", platformDeps.TautArticleTypeVersionUpdate)
			authed.Delete("/platform/taut/article-types/{typeId}/versions/{versionId}", platformDeps.TautArticleTypeVersionDelete)
			authed.Post("/platform/taut/article-types/{typeId}/versions/{versionId}/activate", platformDeps.TautArticleTypeVersionActivate)
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
			authed.Post("/platform/maintenance/retention", platformDeps.MaintenanceRetention)
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
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/crawlhub", platformDeps.CrawlhubConfig)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/crawlhub", platformDeps.CrawlhubAction)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/crawlhub", platformDeps.CrawlhubConfigUpdate)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/crawlhub/targets", platformDeps.CrawlhubTargets)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/crawlhub/targets", platformDeps.CrawlhubTargetsSync)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/gsc-config", platformDeps.GSCConfigGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/gsc-config", platformDeps.GSCConfigPut)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/ga4-configs", platformDeps.GA4ConfigsList)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/ga4-configs", platformDeps.GA4ConfigCreate)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/ga4-configs/{configId}", platformDeps.GA4ConfigGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/ga4-configs/{configId}", platformDeps.GA4ConfigUpdate)
			authed.With(platformDeps.SuperAdminOnly).Delete("/platform/ga4-configs/{configId}", platformDeps.GA4ConfigDelete)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/wa/config", platformDeps.WAGatewayConfig)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/wa/config", platformDeps.WAGatewayConfigUpdate)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/wa/deliveries", platformDeps.WADeliveries)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/wa/templates", platformDeps.WATemplates)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/wa/templates", platformDeps.WATemplateSave)
			authed.With(platformDeps.SuperAdminOnly).Delete("/platform/wa/templates", platformDeps.WATemplateReset)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/wa/test", platformDeps.WATest)
			authed.Get("/platform/rss-feeds", platformDeps.RSSFeedsList)
			authed.Post("/platform/rss-feeds", platformDeps.RSSFeedCreate)
			authed.Get("/platform/rss-feeds/{feedId}", platformDeps.RSSFeedGet)
			authed.Put("/platform/rss-feeds/{feedId}", platformDeps.RSSFeedUpdate)
			authed.Delete("/platform/rss-feeds/{feedId}", platformDeps.RSSFeedDelete)
			authed.Post("/platform/rss-feeds/{feedId}", platformDeps.RSSFeedRotateToken)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/seo/config", platformDeps.SEOConfigGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/seo/config", platformDeps.SEOConfigPut)
			authed.Get("/platform/seo/diagnose", platformDeps.SEODiagnose)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/seo/similarweb-diagnose", platformDeps.SEOSimilarWebDiagnose)

			// Credits / invoices / packages / orders / revenue / settlements / referrals / VIP.
			authed.Get("/platform/finance/report", platformDeps.FinanceReport)
			authed.Get("/platform/finance/partner-payables", platformDeps.FinancePartnerPayables)
			authed.Get("/platform/finance/dormant-brands", platformDeps.FinanceDormantBrands)
			authed.Get("/platform/finance/admin-withdrawals", platformDeps.AdminProfitWithdrawalsList)
			authed.Post("/platform/finance/admin-withdrawals", platformDeps.AdminProfitWithdrawalCreate)
			authed.Put("/platform/finance/admin-withdrawals/{withdrawalId}/void", platformDeps.AdminProfitWithdrawalVoid)
			authed.Get("/platform/finance/client-deposits-detail", platformDeps.FinanceClientDepositsDetail)
			authed.Get("/platform/finance/escrow-detail", platformDeps.FinanceEscrowDetail)
			authed.Get("/platform/finance/partner-unpaid-detail", platformDeps.FinancePartnerUnpaidDetail)
			authed.Get("/platform/finance/profit-breakdown", platformDeps.FinanceProfitBreakdown)
			authed.Get("/platform/credits", platformDeps.CreditsConfigGet)
			authed.Post("/platform/credits", platformDeps.CreditsAdjust)
			authed.Put("/platform/credits", platformDeps.CreditsConfigPut)
			authed.Get("/platform/promo-codes", platformDeps.PromoCodesList)
			authed.Post("/platform/promo-codes", platformDeps.PromoCodesCreate)
			authed.Put("/platform/promo-codes/{code}", platformDeps.PromoCodesSetActive)
			// Invoice & paket kredit: Super Admin saja. Sebelumnya
			// /platform/packages bahkan tidak memeriksa permission sama sekali
			// (cukup "admin"), jadi admin biasa bisa membaca seluruh daftar
			// paket lewat API walau menunya tidak dipakai.
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/invoices", platformDeps.InvoicesList)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/invoices", platformDeps.InvoiceCreate)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/invoices/{invoiceId}", platformDeps.InvoiceGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/invoices/{invoiceId}", platformDeps.InvoiceUpdate)
			authed.With(platformDeps.SuperAdminOnly).Delete("/platform/invoices/{invoiceId}", platformDeps.InvoiceVoid)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/packages", platformDeps.CreditPackagesList)
			authed.With(platformDeps.SuperAdminOnly).Post("/platform/packages", platformDeps.CreditPackageCreate)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/packages/{packageId}", platformDeps.CreditPackageGet)
			authed.With(platformDeps.SuperAdminOnly).Put("/platform/packages/{packageId}", platformDeps.CreditPackageUpdate)
			authed.With(platformDeps.SuperAdminOnly).Delete("/platform/packages/{packageId}", platformDeps.CreditPackageRevoke)
			authed.Get("/platform/orders", platformDeps.OrdersList)
			authed.Get("/platform/orders/campaigns", platformDeps.OrdersCampaignsList)
			authed.Get("/platform/orders/campaigns/{campaignId}", platformDeps.OrdersCampaignDetail)
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
			authed.Get("/platform/loyalty-tiers", platformDeps.LoyaltyTiersList)
			authed.Put("/platform/loyalty-tiers", platformDeps.LoyaltyTierUpsert)
			authed.Delete("/platform/loyalty-tiers/{tierName}", platformDeps.LoyaltyTierDelete)

			// Bug reports / auto-replace / config / gatekeeper / diagnostics / link-monitor.
			authed.Post("/bug-reports", platformDeps.BugReportSubmit)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/bug-reports", platformDeps.BugReportsList)
			authed.With(platformDeps.SuperAdminOnly).Get("/platform/bug-reports/{reportId}", platformDeps.BugReportGet)
			authed.With(platformDeps.SuperAdminOnly).Patch("/platform/bug-reports/{reportId}", platformDeps.BugReportUpdate)
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
			authed.Get("/platform/link-guardian/claims", platformDeps.LinkGuardianClaimsList)
			authed.Post("/platform/link-guardian/claims/{claimId}/step", platformDeps.LinkGuardianClaimStep)
			authed.Post("/platform/link-guardian/claims/{claimId}/refund", platformDeps.LinkGuardianClaimRefund)
			authed.Post("/platform/link-guardian/claims/{claimId}/reject", platformDeps.LinkGuardianClaimReject)
		})

		// TODO fase 2+: peta lengkap di REWRITE/04_API_ENDPOINTS.md.
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Dengarkan pg_notify('mcp_config_changed') supaya rotasi service key di
	// admin UI langsung berlaku di proses API ini.
	listenCtx, listenCancel := context.WithCancel(context.Background())
	defer listenCancel()
	go mcpStore.ListenForChanges(listenCtx)

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
