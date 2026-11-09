# Verifikasi dua langkah (2FA)

> Kartu **Verifikasi dua langkah (2FA)** di tab **Keamanan** menambahkan kode 6 digit dari aplikasi autentikator saat Anda masuk dengan email dan kata sandi, sehingga akun tetap terlindungi walau kata sandi bocor. Halaman ini juga menyediakan kode pemulihan untuk saat HP Anda hilang.

**Lokasi:** **Konfigurasi** › tab **Keamanan** › kartu **Verifikasi dua langkah (2FA)**
**Untuk:** Mitra

## Gambaran singkat

Tanpa 2FA, Anda masuk hanya dengan email dan kata sandi. Dengan 2FA aktif, setelah memasukkan kata sandi Anda juga diminta kode 6 digit yang muncul di aplikasi autentikator di HP Anda, misalnya Google Authenticator atau Authy. Kode ini melindungi akun Anda walau kata sandi bocor.

Saat Anda mengaktifkan 2FA, Tauthub juga memberi sekumpulan kode pemulihan sekali pakai. Kode ini menggantikan kode autentikator bila HP Anda hilang atau rusak.

## Yang tampil di layar

Di kanan judul kartu ada label status **Aktif** atau **Nonaktif**. Isi kartu mengikuti status itu:

- **Nonaktif**: kartu berisi penjelasan singkat dan tombol **Aktifkan verifikasi dua langkah**.
- **Aktif**: kartu menyatakan bahwa login memerlukan kode dari aplikasi autentikator Anda, dan berisi tombol **Nonaktifkan 2FA**.

> **Penting:** Kartu ini tidak membaca status 2FA yang tersimpan saat halaman dibuka. Setiap kali halaman dimuat, kartu mulai dari **Nonaktif** dengan tombol **Aktifkan verifikasi dua langkah**, termasuk bila 2FA sudah pernah Anda aktifkan. Jangan klik tombol itu hanya untuk memeriksa status. Tombol itu memulai penyiapan baru: kunci rahasia lama diganti dan 2FA yang sedang berjalan dimatikan sampai Anda memasukkan kode dari akun baru di aplikasi autentikator. Untuk memastikan status 2FA, buka jendela browser lain lalu masuk. Bila 2FA aktif, Anda diminta **Kode verifikasi** setelah memasukkan kata sandi.

## Cara menggunakan

### Cara mengaktifkan 2FA

Siapkan HP dengan aplikasi autentikator terpasang.

1. Klik **Konfigurasi** di sidebar, lalu klik tab **Keamanan**.
2. Gulir ke kartu **Verifikasi dua langkah (2FA)**. Pastikan Anda memang belum mengaktifkannya (lihat catatan Penting di atas).
3. Klik **Aktifkan verifikasi dua langkah**. Selama proses, tombol berubah menjadi **Menyiapkan…**.
4. Pada langkah **1. Tambahkan akun di aplikasi autentikator**, buka aplikasi autentikator di HP, tambah akun, lalu pindai kode QR. Bila tidak bisa memindai, ketik **Kunci rahasia (secret)** yang tampil di sebelah kode QR. Tautan **Buka tautan otpauth di aplikasi autentikator** juga tersedia bila Anda menyiapkan 2FA dari perangkat yang sama dengan aplikasinya.
5. Pada langkah **2. Verifikasi kode**, ketik kode 6 digit yang sedang tampil di aplikasi autentikator ke kolom kode. Kolom hanya menerima angka dan paling banyak 6 digit.
6. Klik **Aktifkan 2FA**. Selama proses, tombol berubah menjadi **Mengaktifkan…**. Tombol baru aktif setelah 6 digit terisi.
7. Tunggu pesan hijau **2FA aktif. Simpan kode pemulihan sekarang.** Status kartu berubah menjadi **Aktif**.
8. Simpan kode pemulihan yang tampil (lihat bagian berikutnya).

Klik **Batal** pada langkah penyiapan untuk membatalkan tanpa mengaktifkan 2FA. Kunci rahasia dan kode yang sudah Anda ketik dibuang.

> **Penting:** Perlakukan kunci rahasia seperti kata sandi. Siapa pun yang memilikinya bisa membuat kode autentikator untuk akun Anda. Jangan membagikannya.

### Cara menyimpan kode pemulihan

Setelah 2FA aktif, kotak peringatan **Simpan kode pemulihan Anda** menampilkan 10 kode. Keterangannya: "Kode ini hanya ditampilkan sekali. Simpan di tempat aman, dipakai jika HP Anda hilang."

1. Salin kesepuluh kode ke tempat yang aman, misalnya pengelola kata sandi atau catatan tercetak yang Anda simpan terpisah dari HP.
2. Pastikan Anda sudah menyimpannya sebelum meninggalkan halaman.

