# 00 — Master Plan: Rewrite Go + Svelte

> Status: PEDOMAN RESMI. Repo lama (`tauthub-ke-go`, Next.js) hanya referensi baca-saja setelah fase bootstrap.
> Database target: PostgreSQL **`tauthub-go`** (sudah dibuat terpisah dari `ai_writer`, isi = dump produksi 2026-09-26).

---

## 1. Tujuan & Latar Belakang

Next.js 16 + React 19 monolith (`apps/web`, 148.199 baris / 994 file) terasa **bloat**: bundle besar, build berat (Turbopack + React Compiler + NODE_OPTIONS=2GB), dev server lambat saat kompilasi cold, dan runtime Node yang berat untuk VPS kecil. Keputusan: **rewrite total** ke:

| Layer | Pilihan | Alasan |
|---|---|---|
| Backend | **Go + chi + pgx + sqlc** | chi ringan & idiomatis (router-only, bukan framework raksasa); pgx = driver Postgres paling performant; sqlc = type-safe **dari SQL mentah** — cocok dengan realitas codebase lama yang 100% raw SQL (±913 call site, tanpa ORM) |
| Frontend | **SvelteKit 5 + Svelte 5 (runes) + shadcn-svelte + Tailwind v4** | kompilasi ke DOM minimal (tanpa VDOM), bundle kecil; shadcn-svelte = komponen copy-paste berbasis Tailwind — porting paling halus dari @radix-ui/themes |
| Worker | **Go binary terpisah** (`cmd/worker`, `cmd/scheduler`) | job queue tetap DB-polling (tanpa broker baru), worker jadi service systemd |
| Deployment | **VPS systemd** (nginx → api:8080 + web:3000 + worker) | upload file di local disk & job polling butuh proses long-running; skrip deploy VPS sudah ada di repo lama |
| Database | **PostgreSQL `tauthub-go`, schema TIDAK diubah** | DB adalah anchor kontrak. Semua data produksi, sealed secret, dan sesi harus tetap valid |

**Non-goal**: tidak ada redesign UI, tidak ada perubahan API contract, tidak ada migrasi data. Yang berubah hanya implementasi (bahasa + framework).

---

## 2. Definisi Parity (kapan rewrite dinyatakan "sama persis")

Sebuah domain dinyatakan **selesai** hanya jika:

1. **Semua endpoint** domain itu (daftar di `04_API_ENDPOINTS.md`) merespons dengan **request/response schema, status code, dan error envelope yang identik** (`{"detail": "..."}` gaya FastAPI; 500 di-scrub di production).
2. **Semua halaman/section** UI-nya (daftar di `05_UI_PAGES.md`) render data yang sama, aksi CRUD yang sama, dan alur polling yang sama (tanpa SSE/WebSocket — dilarang menambah real-time di fase parity).
3. **Auth bekerja lintas-aplikasi**: access token JWT lama (dari localStorage client lama) dan refresh cookie lama **tetap valid** di backend Go — signing `APP_SECRET`, claims (`sid`, `sub`, `role`, `exp`), dan mekanisme revocation (`user_sessions`) identik.
4. **Sealed secret lama tetap terbaca**: unseal v1 (XOR+HMAC legacy), v2 (`t2:` AES-256-GCM), v3 (`t3:` per-entity) — semua kredensial WordPress/Blogger/payment/AI di DB tidak boleh gagal didekripsi.
5. **Password lama tetap login**: argon2id params m=19456 t=2 p=1 keylen=32.
6. **Smoke test per domain lulus** (port dari `apps/web/scripts/smoke/*.ts`).

---

## 3. Prinsip Eksekusi (dibawa dari REFACTOR_FULL_AI.md, masih berlaku)

- **DB adalah kontrak**: query disalin mentalnya dari SQL lama (`?` → `$1` untuk sqlc/pgx), bukan diimprovisasi. Jika ada bug di SQL lama, salin dulu (parity), catat, perbaiki terpisah.
- **Deterministik di kode, keputusan di AI**: auth, quota, idempotency, HMAC, state machine TETAP di kode Go; hanya reasoning yang dikirim ke AI provider.
- **Vertical slice**: tiap fase membawa backend + frontend + verifikasi domain itu (lihat `07_EXECUTION_PHASES.md`), bukan "semua backend dulu baru semua frontend".
- **Surgical porting**: satu endpoint = satu handler Go; satu section UI = satu komponen Svelte. Jangan menggabung/memisah endpoint saat porting.
- **Tanpa dependensi "ajaib"**: tidak ada ORM, tidak ada global interceptor yang menyembunyikan SQL, tidak ada server state library berat di frontend.
- **Baseline yang dibawa utuh**: `schema.pg.sql` + 172 migration SQL (disalin ke repo baru), `dictionaries.ts` (i18n), konstanta template email/WA, config platform di DB (bukan di kode).

