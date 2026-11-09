# Integrasi, harga, dan Secure Direct

> Halaman ini menjelaskan bagian Integrasi & publikasi dan Harga & kerja sama di formulir media. Halaman ini juga menjelaskan Secure Direct, fitur yang membuat artikel klien terbit tanpa menunggu Anda menyetujuinya satu per satu.

**Lokasi:** menu **Daftar Media** › **Tambah Media** atau **Edit**; tombol **Ajukan Secure Direct** di bagian atas daftar
**Untuk:** Mitra

## Gambaran singkat

Bagian **Integrasi & publikasi** menentukan cara artikel klien sampai ke situs Anda. Bagian **Harga & kerja sama** menentukan harga per artikel. **Secure Direct** adalah fitur terpisah. Anda mengajukannya dari bagian atas daftar, lalu mengaktifkannya per media.

## Bagian Integrasi & publikasi

Teks di bawah judul bagian berbunyi: "Cara naskah masuk ke media: manual atau lewat API." API adalah sambungan otomatis antara Tauthub dan situs Anda. Pilih salah satu dari dua kartu. Pilihan awalnya **Manual**.

| | Manual | API |
|---|---|---|
| Cara terbit | Anda menempel artikel di situs sendiri, lalu mengirim alamat artikelnya di Tauthub. | Artikel dikirim otomatis ke situs Anda. |
| Sambungan | Tidak perlu. | Situs harus tersambung di menu **Integrasi**. |
| Teks di layar | "Status jadi Terbit otomatis saat URL dikirim." | "Status jadi Terbit otomatis setelah klik ACC." |

Langkah menyetujui artikel dan mengirim alamat artikel ada di [Publikasi](../03-publikasi/README.md).

### Platform tujuan

Setelah Anda memilih **API**, bagian **Platform Tujuan** muncul. Pilih satu: **WordPress** (terpilih otomatis saat Anda pertama kali memilih API), **EmDash**, **Blogspot**, atau **Custom Web**. Teks di sebelahnya menjelaskan kebutuhannya: "Butuh koneksi WordPress", "Butuh koneksi EmDash", "Butuh koneksi Blogspot", atau "Butuh endpoint API". Endpoint adalah alamat di situs Anda yang menerima kiriman artikel.

Kotak informasi di bawahnya memberi arahan sesuai platform:

- **WordPress**: setelah tim Tauthub menyetujui media, sambungkan di tab **WordPress** pada menu **Integrasi**.
- **EmDash**: setelah disetujui, buat token akses (kode rahasia pengganti kata sandi) di admin EmDash. Lalu sambungkan di tab **EmDash**.
- **Blogspot**: setelah disetujui, hubungkan akun Google dan pilih blog di tab **Blogspot**.
- **Custom Web**: setelah disetujui, atur alamat endpoint dan API Key (kode rahasia untuk sambungan), lalu uji sambungan di tab **Web Custom**.

Di menu **Integrasi**, tab tiap kanal terbuka setelah Anda menyetujui **Syarat & Ketentuan**. Langkah sambungan lengkapnya ada di [Integrasi](../13-integrasi/README.md).

### Persetujuan ketentuan

Mode **API** menampilkan kotak centang: "Saya telah membaca dan menyetujui Ketentuan Kerja Sama Mitra untuk mode integrasi." Anda wajib mencentang kotak ini sebelum media bisa disimpan. Tautan **Ketentuan Kerja Sama Mitra** membuka halaman ketentuan di tab baru. Kalau Anda mengedit media bermode API, kotak ini sudah tercentang.

### Setelah memilih API

Media bermode API baru terlihat di katalog klien setelah tim Tauthub menyetujuinya dan situsnya tersambung. Selama belum tersambung, baris media menampilkan **Belum terhubung** dan ikon mata abu-abu. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

Kalau Anda mengubah mode atau platform pada media yang sudah **Aktif**, media kembali ke pemeriksaan. Lihat [Mengedit media dan status](05-mengedit-media-dan-status.md).

## Bagian Harga & kerja sama

Teks di bawah judul bagian berbunyi: "Harga dan kesepakatan untuk kerja sama publikasi."

### Harga Pengajuan (IDR)

Anda wajib mengisi kotak ini dengan angka lebih dari 0. Ketik angkanya saja, misalnya `50000`. Kotak menambahkan titik pemisah ribuan sendiri dan membuang huruf atau tanda baca. Harga paling tinggi Rp1.000.000.000. Kalau lebih, penyimpanan ditolak dengan pesan "Invalid price".

Kalau kotak kosong atau 0, pesan merah muncul di bawahnya: "Harga wajib diisi. Media tanpa harga tidak akan tampil di katalog klien dan tidak bisa dipesan." Tombol simpan juga tidak bisa diklik.

Anda sendiri yang menentukan harga. Tidak ada tawar-menawar harga di layar ini.

### Kotak Kesepakatan

