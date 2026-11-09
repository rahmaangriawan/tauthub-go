# Integrasi WordPress

> Panduan ini menyambungkan website WordPress media Anda ke Tauthub memakai Application Password, supaya artikel Klien terkirim sendiri ke WordPress. Panduan ini juga menjelaskan peran akun Editor dan Penulis, serta arti pesan hasil uji koneksi.

**Lokasi:** menu **Integrasi** › tab **WordPress**
**Untuk:** Mitra (tab terbuka setelah Syarat & Ketentuan Integrasi disetujui)

## Gambaran singkat

Tab **WordPress** berisi kartu **Koneksi WordPress**. Setiap koneksi menghubungkan satu media berplatform WordPress dengan satu akun di website WordPress Anda. Tauthub memakai Application Password. Ini kata sandi khusus aplikasi dari WordPress, bukan kata sandi login Anda. Koneksi ini milik Mitra, bukan koneksi brand Klien.

Setelah tersambung, setiap artikel yang Anda setujui dikirim ke WordPress sebagai pos. Menu **Publikasi** juga memakai koneksi yang sama untuk menampilkan draf artikel yang sudah terkirim sebagai draf ke WordPress. Lihat [Publikasi](../03-publikasi/README.md).

## Prasyarat

- Media sudah dibuat di [Daftar Media](../04-daftar-media/README.md) dengan mode **API**, **Platform Tujuan** **WordPress**, dan kolom **Website / Domain** terisi. Tim Tauthub juga sudah menyetujui media itu.
- **Syarat & Ketentuan Integrasi** sudah disetujui. Lihat [Integrasi](README.md).
- Website memakai WordPress versi 5.6 atau lebih baru, bisa dibuka lewat HTTPS (alamat aman yang diawali https), dan REST API-nya aktif. REST API adalah pintu yang dipakai Tauthub untuk mengirim artikel ke WordPress. Sebagian plugin keamanan mematikan pintu ini. Plugin adalah program tambahan di WordPress.
- Anda punya akun WordPress berperan **Editor** (disarankan) atau paling rendah **Penulis**.
- Hosting (penyedia tempat website Anda) meneruskan header Authorization. Header ini adalah keterangan yang dibawa tiap permintaan untuk membuktikan siapa pengirimnya. Sebagian hosting menyaringnya.

### Peran Editor dan Penulis

| Peran akun | Menerbitkan artikel | Membuat kategori baru | Akibatnya |
|---|---|---|---|
| **Editor** (disarankan) | Bisa | Bisa | Kategori yang belum ada dibuat otomatis. |
| **Penulis** | Bisa | Tidak bisa | Kategori yang belum ada tidak dibuat. Artikel masuk ke kategori yang sudah ada, atau ke kategori bawaan WordPress (**Uncategorized**). |

Kalau akun Penulis tidak bisa membuat kategori, Anda menerima notifikasi berjudul "Kategori artikel belum bisa dibuat di …". Notifikasi itu memberi dua jalan keluar. Pertama, naikkan peran akun menjadi Editor di menu **Pengguna** WordPress. Kedua, buat kategorinya sendiri sekali saja di WordPress. Pengiriman berikutnya akan masuk ke kategori yang benar. Notifikasi dikirim paling sering sekali per media setiap 14 hari.

## Cara menyambungkan

### Cara membuat Application Password

1. Masuk ke wp-admin website Anda. Ini halaman pengelola WordPress.
2. Buka menu **Pengguna** › **Profil**.
3. Gulir ke bagian **Application Passwords**.
4. Isi nama aplikasi, misalnya `Tauthub`, lalu klik **Add New Application Password**.
5. Salin password yang muncul. Bentuknya `xxxx xxxx xxxx xxxx xxxx xxxx`, dan hanya tampil sekali.

### Cara menambahkan koneksi

1. Klik menu **Integrasi**, lalu buka tab **WordPress**.
2. Klik **Tambah koneksi**.
3. Pilih media di **Media**. Daftarnya berisi media Anda yang punya alamat website, dengan bentuk "nama media (alamat)". Kalau kosong, layar menampilkan "Belum ada media dengan website URL". Isi dulu alamat website di Daftar Media.
4. Periksa **Nama koneksi**. Kolom ini terisi sendiri dengan nama media diikuti kata WordPress. Ubah kalau perlu. Panjangnya 2 sampai 120 karakter.
5. Periksa **URL WordPress**. Kolom ini terisi sendiri dari media dan tidak bisa diketik.
6. Isi **Username atau email WordPress**.
7. Tempel Application Password di **Application Password**. Spasinya dihapus sendiri.
8. Klik **Simpan koneksi**.

### Cara menguji koneksi

