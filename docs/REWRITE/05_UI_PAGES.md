# 05 — Inventaris Halaman UI (89 page + 33 layout)

> Sumber: `find -name page.tsx` (diukur 2026-09-26). UI lama = React 19 + @radix-ui/themes + Tailwind v4 → port ke Svelte 5 + shadcn-svelte (mapping di `02_FRONTEND_SVELTE.md §6`).
> **Catatan ukuran kerja**: klien & mitra pakai catch-all `[[...section]]` — jumlah route sedikit tapi isinya section raksasa. Checklist di bawah menghitung **section**, bukan file route.
> `[ ]` = checklist parity UI.

---

## 1. Publik & Auth (17)

| Halaman | Tujuan | Fase | ✔ |
|---|---|---|---|
| `/` | Landing page (marketing, pricing, CTA register) | 1 | [x] |
| `/login` | Login (+2FA challenge flow) | 1 | [x] |
| `/login/google-callback` | Callback OAuth Google client-side | 1 | [x] |
| `/login/oauth-link` | Link akun Google | 1 | [x] |
| `/register-role` | Pilih peran (klien/mitra) | 1 | [x] |
| `/register/klien` | Registrasi klien | 1 | [x] |
| `/register/mitra` | Registrasi mitra | 1 | [x] |
| `/forgot-password` | Minta reset | 1 | [x] |
| `/reset-password` | Reset via token | 1 | [x] |
| `/berita-acara/{token}` | Dokumen berita acara publik (print/PDF) | 8 | [x] |
| `/blogger-pick-blog` | Pilih blog Blogger setelah OAuth | 6 | [x] |
| `/docs` | Dokumentasi publishing API partner (publik) | 5 | [x] |
| `/payments/status` | Polling status pembayaran | 8 | [x] |
| `/payments/success`, `/payments/success/{brandId}` | Redirect sukses | 8 | [x] |
| `/payments/cancelled`, `/payments/cancelled/{brandId}` | Redirect batal | 8 | [x] |
| `/payments/sandbox/{tx}` | Gateway sandbox lokal | 8 | [x] |
| Redirects legacy | `/brands→/klien`, `/platform→/admin`, `/partner→/mitra` | 1 | [x] |

## 2. Klien (15 route → 14 section + 9 halaman khusus)

### 2.1 Workspace catch-all `/klien/[brandId]/[[...section]]` (14 section)

| Section | Isi | Fase | ✔ |
|---|---|---|---|
| `overview` | Ringkasan + poll status | 2 | [x] |
| `dashboard` | Dashboard analitik + grafik | 2 | [x] |
| `analytics` | Analitik artikel (views, scroll, klik) | 10 | [x] |
| `campaigns` | List + detail campaign | 7 | [x] |
| `distribution` | **File terbesar (4.251 baris)**: publikasi, mitra, status, tracking | 6 | [x] |
| `media` | Katalog & pilih media (modal detail 721 baris) | 5 | [x] |
| `publications` | Publikasi & approval | 6 | [x] |
| `knowledge` | Knowledge collections & sources | 3 | [x] |
| `members` | Kelola member brand | 2 | [x] |
| `credits` | Saldo TC, topup, ledger | 8 | [x] |
| `invoices` | Invoice | 8 | [x] |
| `notifications` | Bell + list (polling) | 12 | [x] |
| `pr-calculator` | Kalkulator & skor PR (998 baris) | 10 | [x] |
| `profile` | Profil brand, voice, rules, prompts, CMS | 2 | [x] |

### 2.2 Halaman khusus klien (9)

