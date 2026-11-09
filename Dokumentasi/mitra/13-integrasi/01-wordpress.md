# Integrasi WordPress

> Panduan menyambungkan website WordPress media Anda ke Tauthub memakai Application Password, agar artikel Klien terkirim otomatis ke WordPress. Panduan ini juga menjelaskan peran akun Editor dan Penulis, serta arti pesan hasil uji koneksi.

**Lokasi:** menu **Integrasi** › tab **WordPress**
**Untuk:** Mitra (tab terbuka setelah Syarat & Ketentuan Integrasi disetujui)

## Gambaran singkat

Tab **WordPress** berisi kartu **Koneksi WordPress**. Setiap koneksi menghubungkan satu media berplatform WordPress dengan satu akun di website WordPress Anda. Tauthub memakai Application Password, yaitu kata sandi khusus aplikasi dari WordPress, bukan kata sandi login Anda. Koneksi ini milik Mitra, bukan koneksi brand Klien.

Setelah tersambung, setiap artikel yang Anda setujui dikirim ke WordPress sebagai pos. Koneksi yang sama juga dipakai menu **Publikasi** untuk menampilkan draf artikel yang sudah terkirim sebagai draf ke WordPress. Lihat [Publikasi](../03-publikasi/README.md).

## Prasyarat

- Media sudah dibuat di [Daftar Media](../04-daftar-media/README.md) dengan mode **API**, **Platform Tujuan** **WordPress**, dan kolom **Website / Domain** terisi. Media juga sudah disetujui tim Tauthub.
- **Syarat & Ketentuan Integrasi** sudah disetujui. Lihat [Integrasi](README.md).
- Website memakai WordPress versi 5.6 atau lebih baru, dapat dibuka lewat HTTPS, dan REST API-nya aktif. Sebagian plugin keamanan mematikan REST API.
- Anda punya akun WordPress berperan **Editor** (disarankan) atau paling rendah **Penulis**.
- Hosting meneruskan header Authorization. Sebagian hosting menyaringnya.

### Peran Editor dan Penulis

| Peran akun | Menerbitkan artikel | Membuat kategori baru | Akibatnya |
|---|---|---|---|
| **Editor** (disarankan) | Bisa | Bisa | Kategori yang belum ada dibuat otomatis. |
| **Penulis** | Bisa | Tidak bisa | Kategori yang belum ada tidak dibuat. Artikel masuk ke kategori yang sudah ada, atau ke kategori bawaan WordPress (**Uncategorized**). |

Bila akun Penulis tidak bisa membuat kategori, Anda menerima notifikasi berjudul "Kategori artikel belum bisa dibuat di …". Notifikasi itu memberi dua jalan keluar: naikkan peran akun menjadi Editor di menu **Pengguna** WordPress, atau buat kategorinya sendiri sekali saja di WordPress. Pengiriman berikutnya akan masuk ke kategori yang benar. Notifikasi dikirim paling sering sekali per media setiap 14 hari.

## Cara menyambungkan

### Cara membuat Application Password

1. Masuk ke wp-admin website Anda.
2. Buka menu **Pengguna** › **Profil**.
3. Gulir ke bagian **Application Passwords**.
4. Isi nama aplikasi, misalnya `Tauthub`, lalu klik **Add New Application Password**.
5. Salin password yang muncul. Formatnya `xxxx xxxx xxxx xxxx xxxx xxxx`, dan hanya ditampilkan sekali.

### Cara menambahkan koneksi

1. Klik menu **Integrasi**, lalu buka tab **WordPress**.
2. Klik **Tambah koneksi**.
3. Pilih media pada **Media**. Daftar memuat media Anda yang punya alamat website, dengan format "nama media (alamat)". Bila kosong, layar menampilkan "Belum ada media dengan website URL". Isi dulu alamat website di Daftar Media.
4. Periksa **Nama koneksi**. Kolom ini terisi otomatis dengan nama media diikuti kata WordPress. Ubah bila perlu. Panjangnya 2 sampai 120 karakter.
5. Periksa **URL WordPress**. Kolom ini terisi otomatis dari media dan tidak bisa diketik.
6. Isi **Username atau email WordPress**.
7. Tempel Application Password pada **Application Password**. Spasi dihapus otomatis.
8. Klik **Simpan koneksi**.

