# 01 — Arsitektur Backend Go

> Standar koding Rust-style RTK di AGENTS.md tidak berlaku untuk Go, tapi semangatnya sama: **explicit error handling, tanpa magic, fallback tidak dihapus**.
> Semua nilai konstanta di dokumen ini dibaca dari codebase lama — jangan diubah saat porting.

---

## 1. Module & Layout

Satu Go module (`api/`), tiga binary:

- `cmd/api` — chi HTTP server (semua 406 endpoint, peta di `04_API_ENDPOINTS.md`)
- `cmd/worker` — job drainer loop (claim dari tabel `jobs`)
- `cmd/scheduler` — loop 60 detik yang menjalankan `runScheduler()` lalu drain jobs

Package:

| Package | Isi | Port dari |
|---|---|---|
| `internal/config` | env parsing + validasi fail-fast | `src/lib/config.server.ts` |
| `internal/db` | pgxpool, `WithTx`, runner migration | `src/lib/db/client.ts` |
| `internal/httpx` | middleware chi: cors, recover, auth, ratelimit, envelope | `src/proxy.ts` + `src/auth/http.ts` |
| `internal/auth` | session, argon2, totp, google, turnstile, rate-limit, deep-link | `src/auth/*`, `src/lib/auth/*` |
| `internal/crypto` | seal/unseal v1/v2/v3 | `src/lib/ai/provider.ts` (sealSecret*) |
| `internal/security` | safe-fetch anti-SSRF, egress IP | `src/lib/security/safe-fetch.ts`, `src/lib/net/egress-ip.ts` |
| `internal/jobs` | queue + 13 job kind + processJob dispatcher | `src/lib/jobs/queue.ts` |
| `internal/ai` | provider gateway, embeddings lokal, analysis | `src/lib/ai/*`, `src/lib/knowledge-base/*` |
| `internal/publish` | executor + wordpress/blogger/custom-web/rss/tulisio-api | `src/lib/publish/*`, `src/lib/publishing/*` |
| `internal/integrations` | gsc, ga4, seo-metrics, pagespeed, smodin, crawlhub, fonnte, mail | `src/lib/media/*`, `src/lib/ga4/*`, dll. |
| `internal/uploads` | local disk storage + kompresi gambar | `src/lib/uploads.ts` |
| `internal/domains/<x>` | handler chi per domain (brands, campaigns, media, publications, partner, platform, credits, payments, finance, taut, pr, tracking, notifications, content, knowledge, operations) | `src/app/v1/**/route.ts` |

**Gaya**: handler Go = `route.ts` lama, satu file per path. Query SQL mentah di `queries/*.sql` untuk sqlc; query dinamis (WHERE opsional, JSONB dedupe) boleh pakai `pgx` langsung dengan argumen eksplisit. Tanpa ORM. Tanpa layer abstrak generik.

---

## 2. Config / Environment

Port `config.server.ts`: parse env saat startup, **fail-fast jika `APP_SECRET` < 32 char atau DB config hilang**. Tabel lengkap env (nama, default, dipakai di mana) ada di `06_INTEGRATIONS.md §12`. Ringkas:

- App: `APP_ENV` `APP_NAME` (Tulisio) `APP_VERSION` `APP_PORT` `APP_SECRET` `ACCESS_TOKEN_MINUTES=30` `REFRESH_TOKEN_MINUTES=43200`
- DB: `DATABASE_URL` (menang) atau `DB_HOST/PORT/NAME/USER/PASSWORD` + `DB_POOL_SIZE=10`
- AI: `AI_PROVIDER=sumopod` `AI_BASE_URL` `AI_API_KEY` `AI_DEFAULT_MODEL` `AI_ECONOMY_MODEL` `AI_PREMIUM_MODEL` `AI_EMBEDDING_MODEL` `AI_REQUEST_TIMEOUT_SECONDS=120` `AI_MAX_RETRIES=2` (`AI_DAILY_CALL_LIMIT` ada di env lama tapi TIDAK dipakai — jangan sambungkan, parity)
- Auth: `PLATFORM_ADMIN_EMAILS` (csv) `AUTH_RATE_LIMIT_PER_MINUTE=10` `WRITE_RATE_LIMIT_PER_MINUTE=120` `CMS_ENCRYPTION_SECRET` (fallback `APP_SECRET`) `GOOGLE_CLIENT_ID/SECRET/REDIRECT_URI` `TURNSTILE_SITE_KEY/SECRET_KEY`
- Jobs: `JOB_INLINE=true` `JOB_RUNNER_KEY` `CRON_SECRET` `SCHEDULER_INTERVAL_SECONDS=60` `SCHEDULER_MAX_JOBS=20`
- Integrasi: `SMTP_*` `MAIL_FROM*` `CRAWLHUB_*` `GOOGLE_PAGESPEED_API_KEY` `CRAWLER_MASTER_KEY` `TULISIO_SECRET` `SERVER_PUBLIC_IP` `MEDIA_HEALTH_REFRESH_HOURS=336` `MEDIA_AI_SUMMARY_DELAY_DAYS` `PUBLICATION_RETRY_BACKOFF` `BULK_MEDIA_TEMPLATE_URL`
- URL: `NEXT_PUBLIC_APP_URL`/`APP_URL`/`NEXT_PUBLIC_SITE_URL` → di Go jadi `APP_PUBLIC_URL` resolution order: DB `platform_server_config` > env > deteksi request.

