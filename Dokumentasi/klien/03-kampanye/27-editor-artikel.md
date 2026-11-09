# Editor Artikel

> Editor Artikel adalah halaman sederhana untuk menyunting isi sebuah artikel dari daftar Status Publikasi. Setiap kali Anda menyimpan, halaman ini membuat versi baru. Anda juga bisa memulihkan versi lama dari riwayat.

**Lokasi:** menu **Status Publikasi** › klik judul artikel pada baris publikasi
**Untuk:** Klien. Untuk melihat halaman ini, Anda butuh peran Viewer atau lebih tinggi. Untuk menyimpan, Anda butuh peran Writer atau lebih tinggi.

## Gambaran singkat

Halaman ini berbeda dari **Pengelola Naskah** di Penyedia Konten. Editor Artikel hanya mengubah isi naskah. Halaman ini tidak punya kolom gambar utama, kategori, atau cuplikan singkat. Halaman ini juga tidak menjalankan pemeriksaan QC (pemeriksaan kualitas naskah). Untuk menulis ulang dengan AI atau mengatur gambar utama, pakai [Pengelola Naskah](04-pengelola-naskah.md). AI adalah program komputer yang bisa menulis sendiri.

## Yang tampil di layar

- Tautan **Publikasi** dengan panah kiri di pojok kiri atas, untuk kembali ke [Status Publikasi](../05-status-publikasi.md).
- Judul artikel, lalu baris keterangan berisi alamat artikel, status dokumen, dan nomor versi yang aktif, misalnya "versi aktif v3". Status dokumen tampil sebagai kata bahasa Inggris seperti approved (disetujui) atau draft (draf).
- Tombol **Simpan versi baru**. Selama menyimpan, tombol berubah menjadi "Menyimpan…". Di sebelahnya muncul pesan hijau seperti "Versi 4 tersimpan." atau pesan merah.
- Kartu **Konten** dengan editor teks. Deretan tombol dan cara kerjanya sama seperti di [Editor teks naskah](25-editor-teks-naskah.md), dengan satu beda. Tombol **Sisipkan gambar** membuka kotak **Sisipkan Gambar ke Artikel**, bukan Pustaka Gambar AI. Kotak itu menerima gambar JPG, PNG, dan WebP. Tauthub mengecilkan gambar itu otomatis lalu mengunggahnya.
- Kartu **Riwayat versi (N)** yang mendaftar tiap versi, lengkap dengan nomor, asal versi, dan catatan perubahan. Versi yang sedang aktif bertanda "aktif". Versi lain punya tombol **Pulihkan**.

Di layar kecil, kartu riwayat versi turun ke bawah kartu konten.

## Cara menggunakan

### Cara menyunting dan menyimpan

1. Klik judul artikel di **Status Publikasi**.
2. Ubah isi di kartu **Konten**.
3. Klik **Simpan versi baru**.

Setiap kali Anda menyimpan, Tauthub membuat versi baru dengan catatan "Edit manual". Bila isi punya judul besar pertama (H1), judul artikel mengikuti judul itu. Bila isi tidak punya H1, judul artikel tidak berubah. Halaman ini tidak punya kolom judul sendiri.

### Cara memulihkan versi lama

1. Cari versi yang Anda mau di **Riwayat versi**.
2. Klik **Pulihkan**.

Isi versi itu menjadi versi terbaru dengan catatan "Pulihkan v…", lalu pesan "Dokumen dipulihkan dari versi N." tampil. Versi lama tetap ada di daftar.

## Aturan dan batasan

- Isi artikel tidak boleh kosong.
- Bila ada versi yang lebih baru dari versi yang Anda buka, Tauthub menolak penyimpanan dengan pesan "A newer version exists; reload before saving". Muat ulang halaman, lalu ulangi suntingan Anda.
- Saat Anda menyimpan, Tauthub membuang elemen berbahaya di isi, seperti skrip dan sematan dari situs selain Google Maps, YouTube, dan Vimeo.
- Dokumen berstatus approved berubah menjadi draft setelah Anda menyimpan suntingan.
- Halaman ini hanya menyimpan versi baru di Tauthub. Tidak ada tombol untuk mengirim ulang artikel ke Mitra (pemilik media), dan tidak ada pemeriksaan QC. Bila artikel sudah dikirim atau tayang, bicarakan perubahan dengan Mitra lewat [Pesan](../11-pesan.md).

## Pesan yang mungkin muncul

| Pesan | Artinya | Langkah Anda |
|---|---|---|
| "Gagal memuat dokumen." | Dokumen tidak terbaca | Muat ulang halaman |
| "Dokumen tidak ditemukan." | Dokumen tidak ada atau bukan milik brand ini | Kembali ke **Status Publikasi** |
| "Gagal menyimpan." | Penyimpanan gagal | Periksa koneksi, lalu simpan lagi |
| "Gagal memulihkan versi." | Pemulihan gagal | Coba lagi |

## Pertanyaan yang sering diajukan

**Apakah suntingan di sini mengubah artikel yang sudah tayang di situs Mitra?**
Halaman ini tidak punya fitur untuk mengirim ulang artikel ke situs Mitra. Hubungi Mitra lewat Pesan bila Anda perlu mengubah artikel yang sudah tayang.

**Di mana saya mengubah gambar utama atau kategori?**
Kedua pengaturan itu ada di [Pengelola Naskah](04-pengelola-naskah.md) pada tahap Penyedia Konten, selama kampanye belum selesai.

## Lihat juga

- [Editor teks naskah](25-editor-teks-naskah.md)
- [Pengelola Naskah](04-pengelola-naskah.md)
- [Status Publikasi](../05-status-publikasi.md)
- [Pesan](../11-pesan.md)
