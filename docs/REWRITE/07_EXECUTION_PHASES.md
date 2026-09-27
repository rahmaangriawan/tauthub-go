# 07 — Rencana Eksekusi: 13 Fase Vertical-Slice

> Prinsip: **satu fase = backend (Go) + frontend (Svelte) + verifikasi** untuk satu domain. Selesai fase = semua checklist fase itu hijau di `04_API_ENDPOINTS.md` + `05_UI_PAGES.md`.
> App lama tetap hidup sampai fase 12; DB `tauthub-go` dipakai bersama — karena auth & crypto kompatibel, parallel run aman.

---

## Fase 0 — Bootstrap Repo Baru

**Kerja**:
- [ ] Repo baru: `api/` (go.mod, cmd/api, cmd/worker, cmd/scheduler) + `web/` (SvelteKit 5 + adapter-node + Tailwind v4 + shadcn-svelte init)
- [ ] `internal/config` + `internal/db` (pgxpool, WithTx) + health check
- [ ] sqlc setup (`sqlc.yaml`, contoh query auth) + `make sqlc`
- [ ] Salin `schema.pg.sql` + 172 migration → `api/migrations/`; catat versi 168 ke `schema_migrations` (tanpa eksekusi ulang)
- [ ] Salin folder `REWRITE/` ini + docs penting repo lama (`publisher-protocol.md`, `alur-klien.md`, `alur-mitra.md`, `SEO_EEAT_GEO_INTELLIGENCE.md`, `Mail SMTP/`)
- [ ] Env template (semua var dari `01 §2`) + Makefile (`dev`, `test`, `sqlc`, `build`, `deploy`)
- [ ] CI: go vet/test + svelte check (pengganti quality.yml)
- [ ] Vite dev proxy: `/v1`, `/uploads`, `/t`, `/feed` → `localhost:8080`

**Verifikasi**: `curl :8080/healthz` OK; web render halaman kosong; sqlc generate lolos; login page statis tampil.

**DoD**: skeleton hidup, DB terhubung, CI hijau.

---

## Fase 1 — Auth ## Fase 1 — Auth & Identity (fondasi semua fase) Identity (fondasi semua fase) — **DALAM PROGRES** (2026-09-27: crypto/JWT/argon2/TOTP kompat ✅, login/refresh/logout/me/2fa-verify/turnstile ✅, smoke UI ✅; sisa: register, 2fa setup/enable/disable, change/forgot/reset password, Google OAuth, me/notification-preferences, gating halaman server-side)

**API** (18 auth + 3 penunjang): login, logout, refresh, register, me (GET/PATCH), 2FA (4), change/forgot/reset password, Google OAuth (5), turnstile config, `me/notification-preferences` (GET/PUT), rate-limit & lockout, registration guard.
**Crypto/compat wajib diuji**: JWT HS256 + `sid` revocation; argon2id (m=19456, t=2, p=1, len 32) verify hash lama; TOTP + challenge JWT; **unseal v1/v2/v3 ciphertext nyata dari DB**.
**UI**: landing, login (+2FA flow), register-role/klien/mitra, forgot/reset, google-callback, oauth-link, redirect legacy, app shell + sidebar + theme light/dark.
**Infra**: CORS allowlist, error envelope `{detail}`, rate limit middleware, audit log helper.

**Verifikasi**: unit test compat (password lama, JWT lama, unseal, TOTP lama); smoke login dengan user produksi `tauthub-go`; refresh cookie flow di browser.

**DoD**: user produksi bisa login di web baru dengan kredensial lama; token lama dari app lama diterima api baru (dan sebaliknya).

---

## Fase 2 — Brands & Workspace — **SELESAI** (2026-09-27: 14/17 endpoint ✅ — list/create/detail/activate/profile/voice/rules/configuration/dashboard/usage/prompts/members + **overview read-only** (wallet/campaigns/intakes/jobs/publications/prScoreData/trends 7 hari); UI: shell klien (sidebar grup nav + topbar kredit + user chip) + Beranda (hero, KPI wallet, AI Insights, Akses Cepat Keuangan, antrean publikasi, panduan) + placeholder section bertanda fase. Sisa: analytics + analyze + pr-calculator + ga4 (fase 10), assets)

