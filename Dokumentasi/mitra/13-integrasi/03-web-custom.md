# Integrasi Web Custom

> Panduan menyambungkan website buatan sendiri (Custom Web) ke Tauthub lewat alamat penerima artikel dan API key, lengkap dengan kontrak Publisher Protocol v1 untuk developer Anda. Setelah koneksi berstatus Terhubung, artikel Klien dikirim otomatis ke website Anda.

**Lokasi:** menu **Integrasi** › tab **Web Custom**
**Untuk:** Mitra dan developer website Mitra (tab terbuka setelah Syarat & Ketentuan Integrasi disetujui)

## Gambaran singkat

Pada platform **Custom Web**, website Anda menyediakan alamat penerima. Tauthub mengirim artikel ke alamat itu dengan API key milik Anda. Developer Anda membuat penerimanya sesuai Publisher Protocol v1, yang dijelaskan di bagian kontrak di bawah.

Tab **Web Custom** memuat tiga kartu:

- **Publishing API:** kunci API untuk media jenis lama. Lihat [Publishing API](05-publishing-api.md).
- **Website tanpa endpoint penerima:** keterangan tentang jalur tarik sebagai pengganti jalur kirim, lengkap dengan tautan **Panduan Publishing API**.
- **Custom Web Publishing** (Publisher Protocol v1): satu kotak per media Custom Web. Halaman ini membahas kartu ini.

Bila belum ada media Custom Web, kartu **Custom Web Publishing** menampilkan "Belum ada media Custom Web. Pilih platform Custom Web saat menambah media, lalu konfigurasikan di sini".

## Prasyarat

- Media sudah dibuat di [Daftar Media](../04-daftar-media/README.md) dengan mode **API** dan **Platform Tujuan** **Custom Web**, lalu disetujui tim Tauthub.
- **Syarat & Ketentuan Integrasi** sudah disetujui. Lihat [Integrasi](README.md).
- Website Anda punya penerima artikel yang dapat dibuka dari internet lewat HTTPS dan membalas sesuai kontrak di bawah.
- Anda punya API key. Key ini Anda tentukan sendiri di sisi website, lalu Anda isi di Tauthub.

## Cara menyambungkan

### Cara mengisi alamat dan API key

1. Klik menu **Integrasi**, lalu buka tab **Web Custom**.
2. Cari kotak media Anda pada kartu **Custom Web Publishing**.
3. Isi **Endpoint URL** dengan alamat dasar website penerima, misalnya `https://situs-media.example`. Tauthub menambahkan sendiri `/v1/tauthub/ping` untuk uji koneksi dan `/v1/tauthub/publish` untuk pengiriman artikel. Bila penerima Anda berada di sub-alamat, isi alamat sub-alamat itu, misalnya `https://situs-media.example/api`.
4. Isi **API key** dengan key yang dikenali penerima Anda.
5. Klik **Simpan**. Setelah tersimpan, layar menampilkan "Konfigurasi tersimpan. Jalankan Uji koneksi untuk memverifikasi."

> **Penting:** contoh isian di kolom **Endpoint URL** berakhiran `/v1/tulisio/publish`. Jangan memasukkan alamat yang berakhiran `/publish`. Uji koneksi akan menambahkan jalur ping di belakangnya sehingga alamatnya salah, walau pengiriman artikel memakai alamat itu dengan benar. Pakai alamat dasar seperti di langkah 3.

### Cara menguji koneksi

1. Pada kotak media, klik **Uji koneksi**. Tombol nonaktif bila belum ada alamat tersimpan.
2. Tunggu sampai selesai. Tauthub menunggu jawaban paling lama 10 detik.
3. Baca pesan di bawah tombol: hijau berarti berhasil, merah berarti gagal.
4. Pastikan status berubah menjadi **Terhubung**.

### Cara mengganti API key

1. Isi kolom **API key** dengan key baru. Kolom menampilkan titik-titik bila sudah ada key tersimpan. Isi hanya bila Anda ingin menggantinya.
2. Klik **Simpan**.
3. Klik **Uji koneksi**.

Tombol **Buat ulang API key** hanya muncul bila sudah ada key tersimpan. Setelah Anda mengonfirmasi pertanyaan "Buat API key baru? Key lama tidak berlaku — perbarui konfigurasi di website Anda.", Tauthub membuat key acak sebagai pengganti. Layar tidak menampilkan key baru itu. Bila Anda perlu tahu key yang dipakai, isi sendiri kolom **API key** dan klik **Simpan**.

## Yang tampil di layar

Setiap kotak media menampilkan nama media, titik status, dan keterangan "Dicek" beserta tanggal dan jam bila pernah diuji.