**Catatan**: `APP_SECRET` dibaca langsung di 26 file lama (proxy edge tak bisa import config). Di Go: satu sumber (`internal/config`), di-pass eksplisit.

---

## 3. Auth & Session (PORT KRITIS — R2)

### 3.1 Token pair
- **Access token**: JWT **HS256**, signed `APP_SECRET` (jose lama). Dikembalikan di **body JSON**, disimpan client di localStorage, dikirim `Authorization: Bearer`. TTL `ACCESS_TOKEN_MINUTES` (30).
- **Refresh token**: JWT HS256, httpOnly **SameSite=Lax, Secure**, cookie name `refresh_token`. TTL `REFRESH_TOKEN_MINUTES` (43200 = 30 hari).
- Kedua token membawa claim **`sid` = JTI refresh token**. Bearer verification (`getAuthUser`) **selalu cek DB**: `user_sessions.revoked IS NULL AND expires_at > now()` → logout/rotasi/ganti password mencabut akses instan.
- User wajib `is_active AND approved_at IS NOT NULL`.
- Cache auth per-user **30 detik** di memori + `invalidateAuthCache(userId)` pada logout/role change.
- Claims lain: `sub` (user id), `role` (`client|partner|admin`), `exp`. **Salin struktur claim persis** — token yang diterbitkan app lama harus tetap terverifikasi.

### 3.2 Password — argon2id
`golang.org/x/crypto/argon2` dengan parameter persis: **argon2id, m=19456 (19 MiB), t=2, p=1, keyLen=32**. Format string hash mengikuti format `@node-rs/argon2` (PHC string) supaya kolom `password_hash` lama terverifikasi. Wajib unit test: hash lama nyata dari DB `tauthub-go` harus lolos `verify`, dan hash baru dari Go harus lolos di app lama (uji sekali manual).

### 3.3 TOTP 2FA
Lama hand-rolled RFC 6238 (HMAC-SHA1, 6 digit, 30 detik, window ±1, base32). Di Go pakai `github.com/pquerna/otp` (kompatibel) atau port manual — **wajib kompatibel** dengan secret TOTP yang sudah di-scan user. Login flow: jika 2FA aktif → respons `{two_factor_required: true, challenge}` di mana challenge = **JWT 5 menit** (`totp-challenge.ts`) yang ditukar di `/v1/auth/2fa/verify`. Recovery codes: kolom tersimpan (hash), konsumsi sekali.

### 3.4 Lockout & rate limit
- Lockout: `failed_login_count` / `locked_until` di tabel users — salin aturan increment/reset.
- Rate limit: tabel **`auth_rate_limits`** (bucket, count, window_start) dengan **atomic UPSERT** + fallback in-memory (maks 5000 bucket) saat DB down. Dua helper: `checkAuthRate` (login/register/2FA, `AUTH_RATE_LIMIT_PER_MINUTE`) dan `checkWriteRate` (AI/publish/topup/chat, `WRITE_RATE_LIMIT_PER_MINUTE`, nama bucket diberi suffix window). IP dari `x-forwarded-for`/`x-real-ip`.

### 3.5 Google OAuth & Turnstile
- Google: PKCE, state di **signed cookie** (`createOAuthStateCookie/consumeOAuthState`), endpoint `/v1/auth/google/{url,callback,complete,link,exchange-session}`, akun disimpan `user_oauth_accounts`.
- Turnstile: config dari DB `platform_turnstile_config` (cache 60s) dengan fallback env (`TURNSTILE_SITE_KEY/SECRET_KEY`, prefix `env:` di kolom DB), verifikasi ke `https://challenges.cloudflare.com/turnstile/v0/siteverify`.