| Halaman | Isi | Fase | ✔ |
|---|---|---|---|
| `/klien` | Daftar brand + onboarding | 2 | [x] |
| `/klien/{brandId}/campaigns/{campaignId}/flow` | Wizard campaign (3.749 baris: intake → media → budget → produksi → review) | 7 | [x] |
| `/klien/{brandId}/editor/{documentId}` | Editor artikel (versi, review, standard check, publish) | 4 | [x] |
| `/klien/{brandId}/knowledge` | Knowledge center penuh | 3 | [x] |
| `/klien/{brandId}/taut-pintar` | Daftar taut campaign | 9 | [x] |
| `/klien/{brandId}/taut-pintar/{id}` | Ringkasan agent steps (polling) | 9 | [x] |
| `.../{id}/berita-acara` | Berita acara taut | 9 | [x] |
| `.../{id}/gatekeeper` | Audit & distribusi (polling) | 9 | [x] |
| `.../{id}/matchmaker` | Pencocokan media (1.072 baris) | 9 | [x] |
| `.../{id}/profiler`, `/profiler/analyzing`, `/profiler/summary` | Profiler channel 3 layar | 9 | [x] |
| `.../{id}/ringkasan` | Ringkasan kampanye | 9 | [x] |
| `.../{id}/writer` | Produksi artikel per media | 9 | [x] |

## 3. Mitra (2 route → 19 section)

`/mitra` (daftar/redirect) + `/mitra/[partnerId]/[[...section]]` — **sections.tsx 5.866 baris** → pecah 19 komponen:

| # | Section | Isi | Fase | ✔ |
|---|---|---|---|---|
| 1 | `profile` | Profil partner | 5 | [x] |
| 2 | `media` | Kelola media (bulk import 489 baris) | 5 | [x] |
| 3 | `publications` | Publikasi masuk, approve/reject/draft | 6 | [x] |
| 4 | `connections` | Koneksi CMS (WP/custom) | 5 | [x] |
| 5 | `blogger` | Integrasi Blogger | 6 | [x] |
| 6 | `rss-feeds` | Kelola RSS feed | 5 | [x] |
| 7 | `crawler` | Crawler consent & status | 9 | [x] |
| 8 | `earnings` | Penghasilan | 8 | [x] |
| 9 | `settlements` | Settlement & bukti | 8 | [x] |
| 10 | `referral` | Referral & komisi | 11 | [x] |
| 11 | `vip` | Status VIP | 11 | [x] |
| 12 | `vip-dashboard` | Dashboard VIP (blueprint Mas Aqil) | 11 | [x] |
| 13 | `notifications` | Notifikasi | 12 | [x] |
| 14 | `link-monitor` | Monitor link | 6 | [x] |
| 15 | `seo-tool` | SEO check + speed audit | 10 | [x] |
| 16 | `kalkulator-media` | Kalkulator media | 10 | [x] |
| 17 | `publishing-api` | API key + docs | 5 | [x] |
| 18 | `sindikasi-konten` | Sindikasi konten | 6 | [x] |
| 19 | `integrations` | GSC & lainnya | 10 | [x] |

## 4. Admin (54 page) 🛡️

Sidebar dengan gating per modul (28 layout). Fase mengikuti `07_EXECUTION_PHASES.md`:

