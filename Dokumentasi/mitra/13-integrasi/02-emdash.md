# Integrasi EmDash

> Panduan ini menyambungkan website EmDash media Anda ke Tauthub memakai token akses pribadi. Token adalah kunci rahasia berupa deretan karakter. Artikel terkirim lengkap dengan judul, deskripsi meta, isi, cover, dan gambar. Semua gambar dipindahkan ke media library situs Anda.

**Lokasi:** menu **Integrasi** › tab **EmDash**
**Untuk:** Mitra (tab terbuka setelah Syarat & Ketentuan Integrasi disetujui)

## Gambaran singkat

Tab **EmDash** berisi kartu **Koneksi EmDash**. Setiap koneksi menghubungkan satu media berplatform EmDash dengan situs EmDash Anda. Tauthub memakai token akses pribadi (Personal Access Token) yang Anda buat di admin EmDash. Token itu hanya memberi akses sesuai izin yang Anda centang.

Setelah tersambung, setiap artikel yang Anda setujui dikirim ke situs EmDash Anda. Gambar artikel tidak sekadar tertaut ke Tauthub. Gambar diunggah ke media library EmDash milik Anda. Media library adalah tempat situs menyimpan gambar.

## Prasyarat

- Media sudah dibuat di [Daftar Media](../04-daftar-media/README.md) dengan mode **API**, **Platform Tujuan** **EmDash**, dan kolom **Website / Domain** terisi. Tim Tauthub juga sudah menyetujui media itu.
- **Syarat & Ketentuan Integrasi** sudah disetujui. Lihat [Integrasi](README.md).
- Situs EmDash bisa dibuka dari internet, dan isi artikelnya disimpan di koleksi bernama `posts`. Koleksi adalah kumpulan data sejenis di dalam situs.
- Anda punya akses ke panel admin EmDash untuk membuat token.

## Cara menyambungkan

### Cara membuat token akses pribadi

1. Masuk ke panel admin situs EmDash Anda.
2. Buka pengaturan akun, lalu bagian **Personal Access Token**.
3. Buat token baru dengan nama `Tauthub`.
4. Centang izin `media:write` dan `content:write`. Keduanya perlu. `media:write` untuk mengunggah gambar, dan `content:write` untuk membuat artikel.
5. Salin token yang muncul. Token diawali `ec_pat_` dan hanya tampil sekali.

### Cara menambahkan koneksi

1. Klik menu **Integrasi**, lalu buka tab **EmDash**.
2. Klik **Tambah koneksi**.
3. Pilih media di **Media**. Daftarnya berisi media Anda yang punya alamat website. Kalau kosong, layar menampilkan "Belum ada media dengan website URL". Isi dulu alamat website di Daftar Media.
4. Periksa **Nama koneksi**. Kolom ini terisi sendiri dengan nama media diikuti kata EmDash. Ubah kalau perlu.
5. Periksa **URL EmDash**. Kolom ini terisi sendiri dari media dan tidak bisa diketik.
6. Tempel token di **Access Token**. Spasinya dihapus sendiri, dan token minimal 16 karakter.
7. Klik **Simpan koneksi**.

### Cara menguji koneksi

1. Di baris koneksi, klik **Uji koneksi**. Tombol berubah menjadi "Menguji…".
2. Tunggu sampai selesai. Tauthub menunggu jawaban situs paling lama 20 detik.
3. Lihat kolom **Status** dan **Uji terakhir** di baris itu.

### Cara mencabut dan menghapus koneksi

1. Klik **Cabut** pada koneksi aktif. Tombol langsung bekerja, tanpa pertanyaan konfirmasi. Status berubah menjadi **Dicabut**.
2. Klik **Hapus** kalau Anda ingin menghapusnya selamanya. Tombol ini hanya ada pada koneksi yang sudah dicabut.
3. Konfirmasi pada pertanyaan "Hapus permanen koneksi … ? Tindakan ini tidak bisa dibatalkan."

> **Penting:** Layar tidak punya tombol untuk mengubah token. Kalau token diganti atau dicabut di EmDash, cabut koneksi lama lalu tambahkan koneksi baru dengan token baru. Selama tidak ada koneksi aktif, media tidak tampil di Katalog Media, dan artikel yang menunggu dikirim gagal terkirim.

## Yang tampil di layar

Tabel koneksi punya kolom berikut. Di layar HP, setiap koneksi tampil sebagai kartu dengan data yang sama.

| Kolom | Isi |
|---|---|
| **Koneksi** | Nama koneksi dan, di bawahnya, kode singkat koneksi |
| **Endpoint** | Alamat situs EmDash |
| **Status** | **Terhubung**, **Gagal**, **Belum diuji**, atau **Dicabut** |
| **Uji terakhir** | Pesan hasil uji terakhir, atau "Belum diuji" |
| **Aksi** | **Uji koneksi** dan **Cabut**, atau **Hapus** untuk koneksi yang dicabut |

Daftar menampilkan 20 koneksi per halaman. Kalau belum ada koneksi, layar menampilkan "Belum ada koneksi. Tambahkan koneksi EmDash untuk media berplatform EmDash".

## Arti pesan hasil uji

Uji koneksi meminta satu daftar media dari situs EmDash dengan token Anda. Uji ini membuktikan bahwa alamat dan token diterima. Uji ini belum membuktikan bahwa token punya izin `content:write`. Izin itu baru terbukti saat artikel dikirim.