| Status | Artinya |
|---|---|
| **Terhubung** (hijau) | Uji koneksi terakhir berhasil. |
| **Gagal terhubung** (merah) | Uji koneksi terakhir gagal. |
| **Sedang diuji** (kuning) | Uji koneksi sedang berjalan. |
| **Belum terhubung** (abu-abu) | Belum pernah diuji, atau konfigurasi baru saja diubah. |

Menyimpan perubahan alamat atau API key, serta **Buat ulang API key**, selalu mengembalikan status ke **Belum terhubung**. Selama belum **Terhubung**, media tidak tampil di Katalog Media. Jalankan **Uji koneksi** setiap kali selesai mengubah konfigurasi.

Bila belum ada alamat tersimpan, kotak media menampilkan "Isi endpoint URL dan API key lalu simpan untuk mengaktifkan distribusi ke website ini".

### Panduan teknis di dalam dashboard

Tautan **Panduan Publisher Protocol** (dan **Panduan Publishing API**) membuka jendela **Panduan teknis** dengan dua tab: **Publishing API** dan **Publisher Protocol v1**. Isi jendela diambil dari dokumen di server dan menampilkan "Diperbarui" beserta tanggalnya. Bila dokumen tidak tersedia, layar menampilkan "Dokumen belum tersedia di server."

## Arti pesan hasil uji

| Pesan | Artinya | Langkah Anda berikutnya |
|---|---|---|
| "Endpoint terhubung" | Penerima menjawab sesuai kontrak. | Tidak perlu tindakan. |
| "Endpoint atau API key belum dikonfigurasi." | Alamat atau key belum tersimpan. | Isi keduanya lalu klik **Simpan**. |
| "Endpoint tidak dapat dijangkau (timeout)." | Penerima tidak menjawab dalam 10 detik. | Periksa server dan firewall. |
| "Endpoint tidak dapat diakses." | Sambungan gagal: alamat salah, domain tidak ditemukan, atau sertifikat bermasalah. | Periksa alamat dan sertifikat HTTPS. |
| "Endpoint merespons HTTP …" | Penerima menjawab dengan kode selain 2xx, misalnya 401 atau 404. | Periksa jalur `/v1/tauthub/ping` dan API key di penerima. |
| "Response tidak sesuai Tauthub Publisher Protocol." | Jawaban bukan JSON yang benar. | Balas `{"success": true, "service": "tauthub"}`. |
| "Kredensial tersimpan tidak dapat dibuka. Hubungi admin Tauthub untuk menyimpan ulang." | Masalah di sisi Tauthub. | Hubungi [Dukungan](../12-dukungan.md). |

Saat menyimpan, alamat divalidasi. Pesan yang mungkin muncul: "Endpoint URL wajib diisi.", "Endpoint URL tidak valid.", "Endpoint harus menggunakan HTTP atau HTTPS.", atau "Production hanya menerima endpoint HTTPS publik." Pesan terakhir berarti alamat memakai HTTP atau menunjuk ke alamat internal.

## Kontrak Publisher Protocol v1 untuk developer

Bagian ini ditujukan untuk developer website Anda. Penerima harus melayani dua jalur: ping dan publish.

### Header pada setiap permintaan

| Header | Isi |
|---|---|
| `Authorization` | `Bearer` diikuti API key Anda. Penerima wajib memeriksanya. |
| `Content-Type` | `application/json` |
| `X-Tauthub-Protocol-Version` | `1.0` |
| `X-Tauthub-Request-Id` | `ping_` atau `pub_` diikuti pengenal unik. Nilainya sama pada pengiriman ulang. |

Tauthub juga mengirim alias lama dengan awalan `X-Tulisio-` untuk dua header terakhir. Terima kedua awalan.

### Ping

Tauthub mengirim `POST {alamat}/v1/tauthub/ping` dengan isi berikut. Bila jalur ini gagal, Tauthub mencoba jalur lama `/v1/tulisio/ping`.

```json
{
  "protocol_version": "1.0",
  "timestamp": "2026-11-08T09:00:00.000Z",
  "request_id": "ping_contoh-0001"
}
```

Penerima membalas HTTP 2xx dengan JSON berikut. Nilai `service` boleh `tauthub` atau `tulisio`.

```json
{ "success": true, "service": "tauthub", "protocol_version": "1.0" }
```

### Publish

Tauthub mengirim `POST {alamat}/v1/tauthub/publish` dan menunggu jawaban paling lama 30 detik.