---

## 4. Layer DB (pgx + sqlc)

- Pool: `pgxpool` dari `DATABASE_URL` (menang atas DB_*), **max = DB_POOL_SIZE (10)**, idle 30s, connect timeout 10s. SSL mengikuti `sslmode` di URL.
- Helper (port dari `client.ts`):
  - `Query` / `QueryRow` — pgx langsung
  - `Exec` mengembalikan shim `RunResult{RowsAffected, InsertID}` (dari `RETURNING id` bila perlu) agar port handler yang cek `affectedRows` tetap 1:1
  - `WithTx(ctx, fn)` — port `withTransaction` (dipakai 41 file: payments, jobs, reset tokens, publication plans)
- **Placeholder: tulis `$1..$n` langsung** di sqlc/pgx. JANGAN port sistem konversi `?`→`$n` milik lama (R3). Saat menyalin SQL dari kode lama, konversi manual + hati-hati `?` di dalam string literal.
- Pola SQL yang harus dipertahankan (sqlc harus di-konfigurasi mampu): `JSONB` (`payload->>'x'`), `ON CONFLICT ... DO UPDATE`, `RETURNING`, `DISTINCT ON`, `FOR UPDATE SKIP LOCKED`, `INTERVAL '30 days'` (di email dedupe di-interpolasi string — lihat R11), index `jobs(payload)` untuk dedupe scheduler (migration 117).
- sqlc: satu file `queries/<domain>.sql` per domain; mode `emit_json_tags: false` (pakai struct eksplisit); query dinamis tetap pgx manual. Typo kolom = compile error sqlc — ini safety net murah untuk 160 tabel.
- Tenancy: setiap query brand-scoped **wajib** menyaring `brand_id` + membership; akses lintas tenant → **404 (bukan 403)**, port dari perilaku lama.

---

## 5. HTTP Layer (chi)

Rantai middleware `cmd/api` (urutan port dari `proxy.ts` + `auth/http.ts`):

1. `Recoverer` — panic → 500 ter-scrub
2. `RealIP` — x-forwarded-for / x-real-ip
3. **CORS allowlist** `/v1/*`: origin lama = domain produksi (`tulisio.agencyjogsel.web.id`), Vercel preview, `localhost` — `credentials: true`, `OPTIONS` → 204. Salin persis.
4. `RateLimit(write)` untuk grup AI/publish/topup/chat
5. `Auth` (Bearer + cek session DB + cache 30s) untuk grup yang butuh
6. Router chi per domain

Kontrak error (wajib identik):
- Error envelope: **`{"detail": "<pesan>"}`** (gaya FastAPI) — semua handler.
- 500 di `APP_ENV=production` → pesan generik (scrub), log internal lengkap.
- Status codes: salin dari handler lama (mis. 401 token invalid, 404 lintas-tenant, 429 rate limit, 409 konflik state). Saat porting per endpoint, buka `route.ts` lama dan salin semua cabang respons.

---

## 6. Job Queue (DB-backed, tanpa broker)

Tabel `jobs` (kind, payload JSONB, status pending/running/failed/completed, attempts, max_attempts, run_at, locked_at, locked_by, last_error). Port `lib/jobs/queue.ts`:

- **Claim**: transaksi + `SELECT ... FOR UPDATE SKIP LOCKED` → set `status='running', locked_at, locked_by`.
- **Backoff**: `min(30s, 1s * 2^attempt)`; `max_attempts` = 3 untuk `content_generation`/`publication_execution`, lainnya 2.
- **13 job kind** (dispatcher `processJob`):

| kind | Fungsi | Port dari |
|---|---|---|
| `content_generation` | pipeline AI lengkap + moderation gate + insert documents/versions | `lib/content` + `lib/moderation` |
| `sitemap_crawl` | crawl sitemap → knowledge sources | `lib/knowledge/_knowledge.ts` |
| `publication_execution` | publish ke WP/Blogger/custom-web/webhook | `lib/publish/executor.ts` |
| `publication_variant_generation` | generate varian artikel per media | `lib/publication/variants` |
| `pr_analysis` / `pr_verify` | analisis PR + verifikasi link live + recalc skor + notifikasi | `lib/pr` |
| `media_ai_summary` | rangkuman AI media → `media_ai_summary_events` | `lib/media/ai-summary.ts` |
| `media_seo_refresh` | refresh DA/PA/AI-citation | `lib/media/seo-metrics.ts` |
| `domain_verify` | verifikasi token domain media | `lib/media/domain-verification.ts` |
| `publication_link_check` | monitor 404 outbound link | `lib/tracking` |
| `crawl_target_sync` | sinkron status Crawlhub | `lib/crawlhub/sync.ts` |
| `send_email` | proses `email_deliveries` (retry, dedupe) | `lib/email/queue.ts` |
| `send_wa` | kirim WhatsApp Fonnte | `lib/wa/queue.ts` |