Kotak **Kesepakatan** menghitung langsung dari harga yang Anda ketik. Isinya perkiraan penghasilan Anda per artikel dan bagian Tauthub, lengkap dengan persentasenya. Angka yang sama tampil di kolom **Harga** pada daftar media. Aturan pembagian hasil ada di [Penghasilan](../09-penghasilan.md).

### Mengubah harga setelah media Aktif

- Anda mengajukan harga baru ke tim Tauthub. Media tetap tayang dengan harga lama sampai ada keputusan.
- Kalau Anda menurunkan harga sampai di bawah separuh harga yang berlaku, Tauthub menganggapnya mungkin salah ketik. Seluruh media kembali ke status **Menunggu review** dan sementara tidak terlihat di katalog.
- Anda tidak bisa mengosongkan harga lewat **Edit**. Pesannya: "Harga media wajib lebih dari 0. Media tanpa harga tidak bisa dipesan klien."

> **Catatan:** Kalau tim Tauthub sudah menetapkan harga sebagai final, penyimpanan ditolak dengan pesan "Harga sudah final (disepakati). Hubungi Admin untuk perubahan selanjutnya." Hubungi tim Tauthub lewat [Dukungan](../12-dukungan.md).

## Secure Direct

Secure Direct adalah mode terbit otomatis. Teks di layar menjelaskan: kalau Anda mencentangnya pada sebuah media, artikel dari klien langsung diproses dan terbit ke situs media tanpa menunggu persetujuan Anda. Kalau tidak dicentang, Anda harus menyetujui dulu setiap artikel.

Fitur ini punya dua lapis. Lapis pertama adalah izin untuk akun Anda. Tim Tauthub memberikannya setelah Anda mengajukan. Lapis kedua adalah kotak centang pada tiap media.

### Cara mengajukan izin Secure Direct

1. Buka menu **Daftar Media**, tab **Kelola Media**.
2. Klik **Ajukan Secure Direct**. Kotak dengan judul yang sama terbuka.
3. Baca ringkasan **Cara kerjanya**. Isinya: Anda mencentang kotak **Secure Direct** pada media pilihan, dan tim Tauthub memeriksa setiap pengaktifan lebih dulu untuk menjaga mutu distribusi.
4. Klik **Kirim pengajuan**.

Pesan hijau di dalam kotak menyatakan pengajuan terkirim. Anda menerima notifikasi di aplikasi setelah mode ini aktif.

Selama tim Tauthub memeriksa pengajuan:

- Tombol **Ajukan Secure Direct** menampilkan titik kuning.
- Kotak menampilkan "Menunggu persetujuan" dan hanya punya tombol **Tutup**, jadi Anda tidak bisa mengajukan dua kali.

Kalau pengajuan ditolak, kotak menampilkan "Pengajuan kamu sebelumnya ditolak." beserta catatan tim Tauthub kalau ada. Anda boleh mengajukan lagi dari kotak yang sama.

### Setelah izin aktif

Tombol **Ajukan Secure Direct** hilang. Kotak centang **Secure Direct** muncul di kolom **Mode Integrasi** pada tabel, atau di bagian bawah kartu di HP.

1. Centang kotak **Secure Direct** pada media yang ingin Anda terbitkan otomatis.
2. Pesan hijau "Mode otomatis diaktifkan untuk" disertai nama media muncul.
3. Hapus centang untuk mematikannya. Pesannya "Mode otomatis dimatikan untuk" disertai nama media.

Perubahan ini langsung berlaku tanpa pemeriksaan. Kotak tidak bisa diklik sebentar selama perubahan disimpan. Ikon tanda seru di sebelah kotak menampilkan penjelasan mode ini.

Klien yang memesan di media ber-Secure Direct tidak lagi menunggu persetujuan Anda. Di detail media, klien melihat jalur publikasi **Tauthub Secure Direct™**. Media tanpa Secure Direct tampil sebagai **Editorial Review (SLA Tracked)**. SLA adalah janji batas waktu layanan.

> **Penting:** Secure Direct bekerja lewat pengiriman otomatis ke situs Anda. Aktifkan hanya pada media bermode **API**. Pada media **Manual**, sistem tidak mengirim artikel ke situs Anda.

## Pertanyaan yang sering diajukan

**Mengapa tombol Ajukan Secure Direct tidak ada di halaman saya?**
Tombol hanya ada di tab **Kelola Media**, dan hilang kalau izin sudah aktif di akun Anda.

**Mengapa kotak Secure Direct tidak muncul di baris media?**
Kotak itu baru muncul setelah tim Tauthub mengaktifkan izin untuk akun Anda.

**Apakah media Manual saya harus tersambung di Integrasi?**
Tidak. Sambungan di menu **Integrasi** hanya perlu untuk media bermode **API**.

## Lihat juga

- [Menambah media dan identitas media](02-menambah-media-dan-identitas.md)
- [Mengedit media dan status](05-mengedit-media-dan-status.md)
- [Publikasi](../03-publikasi/README.md)
- [Integrasi](../13-integrasi/README.md)