```json
{
  "publication_id": "contoh-id-publikasi",
  "title": "Cara Memulai UMKM Digital",
  "content": "<p>Isi artikel dalam HTML.</p>",
  "excerpt": "Ringkasan singkat artikel",
  "slug": "cara-memulai-umkm-digital",
  "status": "draft",
  "featured_image": { "url": "https://contoh.example/gambar.jpg", "alt": "Cara Memulai UMKM Digital" },
  "category": { "name": "Bisnis", "slug": "bisnis" }
}
```

| Field | Keterangan |
|---|---|
| `publication_id` | Pengenal publikasi di Tauthub. Pakai sebagai kunci anti-duplikat. |
| `title` | Judul artikel. |
| `content` | Isi artikel dalam HTML. Judul utama (H1) sudah dipisahkan ke `title`. |
| `excerpt` | Ringkasan dari deskripsi meta. Bisa kosong. |
| `slug` | Slug yang disarankan. Bisa kosong. |
| `status` | `draft` untuk disimpan sebagai draf, `publish` untuk diterbitkan. |
| `featured_image` | Hanya ada bila artikel punya gambar utama. Berisi `url` dan `alt`. |
| `category` | Hanya ada bila ada kategori. Berisi `name` dan `slug`. |

Gambar di `featured_image` dan di dalam `content` menunjuk ke berkas milik Tauthub lewat tautan publik bertanda tangan. Ambil dan simpan salinannya di website Anda. Bila pengambilan gambar gagal, sebaiknya artikel tetap diproses tanpa gambar.

### Jawaban dari penerima

Jawaban sukses (HTTP 2xx):

```json
{
  "success": true,
  "remote_id": "5842",
  "remote_url": "https://situs-media.example/cara-memulai-umkm-digital"
}
```

- `success` wajib `true`. Tanpa itu, Tauthub menganggap pengiriman gagal.
- `remote_id` opsional, direkam sebagai pengenal artikel di website Anda.
- `remote_url` opsional. Bila terisi, alamat ini menjadi **URL Terbit** artikel di **Publikasi**. Setelah artikel terbit (bukan draf), alamat itu dipantau Link Guardian. Untuk draf yang belum punya alamat publik, kosongkan saja.

Jawaban gagal: balas kode selain 2xx, atau `success: false`, dengan `error_code` dan `message`. Keduanya dicatat sebagai alasan gagal dan tampil di bagian **Riwayat** pada detail publikasi. Jangan menyertakan stack trace, kredensial, atau alamat berkas di server.

### Pengiriman ulang

Bila penerima tidak menjawab dengan benar, Tauthub mencoba mengirim ulang beberapa kali dalam waktu singkat, dengan `X-Tauthub-Request-Id` yang sama. Simpan pengenal itu dan balas dengan hasil pertama, supaya tidak ada artikel ganda. Bila semua percobaan gagal, publikasi berstatus gagal.

## Aturan dan batasan

- Alamat harus HTTPS publik. Alamat internal atau privat ditolak.
- Satu media punya satu alamat dan satu API key.
- Media berstatus **Terhubung** hanya bila uji koneksi terakhir berhasil.
- Jalur penerima ini searah: Tauthub mengirim artikel ke penerima Anda dan tidak membaca data website lewat jalur ini. Pemeriksaan Link Guardian dan crawler membuka halaman publik website secara terpisah.

## Kendala umum

**Uji koneksi menjawab "Endpoint merespons HTTP 404".**
Jalur `/v1/tauthub/ping` belum ada di penerima, atau alamat yang diisi berakhiran `/publish`. Isi alamat dasar.

**Status kembali ke Belum terhubung setelah saya menyimpan.**
Itu perilaku normal. Jalankan **Uji koneksi** lagi.

**Media hilang dari Katalog Media setelah saya mengganti API key.**
Status kembali ke **Belum terhubung**. Media tampil lagi setelah uji koneksi berhasil.

**Uji koneksi berhasil di jalur ping, tetapi pengiriman gagal.**
Pastikan jalur `/v1/tauthub/publish` ada, membalas `success: true`, dan menyelesaikan pekerjaan dalam 30 detik.

**Artikel gagal, dan Riwayat menyebut endpoint URL belum diatur atau API key belum dikonfigurasi.**
Alamat atau key belum tersimpan. Isi keduanya, simpan, lalu uji koneksi.

## Lihat juga

- [Integrasi](README.md): alur umum dan syarat tampil di Katalog Media.
- [Publishing API](05-publishing-api.md): kunci API untuk media jenis lama.
- [Webhook bertanda tangan](06-webhook-bertanda-tangan.md): kontrak publik lain untuk developer.
- [Publikasi](../03-publikasi/README.md): **Riwayat** pengiriman dan **Perbarui URL**.
- [Dukungan](../12-dukungan.md)
