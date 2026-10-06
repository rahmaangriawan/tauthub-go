# Laporan Audit Keamanan — tauthub-go

Tanggal: 2026-10-06 · Lingkup: kode aplikasi (api/ Go + web/ SvelteKit) dan infrastruktur (VPS staging 103.42.245.116, production 43.156.237.97).
Metode: pemetaan permukaan oleh 3 agen eksplorasi (auth/authz, input-SSRF-upload-secrets, frontend-publik) + verifikasi manual + audit infra via SSH.

Status perbaikan: item bertanda **[FIXED]** sudah diterapkan pada tanggal ini dengan prinsip *zero-behavior-change* (tidak menyentuh alur distribusi konten klien → mitra → CMS mitra). Item **[DEFERRED]** sengaja belum dikerjakan beserta alasannya — butuh keputusan produk/desain atau berpotensi mengubah perilaku.

---

## Ringkasan eksekutif

Kondisi dasar aplikasi **baik**: anti-SSRF terpusat, tidak ditemukan SQL injection, webhook fail-closed, secret ter-enkripsi dengan respons hint-only, dan upload dibatasi ketat. Namun ditemukan **1 celah akses lintas-brand (IDOR) yang bisa membaca dan menulis dokumen kampanye brand lain**, penyajian file upload yang masih menerima token sesi mati, dua titik XSS di modal Writer, dan secret sandbox yang ter-ship ke bundle produksi. Semua yang tergolong berbahaya dan aman untuk ditambal **sudah ditambal** (bagian 2). Sisanya berada di bagian 3 menunggu keputusan.

---

## 1. Temuan & status

### 1.1 IDOR lintas-brand pada grup Taut — [TINGGI] **[FIXED]**

- **Bukti**: `api/internal/domains/brandflow/taut_extras.go` — `TautArticleGet`, `TautArticlePut`, `TautArticleUpload`, `TautFilesList`, `TautFileDelete` hanya memverifikasi membership brand (`d.access`) lalu query `taut_campaign_media` / `taut_campaign_files` by `campaign_id` **tanpa** cek `campaigns.brand_id`. Kontras: `TautFilesUpload` (:409) melakukan cek itu.
- **Dampak**: user brand A dengan session sah bisa membaca/mengubah/menghapus artikel & file kampanye brand B hanya dengan mengubah `campaignId` di URL.
- **Fix**: kelima handler kini mengecek `SELECT 1 FROM campaigns WHERE id=$1 AND brand_id=$2` → 404 (pola identik `TautFilesUpload`). User sah tidak terpengaruh karena URL memang ber-scope brand sendiri.

### 1.2 XSS pada pratinjau Writer — [TINGGI] **[FIXED]**

- **Bukti**: `web/src/routes/klien/[brandId]/taut-pintar/[id]/writer/_naskah-modal.svelte:790` merender `{@html streamingHtml}` (output LLM mentah via SSE, tanpa sanitasi); `_editor-modal.svelte:169` `body.innerHTML = rest || initialHtml` tanpa sanitasi. Token sesi tersimpan di `localStorage` (lihat 3.1) → XSS = pembajakan sesi penuh.
- **Fix**: keduanya kini melewati `sanitizeHtml()` (sanitizer yang sama dengan rich-text-editor). Sanitasi hanya di lapisan tampilan — HTML yang tersimpan/dikirim ke CMS mitra tidak berubah.

### 1.3 Secret sandbox payment ter-ship ke bundle — [TINGGI di staging] **[FIXED]**

- **Bukti**: `web/src/routes/payments/sandbox/[tx]/+page.svelte` — `SANDBOX_SECRET = 'tauthub-sandbox-secret'` hardcoded dan ikut ter-build ke produksi.
- **Mitigasi yang sudah ada**: backend menolak provider sandbox saat `APP_ENV=production` (`payments_support.go:1171`), dan staging pun `APP_ENV=production` → simulasi memang sudah tidak berfungsi di luar lokal.
- **Fix**: konstanta + tombol simulasi di-gate `import.meta.env.PROD` → string secret dibuang dari bundle produksi (dead-code elimination Vite). Perilaku efektif tidak berubah di semua lingkungan.

### 1.4 `/uploads/{key}` menerima refresh token yang sudah di-revoke — [SEDANG] **[FIXED]**