**API** (17 core + 3 members): brands CRUD, activate, profile/voice/editorial-rules, prompts (+activate), configuration, overview/dashboard/analytics/usage, analyze/{topic}, assets, members.
**Tabel**: brands, brand_memberships, brand_profiles, voice_profiles, editorial_rule_sets, master_prompt_versions, websites, brand_ai_quotas.
**UI**: `/klien` (daftar brand), workspace section `overview`, `dashboard`, `members`, `profile`; sidebar gating per role + `assertBrandAccess`.

**Verifikasi**: CRUD brand dari UI; readiness score & activate flow; lintas-tenant → 404.

**DoD**: brand lama tampil & editable persis; member management berfungsi.

---

## Fase 3 — Knowledge, RAG & Chat — **SEBAGIAN BESAR SELESAI** (2026-09-27: 7/9 endpoint ✅ — collections/sources/sitemap/chat-learning lengkap; **embeddings bit-identik teruji silang** (7 vektor×128 dim + 28 cosine + 4 Jaccard, δ≤1e-12) — embedding lama di DB valid tanpa re-index; AI gateway core port (runtime DB+env, retry/backoff, usage ledger, mock); crawler sitemap + job queue claim SKIP LOCKED; UI knowledge page ✅. DEFER: route chat → fase 4 (cabang create_article bergantung runContentJob), platform knowledge-bases + content standards admin → fase 11)

**API** (8 knowledge/chat + 7 platform knowledge-bases + content-type standards admin): collections/sources CRUD, upload (TXT/MD/CSV/PDF/DOCX ≤10MB), sitemap crawl, chat (tool-calling), chat-learning.
**Wajib**: embeddings lokal 128-dim **bit-identik** (unit test vektor sama dengan Node); hybrid retrieval cosine+Jaccard; prompt compiler snapshot.
**UI**: section `knowledge`, `/klien/{brandId}/knowledge`, `/admin/knowledge-bases`, `/admin/content-standards`.

**Verifikasi**: upload → chunk → embed → retrieve `[SOURCE N]` trace; crawl sitemap job jalan; chat menjawab dengan grounding.

**DoD**: retrieval menghasilkan chunk yang sama untuk query yang sama (bandingkan dengan app lama).

---

## Fase 4 — Content Pipeline — **SLICE A+B SELESAI** (2026-09-27: engine + 8/24 endpoint ✅ — prompt compiler jurnalistik penuh, runContentJob (outline & draft + aturan min 300 kata parity), editorial rules loader (content-type standard + KB rules hybrid RAG), moderasi lengkap (normalisasi homoglyph/leet, rule layer, AI classifier, cache content_moderation_results, audit), gate moderasi di pipeline (blocked → job failed tanpa dokumen, terverifikasi), processor content_generation (job states, dokumen + versi html, reset intake), API content-jobs (5) + documents (5), UI Generate + Editor. SLICE B (2026-09-27): engines intake — readiness scoring berbasis bobot skema (ambang 80/50, topik wajib, artikel-existing untuk mode artikel), createJobFromIntake (sourceMaterial BAHAN+ARTIKEL, gate moderasi intake, idempotency `intake:{id}` + pembersihan job stale, tier premium) ✅; 7 endpoint intake (jenis/list/create/detail/update/analyze/create-job) ✅; route chat lengkap (readiness gate, brief wizard, RAG grounding via knowledge chunks, benchmark chat_learning dengan cache sim≥0.45, last-article, create_article) ✅; UI halaman chat + menu Generate/Chat/Knowledge di shell ✅. Terverifikasi: gate readiness (skor 60 → pesan + 2 aksi), brief wizard → outline siap, create-job → job completed + 1 dokumen + audit moderasi safe. DEFER: ai-fill + auto-key-messages + recommend (butuh wallet kredit fase 8), batches CSV/XLSX (6), quality rubric det-1 (fase 9), moderation admin (fase 11))

**API**: content-intake-types (brand+platform), content-intakes (7 + CRUD), content-jobs (5), documents (5 + html/standard-check), batches (6), job `content_generation` + moderation gate + quality rubric det-1.
**UI**: section `media-standards`/`content-settings`/`moderations` (admin), editor `/klien/{brandId}/editor/{documentId}` (versi, review state machine, autosave, diff/restore), flow intake, halaman batch.
**Worker**: pipeline checkpoint (queued→retrieving→outline→draft→evaluating→ready_for_review), idempotency key, cancel.

**Verifikasi**: generasi 1 artikel end-to-end dengan data brand nyata; batch CSV/XLSX (fixture file sama dengan lama) → jumlah item & token estimate sama; export DOCX/CSV.

