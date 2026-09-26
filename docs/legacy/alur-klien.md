# Alur Klien — Tulisio (PR & Media Distribution)

Dokumen ini menjelaskan alur dari sudut pandang **klien** (pemilik brand),
sesuai implementasi Phase 1–9 + penyempurnaan berikutnya. Mengacu pada
`Tulisio_Development_Foundation.md` (§3, §6, §7, §17, §35).

## 0. Akun & akses

- **Registrasi**: di `/login` → tab **Daftar** → pilih role **Klien** → nama, email, password. Akun langsung aktif (auto-approve) dan langsung masuk.
- **URL brand memakai public id**: `/klien/{public-id}` (contoh `giovani-barbershop-92520`) — bukan UUID. Brand baru otomatis mendapat public id `{slug}-{5digit}`.
- **Login**: satu form di `/login`; backend menentukan dashboard (klien → `/klien`, mitra → `/mitra`, admin → `/admin`).

## 1. Mental model klien

> "Saya kirim artikel → otomatis terdistribusi ke media → tinggal lihat laporan."

Klien **tidak** mengatur integrasi (WordPress / Web Custom / RSS). Koneksi
dikonfigurasi oleh tim Tulisio (halaman **Platform → Integrations**). Yang
klien lihat hanyalah: artikel, kampanye, publikasi, status, dan laporan.

## 2. Alur lengkap

```
0. Content Intake — halaman "Buat konten" (`/klien/{brand}/generate`)
   └─ pilih apa yang Anda miliki: BAHAN / ARTIKEL / keduanya / belum punya
   └─ pilih jenis konten (Siaran Pers, Event, CSR, Launching, Personal Branding)
   └─ isi kuesioner dinamis (bahan mentah) dan/atau tempel/unggah artikel existing
   └─ Content Readiness: skor 0-100 + daftar field yang kurang (bobot dikelola Admin)
   └─ skor < 80 atau belum punya bahan → "Minta Rekomendasi Tulisio"
        └─ gunakan rekomendasi / edit / coba lagi
   └─ "Lanjut ke AI Writer" → artikel dibuat dari bahan Anda (bahan + artikel
      existing dipakai sebagai dasar penulisan, artikel asli tidak diubah)
1. Buat Kampanye (opsional, disarankan)
   └─ nama, kuota artikel, budget, status draft → Aktifkan
2. Buat Artikel (AI Writer / Generate / Knowledge)
   └─ brief → draft outline → setujui → worker menulis → review di editor
   └─ (masih bisa langsung isi brief tanpa intake lewat tombol "Langsung isi brief")
3. Artikel Approved (siap didistribusikan)
4. Publikasi (Distribusi Media → Publications, atau package dari detail kampanye)
   └─ pilih artikel approved + media (bisa banyak sekaligus = package publish)
   └─ kampanye opsional, jadwal opsional
   └─ kredit di-reserve otomatis
5. Status berjalan otomatis:
   └─ Media non-partner → antre → worker → published (atau failed)
   └─ Media partner → waiting_partner → mitra setujui/tolak
        ├─ disetujui → antre → worker → published
        └─ ditolak → partner_rejected → klien batalkan → kredit refund
6. Tracking link otomatis per publikasi (/t/{code}) → klik tercatat
7. Notifikasi — lonceng di pojok kanan atas (polling 60 detik):
   └─ "Publikasi disetujui mitra" saat mitra menyetujui artikel Anda
   └─ "Publikasi ditolak mitra" (dengan alasan)
   └─ "Artikel terbit" saat artikel berhasil terbit di media
   └─ Klik notifikasi → langsung ke halaman Publications
   └─ Notifikasi yang sama juga dikirim via **email** (template HTML) bila
      SMTP dikonfigurasi tim Tulisio, dan via **webhook** ke endpoint Anda
      (HMAC-signed) bila didaftarkan di halaman Notifikasi
8. Riwayat notifikasi — `/klien/{brandId}/notifications`:
   └─ filter Semua / Belum dibaca / Dibaca, tandai semua dibaca
   └─ kelola webhook endpoint (tambah URL + secret opsional, aktif/hapus)
9. Detail publikasi — tombol Detail di halaman Publications:
   └─ slug + deskripsi artikel, harga, kategori media, URL terbit
   └─ jadwal/terbit, catatan, alasan pembatalan, error job
   └─ riwayat percobaan pengiriman (publication_attempts)
   └─ aksi langsung: batalkan (refund) atau retry dari dialog
9. Laporan (Analytics / Reports)
   └─ jumlah terbit, views, klik, unique visitors, CTR
   └─ estimated media value (Σ harga media terbit) & biaya kampanye
   └─ breakdown per kampanye, filter tanggal
```

## 3. Halaman yang dilihat klien

URL: `/klien/{brandId}/...` (brand unik per klien; jika user punya >1 brand,
dropdown pilih brand muncul di sidebar).

| Halaman | Fungsi |
|---|---|
| Setup | Profil, Voice & Tone, Editorial Rules, Master Prompt, Knowledge (tanpa Integrations — dikelola Tulisio) |
| Generate | Buat artikel baru dari brief |
| Knowledge | Knowledge base / crawl sitemap |
| History | Riwayat job artikel |
| Distribusi Media → Overview | KPI 30 hari, publikasi & kampanye terbaru, saldo kredit |
| → Campaigns | Buat/aktifkan kampanye, package publish dari detail kampanye |
| → Publications | Buat publikasi, batalkan (refund), retry, lihat tracking link |
| → Media | Direktori media aktif (read-only) |
| → Credits | Saldo, paket, riwayat ledger |
| → Invoices | Daftar invoice |
| → Analytics | KPI + breakdown kampanye + seri harian |

## 4. Aturan penting

- Artikel yang bisa dipublikasikan harus **approved** (status dokumen approved).
- Publikasi ber-campaign memakai **kuota kampanye** + **kredit**; tanpa
  kampanye hanya kredit.
- **Pembatalan** hanya dari sisi klien (`partner_rejected` dan status lain yang
  bisa dibatalkan) → kredit dikembalikan (anti double-refund).
- **Retry** publikasi gagal tersedia di halaman Publications.
- Klien tidak pernah melihat error teknis delivery; jika ada kegagalan, tim
  Tulisio melihatnya di **Platform → Error Reports** dan menindaklanjuti.
- Notifikasi in-app per user (tabel `notifications`, migration 012): klien
  menerima `partner_approved` / `partner_rejected` / `published` dengan
  deep-link ke Publications. Best-effort — tidak pernah menggagalkan alur
  publikasi. Email follow-up dikirim ke alamat user bila `SMTP_HOST`
  dikonfigurasi (no-op jika tidak).

## 5. Istilah di laporan

- **Estimated Media Value** = Σ `client_price` publikasi berstatus published.
- **Campaign Cost** = Σ budget kampanye aktif dalam window laporan.
- CTR = klik / views. Unique visitors dihitung dari session unik.
