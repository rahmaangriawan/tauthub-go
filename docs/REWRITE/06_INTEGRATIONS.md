# 06 — Kontrak Integrasi Eksternal

> Semua detail di sini diekstrak dari kode lama — **kontrak wire-level tidak boleh berubah** saat porting ke Go. File sumber dicantumkan per bagian.

---

## 1. AI Provider Gateway — `lib/ai/provider.ts` (1.266 baris)

- Protokol: OpenAI-compatible `POST {base_url}/chat/completions` + Bearer key.
- Runtime config dari DB `platform_ai_configuration` (base_url, **sealed_api_key**, model default/economy/premium, embedding) dengan fallback env (`AI_BASE_URL`, `AI_API_KEY`, `AI_DEFAULT_MODEL`, `AI_ECONOMY_MODEL`, `AI_PREMIUM_MODEL`, `AI_EMBEDDING_MODEL`).
- **Deteksi provider dari URL**: `google-gemini`, `cloudflare`, `openrouter`, `groq`, `openai`, `ollama`, generic `openai-compatible` — salin aturan deteksinya (beda base path & bentuk error).
- Retry: error 429/5xx → exponential backoff, cap 2s, `AI_MAX_RETRIES` (2), timeout `AI_REQUEST_TIMEOUT_SECONDS` (120).
- Usage: setiap call tercatat ke `model_usage_events` (token in/out/total, model, brand).
- Circuit breaker + kill switch: status di `/v1/platform/ai/status`, generasi dimatikan via `platform/operations/generation-switch`.
- Fallback mock bila provider belum dikonfigurasi (dev) — port juga.
- **Embeddings BUKAN call AI**: `lib/knowledge-base/embed.ts` = SHA-256 bag-of-token **128-dim**, L2-normalized, skor hybrid cosine + Jaccard — deterministik, dihitung di aplikasi (tanpa pgvector). Port harus **bit-identik** supaya embedding lama di DB tetap cocok (kolom `embedding jsonb`). Wajib unit test: embedding teks sama → vektor sama dengan versi Node.

## 2. Publishing Executor — `lib/publish/*` (2.506 baris)

Dispatcher `executor.ts`: `publishNow / executePublication / publishToWordpress / publishToWebhook`.

### 2.1 WordPress — `lib/publish/wordpress.ts`
- WP REST `wp/v2`, Basic auth (`username:app-password`).
- Term resolution: slug → name → search → **create bila belum ada**; featured image upload via `/wp/v2/media`; author mapping; custom fields.
- Resolved ID di-cache ke `publication_jobs.payload_snapshot` (idempotent retry).
- Mode default **draft-only** (kolom `publication_mode` media).

### 2.2 Blogger — `lib/publish/blogger.ts` + `app/api/blogger/callback`
- Google OAuth2 **PKCE**, state di tabel `blogger_oauth_states` (media_id, state, code_verifier).
- Token refresh + kredensial ter-seal per media (`enc_key_ver`).
- Publish: `blogger/{blogId}/posts/` (draft atau publish sesuai config).
- Callback endpoint: `/api/blogger/callback` (GET) + UI `/blogger-pick-blog`.

### 2.3 Custom Web — Tulisio/Tauthub Publisher Protocol v1 — `lib/publish/custom-web.ts` (R7)
- Outbound: `POST {endpoint}/v1/tauthub/publish` (legacy: `/v1/tulisio/publish` — **keduanya harus tetap didukung**), healthcheck ping, api_key ter-seal v3 per media.
- Inbound (server kita): `lib/publishing/auth.ts` — Bearer api_key (lookup sha256) **atau** header `X-Tulisio-Signature` = HMAC-SHA256 over `{timestamp}.{body}`, **window replay 5 menit**. Endpoint: `/v1/publishing/callbacks` (POST), `/v1/publishing/feed.json` (GET).
- Error-code catalog bawaan protocol — salin kodenya (di `docs/publisher-protocol.md` repo lama).

### 2.4 RSS — `lib/publish/rss.ts`
- Generate RSS 2.0 XML per partner: `/feed/partner/{token}` (GET) + import `/feed/partner/{token}/import` (POST). Log akses `rss_access_logs`.

### 2.5 Tulisio API adapter — `lib/publish/tulisio-api.ts`
- Publish ke adapter URL dengan `X-Tulisio-Signature` HMAC over `timestamp.body`.

### 2.6 Partner Publishing API keys
- `publishing_api_keys`, rotate/hapus via `/v1/partner/publishing-api-keys`.