| Pesan | Artinya | Langkah Anda berikutnya |
|---|---|---|
| "Terhubung ke EmDash. Pastikan token punya izin media:write dan content:write agar artikel dan gambarnya bisa dikirim" | Alamat dan token diterima. | Pastikan kedua izin tercentang di token Anda. |
| "EmDash menolak token ini. Buat token pribadi baru di admin EmDash dengan izin media:write dan content:write, lalu perbarui koneksi dan uji lagi" | Token salah, kedaluwarsa, atau dicabut. | Buat token baru, lalu cabut koneksi lama dan tambahkan koneksi baru. |
| "Alamat ini tidak terbaca sebagai situs EmDash (…). Periksa kembali alamat situs dan pastikan EmDash sudah terpasang di sana" | Alamat tidak menjawab seperti situs EmDash. | Periksa **Website / Domain** media dan pastikan EmDash terpasang di alamat itu. |
| "EmDash membalas: …" | EmDash menolak permintaan dan menyebut alasannya. | Baca alasannya, lalu perbaiki di EmDash. |
| "EmDash membalas …: …" | Jawaban lain yang tidak dikenali. | Hubungi [Dukungan](../12-dukungan.md) dan sertakan pesannya. |

Masalah jaringan memakai pesan yang sama dengan tab WordPress, misalnya batas waktu habis, sambungan terputus, sertifikat bermasalah, atau domain tidak ditemukan. Artinya ada di [Integrasi WordPress](01-wordpress.md).

## Yang dikirim ke situs EmDash

- **Judul** dan **slug** artikel. Slug adalah bagian alamat artikel yang dibuat dari judul. Kalau artikel belum punya slug, slug dibuat dari judul.
- **Isi artikel**, diubah ke bentuk isi yang dipakai EmDash.
- **Ringkasan** dari deskripsi meta artikel. Deskripsi meta adalah kalimat pendek yang tampil di hasil pencarian Google.
- **Kategori:** kategori yang Anda pilih saat mengedit draf, atau kategori media Anda.
- **Gambar utama:** cover artikel diunggah ke media library EmDash lalu dipasang sebagai gambar utama.
- **Gambar di isi artikel:** diunggah ke media library EmDash, sampai 20 gambar per artikel.
- **Status:** draf atau terbit, sesuai pilihan Anda saat menyetujui artikel di **Publikasi**. EmDash 1.0.1 selalu membuat item baru sebagai draf. Jadi Tauthub menerbitkannya setelah itu kalau statusnya terbit.

Tauthub mengenali sendiri susunan REST situs Anda (EmDash 1.0.1 atau susunan lama), jadi Anda tidak perlu memilihnya. REST adalah pintu yang dipakai Tauthub untuk bicara dengan situs Anda. Kalau di situs sudah ada item dengan slug yang sama, item itu diperbarui dan tidak dibuat ulang. Kalau artikel belum punya cover, Tauthub memakai cover yang tersedia atau cover standar.

## Aturan dan batasan

- Setiap media hanya boleh punya satu koneksi aktif. Koneksi kedua untuk media atau alamat yang sama ditolak dengan pesan "Koneksi EmDash untuk website atau URL ini sudah aktif".
- Agar media tampil di Katalog Media, alamat koneksi harus sama dengan alamat website media (awalan `www.` diabaikan). Hasil uji koneksi tidak ikut dinilai. Jadi tetap jalankan **Uji koneksi** setelah menyimpan.
- Kalau koleksi `posts` Anda tidak punya kolom untuk gambar utama, ringkasan, atau kategori, bagian itu dilewati dan artikel tetap terkirim.
- Kalau cover gagal diunggah, artikel tetap dikirim tanpa gambar utama.
- Hanya 20 gambar pertama di isi artikel yang diunggah.
- Alamat artikel yang dikembalikan EmDash belum otomatis menjadi **URL Terbit** di **Publikasi**. Link Guardian hanya memantau artikel yang **URL Terbit**-nya terisi. Lihat [Link Guardian](../05-link-guardian.md).

## Kendala umum

**Artikel terkirim, tetapi statusnya masih draf di EmDash.**
Mungkin artikel memang dikirim sebagai draf. Mungkin juga Tauthub gagal menerbitkannya setelah item dibuat. Terbitkan artikel itu dari panel admin EmDash.

**Gambar utama tidak muncul.**
Cover gagal diunggah, atau koleksi `posts` tidak punya kolom gambar utama. Periksa susunan koleksi dan izin `media:write` pada token.

**Uji koneksi berhasil, tetapi pengiriman artikel gagal.**
Token kemungkinan tidak punya izin `content:write`. Buat token baru dengan kedua izin, lalu sambungkan ulang.

**Artikel berlabel "URL belum terisi" di Publikasi.**
Buka detail publikasinya, klik **Perbarui URL**, isi alamat artikel di situs EmDash Anda, lalu klik **Simpan URL**.

**Artikel gagal terkirim dengan pesan "No active emdash connection for media …".**
Tidak ada koneksi aktif yang cocok dengan media. Tambahkan koneksi atau periksa alamat website media. Pesan galat pengiriman ada di bagian **Riwayat** pada detail publikasi. Lihat [Publikasi](../03-publikasi/README.md).

## Lihat juga

- [Integrasi](README.md): alur umum, Syarat & Ketentuan, dan syarat tampil di Katalog Media.
- [Integrasi WordPress](01-wordpress.md): pesan jaringan yang sama.
- [Daftar Media](../04-daftar-media/README.md)
- [Dukungan](../12-dukungan.md)