- **Mode eksekusi** (port `JOB_INLINE`):
  - `JOB_INLINE=true` → setelah enqueue di handler, panggil `drainJobs(1)` inline (dev parity).
  - Worker pull: `POST /v1/internal/jobs` header `x-job-runner-key: JOB_RUNNER_KEY`, **satu job per call**.
  - `cmd/worker` = loop drain kontinu (pengganti `scheduler-worker.mjs`).
- **Realisasi Go**: dispatcher bersama di `internal/jobs` (`Register`/`Dispatch`/`Runner`),
  didaftarkan sekali lewat `internal/jobwire.Register` sehingga API, `cmd/worker`, dan
  `cmd/scheduler` memakai registry yang sama. **13/13 kind punya processor**:
  - `internal/domains/brandflow/job_publication_execution.go` (publish ke CMS/webhook/protokol mitra)
  - `internal/domains/brandflow/job_variant.go` (varian artikel per media + agregat batch)
  - `internal/domains/brandflow/job_link_check.go` (pelacak 404, ambang 2 gagal berturut)
  - `internal/domains/platform/job_pr.go` (`pr_analysis` + `pr_verify` + recalc skor & notifikasi)
  - `internal/domains/platform/job_media.go` (SEO refresh, ringkasan AI, crawl sync, verifikasi domain)
  - `internal/domains/platform/job_messaging.go` (drain `email_deliveries` + `wa_deliveries`)
  - `internal/jobwire/content_jobs.go` (`content_generation`, `sitemap_crawl`)
  - Catatan: `jobs` **tidak punya kolom `completed_at`**; `MarkCompleted` hanya menulis
    `status` + `updated_at` (parity `markCompleted` app lama).
- **Dedupe scheduler**: sebelum enqueue publikasi/health/link-check, cek job pending/running via `payload->>'publicationId'` (salin SQL persis, R8).

---

## 7. Scheduler (`cmd/scheduler`)

Port `lib/scheduler/publish-scheduler.ts` — loop `SCHEDULER_INTERVAL_SECONDS` (60):

1. Materialisasi occurrence publication-plan yang jatuh tempo (claim `FOR UPDATE SKIP LOCKED`, `lib/publication/plans.ts`)
2. Safety-sweep publikasi due → enqueue `publication_execution` (dedupe R8)
3. Reminder mitra (partner reminders)
4. Enqueue domain recheck, `publication_link_check`, `crawl_target_sync`, `media_ai_summary`, `media_seo_refresh` (semua dedupe vs pending/running)
5. Retry `email_deliveries` yang jatuh tempo
6. Refresh media health (window `MEDIA_HEALTH_REFRESH_HOURS` = 336)

Pengganti cron Vercel (`/v1/internal/cron/scheduler`): endpoint tetap ada (parity) dengan guard `x-cron-secret`/Bearer `CRON_SECRET`, tapi di VPS yang memanggil adalah `cmd/scheduler` atau systemd timer.

---

## 8. Crypto Sealed Secret (KRITIS — R1)

Port dari `lib/ai/provider.ts` — **tiga format, semua harus tetap bisa di-unseal**:

| Versi | Format | Algoritma | Dipakai untuk |
|---|---|---|---|
| v1 (legacy) | tanpa prefix | **XOR keystream + HMAC-SHA256**, byte-compatible dengan Python `cms_security.py` | kredensial CMS lama |
| v2 | prefix `t2:` | **AES-256-GCM**, key = `sha256(APP_SECRET)` | kredensial umum |
| v3 | prefix `t3:` | AES-256-GCM, key = `sha256(APP_SECRET + ":" + entityId)` — per-entity, diaktifkan kolom `enc_key_ver >= 1` | kredensial per media/gateway baru |

