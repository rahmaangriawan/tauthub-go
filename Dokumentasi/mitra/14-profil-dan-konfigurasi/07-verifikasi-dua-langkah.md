# Verifikasi dua langkah (2FA)

> Kartu **Verifikasi dua langkah (2FA)** di tab **Keamanan** menambah satu pengaman saat Anda masuk. Selain email dan kata sandi, Anda perlu kode 6 angka dari aplikasi autentikator (aplikasi di HP yang menampilkan kode angka). Akun Anda tetap aman walau kata sandi bocor. Halaman ini juga memberi kode pemulihan untuk saat HP Anda hilang.

**Lokasi:** **Konfigurasi** › tab **Keamanan** › kartu **Verifikasi dua langkah (2FA)**
**Untuk:** Mitra

## Gambaran singkat

Tanpa 2FA, Anda masuk hanya dengan email dan kata sandi. Bila 2FA menyala, setelah mengetik kata sandi Anda juga diminta kode 6 angka. Kode itu muncul di aplikasi autentikator di HP Anda, misalnya Google Authenticator atau Authy. Kode ini menjaga akun Anda walau kata sandi bocor.

Saat Anda menyalakan 2FA, Tauthub juga memberi sekumpulan kode pemulihan sekali pakai. Kode ini menggantikan kode autentikator bila HP Anda hilang atau rusak.

## Yang tampil di layar

Di kanan judul kartu ada label status **Aktif** atau **Nonaktif**. Isi kartu mengikuti status itu:

- **Nonaktif**: kartu berisi penjelasan singkat dan tombol **Aktifkan verifikasi dua langkah**.
- **Aktif**: kartu menyatakan bahwa masuk ke akun memerlukan kode dari aplikasi autentikator Anda. Kartu juga berisi tombol **Nonaktifkan 2FA**.

> **Penting:** Saat halaman dibuka, kartu ini tidak membaca status 2FA yang sudah tersimpan. Setiap kali halaman dimuat, kartu mulai dari **Nonaktif** dengan tombol **Aktifkan verifikasi dua langkah**. Ini terjadi juga bila 2FA sudah pernah Anda nyalakan. Jangan klik tombol itu hanya untuk memeriksa status. Tombol itu memulai penyiapan baru. Kunci rahasia lama diganti, dan 2FA yang sedang berjalan mati sampai Anda memasukkan kode dari akun baru di aplikasi autentikator. Untuk memastikan status 2FA, buka jendela peramban lain lalu masuk. Bila 2FA menyala, Anda diminta **Kode verifikasi** setelah mengetik kata sandi.

## Cara menggunakan

### Cara mengaktifkan 2FA

Siapkan HP yang sudah terpasang aplikasi autentikator.

1. Klik **Konfigurasi** di menu samping, lalu klik tab **Keamanan**.
2. Gulir ke kartu **Verifikasi dua langkah (2FA)**. Pastikan Anda memang belum menyalakannya (lihat catatan Penting di atas).
3. Klik **Aktifkan verifikasi dua langkah**. Selama proses, tombol berubah menjadi **Menyiapkan…**.
4. Pada langkah **1. Tambahkan akun di aplikasi autentikator**, buka aplikasi autentikator di HP, tambah akun, lalu pindai kode QR (gambar kotak-kotak) di layar. Bila tidak bisa memindai, ketik **Kunci rahasia (secret)** yang tampil di sebelah kode QR. Ada juga tautan **Buka tautan otpauth di aplikasi autentikator**. Pakai tautan itu bila Anda menyiapkan 2FA dari perangkat yang sama dengan aplikasinya.
5. Pada langkah **2. Verifikasi kode**, ketik kode 6 angka yang sedang tampil di aplikasi autentikator ke kolom kode. Kolom hanya menerima angka dan paling banyak 6 angka.
6. Klik **Aktifkan 2FA**. Selama proses, tombol berubah menjadi **Mengaktifkan…**. Tombol baru bisa ditekan setelah 6 angka terisi.
7. Tunggu pesan hijau **2FA aktif. Simpan kode pemulihan sekarang.** Status kartu berubah menjadi **Aktif**.
8. Simpan kode pemulihan yang tampil (lihat bagian berikutnya).

Klik **Batal** pada langkah penyiapan bila Anda ingin membatalkan tanpa menyalakan 2FA. Kunci rahasia dan kode yang sudah Anda ketik dibuang.

> **Penting:** Jaga kunci rahasia seperti kata sandi. Siapa pun yang memilikinya bisa membuat kode autentikator untuk akun Anda. Jangan membagikannya.

### Cara menyimpan kode pemulihan

Setelah 2FA menyala, kotak peringatan **Simpan kode pemulihan Anda** menampilkan 10 kode. Keterangannya: "Kode ini hanya ditampilkan sekali. Simpan di tempat aman, dipakai jika HP Anda hilang."

1. Salin kesepuluh kode ke tempat yang aman, misalnya pengelola kata sandi atau catatan cetak yang Anda simpan terpisah dari HP.
2. Pastikan Anda sudah menyimpannya sebelum meninggalkan halaman.

