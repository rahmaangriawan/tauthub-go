# Kata sandi

> Di tab **Keamanan**, Anda bisa mengganti kata sandi akun Mitra kapan saja. Layar memeriksa kata sandi lama dan aturan kata sandi baru saat Anda mengetik. Perangkat lain yang sedang masuk akan diminta masuk ulang.

**Lokasi:** **Konfigurasi** › tab **Keamanan** › kartu **Keamanan Akun & Kata Sandi**
**Untuk:** Mitra

## Gambaran singkat

Kartu **Keamanan Akun & Kata Sandi** punya keterangan: "Perbarui kata sandi akun penerbit media Anda secara berkala". Selama formulir belum dibuka, kartu hanya menampilkan tombol **Ubah Password**.

Fitur ini untuk mengganti kata sandi yang masih Anda ingat. Bila Anda lupa kata sandi Anda sekarang, pakai tautan **Lupa password?** di halaman masuk. Langkah masuk dan pemulihan akun ada di [Daftar dan masuk](../../umum/02-daftar-dan-masuk.md).

## Yang tampil di layar

Setelah Anda mengklik **Ubah Password**, tombol itu hilang dan formulir muncul dengan tiga kolom:

| Kolom | Fungsinya |
|---|---|
| **Kata Sandi Saat Ini** | Kata sandi yang Anda pakai sekarang. Teks petunjuk di kolom: "Masukkan kata sandi lama". |
| **Kata Sandi Baru** | Kata sandi pengganti. Teks petunjuk di kolom: "Minimal 10 karakter". |
| **Ulangi Kata Sandi Baru** | Ketik ulang kata sandi baru. Teks petunjuk di kolom: "Ketik ulang kata sandi baru". |

Di kanan setiap kolom ada ikon mata (keterangan **Tampilkan kata sandi**). Ikon itu menampilkan atau menyembunyikan isi kolom.

Di bawah kolom, muncul kotak pesan bila ada yang perlu diperbaiki:

- **Memeriksa password saat ini…** tampil sebentar setelah Anda berhenti mengetik di kolom **Kata Sandi Saat Ini**.
- **Password saat ini cocok.** (hijau) atau **Password saat ini tidak cocok dengan yang terdaftar.** (merah) menunjukkan hasil pemeriksaan itu.
- **Minimal 10 karakter dengan kombinasi huruf besar, huruf kecil, angka, dan karakter khusus.** tampil selama kata sandi baru belum memenuhi aturan.
- **Ulangi kata sandi belum sama dengan kata sandi baru.** tampil selama dua kolom baru belum sama.

Di bawah formulir ada tombol **Batal** dan **Perbarui Kata Sandi**.

## Cara menggunakan

### Cara mengganti kata sandi

1. Klik **Konfigurasi** di menu samping, lalu klik tab **Keamanan**.
2. Klik **Ubah Password** di kanan judul kartu **Keamanan Akun & Kata Sandi**. Formulir terbuka.
3. Isi **Kata Sandi Saat Ini**. Bila benar, dalam beberapa detik layar menampilkan **Password saat ini cocok.**
4. Isi **Kata Sandi Baru** sesuai aturan di bawah.
5. Isi **Ulangi Kata Sandi Baru** dengan kata sandi yang sama.
6. Klik **Perbarui Kata Sandi**. Selama proses, tombol berubah menjadi **Menyimpan…**.
7. Tunggu pesan hijau **Kata sandi berhasil diperbarui.** Formulir menutup dan semua kolom dikosongkan.

Klik **Batal** bila Anda ingin menutup formulir tanpa mengganti kata sandi.

Tombol **Perbarui Kata Sandi** tidak bisa ditekan sampai kata sandi baru memenuhi aturan dan kolom pengulangan sama dengan kata sandi baru.

### Aturan kata sandi baru

Kata sandi baru harus:

- terdiri dari minimal 10 karakter,
- memuat huruf besar,
- memuat huruf kecil,
- memuat angka, dan
- memuat satu karakter khusus, yaitu karakter selain huruf dan angka, misalnya tanda seru atau garis bawah.

Panjang maksimalnya 128 karakter.

## Yang terjadi setelah kata sandi diganti

- Kata sandi baru langsung berlaku untuk masuk berikutnya.
- Perangkat lain dikeluarkan dari akun. Perangkat yang sedang masuk dengan akun Anda akan diminta masuk ulang dengan kata sandi baru. Perangkat yang sedang Anda pakai tetap masuk.
- Jika [verifikasi dua langkah](07-verifikasi-dua-langkah.md) menyala, pengaturan 2FA tidak berubah.

## Aturan dan batasan

- **Panjang:** 10 sampai 128 karakter.
- **Isi kata sandi:** layar mewajibkan huruf besar, huruf kecil, angka, dan karakter khusus.
- **Pemeriksaan kata sandi lama:** berjalan sendiri sekitar 0,7 detik setelah Anda berhenti mengetik. Pemeriksaan ini hanya memberi tahu Anda. Sistem tetap memeriksa lagi saat Anda menekan **Perbarui Kata Sandi**.
- **Batas percobaan:** bila Anda mencoba terlalu sering dalam waktu singkat, muncul "Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi."
- **Siapa yang bisa mengganti:** Anda hanya bisa mengganti kata sandi akun yang sedang masuk.

## Pesan kesalahan yang umum

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Password saat ini tidak cocok dengan yang terdaftar. | Kata sandi lama yang Anda ketik salah. | Ketik ulang. Bila lupa, pakai **Lupa password?** di halaman masuk. |
| Password baru minimal 10 karakter. | Kata sandi baru kurang dari 10 karakter. | Pakai kata sandi yang lebih panjang. |
| Konfirmasi password baru tidak cocok. | Kolom pengulangan berbeda dengan kata sandi baru. | Ketik ulang keduanya dengan teliti. |
| Current password is incorrect | Sistem menolak kata sandi lama. Pesan ini tampil dalam bahasa Inggris. | Ketik kata sandi Anda yang sekarang dengan benar. |
| Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi. | Terlalu banyak percobaan dalam waktu singkat. | Tunggu satu menit lalu coba lagi. |
| Gagal mengubah password. | Sambungan internet atau sistem sedang bermasalah. | Klik **Perbarui Kata Sandi** lagi. |

## Pertanyaan yang sering diajukan

**Mengapa tombol Perbarui Kata Sandi tidak bisa diklik?**
Kata sandi baru belum memenuhi aturan, atau kolom pengulangan belum sama. Baca pesan merah di bawah kolom untuk melihat apa yang masih kurang.

**Apakah saya perlu keluar dan masuk lagi di perangkat ini?**
Tidak. Perangkat yang sedang Anda pakai tetap masuk. Hanya perangkat lain yang diminta masuk ulang.

**Apakah kata sandi baru boleh sama dengan yang lama?**
Layar tidak menolak kata sandi baru yang sama dengan yang lama. Tetapi sebaiknya Anda memakai kata sandi yang berbeda.

## Lihat juga

- [Konfigurasi](README.md)
- [Verifikasi dua langkah](07-verifikasi-dua-langkah.md)
- [Daftar dan masuk](../../umum/02-daftar-dan-masuk.md)