- `CMS_ENCRYPTION_SECRET` fallback ke `APP_SECRET`.
- Resolusi secret (port `secretCandidates()`): DB config → env → **parsing `.env`/`.env.local` naik pohon direktori** (compat path; di Go: replikasi urutan resolusi, atau setujui hanya DB+env — catat di ADR, jangan diam-diam dihilangkan).
- Aturan: **unseal semua versi, seal baru selalu v3**. Unit test dengan ciphertext nyata per versi dari DB `tauthub-go`.

---

## 9. Keamanan Jaringan

- **safe-fetch (anti-SSRF)** — port `lib/security/safe-fetch.ts`: resolve DNS dulu, blok IP privat/loopback/link-local/CGNAT (100.64/10)/cloud metadata (169.254.169.254), IPv4+IPv6, limit redirect 5. Dipakai: custom-web, crawler fetch, domain verification.
- custom-web di produksi **wajib https publik** (aturan tambahan di `lib/publish/custom-web.ts`).
- **egress IP** (`lib/net/egress-ip.ts`): DB `platform_server_config` > env `SERVER_PUBLIC_IP` > deteksi `api.ipify.org` / fallback `ifconfig.me/ip`.

---

## 10. Uploads & Gambar (R4, R6)

- Storage: **local disk** `.data/uploads` (di luar public), fallback legacy `public/uploads`. Simpan path base yang sama di server baru; saat migrasi VPS, **salin direktori storage** (skrip `migrate-uploads.sh` lama sebagai referensi).
- Serving: `GET /uploads/[key]` — **butuh auth refresh cookie**, validasi basename + regex extension. Port persis.
- Kompresi (port sharp): gambar ≤110KB, cover WebP ≤200KB, max width 1600. Di Go: `image` + `golang.org/x/image/draw` (resize) + encoder `image/jpeg`/`image/png`. **WebP encode di Go butuh cgo (libwebp)** — keputusan terpisah: fase parity boleh JPEG/PNG dengan target ukuran sama, WebP menyusul bila perlu (catat deviasi di checklist).

---

## 11. Pengganti Library Node → Go

| Kebutuhan | Lama (Node) | Go | Catatan |
|---|---|---|---|
| XLSX import/export batch | exceljs | `github.com/xuri/excelize/v2` | matang; uji dengan file batch nyata |
| DOCX parse (ingest) | mammoth | unzip + parse `word/document.xml` (text+gambar) | tidak ada 1:1; buat extractor minimal, uji fixture (R5) |
| PDF parse | pdf-parse | `github.com/ledongthuc/pdf` | uji fixture |
| QR code | qrcode | `github.com/skip2/go-qrcode` | |
| Sanitize HTML | dompurify (client) | tetap client-side; server-side `github.com/microcosm-cc/bluemonday` | |
| Markdown | `lib/markdown.ts` (custom 84 baris) | port manual + `goldmark` bila perlu render | |
| Email SMTP | nodemailer | `github.com/wneessen/go-mail` | config `SMTP_*`, queue tetap DB |
| HTTP client | fetch | `net/http` + safe-fetch wrapper | |
| Cron/loop | setInterval / Vercel cron | systemd + loop goroutine | |

---

## 12. Testing

- `go test ./...` — port konsep 153 test Vitest lama (unit murni: crypto, totp, embed, pr scoring, gatekeeper, wa, publication, finance, moderation, dst.).
- **Test compat wajib (fase 1)**: (a) verify password hash lama, (b) unseal v1/v2/v3 ciphertext lama, (c) verify JWT lama + revocation, (d) TOTP secret lama.
- Smoke scripts: port `scripts/smoke/smoke-e2e.ts`, `smoke-admin.ts`, `smoke-topup-gateway.ts` → `api/smoke/` (Go atau script HTTP) — dipakai sebagai definition of done tiap fase.
- E2E manual lama (`test-*.mjs`, WordPress publish) → port bertahap sesuai fase publishing.

## 13. Checklist "Definition of Done" Backend per endpoint

- [ ] Path + method identik (`04_API_ENDPOINTS.md`)
- [ ] Request validation sama ketatnya (zod → validasi manual Go; salin pesan error penting)
- [ ] Semua cabang status code & body `{detail}` sama
- [ ] Tenancy 404, rate limit, audit log (bila ada) diterapkan
- [ ] SQL disalin mentalnya dari lama (bukan ditulis ulang "lebih bagus")
- [ ] Smoke test domain lulus dengan data produksi `tauthub-go`