### Cara menguji koneksi

1. Pada baris koneksi, klik **Uji koneksi**. Tombol berubah menjadi "Menguji…".
2. Tunggu sampai selesai. Tauthub menunggu jawaban website paling lama 20 detik.
3. Lihat kolom **Status** dan **Uji terakhir** pada baris itu.

### Cara mencabut dan menghapus koneksi

1. Klik **Cabut** pada koneksi aktif. Tombol bekerja langsung, tanpa konfirmasi. Status berubah menjadi **Dicabut**.
2. Klik **Hapus** bila Anda ingin menghapusnya permanen. Tombol ini hanya ada pada koneksi yang sudah dicabut.
3. Konfirmasi pada pertanyaan "Hapus permanen koneksi … ? Tindakan ini tidak bisa dibatalkan."

> **Penting:** layar tidak menyediakan tombol untuk mengubah kredensial. Bila username atau Application Password berubah, cabut koneksi lama lalu tambahkan koneksi baru. Mencabut koneksi membuat media tidak tersambung. Media itu hilang dari Katalog Media, dan artikel yang menunggu dikirim gagal terkirim sampai ada koneksi aktif lagi.

## Yang tampil di layar

Kartu **Koneksi WordPress** memuat jumlah koneksi dan, di atas daftar, keterangan tentang peran Editor dan Penulis. Tabel koneksi punya kolom berikut. Di layar HP, setiap koneksi tampil sebagai kartu dengan data yang sama.

| Kolom | Isi |
|---|---|
| **Koneksi** | Nama koneksi dan, di bawahnya, username |
| **Endpoint** | Alamat website WordPress |
| **Status** | **Terhubung**, **Gagal**, **Belum diuji**, atau **Dicabut** |
| **Uji terakhir** | Pesan hasil uji terakhir, atau "Belum diuji" |
| **Aksi** | **Uji koneksi** dan **Cabut**, atau **Hapus** untuk koneksi yang dicabut |

Daftar memuat 20 koneksi per halaman, dengan tombol **Sebelumnya** dan **Berikutnya**. Bila belum ada koneksi, layar menampilkan "Belum ada koneksi. Tambahkan koneksi WordPress untuk media berplatform WordPress".

## Arti pesan hasil uji

Uji koneksi mencoba masuk ke WordPress dengan akun Anda dan memeriksa bahwa akun itu boleh menulis artikel. Pesan berhasil memakai bahasa Inggris pada bagian awalnya.

| Pesan | Artinya | Langkah Anda berikutnya |
|---|---|---|
| "Connected as … with post creation permission" | Berhasil. Akun boleh menulis artikel dan membuat kategori. | Tidak perlu tindakan. |
| "… Akun ini tidak bisa membuat kategori baru (peran Penulis), artikel hanya masuk kategori yang sudah ada" | Berhasil, tetapi akun berperan Penulis. | Naikkan peran ke Editor, atau buat kategori sendiri di WordPress. |
| "WordPress menolak kredensial ini (rest_not_logged_in)…" | Username salah, Application Password tidak aktif, atau hosting menyaring header Authorization. | Periksa username, buat Application Password baru, cabut dan buat ulang koneksi. Bila tetap gagal, minta hosting mengizinkan header Authorization. |
| "Application Password salah untuk username ini (incorrect_password)." | Password tidak cocok dengan username. | Buat Application Password baru lalu buat koneksi baru. |
| "Username tidak dikenali WordPress (invalid_username)." | Username tidak ada di website. | Periksa ejaan username. |
| "User ini tidak punya izin membuat post (rest_forbidden)…" | Peran akun terlalu rendah. | Pakai akun berperan Penulis atau Editor. |
| "REST API situs tidak mengenali endpoint ini (rest_no_route)…" | REST API tidak tersedia. | Pastikan WordPress 5.6 atau lebih baru dan plugin keamanan tidak mematikan REST API. |
| "WordPress authenticated as …, but edit_posts is denied" | Login berhasil, tetapi akun tidak boleh menulis artikel. | Pakai akun berperan Penulis atau Editor. |
| "Situs tidak merespons dalam 20 detik (timeout)…" | Hosting lambat atau tidak terjangkau. | Coba lagi nanti dan periksa status server. |
| "Koneksi diputus oleh jaringan atau server situs (connection reset)…" | Firewall atau hosting memutus koneksi dari server Tauthub. | Periksa firewall hosting Anda. |
| "Sertifikat SSL situs tidak valid atau tidak dapat diverifikasi…" | Sertifikat HTTPS bermasalah. | Perbaiki sertifikat website. |
| "Nama domain situs tidak dapat di-resolve (DNS)…" | Domain tidak ditemukan. | Periksa ejaan alamat dan pengaturan DNS. |
| "Situs menolak koneksi (port 443 tertutup atau server mati)." | Server tidak menerima sambungan HTTPS. | Pastikan server hidup dan port 443 terbuka. |
| "URL situs menunjuk ke alamat internal/privat sehingga diblokir oleh pengaman SSRF." | Alamat bukan alamat publik. | Pakai alamat website yang bisa dibuka dari internet. |
| "WordPress menanggapi status …" | Jawaban lain yang tidak dikenali. | Hubungi [Dukungan](../12-dukungan.md) dengan menyertakan pesannya. |