1. Di baris koneksi, klik **Uji koneksi**. Tombol berubah menjadi "Menguji…".
2. Tunggu sampai selesai. Tauthub menunggu jawaban website paling lama 20 detik.
3. Lihat kolom **Status** dan **Uji terakhir** di baris itu.

### Cara mencabut dan menghapus koneksi

1. Klik **Cabut** pada koneksi aktif. Tombol langsung bekerja, tanpa pertanyaan konfirmasi. Status berubah menjadi **Dicabut**.
2. Klik **Hapus** kalau Anda ingin menghapusnya selamanya. Tombol ini hanya ada pada koneksi yang sudah dicabut.
3. Konfirmasi pada pertanyaan "Hapus permanen koneksi … ? Tindakan ini tidak bisa dibatalkan."

> **Penting:** Layar tidak punya tombol untuk mengubah kredensial. Kredensial adalah data masuk, yaitu username dan password. Kalau username atau Application Password berubah, cabut koneksi lama lalu tambahkan koneksi baru. Kalau koneksi dicabut, media tidak tersambung lagi. Media itu hilang dari Katalog Media, dan artikel yang menunggu dikirim gagal terkirim sampai ada koneksi aktif lagi.

## Yang tampil di layar

Kartu **Koneksi WordPress** menampilkan jumlah koneksi dan, di atas daftar, keterangan tentang peran Editor dan Penulis. Tabel koneksi punya kolom berikut. Di layar HP, setiap koneksi tampil sebagai kartu dengan data yang sama.

| Kolom | Isi |
|---|---|
| **Koneksi** | Nama koneksi dan, di bawahnya, username |
| **Endpoint** | Alamat website WordPress |
| **Status** | **Terhubung**, **Gagal**, **Belum diuji**, atau **Dicabut** |
| **Uji terakhir** | Pesan hasil uji terakhir, atau "Belum diuji" |
| **Aksi** | **Uji koneksi** dan **Cabut**, atau **Hapus** untuk koneksi yang dicabut |

Daftar menampilkan 20 koneksi per halaman, dengan tombol **Sebelumnya** dan **Berikutnya**. Kalau belum ada koneksi, layar menampilkan "Belum ada koneksi. Tambahkan koneksi WordPress untuk media berplatform WordPress".

## Arti pesan hasil uji

Uji koneksi mencoba masuk ke WordPress dengan akun Anda, lalu memeriksa apakah akun itu boleh menulis artikel. Bagian awal pesan berhasil memakai bahasa Inggris.

| Pesan | Artinya | Langkah Anda berikutnya |
|---|---|---|
| "Connected as … with post creation permission" | Berhasil. Akun boleh menulis artikel dan membuat kategori. | Tidak perlu berbuat apa-apa. |
| "… Akun ini tidak bisa membuat kategori baru (peran Penulis), artikel hanya masuk kategori yang sudah ada" | Berhasil, tetapi akun berperan Penulis. | Naikkan peran ke Editor, atau buat kategori sendiri di WordPress. |
| "WordPress menolak kredensial ini (rest_not_logged_in)…" | Username salah, Application Password tidak aktif, atau hosting menyaring header Authorization. | Periksa username, buat Application Password baru, cabut koneksi lalu buat ulang. Kalau tetap gagal, minta hosting mengizinkan header Authorization. |
| "Application Password salah untuk username ini (incorrect_password)." | Password tidak cocok dengan username. | Buat Application Password baru lalu buat koneksi baru. |
| "Username tidak dikenali WordPress (invalid_username)." | Username tidak ada di website. | Periksa ejaan username. |
| "User ini tidak punya izin membuat post (rest_forbidden)…" | Peran akun terlalu rendah. | Pakai akun berperan Penulis atau Editor. |
| "REST API situs tidak mengenali endpoint ini (rest_no_route)…" | REST API tidak tersedia. | Pastikan WordPress 5.6 atau lebih baru, dan plugin keamanan tidak mematikan REST API. |
| "WordPress authenticated as …, but edit_posts is denied" | Login berhasil, tetapi akun tidak boleh menulis artikel. | Pakai akun berperan Penulis atau Editor. |
| "Situs tidak merespons dalam 20 detik (timeout)…" | Hosting lambat atau tidak terjangkau. | Coba lagi nanti dan periksa status server. |
| "Koneksi diputus oleh jaringan atau server situs (connection reset)…" | Firewall atau hosting memutus sambungan dari server Tauthub. Firewall adalah penjaga yang menyaring lalu lintas masuk. | Periksa firewall hosting Anda. |
| "Sertifikat SSL situs tidak valid atau tidak dapat diverifikasi…" | Sertifikat HTTPS bermasalah. Sertifikat adalah tanda bahwa website Anda aman. | Perbaiki sertifikat website. |
| "Nama domain situs tidak dapat di-resolve (DNS)…" | Domain tidak ditemukan. DNS adalah buku alamat internet yang menunjukkan lokasi sebuah domain. | Periksa ejaan alamat dan pengaturan DNS. |
| "Situs menolak koneksi (port 443 tertutup atau server mati)." | Server tidak menerima sambungan HTTPS. | Pastikan server hidup dan port 443 terbuka. Port 443 adalah pintu sambungan HTTPS. |
| "URL situs menunjuk ke alamat internal/privat sehingga diblokir oleh pengaman SSRF." | Alamat bukan alamat publik. | Pakai alamat website yang bisa dibuka dari internet. |
| "WordPress menanggapi status …" | Jawaban lain yang tidak dikenali. | Hubungi [Dukungan](../12-dukungan.md) dan sertakan pesannya. |

