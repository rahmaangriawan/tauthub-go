# 04 — Inventaris API Endpoint (406 route)

> Sumber: ekstraksi `find -name route.ts` + metode HTTP dari export function di tiap file (diukur 2026-09-26).
> `[ ]` = checklist parity. Centang hanya setelah smoke test domain lulus.
> Level akses: 🌐 publik · 🔑 Bearer (client) · 🤝 partner · 🛡️ admin/platform · ⚙️ internal (secret header) · verifikasi detail per handler saat porting.
> Kontrak error global: `{"detail": "..."}` — lihat `01_BACKEND_GO.md §5`.

---

## 1. Non-`/v1` (7) — jalur spesial

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/api/blogger/callback` | GET | OAuth callback Blogger (PKCE) | [x] |
| `/api/gsc/callback` | GET | OAuth callback Google Search Console | [x] |
| `/feed/partner/{token}` | GET | RSS feed partner (XML) | [x] |
| `/feed/partner/{token}/import` | POST | Import artikel dari RSS feed partner | [x] |
| `/register` | GET | Halaman/meta registrasi legacy | [x] |
| `/t/{code}` | GET | Redirect tracking link + catat event | [x] |
| `/uploads/{key}` | GET | Serve file upload (auth refresh cookie) | [x] |

## 2. `/v1/auth` (18) 🔑/🌐 — fase 1

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/auth/2fa/disable` | POST | Nonaktifkan 2FA | [x] |
| `/v1/auth/2fa/enable` | POST | Aktifkan 2FA (konfirmasi kode) | [x] |
| `/v1/auth/2fa/setup` | POST | Generate secret + QR TOTP | [x] |
| `/v1/auth/2fa/verify` | POST | Tukar challenge JWT + kode → token | [x] |
| `/v1/auth/change-password` | POST | Ganti password (revoke sesi) | [x] |
| `/v1/auth/forgot-password` | POST | Kirim token reset | [x] |
| `/v1/auth/google/callback` | GET | Callback OAuth Google | [x] |
| `/v1/auth/google/complete` | POST | Selesaikan provision akun Google | [x] |
| `/v1/auth/google/exchange-session` | POST | Tukar kode → session token | [x] |
| `/v1/auth/google/link` | POST | Link akun Google ke user eksisting | [x] |
| `/v1/auth/google/url` | GET | URL consent OAuth Google | [x] |
| `/v1/auth/login` | POST | Login (+2FA challenge, lockout, rate limit) | [x] |
| `/v1/auth/logout` | POST | Revoke sesi (sid) | [x] |
| `/v1/auth/me` | GET, PATCH | Profil user + update | [x] |
| `/v1/auth/refresh` | POST | Refresh access token (cookie) | [x] |
| `/v1/auth/register` | POST | Registrasi (+registration guard) | [x] |
| `/v1/auth/reset-password` | POST | Reset via token | [x] |
| `/v1/auth/turnstile` | GET | Config Turnstile untuk client | [x] |

## 3. `/v1/brands` (133) 🔑 — fase 2–7, 9

### 3.1 Brand core (17)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands` | GET, POST | List/created brand (membership) | [x] |
| `/v1/brands/{brandId}` | GET | Detail brand + readiness | [x] |
| `/v1/brands/{brandId}/activate` | POST | Aktivasi (skor ≥70) | [x] |
| `/v1/brands/{brandId}/analytics` | GET | Analitik brand | [x] |
| `/v1/brands/{brandId}/analyze/{topic}` | GET | Analisis topik (AI) | [x] |
| `/v1/brands/{brandId}/assets` | POST | Upload asset brand | [x] |
| `/v1/brands/{brandId}/configuration` | GET | Konfigurasi efektif brand | [x] |
| `/v1/brands/{brandId}/dashboard` | GET | Dashboard workspace | [x] |
| `/v1/brands/{brandId}/editorial-rules` | PUT | Update rule set editorial | [x] |
| `/v1/brands/{brandId}/ga4` | GET | Konfig & data GA4 brand | [x] |
| `/v1/brands/{brandId}/overview` | GET | Ringkasan overview | [x] |
| `/v1/brands/{brandId}/pr-calculator` | GET | Data PR calculator | [x] |
| `/v1/brands/{brandId}/profile` | PUT | Update brand profile | [x] |
| `/v1/brands/{brandId}/usage` | GET | Usage AI/quota | [x] |
| `/v1/brands/{brandId}/voice` | PUT | Update voice profile | [x] |
| `/v1/brands/{brandId}/prompts` | POST | Buat master prompt version | [x] |
| `/v1/brands/{brandId}/prompts/{promptId}/activate` | POST | Aktivasi versi prompt | [x] |

### 3.2 Members (3)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/members` | GET, POST | List/tambah member | [x] |
| `/v1/brands/{brandId}/members/{membershipId}` | PUT, DELETE | Ubah role / hapus member | [x] |