- **Bukti**: `platform/uploads_serve.go:54` hanya `VerifyRefreshToken` (HMAC murni) tanpa cek `user_sessions.revoked` / `users.is_active` — token hasil logout/reset password/suspend masih bisa mengunduh dokumen sampai JWT kedaluwarsa.
- **Fix**: kini dicek `revoked/expires_at` + user aktif (pola `Authenticator.User`). User aktif tidak terpengaruh.

### 1.5 Endpoint publik tanpa rate limit — [SEDANG] **[FIXED]**

- **Bukti**: `GET /v1/payments/status/{id}` (`payments_public.go:160`, memicu rekonsiliasi ke provider) dan `GET /v1/campaign-reports/{token}` (`clientread/campaign_report.go:17`, brute-force token berita acara) tanpa limit.
- **Fix**: keduanya dibatasi 120/menit/IP (longgar; jauh di atas pemakaian wajar) via `pubRateLimit` / `CheckAuthRate`.

### 1.6 Security headers & robots.txt — [RENDAH/Sedang, infra] **[FIXED]**

- Nginx kedua server kini mengirim `Strict-Transport-Security`, `X-Frame-Options: SAMEORIGIN`, `Referrer-Policy`, `Permissions-Policy` (additive; CSP penuh sengaja belum — lihat 3.10). Template `deploy/nginx-staging.conf` ikut diperbarui.
- `web/static/robots.txt` kini men-disallow `/admin`, `/klien`, `/mitra`, `/berita-acara/` (sebelumnya allow-all; area privat terindeks mesin pencari).
- Catatan kecil: di production terlihat header duplikat (nginx + aplikasi) — tidak berbahaya (browser memakai nilai pertama), dirapikan nanti.

---

## 2. Praktik yang sudah baik (hasil audit)

1. **Anti-SSRF terpusat**: `internal/security/safefetch.go` — blokir loopback/private/CGNAT/metadata per-koneksi (kebal DNS rebinding), redirect limit, dipakai semua fetch URL dari DB/user (crawler, link-check, CMS, payment verify, RSS).
2. **Tidak ditemukan SQL injection**: semua nilai lewat placeholder; fragment SQL dinamis hanya dari konstanta/kolom whitelist (~40 file SQL manual — rawan regresi, tapi bersih).
3. **Upload**: storage key UUID (nama user tak pernah menyentuh disk), whitelist MIME/ekstensi ganda, cap 10 MB, konversi WebP, `filepath.Base` + tolak traversal di serve-time.
4. **Secret**: `APP_SECRET` fail-fast ≥32 char, envelope AES-256-GCM (v2/v3 per-entity), respons API hanya `*_configured` + hint, log tidak pernah memuat nilai secret, perbandingan constant-time.
5. **Webhook/callback fail-closed**: 4 provider payment (Xendit/Duitku/Midtrans/Tripay) dengan signature per-provider + body cap; publishing callback HMAC + window 300 detik + api_key hash; OAuth state single-use + PKCE; linking akun wajib re-entry password.
6. **Auth**: rotasi refresh token dalam satu transaksi (anti-race), revoke menyeluruh saat ganti password, lockout 5x → 15 menit, 2FA TOTP, audit log login, rate limit DB-backed per IP/email/user.
7. **Frontend**: SSR gate fail-closed berlapis (hooks + ssr-gate backend), `safeReturnTo` anti open-redirect, CORS exact-allowlist, sanitizer HTML dua lapis (server + client) di jalur artikel, API key agent write-only (sealed, tidak pernah keluar).

---

## 3. Temuan [DEFERRED] — direkomendasikan, belum dikerjakan

Urutan sesuai prioritas. Alasan ditunda: menyentuh alur distribusi, butuh keputusan produk, atau butuh migrasi data.