**DoD**: artikel hasil generasi + evaluasi tersimpan seperti lama; editor review flow lengkap.

---

## Fase 5 — Media, Mitra & Koneksi CMS

**API**: platform media (18) + media-packages/standards/summaries (6) + partners admin (8) + **partner portal 63 endpoint** + cms-connections (brand 5 + partner 5) + rss-feeds + crawler registrasi + domain-verification + dewan-pers.
**Integrasi**: WordPress publish, custom-web (R7 dual naming), RSS, publishing API keys, Crawlhub, domain verify, bulk media import (excelize).
**UI**: `/admin/media*`, `/admin/partners`, `/admin/rss-feeds`, `/admin/dewan-pers-reviews`, `/admin/domain-diagnostics`; section mitra: profile, media, connections, rss-feeds, crawler, publishing-api; section klien `media`.

**Status**: ✅ **Portal mitra (63 endpoint + 12 halaman UI) SELESAI.** Sisa Fase 5 (di luar portal mitra) belum: platform media (18), partners admin (8), cms-connections brand (5), registrasi crawler admin, halaman admin terkait.

Catatan implementasi portal mitra (paket `api/internal/domains/partner/`, 90 route di `cmd/api/main.go`):
- Semua endpoint terverifikasi dengan akun mitra nyata: 25+ endpoint baca, CRUD media, alur publikasi approve→URL→published, kredit (capture/release dengan kunci idempotensi), notifikasi, bulk import XLSX, CMS/webhook test, RSS, publishing API keys, GSC, crawler.
- **Parser XLSX tanpa dependensi baru** (`archive/zip` + `encoding/xml`) karena modul `excelize` belum tersedia offline — perilaku validasi/alias header sama dengan app lama.
- **Test kompatibilitas**: `crawler_compat_test.go` memverifikasi tuple canonical, HMAC signature, fingerprint, dan PKCE challenge byte-identik dengan `lib/crawler/verification.test.ts` (Worker Cloudflare mitra memakai format yang sama).
- **Rahasia tidak bocor**: `publishing-api-keys` tidak pernah mengembalikan `api_key_hash`/`sealed`, webhook tidak mengembalikan `secret`; sealing memakai v2 (`t2:`) untuk API key dan v3 (`t3:` + `enc_key_ver=1`) untuk CMS connection.
- **2 perbaikan bug laten app lama** (disengaja, didokumentasikan di kode):
  1. `PUBLICATION_TRANSITIONS.partner_approved` tidak memuat `published`, padahal route `POST /partner/publications/{id}/url` memanggil `transitionPublication(..., "published")` untuk Mode Manual → jalur publish manual selalu gagal. Tabel transisi repo Go menambahkan `published`.
  2. `csrfViolation` app lama membandingkan Origin ke host request API; arsitektur baru memisahkan web (3001) dan API (8080) sehingga setiap POST dari browser ditolak 500 "CSRF protection: invalid Origin". Repo Go membandingkan ke allowlist origin publik yang sama dengan CORS (`httpx.OriginAllowed`).

**Verifikasi**: publikasi Mode Manual end-to-end di UI (approve → kirim URL → status `published`, `publish_status=publish`, attempt + job `pr_analysis`/`pr_verify` + capture TC); shell mitra terukur sama dengan app lama (sidebar 264px, nav item 40px padding `0 10px` gap 11px radius 9px `13px/550` tanpa margin, topbar 64px, konten `32px 28px 64px`, label `8px 10px 5px` letter-spacing 1.1px; pakai public_id 6 karakter di URL).
**Sisa**: publish draft ke WordPress nyata (butuh kredensial WP), RSS feed XML valid, domain verification staged diagnostics, Blogger OAuth (butuh Client ID/Secret platform), SimilarWeb & RapidAPI/PageSpeed live (butuh API key) — masing-masing mengembalikan pesan konfigurasi yang jelas sampai kredensial diisi.

**DoD (portal mitra)**: tercapai — mitra bisa kelola media (termasuk bulk XLSX), menyetujui/menolak/menerbitkan artikel, menarik saldo, mengelola integrasi & kredensial.

---

## Fase 6 — Publications & Publishing

**API**: publications (13) + publication-plans (6) + tracking (`/t/{code}`, events) + job `publication_execution`/`publication_link_check`/`publication_variant_generation` + scheduler materialize occurrences + partner publications (8) + auto-replace + link-monitor.
**Integrasi**: Blogger OAuth publish, tracking pixel/event.
**UI**: section `distribution` (4.251 baris — pecah per tab), `publications`, link-monitor klien; mitra `publications`, `sindikasi-konten`, `link-monitor`, `blogger`; admin `publications`, `variants`, `auto-replace`, `link-monitor`, `sindikasi-konten`.

