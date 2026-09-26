# Alur Mitra (Media Partner) — Tulisio

Dokumen ini menjelaskan alur dari sudut pandang **mitra / media partner**,
sesuai implementasi Phase 1–9 + alur verifikasi RSS. Mengacu pada
`Tulisio_Development_Foundation.md` (§3, §13, §35).

## 0. Akun & akses

- **Registrasi mitra**: di `/login` → tab **Daftar** → pilih role **Mitra** → nama perusahaan, email, password. Akun langsung aktif dan **langsung masuk** ke portal mitra.
- **URL portal memakai public id**: `/mitra/{public-id}` (contoh `smoke-media-partner-32959`) — bukan UUID.
- **Media Anda**: portal mitra → menu **Media** — CRUD penuh media milik partner sendiri (tambah/edit/hapus); `partner_id` ditentukan dari sesi, media partner lain tidak bisa diakses.

## 1. Mental model mitra

> "Saya terima permintaan publikasi → setujui/tolak → artikel terbit di media
> saya → saya dapat bagian (earning). RSS yang saya pasang harus terverifikasi.
> Kalau media saya ber-integrasi WordPress, artikel dikirim lewat koneksi
> WordPress milik saya sendiri — bukan koneksi brand klien."

## 2. Alur lengkap

```
1. Login portal mitra → /mitra/{partnerId}/... (redirect otomatis dari /mitra)
2. Publications — antrean permintaan dari klien
   └─ Detail → dialog detail publikasi: info artikel (slug, deskripsi),
        harga, jadwal/terbit, catatan klien, URL terbit, dan riwayat
        percobaan pengiriman (publication_attempts)
   └─ Setujui → artikel langsung masuk queue eksekusi (auto publish/feed)
   └─ Tolak → wajib alasan → status partner_rejected (klien memutuskan refund)
3. Media — lihat media milik Anda (harga, integrasi, status)
4. RSS Feeds — channel feed per media (pull mechanism, token rahasia)
   └─ Salin URL feed → pasang di website:
        <link rel="alternate" type="application/rss+xml" href="https://tulisio.id/feed/partner/{token}" />
   └─ Klik "Sudah dipasang" → Tulisio memverifikasi:
        - mengambil homepage website media Anda
        - mencari URL feed di HTML-nya
        - jika ketemu → status Terpasang (verified); jika tidak → Gagal + alasan
   └─ Setelah terverifikasi, artikel klien otomatis muncul di feed Anda
5. Import callback status (foundation §13) — importer CMS Anda melapor balik:
   └─ POST /feed/partner/{token}/import  { guid, status: "imported" | "published" }
   └─ status per publikasi: Feed Published → Partner Imported → Partner Published
   └─ Tampil sebagai badge di Publications portal mitra
6. Koneksi — kelola koneksi WordPress / Web Custom milik Anda:
   └─ Tambah koneksi (nama, URL situs, username, application password)
   └─ Test → Tulisio verifikasi kredensial ke situs Anda
   └─ Publikasi ke media Anda ber-integrasi WordPress dikirim lewat koneksi ini
   └─ (bukan koneksi brand klien); tanpa koneksi, publikasi gagal no_connection
7. Notifikasi — lonceng di pojok kanan atas (polling 60 detik):
   └─ "Publikasi menunggu persetujuan" saat ada artikel baru untuk Anda
   └─ "Artikel terbit di media Anda" saat artikel sudah terbit
   └─ Klik notifikasi → langsung ke halaman Publications; Tandai dibaca
8. Earnings — pending per publikasi terbit → settled saat settlement disetujui
9. Settlements — payout per periode
```

## 3. Detail RSS (foundation §13)

- RSS adalah **pull mechanism**: Tulisio merender feed per partner/media di
  balik secure token; CMS/aggregator Anda menariknya.
- Token bersifat rahasia (otentikasi) — jangan dibagikan. Admin dapat
  **rotasi token** atau **revoke** feed.
- Feed **rate limited** (30 pull/menit/feed) dan **access log** tercatat.
- Status pemasangan: `pending` (belum dipasang) → `verified` (terpasang &
  terverifikasi) / `failed` (gagal + alasan). Feed `failed` bisa dicoba lagi
  kapan saja dengan tombol "Sudah dipasang".
- **Import callback**: setelah feed terverifikasi, importer CMS mitra memanggil
  `POST /feed/partner/{token}/import` dengan `guid` item + status
  `imported`/`published` → tercatat di `publications.rss_import_status`
  (Feed Published → Partner Imported → Partner Published). Endpoint publik,
  token sebagai otentikasi, rate limited (60/menit), access log tercatat.
- **Rotasi token & cabut dari portal mitra**: tombol "Rotasi token"
  (`POST /v1/partner/rss-feeds/{id}/rotate`) mengganti token seketika dan
  mereset pemasangan ke `pending` (harus pasang ulang); tombol "Cabut"
  (`DELETE /v1/partner/rss-feeds/{id}`) me-revoke feed tanpa menghapus baris
  (audit log `rss_feed.token_rotated` / `rss_feed.revoked`).