### 3.3 Content intakes (8)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/content-intake-types` | GET | Tipe intake aktif | [x] |
| `/v1/brands/{brandId}/content-intakes` | GET, POST | List/buat intake (bahan/artikel) | [x] |
| `/v1/brands/{brandId}/content-intakes/{intakeId}` | GET, PUT | Detail/update intake | [x] |
| `/v1/brands/{brandId}/content-intakes/{intakeId}/ai-fill` | POST | AI isi jawaban intake | [x] |
| `/v1/brands/{brandId}/content-intakes/{intakeId}/analyze` | POST | Analisis kesiapan | [x] |
| `/v1/brands/{brandId}/content-intakes/{intakeId}/auto-key-messages` | POST | AI ekstrak key messages | [x] |
| `/v1/brands/{brandId}/content-intakes/{intakeId}/create-job` | POST | Intake → content job | [x] |
| `/v1/brands/{brandId}/content-intakes/{intakeId}/recommend` | POST | Rekomendasi mode intake | [x] |

### 3.4 Content jobs & documents (10)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/content-jobs` | GET, POST | List/buat job generasi | [x] |
| `/v1/brands/{brandId}/content-jobs/{jobId}` | GET | Status/progress job | [x] |
| `/v1/brands/{brandId}/content-jobs/{jobId}/approve-outline` | POST | Setujui outline | [x] |
| `/v1/brands/{brandId}/content-jobs/{jobId}/cancel` | POST | Cancel job | [x] |
| `/v1/brands/{brandId}/content-jobs/{jobId}/outline` | PUT | Edit outline (approval mode) | [x] |
| `/v1/brands/{brandId}/documents` | GET, POST | List/buat dokumen | [x] |
| `/v1/brands/{brandId}/documents/{documentId}` | GET, PUT, PATCH | Detail/simpan/autosave versi | [x] |
| `/v1/brands/{brandId}/documents/{documentId}/html` | GET | Render HTML dokumen | [x] |
| `/v1/brands/{brandId}/documents/{documentId}/publish` | POST | Publish draft ke CMS | [x] |
| `/v1/brands/{brandId}/documents/{documentId}/standard-check` | POST | Cek standar konten | [x] |

### 3.5 Batches (6)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/batches` | GET, POST | List/buat batch (CSV/XLSX) | [x] |
| `/v1/brands/{brandId}/batches/{batchId}` | GET | Detail + items | [x] |
| `/v1/brands/{brandId}/batches/{batchId}/start` | POST | Mulai batch | [x] |
| `/v1/brands/{brandId}/batches/{batchId}/pause` | POST | Pause | [x] |
| `/v1/brands/{brandId}/batches/{batchId}/cancel` | POST | Cancel | [x] |
| `/v1/brands/{brandId}/batches/{batchId}/retry` | POST | Retry gagal | [x] |

### 3.6 Knowledge & chat (8)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/knowledge/collections` | GET, POST | List/buat koleksi | [x] |
| `/v1/brands/{brandId}/knowledge/collections/{collectionId}` | GET, PUT, DELETE | CRUD koleksi | [x] |
| `/v1/brands/{brandId}/knowledge/collections/{collectionId}/sources` | GET, POST | List/upload sumber | [x] |
| `/v1/brands/{brandId}/knowledge/sources/{sourceId}` | GET, DELETE | Detail/hapus sumber | [x] |
| `/v1/brands/{brandId}/knowledge/sources/{sourceId}/content` | GET | Isi sumber | [x] |
| `/v1/brands/{brandId}/knowledge/sitemap` | GET, POST | Crawl sitemap → sumber | [x] |
| `/v1/brands/{brandId}/chat` | POST | Chat AI brand (tool-calling) | [x] |
| `/v1/brands/{brandId}/chat-learning` | GET, DELETE | Riwayat/hapus learning chat | [x] |

### 3.7 Campaigns & flow (14)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/campaigns` | GET, POST | List/buat campaign | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}` | GET, PUT, DELETE | CRUD campaign | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}/flow` | GET, POST | State wizard flow | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}/flow/analyze` | POST | Analisis brief | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}/flow/article-regen` | POST | Regenerasi artikel | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}/flow/checkout` | POST | Checkout (reservasi TC) | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}/flow/media` | POST | Pilih media | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}/flow/media-type` | POST | Pilih tipe media | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}` | PATCH, DELETE | Ubah/hapus item | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}/angle` | POST | Pilih angle | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}/angle-options` | GET | Opsi angle | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}/flow/media-items/{itemId}/paraphrase` | POST | Paraphrase varian | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}/publications` | GET, POST | Publikasi campaign | [x] |
| `/v1/brands/{brandId}/campaigns/{campaignId}/report/share` | POST | Buat share token laporan | [x] |