Halaman ini tidak punya tombol untuk melihat kode pemulihan lagi. Setelah Anda memuat ulang halaman atau menonaktifkan 2FA, kode yang tampil tidak bisa dibuka lagi. Setiap kode hanya bisa dipakai satu kali.

### Cara masuk ketika 2FA aktif

1. Masuk dengan email dan kata sandi seperti biasa. Halaman **Verifikasi Dua Langkah** muncul.
2. Ketik 6 digit kode dari aplikasi autentikator di kolom **Kode verifikasi**.
3. Bila HP tidak ada, ketik salah satu kode pemulihan di kolom yang sama. Kode itu hangus setelah dipakai.

Panduan lengkap masuk ada di [Daftar dan masuk](../../umum/02-daftar-dan-masuk.md).

### Cara menonaktifkan 2FA

Tombol **Nonaktifkan 2FA** hanya tampil saat kartu berstatus **Aktif**. Karena kartu mulai dari **Nonaktif** setiap kali halaman dimuat, tombol ini terlihat setelah Anda mengaktifkan 2FA di halaman yang sedang terbuka. Bila Anda perlu mematikan 2FA yang sudah aktif sejak sebelumnya, sampaikan lewat [Dukungan](../12-dukungan.md).

1. Klik **Nonaktifkan 2FA**. Selama proses, tombol berubah menjadi **Memproses…**.
2. Pada jendela yang muncul, ketik kata sandi Anda. Jendela berbunyi: "Masukkan password Anda untuk menonaktifkan verifikasi dua langkah:".
3. Klik OK di jendela itu.
4. Tunggu pesan hijau **Verifikasi dua langkah dinonaktifkan.** Status kartu kembali ke **Nonaktif**.

Menonaktifkan 2FA menghapus kunci rahasia dan seluruh kode pemulihan. Untuk mengaktifkannya lagi, ulangi "Cara mengaktifkan 2FA" dengan akun baru di aplikasi autentikator dan kode pemulihan yang baru.

## Aturan dan batasan

- **Kode verifikasi:** 6 digit angka, berasal dari aplikasi autentikator.
- **Kode pemulihan:** 10 kode, sekali pakai, hanya ditampilkan sekali.
- **Kata sandi untuk menonaktifkan:** wajib. Ini mencegah orang lain yang memakai sesi Anda mematikan 2FA.
- **Batas percobaan:** paling banyak 5 percobaan per menit untuk mengaktifkan, dan 5 per menit untuk menonaktifkan. Bila terlampaui, muncul "Terlalu banyak percobaan. Tunggu satu menit."
- **Satu akun autentikator per akun Tauthub:** menyiapkan ulang 2FA mengganti kunci rahasia lama.
- **Hak akses:** pengaturan ini hanya untuk akun Mitra yang sedang masuk.

## Pesan galat yang umum

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Masukkan kode 6 digit dari aplikasi autentikator. | Kode kurang dari 6 digit. | Ketik kode lengkap dari aplikasi. |
| Kode verifikasi salah. Coba lagi. | Kode salah atau sudah berganti. | Tunggu kode berikutnya muncul di aplikasi lalu ketik lagi. Pastikan jam HP Anda diatur otomatis. |
| Terlalu banyak percobaan. Tunggu satu menit. | Lebih dari 5 percobaan dalam satu menit. | Tunggu satu menit lalu coba lagi. |
| Password salah. | Kata sandi yang Anda ketik di jendela nonaktifkan keliru. | Ketik kata sandi akun yang benar. |
| 2FA belum aktif. | Server mencatat 2FA belum aktif saat Anda mencoba menonaktifkan. Kartu lalu berubah ke **Nonaktif**. | Tidak perlu tindakan. |
| Permintaan gagal. | Koneksi atau server bermasalah. | Coba lagi. |
| Gagal menonaktifkan 2FA. | Penonaktifan gagal. | Coba lagi. |

## Pertanyaan yang sering diajukan

**HP saya hilang. Bagaimana saya masuk?**
Masuk dengan email dan kata sandi, lalu ketik salah satu kode pemulihan di kolom **Kode verifikasi**. Setelah masuk, siapkan 2FA kembali dengan HP baru. Bila Anda tidak lagi memiliki kode pemulihan maupun akses ke aplikasi autentikator, Anda membutuhkan bantuan tim Tauthub. Panduan ini tidak mencakup prosedurnya.

**Mengapa kartu menunjukkan Nonaktif padahal saya sudah mengaktifkannya?**
Kartu tidak membaca status tersimpan saat dimuat. Baca catatan Penting di bagian atas halaman ini sebelum menekan tombol apa pun.

**Aplikasi autentikator apa yang bisa dipakai?**
Layar menyebut Google Authenticator dan Authy.

## Lihat juga

- [Konfigurasi](README.md)
- [Kata sandi](06-kata-sandi.md)
- [Daftar dan masuk](../../umum/02-daftar-dan-masuk.md)
- [Dukungan](../12-dukungan.md)