Sebagian pesan menyertakan alamat yang diuji di bagian akhir.

## Yang dikirim ke website

- **Judul, slug, dan isi artikel** dalam bentuk HTML. Slug adalah bagian alamat artikel yang dibuat dari judul. HTML adalah bahasa yang dipakai website untuk menata isi halaman. Judul utama (H1) tidak diulang di isi, karena sudah menjadi judul pos.
- **Ringkasan** (excerpt) dari deskripsi meta artikel. Deskripsi meta adalah kalimat pendek yang tampil di hasil pencarian Google. Judul SEO dan deskripsi meta juga dikirim ke kolom plugin Yoast dan Rank Math kalau plugin itu terpasang. Plugin lain mengabaikannya.
- **Kategori:** kategori yang Anda pilih saat mengedit draf, atau kategori media Anda. Kalau kategorinya belum ada dan akun boleh membuatnya, kategori dibuat sendiri.
- **Gambar utama:** gambar cover artikel diunggah ke media library WordPress Anda dan dipasang sebagai gambar unggulan. Media library adalah tempat WordPress menyimpan gambar. Kalau artikel belum punya cover, Tauthub memakai cover yang tersedia atau cover standar.
- **Gambar di isi artikel:** diunggah ke media library WordPress Anda, sampai 20 gambar per artikel. Nama berkas diambil dari judul artikel.
- **Status:** draf atau terbit, sesuai pilihan Anda saat menyetujui artikel di **Publikasi**.

Tauthub mencocokkan pos lewat slug. Kalau di website sudah ada pos dengan slug yang sama, pos itu diperbarui dan tidak dibuat ulang. Pastikan tidak ada pos lain yang memakai slug serupa. Saat Anda menerima revisi dari Klien, artikel dikirim ulang dan memperbarui pos yang sama.

## Aturan dan batasan

- Setiap media hanya boleh punya satu koneksi aktif. Koneksi kedua untuk media atau alamat yang sama ditolak dengan pesan "Koneksi WordPress untuk website atau URL ini sudah aktif".
- Agar media tampil di Katalog Media, alamat koneksi harus sama dengan alamat website media (awalan `www.` diabaikan). Hasil uji koneksi tidak ikut dinilai. Jadi tetap jalankan **Uji koneksi** setelah menyimpan.
- Kalau gambar cover gagal diunggah, pengiriman berhenti sebelum terbit dengan pesan "Gambar cover artikel tidak bisa diunggah ke WordPress". Kalau cover standar yang gagal diunggah, pengiriman tidak berhenti.
- Hanya 20 gambar pertama di isi artikel yang diunggah.
- Kalau artikel terbit sebagai draf, Link Guardian belum memantaunya. Lihat [Link Guardian](../05-link-guardian.md).

## Kendala umum

**Artikel masuk kategori Uncategorized.**
Akun berperan Penulis dan kategorinya belum ada. Naikkan peran ke Editor atau buat kategorinya di WordPress.

**Muncul pesan "Koneksi WordPress untuk website atau URL ini sudah aktif".**
Sudah ada koneksi aktif untuk website itu. Cabut koneksi lama kalau Anda ingin menggantinya.

**Artikel gagal terkirim dengan pesan "No active wordpress connection for media …".**
Tidak ada koneksi aktif yang alamatnya cocok dengan website media. Tambahkan koneksi atau periksa alamat website media. Pesan galat pengiriman ada di bagian **Riwayat** pada detail publikasi.

**Media tidak muncul di pilihan Media.**
Media belum punya alamat website, atau belum memakai platform WordPress. Perbaiki lewat **Edit** di Daftar Media.

**Uji koneksi berhasil, tetapi pengiriman artikel gagal.**
Uji koneksi hanya memeriksa izin menulis artikel. Pengiriman juga mengunggah gambar. Pastikan akun boleh mengunggah berkas ke media library.

## Lihat juga

- [Integrasi](README.md): alur umum, Syarat & Ketentuan, dan syarat tampil di Katalog Media.
- [Daftar Media](../04-daftar-media/README.md)
- [Publikasi](../03-publikasi/README.md)
- [Dukungan](../12-dukungan.md)