### 3.8 Publications (13)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/publications` | GET, POST | List/buat publikasi | [x] |
| `/v1/brands/{brandId}/publications/{publicationId}` | GET | Detail publikasi | [x] |
| `/v1/brands/{brandId}/publications/{publicationId}/accept` | POST | Terima hasil publikasi | [x] |
| `/v1/brands/{brandId}/publications/{publicationId}/auto-replace` | POST | Auto-replace media | [x] |
| `/v1/brands/{brandId}/publications/{publicationId}/cancel` | POST | Cancel | [x] |
| `/v1/brands/{brandId}/publications/{publicationId}/replacement` | GET | Kandidat pengganti | [x] |
| `/v1/brands/{brandId}/publications/{publicationId}/request-revision` | POST | Minta revisi mitra | [x] |
| `/v1/brands/{brandId}/publications/{publicationId}/request-url` | POST | Minta URL publikasi | [x] |
| `/v1/brands/{brandId}/publications/{publicationId}/reset-approval` | POST | Reset approval | [x] |
| `/v1/brands/{brandId}/publications/{publicationId}/retry` | POST | Retry publikasi | [x] |
| `/v1/brands/{brandId}/publications/{publicationId}/tracking` | GET, POST | Data/trigger tracking | [x] |
| `/v1/brands/{brandId}/publications/variants` | GET, POST | List/buat batch varian | [x] |
| `/v1/brands/{brandId}/publications/variants/{batchId}` | GET | Detail batch varian | [x] |

### 3.9 Publication plans (6)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/publication-plans` | GET, POST | List/buat plan | [x] |
| `/v1/brands/{brandId}/publication-plans/{planId}` | GET | Detail plan | [x] |
| `/v1/brands/{brandId}/publication-plans/{planId}/cancel` | POST | Cancel plan | [x] |
| `/v1/brands/{brandId}/publication-plans/{planId}/pause` | POST | Pause plan | [x] |
| `/v1/brands/{brandId}/publication-plans/{planId}/resume` | POST | Resume plan | [x] |
| `/v1/brands/{brandId}/publication-plans/occurrences/{occurrenceId}/skip` | POST | Skip occurrence | [x] |

### 3.10 CMS connections (3)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/cms-connections` | GET, POST | List/tambah koneksi CMS | [x] |
| `/v1/brands/{brandId}/cms-connections/{connectionId}` | GET, PUT, DELETE | CRUD koneksi | [x] |
| `/v1/brands/{brandId}/cms-connections/{connectionId}/test` | POST | Tes koneksi | [x] |

### 3.11 Media (brand-side) (4)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/media` | GET | Katalog media | [x] |
| `/v1/brands/{brandId}/media/recommendations` | GET | Rekomendasi media (AI) | [x] |
| `/v1/brands/{brandId}/media/{mediaId}/crawl-status` | GET | Status crawl media | [x] |
| `/v1/brands/{brandId}/media/{mediaId}/seo` | GET | Metrik SEO media | [x] |

### 3.12 Credits, invoices, packages (6)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/credits` | GET | Saldo & ledger TC | [x] |
| `/v1/brands/{brandId}/credits/topup` | GET, POST | Opsi & buat topup | [x] |
| `/v1/brands/{brandId}/credit-reports` | POST | Laporan topup manual | [x] |
| `/v1/brands/{brandId}/invoices` | GET | List invoice | [x] |
| `/v1/brands/{brandId}/packages` | GET | Katalog paket | [x] |
| `/v1/brands/{brandId}/media-packages` | GET | Paket media custom | [x] |

### 3.13 Reports, notifications, link-monitor (8)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/reports` | GET | Laporan publikasi/analytics | [x] |
| `/v1/brands/{brandId}/reports/link-clicks` | GET | Klik outbound link | [x] |
| `/v1/brands/{brandId}/notifications` | GET, POST | List/buat notifikasi | [x] |
| `/v1/brands/{brandId}/notifications/{notificationId}/read` | POST | Tandai dibaca | [x] |
| `/v1/brands/{brandId}/notification-webhooks` | GET, POST | List/daftar webhook | [x] |
| `/v1/brands/{brandId}/notification-webhooks/{webhookId}` | PUT, DELETE | Ubah/hapus webhook | [x] |
| `/v1/brands/{brandId}/link-monitor` | GET | Monitor link | [x] |
| `/v1/brands/{brandId}/link-monitor/request-replacement` | POST | Minta penggantian link | [x] |

### 3.14 Taut Pintar (28) — fase 9

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/brands/{brandId}/taut/agent-status` | GET | Status agent taut | [x] |
| `/v1/brands/{brandId}/taut/campaigns` | GET, POST | List/buat taut campaign | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}` | GET, POST, PUT, DELETE | CRUD + aksi taut campaign | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/advance` | POST | Lanjut tahap agent | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/articles/{mediaId}` | GET, PUT | Artikel per media | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/articles/{mediaId}/upload` | POST | Upload artikel | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/budget-estimate` | GET | Estimasi budget | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/crawl` | POST | Trigger crawl media | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/files` | GET, POST | File kampanye | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/files/{fileId}` | DELETE | Hapus file | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/finish` | POST | Finalisasi | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/gatekeeper/audit-all` | POST | Audit semua artikel | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/gatekeeper/audits` | GET | Hasil audit | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/gatekeeper/distribute` | POST | Distribusi hasil audit | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/placement` | GET, POST, PUT | Penempatan media | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/produce` | POST | Produksi batch | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/produce/{mediaId}` | POST | Produksi per media | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/profile` | GET, POST, PUT | Profiler channel | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/profile/analyze` | POST | Analisis profil | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/profile/draft` | GET, PUT | Draft profil | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/profile/result` | DELETE | Hapus hasil | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/profile/results/{resultId}` | GET | Detail hasil profiler | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/profile/verify` | POST | Verifikasi profil | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/profiler/options` | GET | Opsi profiler | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/schedule` | POST | Jadwalkan publikasi | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/submit-final` | GET, POST, PUT, PATCH, DELETE | Submit akhir multi-aksi | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/validate` | GET, POST | Validasi kampanye | [x] |
| `/v1/brands/{brandId}/taut/campaigns/{campaignId}/writer-links` | GET, PUT | Link writer | [x] |