---

## 4. Struktur Repo Baru

```
tauthub-go/                      # repo baru
├── api/                         # Go module (satu module untuk semua binary)
│   ├── cmd/
│   │   ├── api/main.go          # HTTP server (chi) — port 406 endpoint
│   │   ├── worker/main.go       # job drainer loop (pengganti scheduler-worker.mjs + /v1/internal/jobs)
│   │   └── scheduler/main.go    # scheduler loop (pengganti runScheduler + cron Vercel)
│   ├── internal/
│   │   ├── config/              # env (godotenv + validasi fail-fast ala config.server.ts)
│   │   ├── db/                  # pgxpool, helper tx, migrasi, RunResult shim
│   │   ├── httpx/               # middleware: cors, recover, auth bearer, ratelimit, error envelope
│   │   ├── auth/                # session JWT, argon2, totp, google oauth, turnstile, rate-limit
│   │   ├── crypto/              # seal/unseal v1/v2/v3 (kompat byte-level)
│   │   ├── security/            # safe-fetch (anti-SSRF), egress-ip
│   │   ├── jobs/                # queue (SKIP LOCKED), 12 job kind, scheduler sweep
│   │   ├── ai/                  # provider gateway + embeddings lokal
│   │   ├── publish/             # wordpress, blogger, custom-web, rss, tulisio-api
│   │   ├── integrations/        # gsc, ga4, seo-metrics, pagespeed, smodin, crawlhub, fonnte, turnstile
│   │   ├── uploads/             # local disk + kompresi gambar
│   │   └── domains/             # handler per domain (brands, campaigns, media, publications,
│   │                            #   partner, platform, credits, payments, finance, taut, pr, ...)
│   ├── queries/                 # file .sql untuk sqlc, per domain
│   ├── migrations/              # BASELINE: salinan schema.pg.sql + 172 additive migration
│   ├── sqlc.yaml
│   └── go.mod
├── web/                         # SvelteKit 5 (adapter-node) — port 89 halaman
│   ├── src/routes/…             # struktur route = peta di 05_UI_PAGES.md
│   ├── src/lib/…                # api client, auth, cache, i18n, komponen
│   └── svelte.config.js
├── deploy/                      # systemd units, nginx conf, skrip deploy (port dari scripts/production)
├── docs/                        # salin folder REWRITE/ ini + docs/ lama yang relevan
└── Makefile                     # dev, test, sqlc, build, deploy
```

**Prinsip layout**: satu package Go per domain (`internal/domains/<domain>`), handler chi per file mirip `route.ts` per path. Jangan bikin layer abstrak "repository/service/controller" generik — codebase lama flat dan langsung; pertahankan gaya itu.

---

## 5. Strategi Eksekusi & Cutover

1. **Fase 0** buat repo baru + skeleton + CI + DB terhubung (`tauthub-go`).
2. Fase 1–12 = vertical slice per domain (peta lengkap di `07_EXECUTION_PHASES.md`). Setiap fase selesai → smoke test domain itu lulus menjalankan **data produksi yang sama** di DB `tauthub-go`.
3. **Parallel run**: di VPS, app lama tetap hidup sampai fase 12 selesai. App baru jalan di port terpisah, diuji via smoke test + domain produksi tertentu (mis. akses admin baru).
4. **Cutover**: nginx switch → web SvelteKit + api Go melayani domain publik. App lama dimatikan (systemd stop, bukan delete).
5. **Rollback plan**: nginx balik ke app lama dalam 1 menit (karena DB sama dan auth token kompatibel, rollback aman kapan pun).
6. **Decommission** (setelah ±2 minggu stabil): arsipkan repo lama, hapus service systemd lama, hapus cron Vercel.

### Kriteria decommission
- [ ] Semua checkbox `04_API_ENDPOINTS.md` dan `05_UI_PAGES.md` hijau
- [ ] Smoke test e2e + admin + topup lulus (port dari `scripts/smoke/`)
- [ ] Bundle web baru < 30% ukuran bundle lama (ukur via `measure-bundle` port)
- [ ] 2 minggu tanpa bug parity di produksi

---

## 6. Register Risiko (wajib dibaca sebelum port domain terkait)