## 3. Google Search Console — `lib/media/gsc.ts`
- OAuth webmasters.readonly + PKCE; callback `/api/gsc/callback`; state `gsc_oauth_states`.
- Token refresh + sealed tokens; query `searchAnalytics/query` per property → `gsc_snapshots` (period 7/28/30/90).
- Platform-level config: `platform_gsc_config` (OAuth client platform) via `/v1/platform/gsc-config`.

## 4. GA4 Measurement Protocol — `lib/ga4/service.ts`
- `POST https://www.google-analytics.com/mp/collect?measurement_id=...&api_secret=...` per brand (`ga4_configs`, api_secret ter-seal), event mapping jsonb, client_id dari session id, `consent_mode`.

## 5. Payment Gateways — `lib/payments/adapters.ts` (1.740 baris, 6 gateway)

| Gateway | API | Auth | Webhook |
|---|---|---|---|
| **Xendit** | `POST /v2/invoices` | `x-api-key` | callback token verification |
| **Duitku** | inquiry v2 | merchant code + signature | callback signature |
| **Midtrans Snap** | `POST /snap/v1/transactions` | server key (Basic auth) | signature sha512 |
| **Tripay** | `POST /transaction/create` | private_key HMAC | callback HMAC |
| **SumoPod** | `api-pay[-sandbox].sumopod.com/api/v1/payments` | `X-Api-Key` + webhook token (Svix-style `whsec_`) | **wajib return URL https-only** |
| **Sandbox** | lokal | HMAC `tauthub-sandbox-secret` | webhook internal |

- Kredensial per gateway ter-seal di DB (`payment_gateway_configs`, migration 015).
- Webhook masuk: `POST /v1/payments/webhooks/{provider}` → transisi status dengan `FOR UPDATE` (idempotent; `payment_webhook_events` untuk log/dedupe).
- Aktif/nonaktif + test dari admin (`/v1/platform/payments/gateways/*`).

## 6. Kredit TC (Tulisio Credit) — `lib/credits/wallet.ts` (931 baris)
- Wallet per brand: available/reserved/captured (CHECK constraint ≥0).
- Siklus: **topup → reserve → capture/release/refund** per publikasi (transaksi `credit_ledger_entries`, idempotency_key).
- Config di `tc_config` (rate IDR, min withdraw). Topup manual via `credit_topup_reports` (review admin).
- Laporan topup → invoice + berita-acara.