**Subtotal `/v1/brands`: 17+2+8+10+6+8+14+13+6+3+4+6+8+28 = 133 ✓**

## 4. `/v1/partner` (63) 🤝 — fase 5

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/partner/me` | GET, PATCH | Profil partner | [x] |
| `/v1/partner/me/vip` | GET | Status VIP | [x] |
| `/v1/partner/media` | GET, POST | List/tambah media mitra | [x] |
| `/v1/partner/media/categories` | GET | Kategori media | [x] |
| `/v1/partner/media/import` | POST | Import bulk media (template) | [x] |
| `/v1/partner/media/template-url` | GET | URL template import | [x] |
| `/v1/partner/media/{mediaId}` | GET, PUT, DELETE | CRUD media | [x] |
| `/v1/partner/media/{mediaId}/ai-summary` | GET | Rangkuman AI media | [x] |
| `/v1/partner/media/{mediaId}/blogger` | POST, DELETE | Hubungkan/hapus Blogger | [x] |
| `/v1/partner/media/{mediaId}/blogger/blogs` | GET | Daftar blog Blogger | [x] |
| `/v1/partner/media/{mediaId}/blogger/connect` | GET | Mulai OAuth Blogger | [x] |
| `/v1/partner/media/{mediaId}/blogger/test-connection` | POST | Tes koneksi Blogger | [x] |
| `/v1/partner/media/{mediaId}/category` | PATCH | Ubah kategori | [x] |
| `/v1/partner/media/{mediaId}/custom-web` | PATCH, POST | Konfig custom web adapter | [x] |
| `/v1/partner/media/{mediaId}/custom-web/test-connection` | POST | Tes custom web | [x] |
| `/v1/partner/media/{mediaId}/dewan-pers-document` | POST | Upload dokumen Dewan Pers | [x] |
| `/v1/partner/media/{mediaId}/dewan-pers-verification` | POST | Ajukan verifikasi | [x] |
| `/v1/partner/media/{mediaId}/domain-verification` | GET, POST | Verifikasi kepemilikan domain | [x] |
| `/v1/partner/media/{mediaId}/negotiation` | GET, POST | Negosiasi harga | [x] |
| `/v1/partner/media/{mediaId}/seo` | GET, POST | Metrik SEO media | [x] |
| `/v1/partner/media/{mediaId}/similarweb` | GET, POST | Data SimilarWeb | [x] |
| `/v1/partner/media/{mediaId}/submit` | POST | Submit media untuk review | [x] |
| `/v1/partner/media/{mediaId}/test-publishing-api` | POST | Tes Tulisio publishing API | [x] |
| `/v1/partner/cms-connections` | GET, POST | Koneksi CMS partner | [x] |
| `/v1/partner/cms-connections/{connectionId}` | PUT, DELETE | Ubah/hapus koneksi | [x] |
| `/v1/partner/cms-connections/{connectionId}/test` | POST | Tes koneksi | [x] |
| `/v1/partner/crawler` | GET, POST | Status/registrasi crawler | [x] |
| `/v1/partner/documents` | GET, POST | Dokumen partner | [x] |
| `/v1/partner/documents/{id}` | DELETE | Hapus dokumen | [x] |
| `/v1/partner/earnings` | GET | Penghasilan mitra | [x] |
| `/v1/partner/gsc` | GET, POST | Koneksi & data GSC | [x] |
| `/v1/partner/gsc/connect` | GET | Mulai OAuth GSC | [x] |
| `/v1/partner/gsc/properties` | GET, POST | Pilih properti GSC | [x] |
| `/v1/partner/gsc/snapshots` | POST | Ambil snapshot GSC | [x] |
| `/v1/partner/link-monitor` | GET | Monitor link mitra | [x] |
| `/v1/partner/notification-preferences` | GET, PUT | Preferensi notifikasi | [x] |
| `/v1/partner/notification-webhooks` | GET, POST | Webhook mitra | [x] |
| `/v1/partner/notification-webhooks/{webhookId}` | PUT, DELETE | Ubah/hapus webhook | [x] |
| `/v1/partner/notifications` | GET, POST | Notifikasi mitra | [x] |
| `/v1/partner/notifications/{notificationId}/read` | POST | Tandai dibaca | [x] |
| `/v1/partner/publications` | GET | Publikasi masuk | [x] |
| `/v1/partner/publications/{publicationId}` | GET | Detail publikasi | [x] |
| `/v1/partner/publications/{publicationId}/approve` | POST | Approve publikasi | [x] |
| `/v1/partner/publications/{publicationId}/draft` | GET, PUT | Draft artikel (partner edit) | [x] |
| `/v1/partner/publications/{publicationId}/draft/image` | POST | Upload gambar draft | [x] |
| `/v1/partner/publications/{publicationId}/reject` | POST | Tolak publikasi | [x] |
| `/v1/partner/publications/{publicationId}/rewrite` | POST | Rewrite artikel | [x] |
| `/v1/partner/publications/{publicationId}/url` | POST | Isi URL hasil publish | [x] |
| `/v1/partner/publishing-api/docs` | GET | Docs publishing API | [x] |
| `/v1/partner/publishing-api-keys` | GET, POST | Kelola API key | [x] |
| `/v1/partner/publishing-api-keys/{keyId}` | DELETE, POST | Hapus/rotate key | [x] |
| `/v1/partner/referrals` | GET | Data referral | [x] |
| `/v1/partner/rss-feeds` | GET | RSS feed mitra | [x] |
| `/v1/partner/rss-feeds/{feedId}` | DELETE | Hapus feed | [x] |
| `/v1/partner/rss-feeds/{feedId}/rotate` | POST | Rotasi token feed | [x] |
| `/v1/partner/rss-feeds/{feedId}/test` | POST | Tes feed | [x] |
| `/v1/partner/rss-feeds/{feedId}/verify` | POST | Verifikasi feed | [x] |
| `/v1/partner/seo/check` | GET, POST | SEO tool check | [x] |
| `/v1/partner/seo/speed-audit` | GET, POST | Audit PageSpeed | [x] |
| `/v1/partner/settlements` | GET, POST | Settlement & request | [x] |
| `/v1/partner/vip/brand` | POST | Set brand VIP | [x] |
| `/v1/partner/vip/domains` | GET | Domain VIP | [x] |
| `/v1/partner/vip/wallet` | GET | Wallet VIP | [x] |

## 5. `/v1/payments` (4) 🔑/🌐 — fase 8

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/payments` | GET, POST | List/buat transaksi pembayaran | [x] |
| `/v1/payments/gateway/active` | GET | Gateway aktif | [x] |
| `/v1/payments/status/{transactionId}` | GET | Polling status pembayaran | [x] |
| `/v1/payments/webhooks/{provider}` | POST | Webhook gateway (Xendit/Duitku/Midtrans/Tripay/SumoPod/Sandbox) | [x] |

