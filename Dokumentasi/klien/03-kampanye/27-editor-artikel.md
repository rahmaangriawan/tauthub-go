# Editor Artikel

> Editor Artikel adalah halaman sederhana untuk menyunting isi sebuah artikel dari daftar Status Publikasi. Halaman ini menyimpan setiap suntingan sebagai versi baru dan menyediakan riwayat versi untuk dipulihkan.

**Lokasi:** menu **Status Publikasi** › klik judul artikel pada baris publikasi
**Untuk:** Klien. Melihat halaman butuh peran Viewer atau lebih tinggi. Menyimpan butuh peran Writer atau lebih tinggi.

## Gambaran singkat

Halaman ini berbeda dari **Pengelola Naskah** di Penyedia Konten. Editor Artikel hanya mengubah isi naskah. Halaman ini tidak punya kolom gambar utama, kategori, atau cuplikan singkat, dan tidak menjalankan pemeriksaan QC. Untuk menulis ulang dengan AI atau mengatur gambar utama, gunakan [Pengelola Naskah](04-pengelola-naskah.md).

## Yang tampil di layar

- Tautan **Publikasi** berpanah kiri di kiri atas untuk kembali ke [Status Publikasi](../05-status-publikasi.md).
- Judul artikel, lalu baris keterangan berisi alamat artikel, status dokumen, dan nomor versi aktif, misalnya "versi aktif v3". Status dokumen tampil sebagai kata teknis seperti approved atau draft.
- Tombol **Simpan versi baru**. Selama menyimpan, tombol berubah menjadi "Menyimpan…". Pesan hijau seperti "Versi 4 tersimpan." atau pesan merah muncul di sebelahnya.
- Kartu **Konten** dengan editor teks. Toolbar dan perilakunya sama dengan yang dijelaskan di [Editor teks naskah](25-editor-teks-naskah.md), dengan satu perbedaan: tombol **Sisipkan gambar** membuka dialog **Sisipkan Gambar ke Artikel**, bukan Pustaka Gambar AI. Dialog itu menerima gambar JPG, PNG, dan WebP, yang dikompres otomatis lalu diunggah.
- Kartu **Riwayat versi (N)** yang mencantumkan tiap versi dengan nomor, asal versi, dan catatan perubahan. Versi yang sedang aktif bertanda "aktif", sedangkan versi lain punya tombol **Pulihkan**.

Di layar kecil, kartu riwayat versi turun ke bawah kartu konten.

## Cara menggunakan

### Cara menyunting dan menyimpan

1. Klik judul artikel di **Status Publikasi**.
2. Ubah isi di kartu **Konten**.
3. Klik **Simpan versi baru**.

Setiap penyimpanan membuat versi baru dengan catatan "Edit manual". Judul artikel mengikuti heading pertama (H1) di dalam isi bila ada. Bila isi tidak memuat H1, judul tidak berubah. Halaman ini tidak punya kolom judul terpisah.

### Cara memulihkan versi lama

1. Cari versi yang diinginkan di **Riwayat versi**.
2. Klik **Pulihkan**.

Isi versi itu menjadi versi terbaru dengan catatan "Pulihkan v…", dan pesan "Dokumen dipulihkan dari versi N." tampil. Versi lama tetap ada di daftar.

## Aturan dan batasan

- Isi artikel tidak boleh kosong.
- Bila ada versi yang lebih baru dari yang Anda buka, penyimpanan ditolak dengan pesan "A newer version exists; reload before saving". Muat ulang halaman, lalu ulangi suntingan.
- Elemen berbahaya di dalam isi, seperti skrip dan sematan dari situs selain Google Maps, YouTube, dan Vimeo, dibuang saat disimpan.
- Dokumen yang berstatus approved berubah menjadi draft setelah Anda menyimpan suntingan.
- Halaman ini hanya menyimpan versi baru di Tauthub. Tidak ada tombol untuk mengirim ulang artikel ke Mitra dan tidak ada pemeriksaan QC. Bila artikel sudah dikirim atau tayang, koordinasikan perubahan dengan Mitra lewat [Pesan](../11-pesan.md).

## Pesan yang mungkin muncul

| Pesan | Artinya | Langkah Anda |
|---|---|---|
| "Gagal memuat dokumen." | Dokumen tidak terbaca | Muat ulang halaman |
| "Dokumen tidak ditemukan." | Dokumen tidak ada atau bukan milik brand ini | Kembali ke **Status Publikasi** |
| "Gagal menyimpan." | Penyimpanan gagal | Periksa koneksi, lalu simpan lagi |
| "Gagal memulihkan versi." | Pemulihan gagal | Coba lagi |

## Pertanyaan yang sering diajukan

**Apakah suntingan di sini mengubah artikel yang sudah tayang di situs Mitra?**
Halaman ini tidak punya fitur untuk mengirim ulang artikel ke situs Mitra. Hubungi Mitra lewat Pesan bila perlu perubahan pada artikel yang sudah tayang.

**Di mana saya mengubah gambar utama atau kategori?**
Kedua pengaturan itu ada di [Pengelola Naskah](04-pengelola-naskah.md) pada tahap Penyedia Konten, selama kampanye belum selesai.

## Lihat juga

- [Editor teks naskah](25-editor-teks-naskah.md)
- [Pengelola Naskah](04-pengelola-naskah.md)
- [Status Publikasi](../05-status-publikasi.md)
- [Pesan](../11-pesan.md)