| Halaman | Tujuan | Fase | ✔ |
|---|---|---|---|
| `/admin` (index) | Dashboard admin | 11 | [x] |
| `/admin/users` | Kelola user (kuota, role, status, password) | 11 | [x] |
| `/admin/roles` | Role & akses | 11 | [x] |
| `/admin/brands` | Semua brand | 11 | [x] |
| `/admin/media` | Kelola media (1.383 baris subsection) | 5 | [x] |
| `/admin/media-otomatis` | Media otomatis (crawl/sinkron) | 5 | [x] |
| `/admin/media-intelligence` | Media intelligence | 10 | [x] |
| `/admin/media-packages` | Paket media | 7 | [x] |
| `/admin/media-standards` | Standar konten media | 4 | [x] |
| `/admin/media-summaries` | Rangkuman AI media | 5 | [x] |
| `/admin/moderations` | Moderasi konten | 4 | [x] |
| `/admin/content-settings` | Setting konten global | 4 | [x] |
| `/admin/content-standards` | Standar konten per tipe | 4 | [x] |
| `/admin/knowledge-bases` | KB rules & versi | 3 | [x] |
| `/admin/publications` + `[publicationId]` | Semua publikasi | 6 | [x] |
| `/admin/variants` + `variants/[batchId]` | Batch varian | 6 | [x] |
| `/admin/pr-calculator` | PR calculator admin | 10 | [x] |
| `/admin/pr-analysis-history` | Riwayat analisis PR | 10 | [x] |
| `/admin/pr-brand-standards` | Standar PR per brand | 10 | [x] |
| `/admin/integrasi-gsc` | GSC platform | 10 | [x] |
| `/admin/ga4-configs` | GA4 config | 10 | [x] |
| `/admin/integrations` | Integrasi eksternal | 11 | [x] |
| `/admin/ai` | AI gateway (config, model, knowledge, usage — 1.143 baris) | 11 | [x] |
| `/admin/ai-usage` | Usage AI | 11 | [x] |
| `/admin/credits` | Konfig TC | 8 | [x] |
| `/admin/credit-reports` | Laporan topup | 8 | [x] |
| `/admin/payments` | Gateway & transaksi | 8 | [x] |
| `/admin/packages` | Paket kredit | 8 | [x] |
| `/admin/orders` | Order campaign | 8 | [x] |
| `/admin/invoices` | Invoice | 8 | [x] |
| `/admin/revenue-shares` | Revenue share | 8 | [x] |
| `/admin/settlements` | Settlement | 8 | [x] |
| `/admin/partners` | Kelola mitra | 5 | [x] |
| `/admin/vip-partners` | VIP partner | 11 | [x] |
| `/admin/referrals` | Referral | 11 | [x] |
| `/admin/referral-audit` | Audit referral | 11 | [x] |
| `/admin/referred-domains` | Domain referral | 11 | [x] |
| `/admin/rss-feeds` | RSS global | 5 | [x] |
| `/admin/sindikasi-konten` | Sindikasi | 6 | [x] |
| `/admin/dewan-pers-reviews` | Verifikasi Dewan Pers | 5 | [x] |
| `/admin/link-monitor` | Monitor link | 6 | [x] |
| `/admin/domain-diagnostics` | Diagnostik domain | 5 | [x] |
| `/admin/operations` | Operations center | 11 | [x] |
| `/admin/errors` | Error monitor | 11 | [x] |
| `/admin/audit-log` | Audit log | 11 | [x] |
| `/admin/auto-replace` | Auto-replace config | 6 | [x] |
| `/admin/bug-reports` | Laporan bug | 12 | [x] |
| `/admin/taut-pintar` | Monitor taut campaign | 9 | [x] |
| `/admin/taut-agents` | Config agent | 9 | [x] |
| `/admin/taut-prompts` | Prompt & angle | 9 | [x] |
| `/admin/konfigurasi` | Settings (email, turnstile, pagespeed, server) | 11 | [x] |
| `/admin/wa-gateway` | WA gateway | 12 | [x] |

## 5. Layouts (33)

- Root layout (font, theme, toaster)
- `admin/layout.tsx` + 27 layout submodule (sidebar gating per modul)
- `klien/layout.tsx` (gate: refresh token + membership → port `gate.ts`)
- `mitra/layout.tsx` (gate role partner)
- Landing & auth layout publik

## 6. Komponen Bersama (port per fase)

`src/components/` (52 file / 7.405 baris): `shells/product-shell.tsx` (574), `app-shell.tsx` (414), `domain/media-detail-modal.tsx` (721), `profile/{client,partner}-profile.tsx`, `mitra/bulk-media-import.tsx` (489), `credit/*`, `taut/*`, `landing/*`, `site/*`, `theme/*`. Port mengikuti fase domain masing-masing; shell & theme di fase 1–2.

## 7. Rekap

| Area | Jumlah halaman/route |
|---|---|
| Admin | 54 |
| Klien | 15 (14 section catch-all + 9 halaman khusus di dalamnya) |
| Mitra | 2 route / 19 section |
| Login | 3 |
| Payments | 6 |
| Register/reset | 5 |
| Publik lain | 4 (landing, berita-acara, blogger-pick-blog, docs) |
| **Total `page.tsx`** | **89** (+33 layout) |