**Verifikasi**: alur publikasi lengkap draft→queued→published dengan retry & attempt log; varian per-media; occurrence ter-schedule & dieksekusi scheduler; tracking click tercatat.

**DoD**: publikasi end-to-end ke media manual + WP + Blogger; scheduler menerbitkan on-time di worker.

---

## Fase 7 — Campaigns & Flow Wizard

**API**: campaigns CRUD + flow (14) + packages/media-packages brand + campaign report share + berita-acara data.
**UI**: section `campaigns`, wizard `/klien/{brandId}/campaigns/{campaignId}/flow` (3.749 baris → step komponen), admin `orders`, `media-packages`.
**Kredit**: integrasi reserve TC di checkout (read-only dulu — penuh di fase 8).

**Verifikasi**: wizard end-to-end: intake → pilih media → angle/paraphrase → checkout → publikasi terbentuk.

**DoD**: campaign flow identik langkah & state-nya dengan app lama.

---

## Fase 8 — Credits, Payments & Finance

**API**: credits/topup/credit-reports + payments (4) + platform payments (4) + platform credits + invoices (2×2) + settlements (5) + revenue-shares (4) + pricing + packages + tc_config + partner earnings/settlements + wallet VIP.
**Integrasi**: 6 payment gateway + webhook idempotent (R12 SumoPod https-only), TC reserve→capture/release, email queue (topup receipt).
**UI**: section `credits`, `invoices` (klien); `earnings`, `settlements` (mitra); admin `credits`, `credit-reports`, `payments`, `packages`, `invoices`, `revenue-shares`, `settlements`, `orders`; halaman `/payments/*` + `/berita-acara`.

**Verifikasi**: topup sandbox → wallet bertambah → reserve → capture → ledger konsisten; webhook replay aman; settlement calc = app lama (bandingkan angka mitra nyata).

**DoD**: uang bergerak benar; smoke-topup-gateway lulus.

---

## Fase 9 — Taut Pintar (agent system)

**API**: brand taut (28) + platform taut (20) + gatekeeper + crawler credentials/consent + job crawl sync.
**UI**: `/klien/{brandId}/taut-pintar/*` (9 halaman, polling) + admin `taut-pintar`, `taut-agents`, `taut-prompts`; mitra `crawler`.
**Integrasi**: Crawlhub HMAC, crawler credential protocol.

**Verifikasi**: satu taut campaign end-to-end: profiler → placement → produce → gatekeeper → distribute → submit-final dengan polling agent-status.

**DoD**: semua state machine agent (`taut_status`: draft→…→submitted) identik.

---

## Fase 10 — PR Calculator, Analytics & SEO Integrasi

**API**: platform pr (9) + brand pr-calculator + media intelligence + jobs `pr_analysis`/`pr_verify` + reports + analytics + GSC (partner 5 + platform 2 + callback) + GA4 + seo endpoints + partner seo/speed-audit/kalkulator-media.
**Integrasi**: GSC OAuth, GA4 MP, RapidAPI (DA/PA), PageSpeed, Serpens, Smodin.
**UI**: section `analytics`, `pr-calculator` (klien); `seo-tool`, `kalkulator-media`, `integrations` (mitra); admin `pr-*`, `media-intelligence`, `integrasi-gsc`, `ga4-configs`.

**Verifikasi**: analisis PR 1 publikasi nyata → skor komponen sama; snapshot GSC terisi; PDF/print berita-acara & laporan.

**DoD**: skor PR & metrik identik dengan versi lama untuk input sama.

---

## Fase 11 — Platform Admin & Operations

**API**: operations (32) + admins (3) + users mgmt + settings (8) + AI platform (7) + moderations + bug-reports + server-config + oauth config + short-link-config + config/template-url + referral admin (4) + vip-partners + domain-diagnostics + gatekeeper logs/stats + ai usage.
**UI**: semua halaman admin tersisa (index, users, roles, operations, errors, audit-log, ai, ai-usage, konfigurasi, integrations, referrals*, vip-partners, bug-reports).
**Job ops**: dead-letter view/retry, generation kill switch.

**Verifikasi**: ops summary & audit log tampil; kelola user (role/status/quota/password) efektif setelah cache 30s invalidate.