## 6. `/v1/platform` (173) 🛡️ — fase 5, 8–11

### 6.1 Admins (3)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/platform/admins` | GET, POST | List/tambah admin | [x] |
| `/v1/platform/admins/me` | GET | Profil admin | [x] |
| `/v1/platform/admins/{adminId}` | PATCH, DELETE | Ubah/hapus admin | [x] |

### 6.2 AI Gateway (7)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/platform/ai/configuration` | GET, PUT | Config provider (sealed key) | [x] |
| `/v1/platform/ai/models` | GET | Daftar model + knowledge | [x] |
| `/v1/platform/ai/models/{modelId}/knowledge` | GET, PUT, DELETE | Knowledge per model | [x] |
| `/v1/platform/ai/probe` | POST | Probe koneksi provider | [x] |
| `/v1/platform/ai/status` | GET | Status/circuit breaker | [x] |
| `/v1/platform/ai/test-generation` | POST | Tes generasi | [x] |
| `/v1/platform/ai/usage` | GET | Usage ledger | [x] |

### 6.3 Media admin (18)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/platform/media` | GET, POST | List/tambah media | [x] |
| `/v1/platform/media/{mediaId}` | GET, PUT, DELETE | CRUD media | [x] |
| `/v1/platform/media/{mediaId}/audit` | GET | Audit media | [x] |
| `/v1/platform/media/{mediaId}/recompute-health` | POST | Hitung ulang health | [x] |
| `/v1/platform/media/{mediaId}/review` | POST | Review/approve media | [x] |
| `/v1/platform/media/{mediaId}/seo` | GET, POST | Metrik SEO | [x] |
| `/v1/platform/media/{mediaId}/similarweb` | GET, POST | SimilarWeb | [x] |
| `/v1/platform/media/archive-all` | POST | Arsip massal | [x] |
| `/v1/platform/media/archive-impact` | GET | Dampak arsip | [x] |
| `/v1/platform/media/bulk` | POST, DELETE | Bulk upsert/hapus media | [x] |
| `/v1/platform/media/dewan-pers-reviews` | GET | Review Dewan Pers | [x] |
| `/v1/platform/media/dewan-pers-reviews/{reviewId}` | POST | Putusan review | [x] |
| `/v1/platform/media/intelligence` | GET | Media intelligence | [x] |
| `/v1/platform/media/recompute-health` | POST | Recompute massal | [x] |
| `/v1/platform/media/restore-archived` | POST | Pulihkan arsip | [x] |
| `/v1/platform/media/seo-bulk` | POST | Refresh SEO massal | [x] |
| `/v1/platform/media/similarweb-bulk` | POST | SimilarWeb massal | [x] |
| `/v1/platform/media/sync-credit-cost` | POST | Sinkron credit_cost | [x] |