| # | Temuan | Bukti | Rekomendasi |
|---|--------|-------|-------------|
| 3.1 | **ACL `/uploads` flat** — semua user login bisa membaca semua file (bukti transfer, sertifikat) selama tahu key UUID | `uploads_serve.go:64-92` | Desain pemetaan owner↔key dulu; URL `/uploads` ikut ter-embed ke artikel yang dipublikasikan ke CMS mitra, jadi pembatasan butuh analisis konsumen URL (browser klien, peFetchUploadBytes server-side, pratinjau). |
| 3.2 | **Admin reset password & role change tanpa step-up** — super_admin bisa set password akun siapa pun tanpa konfirmasi password/TOTP | `platform/operations.go:2262-2331` | Tambah konfirmasi password admin / TOTP step-up untuk operasi kredensial. |
| 3.3 | **Fallback super_admin via `PLATFORM_ADMIN_EMAILS`** — email terverifikasi = kontrol penuh bila cocok env; aktif juga saat query DB error (dibungkam) | `adminshared/shared.go:127-142` | Pertahankan tapi: gagal-closed saat DB error, dan catat env ini sebagai secret. |
| 3.4 | **Rate limit berbasis XFF spoofable**; `TrackingEventCreate` skip limit bila IP kosong | `ratelimit.go:120-132`, `public_endpoints.go:399` | Kunci IP dari proxy terpercaya (Cloudflare) atau `real_ip` module; jangan skip limit. |
| 3.5 | **`decodeJSON` tanpa LimitReader** (±149 handler) → body besar = DoS memori | `platform/types.go:73-79`, `auth/register.go:64` | Middleware `http.MaxBytesReader` global 4-10 MB; uji payload besar (import RSS, artikel panjang) sebelum aktif. |
| 3.6 | **LLM gateway tanpa SafeClient** — BaseURL di-set admin; SSRF admin-only (key Bearer terkirim ke host tujuan) | `clienttaut/agent.go:255`, `internal/ai/provider.go:468` | Pakai `SafeClient` + validasi host BaseURL; uji ke semua provider yang dipakai. |
| 3.7 | **Token sesi di localStorage** — XSS = pencurian sesi penuh | `web/src/lib/auth.ts:32,89` | Jangka pendek: CSP ketat. Jangka panjang: access token in-memory + refresh via cookie HttpOnly untuk API. |
| 3.8 | **`/t/{code}` tanpa validasi skema + redirect 301 permanen** — klien bisa memakai domain Tauthub untuk phishing; 301 di-cache browser | `shortlink.go:50-86`, `public_endpoints.go:569` | Validasi tujuan http(s) + pertimbangkan 302; hati-hati dampak SEO/analitik. |
| 3.9 | **`notification_webhooks.secret` plaintext at-rest** | `brandflow/integrations.go:613,688`, `partner/misc.go:358` | Seal seperti secret lain + migrasi data existing. |
| 3.10 | **CSP belum dipasang**; CORS masih memuat origin legacy (`ai-writer-five-coral.vercel.app`, `tulisio…`, localhost) | `internal/httpx/cors.go:6-20` | Naikkan CSP bertahap (report-only → enforce); bersihkan CORS origin yang tak terpakai. |
| 3.11 | **Google OAuth auto-approve** akun baru, sementara register password menunggu approval | `auth/google.go:258` | Keputusan produk: samakan gerbang approval. |
| 3.12 | **Token di query string** (feed/report share) berisiko masuk access log | `rss_feed.go`, `campaign_report.go` | Terima via header bila memungkinkan, atau rotasi token berkala. |
| 3.13 | **Bind API `0.0.0.0:8080`** — port aplikasi ter-expose publik (security group provider menahan, tapi defense-in-depth lebih baik) | `ss -tlnp` kedua server | Bind `127.0.0.1:8080` (scheduler & nginx memanggil via localhost) — uji dulu semua pemanggil. |
| 3.14 | **Layanan/permukaan server lain**: proftpd (:21), postfix (:25), varnish aktif di production; kredensial SSH pernah dibagikan lewat chat | `ss -tlnp`, sshd_config | Matikan layanan tak terpakai, tutup port di security group, **rotasi password SSH** kedua server, pertimbangkan key-only. |

---

## 4. Roadmap yang disarankan

1. **Keputusan produk**: ACL `/uploads` (3.1) — desain pemetaan kepemilikan file terlebih dulu.
2. **Cepat & aman**: step-up admin (3.2), seal webhook secret (3.9), bersihkan CORS (3.10).
3. **Butuh pengujian**: MaxBytesReader global (3.5), SafeClient LLM (3.6), bind 127.0.0.1 (3.13).
4. **Infra/ops**: rotasi kredensial SSH (3.14), real_ip untuk rate limit (3.4), CSP bertahap (3.10).
5. **Desain jangka panjang**: sesi cookie-based untuk API (3.7), kebijakan `/t` (3.8).

---

*Catatan verifikasi fix 2026-10-06: `go build ./... && go vet ./...` lolos; build web lolos; header terverifikasi di kedua domain; uji fungsional (Taut lintas-brand → 404, uploads token revoked → 401) dilakukan di staging setelah deploy.*
