# Webhook bertanda tangan (API Web Custom)

> Kontrak publik untuk developer yang membuat penerima artikel Tauthub berbentuk webhook: nama event, header tanda tangan HMAC, aturan tanda tangan, dan contoh isi pesan. Kontrak ini terpisah dari tab Web Custom di Dashboard Mitra, yang memakai Publisher Protocol.

**Lokasi:** halaman dokumentasi API Web Custom di situs Tauthub, untuk developer
**Untuk:** developer website Mitra

## Gambaran singkat

Pada kontrak ini, Tauthub mengirim artikel ke satu alamat milik website Anda lewat permintaan `POST`. Setiap permintaan ditandatangani dengan HMAC-SHA256 memakai Shared Secret, supaya website Anda yakin permintaan itu benar-benar dari Tauthub.

Dashboard Mitra tidak punya layar untuk menyiapkan jenis ini. Tab **Web Custom** memakai kontrak lain, yaitu Publisher Protocol di [Integrasi Web Custom](03-web-custom.md). Di Daftar Media, media yang memakai kontrak webhook ini berlabel **Webhook** pada kolom platform. Bila Anda diminta membuat penerimanya atau memerlukan Endpoint URL dan Shared Secret, hubungi [Dukungan](../12-dukungan.md).

## Yang perlu disiapkan

- **Endpoint URL:** alamat penerima di website Anda yang dapat dibuka dari internet, misalnya `https://situs-media.example/api/tauthub/content`.
- **Shared Secret:** kunci rahasia bersama untuk menghitung tanda tangan. Pakai nilai acak yang panjang. Simpan di variabel lingkungan aplikasi Anda, jangan di kode atau repositori publik.

## Header pada setiap permintaan

| Header | Isi |
|---|---|
| `Content-Type` | `application/json`. Isi permintaan selalu JSON. |
| `x-tulisio-event` | Nama event: `content.delivered` atau `content.ping`. |
| `x-tulisio-event-id` | Pengenal unik event, berupa UUID. |
| `x-tulisio-timestamp` | Waktu kirim dalam format ISO-8601. |
| `x-tulisio-signature` | Tanda tangan, berbentuk `sha256=` diikuti huruf heksadesimal. |

## Event

### content.ping

Dikirim saat Tauthub menguji koneksi. Penerima membalas `{ "ok": true }` agar uji lulus.

```json
{
  "event": "content.ping",
  "event_id": "4f8c1b2e-3a7d-4f9e-9b1c-2d5e6f7a8b9c",
  "timestamp": "2026-11-08T09:00:00Z"
}
```

Hasil uji dari sisi Tauthub:

| Pesan | Artinya |
|---|---|
| "Webhook received ping and responded ok:true" | Berhasil. |
| "Webhook returned …: …" | Penerima menjawab dengan kode selain 2xx. |
| "Webhook did not respond with ok:true (…)" | Penerima menjawab 2xx, tetapi isinya bukan `{ "ok": true }`. |
| "Connection failed" | Sambungan ke penerima gagal. |

### content.delivered

Dikirim saat sebuah artikel diteruskan ke website Anda. Contoh ringkas:

```json
{
  "event": "content.delivered",
  "event_id": "7c2f9c1e-5b4a-4d3e-8f2a-1b6c7d8e9f0a",
  "timestamp": "2026-11-08T09:00:00Z",
  "document": {
    "id": "contoh-id-dokumen",
    "title": "Cara Memulai UMKM Digital",
    "slug": "cara-memulai-umkm-digital",
    "language": "id-ID",
    "content_markdown": "# Cara Memulai UMKM Digital …",
    "content_html": "<h2>Cara Memulai UMKM Digital</h2> …",
    "excerpt": "Ringkasan singkat artikel",
    "primary_keyword": "umkm digital",
    "secondary_keywords": ["modal kecil", "platform"],
    "tags": ["umkm", "digital"],
    "categories": ["Bisnis"],
    "author": null,
    "publish_status": "draft",
    "generated_at": "2026-11-08T09:00:00Z"
  }
}
```

Isi `document` juga bisa memuat field tambahan seperti `featured_image_url` dan `custom_fields`. Abaikan field yang tidak Anda perlukan.