| # | Risiko | Dampak | Mitigasi | Detail |
|---|---|---|---|---|
| R1 | Unseal v1 (XOR+HMAC legacy, kompat dengan `cms_security.py`) salah implementasi | Semua kredensial CMS/payment gagal dekripsi | Port byte-level + unit test dengan ciphertext nyata dari DB | `01_BACKEND_GO.md §8` |
| R2 | Argon2/JWT beda params/claims | Semua user tak bisa login | Unit test hash+token nyata; params tertulis di `01 §3` | `01_BACKEND_GO.md §3` |
| R3 | `?` → `$n` konversi placeholder salah (string literal berisi `?`) | Query error / data korup | Go langsung tulis `$n` di sqlc; jangan port sistem konversi | `01_BACKEND_GO.md §4` |
| R4 | Uploads di local disk (`.data/uploads`) path berbeda di server baru | Gambar lama 404 | Salin direktori storage ke path yang sama; serve legacy `public/uploads` juga | `01_BACKEND_GO.md §10` |
| R5 | Parser file (docx/pdf/xlsx) deviasi hasil vs mammoth/pdf-parse/exceljs | Konten batch/ingest berubah | Port dengan test fixture file nyata; xlsx pakai excelize (matang) | `01_BACKEND_GO.md §11` |
| R6 | Pipeline gambar (sharp) deviasi ukuran/kualitas | Media cover beda ukuran | **Selesai**: encoder WebP pure-Go (`nativewebp`, VP8L **lossless**) + resizer `golang.org/x/image` (CatmullRom, max 1600px) di `internal/imageconvert`. Terpasang di 5 jalur upload: assets brand (klien), dokumen mitra, dokumen Dewan Pers, gambar draft publikasi mitra, bukti settlement. Deviasi tersisa: lossless ≠ lossy sharp, jadi foto bisa lebih besar dari hasil sharp (diuji: PNG 12KB→876B, JPEG 76KB→32KB pada gambar uji). Konversi gagal → byte asli disimpan, upload tidak pernah gagal | `01_BACKEND_GO.md §10` |
| R7 | Naming ganda `tulisio` vs `tauthub` di protocol publishing | Partner adapter lama gagal callback | Pertahankan KEDUA path & header legacy (`X-Tulisio-*`) | `06_INTEGRATIONS.md §2.3` |
| R8 | Job dedupe berbasis `payload->>'publicationId'` hilang | Job ganda / publikasi dobel | Salin SQL dedupe mentah ke scheduler port | `01_BACKEND_GO.md §6` |
| R9 | Next.js proxy (edge) hilang → gating halaman bocor | Halaman admin terlihat tanpa login | Port proxy ke `hooks.server.ts` + layout guard | `02_FRONTEND_SVELTE.md §3` |
| R10 | CSP/security headers & CORS allowlist beda | Turnstile/OAuth gagal di produksi | Salin persis dari `next.config.ts` + `proxy.ts` | `02_FRONTEND_SVELTE.md §9` |
| R11 | Email dedupe pakai `INTERVAL '...'` interpolasi string | Email dobel/berulang | Salin query persis; test window dedupe | `06_INTEGRATIONS.md §7` |
| R12 | SumoPod wajib return URL https-only | Topup sandbox gagal | Detail di doc integrasi, jangan ubah aturan validasi | `06_INTEGRATIONS.md §5` |

---

## 7. Peta Domain (glossary — dipakai konsisten di semua dokumen)

| Domain | Cakupan | Fase |
|---|---|---|
| **Auth & Identity** | register/login/2FA/Google OAuth/sessions/rate-limit/turnstile | 1 |
| **Brands & Workspace** | brands, members, profile, voice, editorial rules, prompts, readiness | 2 |
| **Knowledge & RAG** | collections/sources/chunks, sitemap crawl, embeddings lokal, chat AI | 3 |
| **Content Pipeline** | intake, briefs, content jobs, documents/versions, batches, moderation, quality | 4 |
| **Media & Mitra** | media inventory, partner portal, cms-connections, dewan pers, domain verify, RSS | 5 |
| **Publications & Publishing** | publications, executors (WP/Blogger/custom-web), plans, variants, tracking | 6 |
| **Campaigns & Flow** | campaigns, campaign flow wizard, packages, media-packages | 7 |
| **Credits & Finance** | TC wallet, topup, payments (6 gateway), invoices, settlements, revenue share | 8 |
| **Taut Pintar** | agent system, gatekeeper, profiler, channels, angles, crawler creds | 9 |
| **PR & Analytics** | PR calculator, scoring, verification, GSC, GA4, reports, berita-acara | 10 |
| **Platform Admin** | operations, users mgmt, settings, AI config, moderations, referrals | 11 |
| **Comms & Hardening** | notifications, email queue, WA, webhooks, security headers, cutover | 12 |