- Jika verifikasi gagal, tim Tulisio melihatnya di **Platform → Error Reports**
  dan dapat membantu.

## 4. Koneksi CMS milik mitra (WordPress / Web Custom)

- Menu **Koneksi** di portal mitra (`/mitra/{partnerId}/connections`): partner
  menambah, menguji, dan mencabut koneksi sendiri — `GET/POST
  /v1/partner/cms-connections`, `PUT/DELETE /{id}`, `POST /{id}/test`.
- Koneksi tersimpan di `cms_connections` dengan `partner_id` (bukan
  `brand_id`); satu partner maksimal **satu koneksi WordPress aktif**.
- **Executor**: publikasi ke media partner ber-`integration_type =
  wordpress`/`webhook` di-resolve ke koneksi aktif **mitra** — bukan koneksi
  brand klien (foundation §3 literal). Tanpa koneksi mitra, publikasi gagal
  `no_connection` dan muncul di **Platform → Error Reports**.
- **`media.publish_defaults` di-resolve terhadap situs mitra**: author_id,
  kategori/tag (search atau auto-create), dan upload featured image dilakukan
  ke `connection.base_url` milik mitra; hasil term/media ID disimpan di
  `publication_jobs.payload_snapshot.resolved` untuk dipakai ulang saat retry
  (tidak re-query/re-upload).
- **Halaman Media menampilkan status koneksi**: media ber-integrasi
  WordPress/webhook diberi badge "Koneksi aktif" (ada koneksi aktif milik
  mitra untuk provider itu) atau "Perlu koneksi" — `GET /v1/partner/media`
  kini menyertakan `has_connection`.
- **Payload webhook menyertakan identitas mitra**: untuk media partner
  ber-integrasi webhook, payload `content.delivered` ditambah blok `media`
  (id, name, website_url, partner_id) dan `partner` (id, name) — penerima
  tahu artikel dikirim melalui media/partner mana. Retry tetap memakai
  koneksi mitra yang sama (resolve ulang ke `cms_connections.partner_id`).
- Admin tetap bisa melihat/menguji/mencabut koneksi mitra di
  **Platform → Integrations** (mode "Partner (mitra)").

## 5. Pembagian pendapatan (foundation §18)

- Default revenue share: **Tulisio 65% / Mitra 35%** dari `client_price`.
- Earning dicatat otomatis saat publikasi berstatus **published** (status
  pending), lalu diagregasi menjadi **settlement** oleh admin.

## 6. Notifikasi (in-app)

- Tabel `notifications` (migration 012): per user (partner user atau brand
  member), tipe `publication.*`, `read_at` untuk status baca.
- **Mitra** menerima: `waiting_partner` (ada artikel menunggu persetujuan)
  dan `published` (artikel terbit di media-nya).
- **Klien** menerima: `partner_approved`, `partner_rejected` (beserta alasan),
  dan `published` — semua dengan deep-link ke halaman Publications.
- API: `GET/POST /v1/partner/notifications` + `POST /{id}/read`
  (list mendukung filter `status=all|read|unread` + `offset`);
  klien: `GET/POST /v1/brands/{brandId}/notifications` + `POST /{id}/read`.
- **Halaman riwayat** (`/mitra/{partnerId}/notifications`): filter
  Semua/Belum dibaca/Dibaca, tandai semua dibaca, dan **kelola webhook
  endpoint** (tambah URL + secret opsional, aktif/nonaktif, hapus).
- **Webhook callback** (migration 013): endpoint milik Anda menerima POST
  `{ event: "notification", notification: { type, title, body, link,
  publication_id } }` — ditandatangani HMAC-SHA256 (`x-tulisio-signature`,
  `x-tulisio-event`, `x-tulisio-event-id`, `x-tulisio-timestamp`) bila secret
  diisi. Best-effort dengan timeout 10 detik.
- Detail publikasi mitra: `GET /v1/partner/publications/{id}` (dokumen + media
  + timeline `publication_attempts` + job terakhir).
- Notifikasi **best-effort**: kegagalan menulis notifikasi tidak pernah
  menggagalkan persetujuan/penolakan/penerbitan artikel. Event yang sama
  dikirim via **email** (template HTML, `lib/mail/service.ts` via SMTP) bila
  `SMTP_HOST` dikonfigurasi; no-op jika tidak.

## 7. Aturan penting

- Mitra hanya melihat publikasi untuk **media miliknya** (tenant scoping).
- Persetujuan mitra hanya untuk media ber-`partner_id`; media non-partner
  tidak melewati gate ini.
- Menolak publikasi **tidak otomatis refund** — refund terjadi saat klien
  membatalkan publikasi yang ditolak (transisi `partner_rejected → cancelled`).