Artikel selalu dikirim dengan `publish_status` bernilai `draft`. Keputusan menerbitkan ada di website Anda.

## Aturan tanda tangan

```
signature = "sha256=" + hex( HMAC-SHA256( secret, timestamp + "." + eventId + "." + rawBody ) )
```

- `timestamp` adalah nilai header `x-tulisio-timestamp`, bukan nilai `timestamp` di dalam isi JSON.
- `eventId` adalah nilai header `x-tulisio-event-id`.
- `rawBody` adalah isi permintaan mentah, persis seperti diterima, sebelum dibaca sebagai JSON.

### Langkah verifikasi di penerima

1. Baca isi permintaan mentah sebelum mengubahnya menjadi JSON.
2. Periksa `x-tulisio-timestamp`. Tolak permintaan bila selisihnya dengan waktu server Anda lebih dari 5 menit.
3. Hitung tanda tangan dengan rumus di atas, lalu bandingkan dengan `x-tulisio-signature` memakai pembanding yang tahan serangan waktu (timing-safe).
4. Periksa apakah artikel yang sama sudah pernah diproses. Lihat bagian pengiriman ulang di bawah.
5. Simpan artikel, lalu balas sesuai bagian berikut.

Bila tanda tangan atau waktu tidak valid, balas `401`. Bila isi tidak sesuai kontrak, balas `422`.

## Jawaban yang diharapkan

| HTTP | Isi | Artinya |
|---|---|---|
| 200 | `{ "ok": true }` | Berhasil. Tauthub menandai pengiriman selesai. |
| 200 | `{ "ok": true, "remote_id": "post-123" }` | Berhasil, dan menyertakan pengenal artikel di sistem Anda. |
| 4xx | `{ "ok": false }` | Penolakan. Pesan Anda dicatat sebagai alasan gagal. |
| 5xx atau tanpa jawaban | tidak ada | Dianggap gagal. |

Tauthub menunggu jawaban paling lama 30 detik untuk `content.delivered` dan 20 detik untuk `content.ping`. Jawaban sukses harus berkode 2xx dan memuat `"ok": true`. Jawaban lain dianggap gagal.

## Pengiriman ulang dan anti-duplikat

Bila penerima tidak menjawab `ok: true`, Tauthub mencoba mengirim ulang beberapa kali dalam waktu singkat. Bila semua percobaan gagal, pengiriman dianggap gagal.

> **Penting:** Tauthub membuat `event_id` baru untuk setiap percobaan kirim. Jangan hanya mengandalkan `event_id` untuk mencegah artikel ganda. Pakai juga `document.id` sebagai kunci anti-duplikat, dan balas `{ "ok": true }` bila artikel dengan `document.id` itu sudah tersimpan.

## Aturan dan batasan

- Artikel selalu dikirim sebagai draf. Tidak ada pengaturan untuk langsung menerbitkan.
- Jalur ini searah: Tauthub mengirim artikel ke penerima Anda dan tidak membaca data website lewat jalur ini.
- Bila Shared Secret bocor, hubungi [Dukungan](../12-dukungan.md) untuk mencabut koneksi dan membuat secret baru.
- Alamat webhook yang disimpan harus bisa dijangkau dari internet. Alamat internal atau privat tidak bisa dipakai.

## Kendala umum

**Tanda tangan selalu dianggap salah.**
Biasanya penyebabnya salah satu dari ini: isi mentah sudah diubah sebelum dihitung, `timestamp` diambil dari isi JSON padahal harus dari header, atau Shared Secret berbeda.

**Waktu dianggap kedaluwarsa.**
Jam server Anda tidak sinkron. Sinkronkan jam server dengan sumber waktu yang tepat.

**Artikel masuk dua kali.**
Penerima mengandalkan `event_id`, padahal setiap percobaan kirim membawa `event_id` baru. Pakai `document.id`.

## Lihat juga

- [Integrasi Web Custom](03-web-custom.md): Publisher Protocol yang dipakai tab Web Custom.
- [Publishing API](05-publishing-api.md): kontrak lain dengan tanda tangan dan kunci API.
- [Integrasi](README.md)
- [Dukungan](../12-dukungan.md)