**DoD**: admin dapat mengoperasikan platform sepenuhnya dari web baru.

---

## Fase 12 — Notifications, Email/WA, Hardening & Cutover

**API**: notifications (3 grup role) + notification-webhooks (HMAC 10 menit) + WA (4) + templates + bug-reports POST + tracking events finalisasi.
**UI**: bell + list notifikasi (klien/mitra/admin), `/admin/wa-gateway`, bug report widget.
**Hardening**: security headers + CSP report-only, rate limit semua write, smoke e2e + admin + topup penuh, measure-bundle (target <30% bundle lama), backup systemd timer, monitoring log.
**Cutover** (lihat `00 §5`): staging VPS → smoke → nginx switch → monitor 2 minggu → decommission app lama.

**DoD**: checklist `04` + `05` 100% hijau; rollback path teruji.

---

## Aturan Main Selama Eksekusi

1. **Jangan lompat fase**: auth & crypto (fase 1) adalah prasyarat semuanya; publishing (6) butuh media (5); flow (7) butuh publications (6).
2. **Endpoint dikerjakan per-file `route.ts`**: buka file lama, salin semua cabang. Jangan "perbaiki" kontrak.
3. **Commit kecil per endpoint/halaman**, pesan commit menyebut path endpoint/halaman agar checklist bisa diaudit.
4. **Deviasi yang tidak terhindarkan** (mis. webp encode, docx parse) dicatat di tabel deviasi fase itu + flag di `00 §6`.
5. **Setiap fase ditutup dengan**: smoke test lulus + checklist dicentang + catatan deviasi (bila ada).

## Estimasi Bobot (indikatif, bukan deadline)

| Fase | Bobot relatif | Catatan |
|---|---|---|
| 0–1 | 15% | fondasi + crypto compat paling menentukan |
| 2–3 | 15% | CRUD + RAG |
| 4 | 15% | pipeline terdalam (job, moderation, editor) |
| 5–6 | 25% | publishing + mitra = integrasi terbanyak |
| 7–8 | 15% | wizard + uang (harus presisi) |
| 9 | 5% | besar tapi sekadar port state machine |
| 10–12 | 10% | panjang tapi mekanis |

---

## Catatan Status Terkini (port UI menyeluruh)

**Cakupan backend**: 306 dari 406 endpoint tercatat `[x]` di `04_API_ENDPOINTS.md`. Yang selesai:
auth (18), partner portal (63), payments (4), **seluruh platform (173: operations 32, admins 3, AI 10,
media 18, media-packages/standards/summaries 6, partners 8, settings 11, PR 12, taut 20, dan sisanya),
serta 8 endpoint §7 (bug-reports, campaign-reports, internal jobs/cron, publishing callbacks/feed,
tracking events)**. Sisa 100 endpoint ada di grup `/v1/brands` (fase lanjutan: publications,
distribution, members, chat-learning, reports, dll).

**Cakupan UI**: 115 halaman Svelte. Yang selesai:
- **Admin 54 halaman** — seluruh kontrol-plane (shell permission-gated, dashboard, media, publikasi,
  keuangan, mitra, AI, Taut, PR, operasi, konfigurasi). Terverifikasi render tanpa error console.
- **Mitra 21 section** — termasuk `connections`/`publishing-api`/`blogger` sebagai 3 tab halaman
  Integrasi (parity app lama yang me-redirect section lama ke `#hash`).
- **Klien 18 route** — termasuk dashboard, members, notifications, knowledge.
- **Publik 14** — landing redirect, login (termasuk `?mode=register` legacy), register, reset password,
  docs, berita-acara, blogger-pick-blog, 6 halaman payments, redirect legacy (/brands→/klien,
  /platform→/admin, /partner→/mitra via hooks.server.ts).

**Sisa pekerjaan UI** (11 item di `05_UI_PAGES.md`): section `distribution` klien, wizard
`campaigns/{id}/flow`, `taut-pintar` klien (10 sub-halaman), dan knowledge center penuh.

**Verifikasi terpusat yang dijalankan**: `go build ./...`, `go vet ./...`, `go test ./...` bersih;
`svelte-check` 0 error 0 warning; `npm run build` sukses; smoke test API dengan akun admin & mitra
nyata (41+ endpoint admin mengembalikan 200 dengan data DB, gate non-admin 404 parity, callback
publishing HMAC sah 200 / salah 403, feed JSON artikel lengkap, tracking event tercatat, cron/jobs
403 tanpa secret).