### 6.4 Media packages, standards, summaries (6)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/platform/media-packages` | GET, POST | Paket media | [x] |
| `/v1/platform/media-packages/{packageId}` | PUT, DELETE | Ubah/hapus paket | [x] |
| `/v1/platform/media-standards` | GET, POST | Standar konten media | [x] |
| `/v1/platform/media-standards/{mediaId}` | GET, PUT, DELETE | CRUD standar | [x] |
| `/v1/platform/media-summaries` | GET | Rangkuman media | [x] |
| `/v1/platform/media-summaries/{mediaId}/regenerate` | POST | Regenerate rangkuman | [x] |

### 6.5 Operations (32) — fase 11

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/platform/operations/access` | GET | Matriks akses | [x] |
| `/v1/platform/operations/audit` | GET | Audit ringkas | [x] |
| `/v1/platform/operations/audit-log` | GET | Log audit | [x] |
| `/v1/platform/operations/brands` | GET | Semua brand | [x] |
| `/v1/platform/operations/brands/{brandId}/campaigns` | GET | Campaign per brand | [x] |
| `/v1/platform/operations/brands/{brandId}/connections` | GET, POST | Koneksi CMS brand | [x] |
| `/v1/platform/operations/brands/{brandId}/connections/{connectionId}` | DELETE | Hapus koneksi | [x] |
| `/v1/platform/operations/brands/{brandId}/connections/{connectionId}/test` | POST | Tes koneksi | [x] |
| `/v1/platform/operations/brands/{brandId}/content-settings` | GET, PUT | Setting konten brand | [x] |
| `/v1/platform/operations/credit-adjustment` | POST | Penyesuaian saldo TC | [x] |
| `/v1/platform/operations/credit-reports` | GET | Laporan topup | [x] |
| `/v1/platform/operations/credit-reports/{reportId}/review` | POST | Review laporan topup | [x] |
| `/v1/platform/operations/dead-letters` | GET | Job dead-letter | [x] |
| `/v1/platform/operations/dead-letters/{id}/retry` | POST | Retry dead-letter | [x] |
| `/v1/platform/operations/errors` | GET | Error terbaru | [x] |
| `/v1/platform/operations/generation-switch` | PUT | Kill switch generasi | [x] |
| `/v1/platform/operations/notifications` | GET | Notifikasi sistem | [x] |
| `/v1/platform/operations/publications` | GET | Semua publikasi | [x] |
| `/v1/platform/operations/publications/{publicationId}` | GET | Detail publikasi | [x] |
| `/v1/platform/operations/publications/{publicationId}/retry` | POST | Retry publikasi | [x] |
| `/v1/platform/operations/running-batches` | GET | Batch berjalan | [x] |
| `/v1/platform/operations/summary` | GET | Ringkasan ops | [x] |
| `/v1/platform/operations/users` | GET, POST | Kelola user | [x] |
| `/v1/platform/operations/users/{userId}/brands` | GET | Brand milik user | [x] |
| `/v1/platform/operations/users/{userId}/daily-quota` | GET, PUT | Kuota harian | [x] |
| `/v1/platform/operations/users/{userId}/generate-quota` | GET, PUT | Kuota generasi | [x] |
| `/v1/platform/operations/users/{userId}/password` | PUT | Reset password | [x] |
| `/v1/platform/operations/users/{userId}/role` | PUT | Ubah role | [x] |
| `/v1/platform/operations/users/{userId}/status` | PUT | Aktif/nonaktif | [x] |
| `/v1/platform/operations/variant-batches` | GET | Batch varian | [x] |
| `/v1/platform/operations/variant-batches/{batchId}` | GET | Detail batch | [x] |
| `/v1/platform/operations/variant-batches/{batchId}/retry` | POST | Retry batch | [x] |

### 6.6 Partners admin (8)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/platform/partners` | GET, POST | Kelola mitra | [x] |
| `/v1/platform/partners/{partnerId}` | GET, PUT, DELETE | CRUD mitra | [x] |
| `/v1/platform/partners/{partnerId}/vip` | GET, PUT | Status VIP mitra | [x] |
| `/v1/platform/partners/{partnerId}/connections` | GET, POST | Koneksi CMS mitra | [x] |
| `/v1/platform/partners/{partnerId}/connections/{connectionId}` | PUT, DELETE | Ubah/hapus | [x] |
| `/v1/platform/partners/{partnerId}/connections/{connectionId}/test` | POST | Tes | [x] |
| `/v1/platform/partners/{partnerId}/documents` | GET | Dokumen mitra | [x] |
| `/v1/platform/partners/{partnerId}/documents/{id}` | PUT | Update dokumen | [x] |

### 6.7 Payments admin (4)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/platform/payments/gateways` | GET, POST | Config gateway (sealed) | [x] |
| `/v1/platform/payments/gateways/{gatewayId}/activate` | POST | Aktivasi gateway | [x] |
| `/v1/platform/payments/gateways/{gatewayId}/test` | POST | Tes gateway | [x] |
| `/v1/platform/payments/transactions` | GET | Semua transaksi | [x] |

