# Publishing API

> Kartu Publishing API di tab Web Custom membuat dan mengelola kunci API untuk media jenis lama. Website Anda memakai kunci ini untuk menerima artikel dari Tauthub dengan tanda tangan digital. Halaman ini juga memuat aturan tanda tangan untuk developer.

**Lokasi:** menu **Integrasi** › tab **Web Custom** › kartu **Publishing API**
**Untuk:** Mitra dan developer website Mitra

## Gambaran singkat

Kartu **Publishing API** memuat pasangan kunci: API key dan secret. Tauthub memakainya untuk mengirim artikel ke alamat penerima (Adapter URL) milik sebuah media. Pengiriman memakai API key sebagai tanda pengenal dan secret untuk membuat tanda tangan digital yang bisa Anda periksa.

Kartu ini berlaku untuk media yang aktif dan berjenis lama. Di Daftar Media, jenis itu berlabel **Tulisio API**. Formulir media Anda tidak menawarkan jenis ini: pilihan yang tersedia hanya **WordPress**, **EmDash**, **Blogspot**, dan **Custom Web**. Bila media Anda berjenis ini, alamat penerimanya (Adapter URL) disiapkan oleh tim Tauthub. Untuk website baru, pakai [Integrasi Web Custom](03-web-custom.md).

Tab **Web Custom** juga memuat kartu **Website tanpa endpoint penerima** dengan judul kecil "alternatif push API". Kartu ini menjelaskan bahwa website statis, atau website yang tidak bisa membuka alamat penerima, dapat menarik artikel dalam format JSON secara berkala lewat jalur tarik (pull feed), dengan jadwal dan tampilan yang diatur script atau CI Anda sendiri. Kartu ini hanya berisi keterangan dan tautan **Panduan Publishing API**. Token untuk jalur tarik tidak dibuat di tab ini. Tanyakan lewat [Dukungan](../12-dukungan.md).

## Yang tampil di layar

Kartu **Publishing API** menampilkan jumlah kunci dan dua tombol: **Panduan** (membuka jendela **Panduan teknis**) dan **Buat API key**. Bila belum ada kunci, layar menampilkan "Belum ada API key. Buat kunci untuk media Tauthub Publishing API Anda".

| Kolom | Isi |
|---|---|
| **Label** | Nama kunci dan kode singkatnya |
| **Media** | Nama media yang dibatasi kunci ini, atau "Semua media" |
| **Status** | **Aktif** atau **Dicabut** |
| **Terakhir dipakai** | Waktu terakhir kunci tercatat dipakai, atau "Belum dipakai" |
| **Aksi** | **Rotasi** dan **Cabut** untuk kunci aktif. Kunci yang dicabut hanya menampilkan "Dicabut" |

Kolom **Terakhir dipakai** belum selalu diperbarui saat kunci dipakai, jadi "Belum dipakai" tidak membuktikan bahwa kunci menganggur.

## Cara menggunakan

### Cara membuat API key

1. Klik menu **Integrasi**, lalu buka tab **Web Custom**.
2. Pada kartu **Publishing API**, klik **Buat API key**.
3. Isi **Label**, misalnya `Website utama`. Panjangnya 2 sampai 120 karakter.
4. Pilih **Media** bila Anda ingin membatasi kunci ke satu media. Biarkan **Semua media Tauthub API** bila kunci boleh dipakai semua media Anda yang berjenis ini. Kolom ini opsional.
5. Klik **Buat kunci**.
6. Salin **API key** dan **Secret** dengan tombol **Salin** di sampingnya. Kartu "Kredensial baru, simpan sekarang" hanya menampilkannya sekali.
7. Pasang keduanya di konfigurasi penerima website Anda, lalu klik **Tutup**.

Bila Anda memilih media tertentu, artikel ke media itu memakai kunci tersebut, dan media lain memakai kunci tanpa batasan media.

### Cara merotasi kunci