## 7. Email — dua layer
- **Direct SMTP** `lib/mail/service.ts` (nodemailer): config `SMTP_HOST/PORT/USER/PASS/SECURE`, `MAIL_FROM(_ADDRESS)`; dipakai email event publikasi, skor PR, kegagalan batch. Config alternatif dari `smtp_settings` (admin settings).
- **Queued** `lib/email/*`: tabel `email_deliveries` status queued→sending→sent/failed/skipped/rate_limited, attempts ≤3, `next_attempt_at`, **dedupe_key + window** (query pakai `INTERVAL '...'` interpolasi — R11), template registry `lib/email/templates/`, routing `lib/email/routing.ts` dengan **fallback magic-link bila SMTP off**. Diproses job `send_email` + retry scheduler.
- Go: `wneessen/go-mail` atau `net/smtp`; antrean tetap DB.
- **Provider aktif: Kirim.id (SMTP relay)**, menggantikan Brevo. Setelan dari
  dashboard [panduan.kirim.id — SMTP relay](https://panduan.kirim.id/email/developer/smtp-relay):

  | Setelan | Nilai |
  |---|---|
  | Host | `mail.kirim.id` |
  | Port | `587` (STARTTLS) atau `465` (TLS, centang Secure) |
  | Username | `kir` + 6 angka |
  | Password | 32 karakter |
  | Auth | PLAIN / LOGIN (sudah didukung `internal/email/smtp.go`) |
  | Syarat | domain alamat From wajib `active` di akun Kirim.id |

  **Kredensial tidak pernah ditaruh di `.env`, docs, atau kode.** Admin mengisinya
  sendiri di `/admin/konfigurasi` → tab **Email / SMTP**; password disimpan terenkripsi
  (`crypto.Seal`, AES-256-GCM, envelope `t2:`) di tabel `smtp_settings` dan hanya
  dikembalikan ke klien sebagai `password_configured: bool`. Menyimpan konfigurasi
  membuang cache config email (`email.InvalidateConfigCache`) supaya pergantian
  provider langsung berlaku.

## 8. WhatsApp (Fonnte) — `lib/wa/*` (752 baris)
- `POST https://api.fonnte.com/send` (+ `/device`), token dari `platform_wa_config`.
- Queue job `send_wa`, template `wa_templates`, log `wa_deliveries`, test endpoint admin.

## 9. Metrik SEO — `lib/media/*`
- **RapidAPI DA/PA** (migrasi 279): `bulk-da-pa-checker-semrush.p.rapidapi.com` (`POST /bulk-dapa.php`, "Bulk DA PA Checker Semrush" oleh KSDeveloper) lewat paket `internal/seo` — dipakai metrik media (`media_seo_metrics.da/pa`) DAN tool Cek DA/PA mitra; host/path/nama field body bisa ditimpa di `platform_seo_config` (provider_host/provider_path/provider_domain_field). Fallback spam tetap `website-spam-checker.p.rapidapi.com` dengan key yang sama. Host lama `domain-da-pa-checker`/`domain-metrics-check` sudah tidak dipakai.
- **PageSpeed Insights** (`lib/media/page-speed.ts`): key `platform_pagespeed_config` / env `GOOGLE_PAGESPEED_API_KEY`.
- **Serpens AI-citation**: `serpens.p.rapidapi.com/api/v1/search_ai_overview` — cek domain muncul di AI Overview.
- `ai_trust_score`: fetch situs langsung + deteksi schema.org.
- Bulk refresh via job `media_seo_refresh`.

## 10. Crawlhub (layanan crawler eksternal) — `lib/crawlhub/*`
- HTTP ke `CRAWLHUB_BASE_URL`, HMAC headers `x-crawlhub-timestamp` / `x-crawlhub-signature` (secret ter-seal `platform_crawlhub_config`).
- Register target / status / re-crawl; hasil upsert ke `crawl_targets`; ingest kembali via `CRAWLHUB_INGEST_SECRET`.
- Protokol kredensial crawler sendiri: `crawler_credentials` (sealed bytea + fingerprint), nonce `crawler_nonces`, `crawler_request_log`, Cloudflare block classification (`crawler_diagnostics`), consent (`lib/crawler/consent.ts`).

## 11. Cloudflare Turnstile — `lib/auth/turnstile.ts`
- Config `platform_turnstile_config` (cache 60s) fallback env; verifikasi `challenges.cloudflare.com/turnstile/v0/siteverify`; config endpoint publik `/v1/auth/turnstile`.

## 12. Notifikasi & Webhooks — `lib/notifications/*`, `lib/push/signature.ts`
- In-app: `notifications` + `notification_preferences` (per role: klien/mitra/admin).
- Outbound webhook user/brand: URL terdaftar di `notification_webhooks`, headers `x-tulisio-signature` / `x-tulisio-event` / `x-tulisio-event-id` / timestamp, **HMAC drift 10 menit**. Port persis (nama header legacy dipertahankan, R7).
- Egress IP helper untuk partner allowlist WAF (`lib/net/egress-ip.ts`).

## 13. Lain-lain
- **Plagiarism Smodin**: `lib/plagiarism/smodin.ts` → `/plagiarism`.
- **Domain verification**: token `tauthub-verification` di halaman situs media, staged diagnostics (dns/connect/http/bot_challenge/token/token_mismatch) → job `domain_verify`.
- **Tracking**: `/t/{code}` redirect + `tracking_events` + `analytics_daily` (event_type: article_view, article_scroll, outbound_click, cta_click, tracking_link_click — CHECK constraint).
- **Uploads**: `lib/uploads.ts` — disk lokal, auth cookie, kompresi (lihat `01 §10`).
- **Registration guard**: `registration_guard_config` + `registration_history` (anti-abuse registrasi).

## 14. Checklist integrasi per fase

| Fase | Integrasi yang harus hidup |
|---|---|
| 1 | Turnstile, Google OAuth, email direct (magic-link fallback) |
| 3 | AI gateway + embeddings lokal |
| 5 | WordPress, custom-web, RSS, publishing API keys, Crawlhub, domain verify |
| 6 | Blogger OAuth, tracking |
| 8 | 6 payment gateway, TC wallet, email queue |
| 9 | Crawler credentials/consent |
| 10 | GSC, GA4, RapidAPI SEO, PageSpeed, Serpens, Smodin |
| 12 | Fonnte WA, notification webhooks |