### 6.8 PR admin (9)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/platform/pr/analysis-history` | GET | Riwayat analisis | [x] |
| `/v1/platform/pr/brand-standards` | GET | Standar PR brand | [x] |
| `/v1/platform/pr/brand-standards/{brandId}` | PUT | Update standar | [x] |
| `/v1/platform/pr/config` | GET, PUT | Config PR global | [x] |
| `/v1/platform/pr/media-rules` | GET, PUT | Aturan media PR | [x] |
| `/v1/platform/pr/recompute-scores` | POST | Recompute skor | [x] |
| `/v1/platform/pr/rerun-analysis` | POST | Rerun analisis | [x] |
| `/v1/platform/pr/scoring-profiles` | GET, POST | Profil skor | [x] |
| `/v1/platform/pr/scoring-profiles/{profileId}/activate` | POST | Aktivasi profil | [x] |

### 6.9 Settings (8)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/platform/settings/email` | GET, PUT | Config SMTP | [x] |
| `/v1/platform/settings/email/deliveries` | GET | Log email | [x] |
| `/v1/platform/settings/email/test-connection` | POST | Tes SMTP | [x] |
| `/v1/platform/settings/email/test-send` | POST | Kirim tes | [x] |
| `/v1/platform/settings/pagespeed` | GET, PUT | API key PageSpeed | [x] |
| `/v1/platform/settings/pagespeed/test` | POST | Tes PageSpeed | [x] |
| `/v1/platform/settings/turnstile` | GET, PUT | Config Turnstile | [x] |
| `/v1/platform/settings/turnstile/test` | POST | Tes Turnstile | [x] |

### 6.10 Taut platform (16) + agents (4)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/platform/taut/ai-usage` | GET | Usage AI taut | [x] |
| `/v1/platform/taut/angles/{angleId}` | GET, PATCH, DELETE | CRUD angle | [x] |
| `/v1/platform/taut/angles/{angleId}/test` | POST | Tes angle | [x] |
| `/v1/platform/taut/angles/{angleId}/versions` | GET, POST | Versi angle | [x] |
| `/v1/platform/taut/angles/{angleId}/versions/{versionId}` | GET, PATCH, DELETE | CRUD versi | [x] |
| `/v1/platform/taut/angles/{angleId}/versions/{versionId}/activate` | POST | Aktivasi versi | [x] |
| `/v1/platform/taut/base-prompt` | GET, POST | Base prompt agent | [x] |
| `/v1/platform/taut/base-prompt/{versionId}` | GET, PATCH, DELETE | CRUD versi prompt | [x] |
| `/v1/platform/taut/base-prompt/{versionId}/activate` | POST | Aktivasi versi | [x] |
| `/v1/platform/taut/channels` | GET, POST | Kanal media | [x] |
| `/v1/platform/taut/channels/{channelId}` | PATCH, DELETE | Ubah/hapus kanal | [x] |
| `/v1/platform/taut/channels/{channelId}/angles` | GET, POST | Angle per kanal | [x] |
| `/v1/platform/taut/runs` | GET | Riwayat run agent | [x] |
| `/v1/platform/taut/runs/{campaignId}` | GET | Run per campaign | [x] |
| `/v1/platform/taut/runs/cleanup` | POST | Bersihkan run | [x] |
| `/v1/platform/taut/runs/retry/{runId}` | POST | Retry run | [x] |
| `/v1/platform/taut-agents` | GET | Daftar agent | [x] |
| `/v1/platform/taut-agents/{agent}` | GET, PUT | Config agent | [x] |
| `/v1/platform/taut-agents/{agent}/test` | POST | Tes agent | [x] |
| `/v1/platform/taut-agent-logs` | GET | Log agent | [x] |