1. Pada baris kunci aktif, klik **Rotasi**.
2. Konfirmasi pada pertanyaan "Rotasi API key ini? Kredensial lama langsung tidak berlaku."
3. Salin API key dan secret baru dari kartu yang muncul.
4. Ganti keduanya di penerima website Anda.

Rotasi mengganti API key dan secret sekaligus. Selama penerima belum diperbarui, pengiriman artikel ditolak.

### Cara mencabut kunci

1. Klik **Cabut** pada baris kunci aktif.
2. Konfirmasi pada pertanyaan "Cabut API key ini?"
3. Status berubah menjadi **Dicabut**. Dari layar, kunci yang dicabut tidak bisa diaktifkan lagi. Buat kunci baru bila Anda masih memerlukannya.

### Cara menguji koneksi

1. Buka [Daftar Media](../04-daftar-media/README.md).
2. Pada baris media berjenis **Tulisio API**, buka menu aksi (tiga titik), lalu klik **Test Koneksi**. Menu ini hanya ada pada media jenis ini.
3. Baca pesan hasilnya, yang tampil dengan awalan nama media.

| Pesan | Artinya | Langkah Anda berikutnya |
|---|---|---|
| "Koneksi berhasil — adapter memverifikasi signature dan merespons accepted:true." | Penerima menerima tanda tangan dan menjawab sesuai kontrak. | Tidak perlu tindakan. |
| "Adapter URL atau API key belum diatur untuk media ini. Lengkapi Adapter URL dan buat API key di menu Publishing API." | Belum ada alamat penerima atau belum ada kunci aktif. | Buat kunci, atau hubungi [Dukungan](../12-dukungan.md) untuk alamat penerima. |
| "Adapter tidak dapat dijangkau: …" | Sambungan ke penerima gagal. | Periksa alamat, sertifikat, dan firewall penerima. |
| "Adapter merespons status …: …" | Penerima menjawab dengan kode selain 2xx, atau tanpa `accepted: true`. | Baca alasannya di pesan, lalu perbaiki penerima. |
| "Kredensial tersimpan tidak dapat dibuka. Buat ulang API key di menu Publishing API atau hubungi admin Tauthub." | Masalah di sisi Tauthub. | Rotasi kunci, atau hubungi Dukungan. |

## Kontrak untuk developer

Bagian ini ditujukan untuk developer website Anda.

### Permintaan dari Tauthub

Tauthub mengirim `POST` ke Adapter URL media, dengan isi JSON dan header berikut.

| Header | Isi |
|---|---|
| `Authorization` | `Bearer` diikuti API key |
| `Content-Type` | `application/json` |
| `X-Tauthub-Timestamp` | Waktu kirim dalam detik (Unix) |
| `X-Tauthub-Signature` | Tanda tangan digital, huruf heksadesimal |
| `X-Tauthub-Idempotency-Key` | Pengenal unik pengiriman. Sama pada pengiriman ulang |

Setiap header terakhir juga dikirim dengan alias lama berawalan `X-Tulisio-`. Pada **Test Koneksi**, hanya awalan `X-Tulisio-` yang dikirim. Terima kedua awalan.

### Aturan tanda tangan

```
tanda_tangan = hex( HMAC-SHA256( secret, timestamp + "." + isi_permintaan_mentah ) )
```

- `timestamp` adalah nilai header `X-Tauthub-Timestamp`.
- `isi_permintaan_mentah` adalah isi JSON persis seperti diterima. Jangan mengubah susunannya sebelum menghitung.
- Bandingkan hasilnya dengan header `X-Tauthub-Signature` memakai pembanding yang tahan serangan waktu.
- Tolak permintaan yang waktunya terlalu lama, misalnya lebih dari 5 menit.

### Isi permintaan

Isinya empat kelompok. Contoh ringkas:

