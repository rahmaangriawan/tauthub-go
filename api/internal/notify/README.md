# Paket notify

Satu sumber kebenaran untuk memutuskan kanal mana yang dipakai sebuah event
notifikasi. Paket ini **hanya memutuskan**, tidak mengirim: SMTP dan gateway
WhatsApp tetap dipanggil oleh pemilik jalur masing-masing.

## Kenapa paket ini ada

Sebelumnya keputusan "event ini kirim email?" dan "event ini kirim WA?" ditulis
sebagai `if` di banyak file pengirim. Menambah atau mematikan satu kanal berarti
menyunting beberapa tempat, dan tidak ada satu halaman yang bisa menjawab
"event apa saja yang menghabiskan biaya".

## Matriks tier

| Tier | Isi | In-app | Push | Email | WA |
|---|---|---|---|---|---|
| 0 | Keamanan dan akses | tidak | tidak | immediate | off |
| 1 | Uang | ya | ya | off | off |
| 2 | Tindakan bertenggat | ya | ya | hanya `reminder` dan `late_approval` | off |
| 3 | Informasi | ya | ya | off | off |

Tier 1 dan sebagian tier 2 sengaja `off` untuk email karena **belum ada
pengirimnya**. `settNotifyPartner` dan insert referral hanya menulis baris
in-app; event publikasi di `partner/shared.go` dan `brandflow/support.go` juga
tidak memanggil `mpEmailFollowUp`. Registry mencatat kanal yang benar-benar ada,
bukan niat yang belum diimplementasikan. Saat pengirim ditambahkan, cukup ubah
mode di `policy.go`; gate baca, dedupe, dan kuota otomatis berlaku.

Hanya empat jalur yang benar-benar mengirim email hari ini: `mpEmailFollowUp`
(media review, dewan pers, tiket), `schEmailFollowUp` (pengingat tenggat), dan
`SendTransactionalEmail` (password reset/berubah, undangan akun).

Tipe yang belum terdaftar jatuh ke tier 3. Arah default itu disengaja: menambah
event baru tidak akan diam-diam mengirim email berbayar.

Prefix yang dipetakan: `security` (tier 0), `withdrawal` dan `referral`
(tier 1). `publication` sengaja **tidak** dipetakan karena keluarganya campuran
antara tier 2 dan tier 3; tipe publication baru harus didaftarkan eksplisit.

## Urutan keputusan `ShouldSend`

1. Mode kanal di registry. `off` dan `digest` langsung menolak.
2. Kelengkapan kontak. Email butuh `users.email`; WA butuh `wa_number` **dan**
   `wa_consent`.
3. Gate baca in-app: kalau notifikasi bertipe sama sudah dibaca dalam jendela
   waktu, kanal berbayar dibatalkan.
4. Dedupe per event pada kanal itu.
5. Kuota per jam per user per kanal.

Setiap error database menghasilkan penolakan. Query yang rusak tidak boleh
berubah menjadi tagihan.

## Ambang yang bisa disesuaikan

Semua di `deliver.go`:

| Const | Nilai | Arti |
|---|---|---|
| `EmailReadWindow` | 24 jam | batas gate baca untuk email |
| `EmailDedupeWindow` | 24 jam | jeda minimum antar email untuk event sama |
| `EmailHourlyLimit` | 20 | email per user per jam |
| `WAReadWindow` | 6 jam | batas gate baca untuk WA |
| `WADedupeWindow` | 72 jam | jeda minimum antar WA untuk event sama |
| `WAHourlyLimit` | 5 | WA per user per jam |
| `QuotaWindow` | 1 jam | lebar bucket kuota |

## Checklist pemasangan

Delapan titik kirim yang harus memanggil `notify.ShouldSend` sebelum mengirim,
lengkap dengan isi `DedupeKey` yang benar:

| Titik | Kanal | `Type` | `DedupeKey` |
|---|---|---|---|
| `platform/media_support.go:2482` `mpEmailFollowUp` | email | tipe event | `<event>:<mediaID>` |
| `platform/media_support.go:2594` `mpWAFollowUp` | wa | tipe event | `<event>:<mediaID>` |
| `platform/tickets.go:313` (email tiket) | email | `ticket.replied` | `ticket.replied:<ticketID>:<replyID>` |
| `platform/tickets.go:315` (WA tiket) | wa | `ticket.replied` | `ticket.replied:<ticketID>:<replyID>` |
| `platform/dewan_pers.go:171` | email | `media.dewan_pers.*` | `<event>:<mediaID>` |
| `platform/media.go:882` | email | `media.approved` / `media.rejected` | `<event>:<mediaID>` |
| `platform/media.go:888` | wa | `media.approved` / `media.rejected` | `<event>:<mediaID>` |
| `brandflow/scheduler.go:537` `schEmailFollowUp` | email | `publication.reminder` | `publication.reminder:<publicationID>` |
| `platform/partner_wa_reminder.go:113` | email | `partner.wa_required` | `partner.wa_required:<partnerID>` |

Catatan penting untuk tiket: `notifyTicketPartner` (`tickets.go:283`) menyisipkan
balasan lewat CTE di `tickets.go:200-208` dan memanggil follow-up hanya dengan
`ticketID`. Id balasan tidak pernah diambil kembali, sehingga dua balasan admin
pada satu tiket akan bertabrakan di kunci dedupe dan balasan kedua hilang
selama 24 jam. CTE itu harus memakai `RETURNING id` dan id-nya diteruskan ke
`DedupeKey`.

## Yang belum dipasang

- Pemanggilan `ShouldSend` di delapan titik di atas.
- `EnqueueDigestJobs` belum dipanggil dari siklus scheduler.
- Trigger push (`175_push_notifications.sql`) masih mengirim untuk semua tipe;
  `ChannelPolicy.Push` baru dokumentasi sampai trigger itu menghormati policy.