Sebagian pesan menyertakan alamat yang diuji di bagian akhir.

## Yang dikirim ke website

- **Judul, slug, dan isi artikel** dalam bentuk HTML. Judul utama (H1) tidak diulang di isi, karena sudah menjadi judul pos.
- **Ringkasan** (excerpt) dari deskripsi meta artikel. Judul SEO dan deskripsi meta juga dikirim ke kolom plugin Yoast dan Rank Math bila plugin itu terpasang. Plugin lain mengabaikannya.
- **Kategori:** kategori yang Anda pilih saat mengedit draf, atau kategori media Anda. Bila kategorinya belum ada dan akun boleh membuatnya, kategori dibuat otomatis.
- **Gambar utama:** gambar cover artikel diunggah ke media library WordPress Anda dan dipasang sebagai gambar unggulan. Bila artikel belum punya cover, Tauthub memakai cover yang tersedia atau cover standar.
- **Gambar di isi artikel:** diunggah ke media library WordPress Anda, sampai 20 gambar per artikel. Nama berkas diambil dari judul artikel.
- **Status:** draf atau terbit, sesuai pilihan saat artikel disetujui di **Publikasi**.

Tauthub mencocokkan pos lewat slug. Bila di website sudah ada pos dengan slug yang sama, pos itu diperbarui, bukan dibuat ulang. Pastikan tidak ada pos lain yang memakai slug serupa. Saat Anda menerima revisi dari Klien, artikel dikirim ulang dan memperbarui pos yang sama.

## Aturan dan batasan

- Setiap media hanya boleh punya satu koneksi aktif. Koneksi kedua untuk media atau alamat yang sama ditolak dengan pesan "Koneksi WordPress untuk website atau URL ini sudah aktif".
- Agar media tampil di Katalog Media, alamat koneksi harus sama dengan alamat website media (awalan `www.` diabaikan). Hasil uji koneksi tidak ikut dinilai, jadi tetap jalankan **Uji koneksi** setelah menyimpan.
- Bila gambar cover gagal diunggah, pengiriman berhenti sebelum terbit dengan pesan "Gambar cover artikel tidak bisa diunggah ke WordPress". Cover standar yang gagal diunggah tidak menghentikan pengiriman.
- Hanya 20 gambar pertama di isi artikel yang diunggah.
- Bila artikel terbit sebagai draf, Link Guardian belum memantaunya. Lihat [Link Guardian](../05-link-guardian.md).

## Kendala umum

**Artikel masuk kategori Uncategorized.**
Akun berperan Penulis dan kategorinya belum ada. Naikkan peran ke Editor atau buat kategorinya di WordPress.

**Muncul pesan "Koneksi WordPress untuk website atau URL ini sudah aktif".**
Sudah ada koneksi aktif untuk website itu. Cabut koneksi lama bila Anda ingin menggantinya.

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