Halaman ini tidak punya tombol untuk melihat kode pemulihan lagi. Setelah Anda memuat ulang halaman atau mematikan 2FA, kode yang tampil tidak bisa dibuka lagi. Setiap kode hanya bisa dipakai satu kali.

### Cara masuk ketika 2FA aktif

1. Masuk dengan email dan kata sandi seperti biasa. Halaman **Verifikasi Dua Langkah** muncul.
2. Ketik 6 angka kode dari aplikasi autentikator di kolom **Kode verifikasi**.
3. Bila HP tidak ada, ketik salah satu kode pemulihan di kolom yang sama. Kode itu hangus setelah dipakai.

Panduan lengkap untuk masuk ada di [Daftar dan masuk](../../umum/02-daftar-dan-masuk.md).

### Cara menonaktifkan 2FA

Tombol **Nonaktifkan 2FA** hanya tampil saat kartu berstatus **Aktif**. Kartu selalu mulai dari **Nonaktif** setiap kali halaman dimuat. Jadi tombol ini baru terlihat setelah Anda menyalakan 2FA di halaman yang sedang terbuka. Bila Anda perlu mematikan 2FA yang sudah menyala sejak sebelumnya, hubungi tim lewat [Dukungan](../12-dukungan.md).

1. Klik **Nonaktifkan 2FA**. Selama proses, tombol berubah menjadi **Memproses…**.
2. Pada jendela yang muncul, ketik kata sandi Anda. Jendela berbunyi: "Masukkan password Anda untuk menonaktifkan verifikasi dua langkah:".
3. Klik OK di jendela itu.
4. Tunggu pesan hijau **Verifikasi dua langkah dinonaktifkan.** Status kartu kembali ke **Nonaktif**.

Saat 2FA dimatikan, kunci rahasia dan seluruh kode pemulihan terhapus. Bila ingin menyalakannya lagi, ulangi "Cara mengaktifkan 2FA" dengan akun baru di aplikasi autentikator dan kode pemulihan yang baru.

## Aturan dan batasan

- **Kode verifikasi:** 6 angka, dari aplikasi autentikator.
- **Kode pemulihan:** 10 kode, sekali pakai, dan hanya tampil sekali.
- **Kata sandi untuk mematikan:** wajib. Ini mencegah orang lain yang memakai akun Anda yang sedang terbuka mematikan 2FA.
- **Batas percobaan:** paling banyak 5 percobaan per menit untuk menyalakan, dan 5 per menit untuk mematikan. Bila terlampaui, muncul "Terlalu banyak percobaan. Tunggu satu menit."
- **Satu akun autentikator untuk satu akun Tauthub:** bila Anda menyiapkan ulang 2FA, kunci rahasia lama diganti.
- **Siapa yang bisa mengatur:** pengaturan ini hanya untuk akun Mitra yang sedang masuk.

## Pesan kesalahan yang umum

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Masukkan kode 6 digit dari aplikasi autentikator. | Kode kurang dari 6 angka. | Ketik kode lengkap dari aplikasi. |
| Kode verifikasi salah. Coba lagi. | Kode salah atau sudah berganti. | Tunggu kode berikutnya muncul di aplikasi, lalu ketik lagi. Pastikan jam HP Anda diatur otomatis. |
| Terlalu banyak percobaan. Tunggu satu menit. | Lebih dari 5 percobaan dalam satu menit. | Tunggu satu menit lalu coba lagi. |
| Password salah. | Kata sandi yang Anda ketik di jendela nonaktifkan keliru. | Ketik kata sandi akun yang benar. |
| 2FA belum aktif. | Sistem mencatat 2FA belum menyala saat Anda mencoba mematikannya. Kartu lalu berubah ke **Nonaktif**. | Tidak perlu tindakan. |
| Permintaan gagal. | Sambungan internet atau sistem sedang bermasalah. | Coba lagi. |
| Gagal menonaktifkan 2FA. | Penonaktifan gagal. | Coba lagi. |

## Pertanyaan yang sering diajukan

**HP saya hilang. Bagaimana saya masuk?**
Masuk dengan email dan kata sandi, lalu ketik salah satu kode pemulihan di kolom **Kode verifikasi**. Setelah masuk, siapkan 2FA lagi dengan HP baru. Bila Anda sudah tidak punya kode pemulihan dan tidak bisa membuka aplikasi autentikator, Anda perlu bantuan tim Tauthub. Panduan ini tidak membahas caranya.

**Mengapa kartu menunjukkan Nonaktif padahal saya sudah menyalakannya?**
Saat dimuat, kartu tidak membaca status yang tersimpan. Baca catatan Penting di bagian atas halaman ini sebelum menekan tombol apa pun.

**Aplikasi autentikator apa yang bisa dipakai?**
Layar menyebut Google Authenticator dan Authy.

## Lihat juga

- [Konfigurasi](README.md)
- [Kata sandi](06-kata-sandi.md)
- [Daftar dan masuk](../../umum/02-daftar-dan-masuk.md)
- [Dukungan](../12-dukungan.md)