### 6.11 Platform lainnya (70) — fase 10–11

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/platform/auto-replace` | GET, PUT | Config auto-replace | [x] |
| `/v1/platform/bug-reports` | GET | Laporan bug | [x] |
| `/v1/platform/bug-reports/{reportId}` | GET, PATCH | Detail/penanganan bug | [x] |
| `/v1/platform/config/template-url` | GET, PUT | URL template import | [x] |
| `/v1/platform/content-intake-types` | GET, POST | Tipe intake | [x] |
| `/v1/platform/content-intake-types/{code}` | GET, PUT, DELETE | CRUD tipe intake | [x] |
| `/v1/platform/content-intake-types/{code}/standards` | GET, PUT | Standar konten tipe | [x] |
| `/v1/platform/content-intake-types/{code}/standards/versions/{versionId}/activate` | POST | Aktivasi versi standar | [x] |
| `/v1/platform/crawlhub` | GET, PUT, POST | Config Crawlhub | [x] |
| `/v1/platform/crawlhub/targets` | GET, POST | Target crawl | [x] |
| `/v1/platform/credits` | GET, PUT, POST | Config TC (tc_config) | [x] |
| `/v1/platform/domain-diagnostics` | GET | Diagnostik domain | [x] |
| `/v1/platform/domain-diagnostics/recheck` | POST | Recheck domain | [x] |
| `/v1/platform/ga4-configs` | GET, POST | Config GA4 | [x] |
| `/v1/platform/ga4-configs/{configId}` | GET, PUT, DELETE | CRUD GA4 | [x] |
| `/v1/platform/gatekeeper/logs` | GET | Log gatekeeper | [x] |
| `/v1/platform/gatekeeper/stats` | GET | Statistik gatekeeper | [x] |
| `/v1/platform/gsc-config` | GET, PUT | Config GSC platform | [x] |
| `/v1/platform/invoices` | GET, POST | Kelola invoice | [x] |
| `/v1/platform/invoices/{invoiceId}` | GET, PUT, DELETE | CRUD invoice | [x] |
| `/v1/platform/knowledge-bases` | GET, POST | Knowledge base rules | [x] |
| `/v1/platform/knowledge-bases/{knowledgeBaseId}` | GET, PATCH | Detail KB | [x] |
| `/v1/platform/knowledge-bases/{knowledgeBaseId}/versions` | GET, POST | Versi KB | [x] |
| `/v1/platform/knowledge-bases/{knowledgeBaseId}/versions/{versionId}` | GET, PATCH, DELETE | CRUD versi KB | [x] |
| `/v1/platform/knowledge-bases/{knowledgeBaseId}/versions/{versionId}/activate` | POST | Aktivasi versi KB | [x] |
| `/v1/platform/knowledge-bases/{knowledgeBaseId}/versions/{versionId}/rules` | POST | Aturan versi KB | [x] |
| `/v1/platform/link-clicks` | GET | Klik tracking | [x] |
| `/v1/platform/link-monitor` | GET | Monitor link global | [x] |
| `/v1/platform/link-monitor/check` | POST | Trigger cek | [x] |
| `/v1/platform/link-monitor/replace` | POST | Ganti link | [x] |
| `/v1/platform/moderations` | GET | Moderasi konten | [x] |
| `/v1/platform/moderations/{auditId}/review` | POST | Keputusan review | [x] |
| `/v1/platform/oauth/google` | GET, PUT | OAuth Google platform | [x] |
| `/v1/platform/orders` | GET | Order code campaign | [x] |
| `/v1/platform/packages` | GET, POST | Paket kredit | [x] |
| `/v1/platform/packages/{packageId}` | GET, PUT, DELETE | CRUD paket | [x] |
| `/v1/platform/pricing` | GET, PUT | Config pricing | [x] |
| `/v1/platform/referral-audit` | GET | Audit referral | [x] |
| `/v1/platform/referrals` | GET | Referral global | [x] |
| `/v1/platform/referred-domains` | GET, PUT | Domain referral | [x] |
| `/v1/platform/revenue-shares` | GET, POST | Aturan revenue share | [x] |
| `/v1/platform/revenue-shares/{ruleId}` | GET, PUT, DELETE | CRUD aturan | [x] |
| `/v1/platform/rss-feeds` | GET, POST | RSS feed global | [x] |
| `/v1/platform/rss-feeds/{feedId}` | GET, PUT, DELETE, POST | CRUD feed | [x] |
| `/v1/platform/seo/config` | GET, PUT | Config SEO platform | [x] |
| `/v1/platform/seo/diagnose` | GET | Diagnosa SEO | [x] |
| `/v1/platform/server-config` | GET, PUT | Server config (IP dll.) | [x] |
| `/v1/platform/settlements` | GET, POST | Settlement | [x] |
| `/v1/platform/settlements/config` | GET, PUT | Config settlement | [x] |
| `/v1/platform/settlements/{settlementId}` | GET, PUT, DELETE | CRUD settlement | [x] |
| `/v1/platform/settlements/{settlementId}/receipt` | POST | Upload bukti | [x] |
| `/v1/platform/short-link-config` | GET, PUT | Config short link | [x] |
| `/v1/platform/vip-partners` | GET | Daftar VIP partner | [x] |
| `/v1/platform/vip-partners/{partnerId}/domains` | GET | Domain VIP | [x] |
| `/v1/platform/wa/config` | GET, PUT | Config WA Fonnte | [x] |
| `/v1/platform/wa/deliveries` | GET | Log WA | [x] |
| `/v1/platform/wa/templates` | GET, PUT, DELETE | Template WA | [x] |
| `/v1/platform/wa/test` | POST | Tes kirim WA | [x] |

## 7. `/v1` lainnya (8)

| Endpoint | Method | Tujuan | ✔ |
|---|---|---|---|
| `/v1/bug-reports` | POST | Kirim laporan bug (semua role) | [x] |
| `/v1/campaign-reports/{token}` | GET | Laporan publik via share token | [x] |
| `/v1/internal/cron/scheduler` | GET, POST | Trigger scheduler (guard CRON_SECRET) | [x] |
| `/v1/internal/jobs` | POST | Pull 1 job (guard JOB_RUNNER_KEY) | [x] |
| `/v1/me/notification-preferences` | GET, PUT | Preferensi notifikasi user | [x] |
| `/v1/publishing/callbacks` | POST | Callback publishing (HMAC, Bearer api_key) | [x] |
| `/v1/publishing/feed.json` | GET | Feed JSON partner | [x] |
| `/v1/tracking/events` | POST | Catat event tracking artikel | [x] |

---

## Rekap

| Grup | Jumlah route |
|---|---|
| Non-/v1 | 7 |
| /v1/auth | 18 |
| /v1/brands | 133 |
| /v1/partner | 63 |
| /v1/payments | 4 |
| /v1/platform | 173 |
| /v1 lainnya | 8 |
| **Total** | **406** |