```json
{
  "publication": { "id": "contoh-id-publikasi", "publish_status": "draft" },
  "article": {
    "id": "contoh-id-artikel",
    "title": "Cara Memulai UMKM Digital",
    "slug": "cara-memulai-umkm-digital",
    "content_markdown": "## Bagian\n\nParagraf …",
    "content_html": "<h2>Bagian</h2><p>Paragraf …</p>",
    "excerpt": "Ringkasan singkat",
    "language": "id",
    "category": ""
  },
  "media": { "id": "contoh-id-media", "name": "Media Contoh", "website_url": "https://situs-media.example" },
  "tracking": { "publication_id": "contoh-id-publikasi" }
}
```

- `publication` memuat pengenal publikasi, `publish_status` (`draft` atau `publish`), dan keterangan waktu serta harga.
- `article` memuat judul, slug, isi dalam dua bentuk (Markdown dan HTML), ringkasan, judul dan deskripsi meta, bahasa, kata kunci utama, dan `category` (kategori pilihan Anda saat mengedit draf, kosong bila tidak ada).
- `media` memuat pengenal, nama, dan alamat website media.
- `tracking` memuat pengenal publikasi untuk keperluan pelacakan.

### Jawaban dari penerima

Jawaban sukses memakai HTTP 2xx:

```json
{ "accepted": true, "externalId": "post-12345", "status": "https://situs-media.example/artikel/judul" }
```

- `accepted` wajib `true`. Tanpa itu, Tauthub menganggap pengiriman gagal.
- `externalId` opsional, direkam sebagai pengenal artikel di website Anda.
- `status` boleh berisi alamat artikel yang terbit. Alamat ini disimpan sebagai hasil integrasi, tetapi belum otomatis menjadi **URL Terbit**. Lengkapi lewat **Perbarui URL** di **Publikasi** supaya Link Guardian memantau artikel itu.

Jawaban gagal memakai HTTP 4xx atau 5xx, atau `accepted: false`, dengan `message` berisi alasan. Alasan itu tampil di bagian **Riwayat** pada detail publikasi.

Pada **Test Koneksi**, Tauthub mengirim isi `{"event": "ping", …}` dengan `media_id`. Cukup balas `{ "accepted": true }`, tetap setelah memeriksa tanda tangan. Tauthub menunggu jawaban ping paling lama 15 detik dan jawaban pengiriman artikel paling lama 30 detik.

### Idempotensi

Pada pengiriman ulang, `X-Tauthub-Idempotency-Key` bernilai sama. Simpan nilainya dan balas dengan hasil pertama, supaya tidak ada artikel ganda.

### Callback dan jalur tarik

Penerima juga dapat melaporkan alamat artikel kembali ke Tauthub lewat callback bertanda tangan memakai kunci yang sama. Format callback dan jalur tarik dijelaskan di jendela **Panduan teknis**, tab **Publishing API**. Buka lewat tombol **Panduan**.

## Aturan dan batasan

- API key dan secret hanya tampil sekali, saat dibuat atau dirotasi. Bila hilang, rotasi kunci.
- Satu kunci bisa dibatasi ke satu media, atau berlaku untuk semua media Anda yang berjenis ini.
- Tanpa kunci aktif atau tanpa Adapter URL, pengiriman artikel gagal. Bagian **Riwayat** pada detail publikasi menyebut bahwa adapter URL belum diatur atau publishing API key belum dibuat.
- Media berjenis ini dianggap tersambung bila Adapter URL-nya terisi. Hasil **Test Koneksi** tidak ikut dinilai untuk tampil di Katalog Media.

## Lihat juga

- [Integrasi Web Custom](03-web-custom.md): jalur yang disarankan untuk website baru.
- [Webhook bertanda tangan](06-webhook-bertanda-tangan.md): kontrak publik lain untuk developer.
- [Integrasi](README.md)
- [Daftar Media](../04-daftar-media/README.md)
- [Dukungan](../12-dukungan.md)
