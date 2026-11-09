# Integrasi, harga, dan Secure Direct

> Halaman ini menjelaskan bagian Integrasi & publikasi dan Harga & kerja sama pada formulir media, serta fitur Secure Direct yang membuat artikel klien terbit tanpa menunggu persetujuan Anda satu per satu.

**Lokasi:** menu **Daftar Media** › **Tambah Media** atau **Edit**; tombol **Ajukan Secure Direct** di bagian atas daftar
**Untuk:** Mitra

## Gambaran singkat

Bagian **Integrasi & publikasi** menentukan cara artikel klien sampai ke situs Anda. Bagian **Harga & kerja sama** menentukan harga per artikel. **Secure Direct** adalah fitur terpisah yang diajukan dari bagian atas daftar dan diaktifkan per media.

## Bagian Integrasi & publikasi

Teks di bawah judul bagian: "Cara naskah masuk ke media: manual atau lewat API." Pilih salah satu dari dua kartu. Pilihan awalnya **Manual**.

| | Manual | API |
|---|---|---|
| Cara terbit | Anda menempel artikel di situs sendiri, lalu mengirim alamat artikelnya di Tauthub. | Artikel dikirim otomatis ke situs Anda. |
| Koneksi | Tidak perlu. | Situs harus tersambung di menu **Integrasi**. |
| Teks di layar | "Status jadi Terbit otomatis saat URL dikirim." | "Status jadi Terbit otomatis setelah klik ACC." |

Langkah menyetujui artikel dan mengirim alamat artikel dijelaskan di [Publikasi](../03-publikasi/README.md).

### Platform tujuan

Setelah memilih **API**, bagian **Platform Tujuan** muncul. Pilih satu: **WordPress** (terpilih otomatis saat Anda pertama kali memilih API), **EmDash**, **Blogspot**, atau **Custom Web**. Teks di sebelahnya menjelaskan kebutuhannya: "Butuh koneksi WordPress", "Butuh koneksi EmDash", "Butuh koneksi Blogspot", atau "Butuh endpoint API".

Kotak informasi di bawahnya memberi arahan sesuai platform:

- **WordPress**: setelah media disetujui tim Tauthub, koneksikan di tab **WordPress** pada menu **Integrasi**.
- **EmDash**: setelah disetujui, buat token akses di admin EmDash, lalu koneksikan di tab **EmDash**.
- **Blogspot**: setelah disetujui, hubungkan akun Google dan pilih blog di tab **Blogspot**.
- **Custom Web**: setelah disetujui, atur alamat endpoint, API Key, dan uji koneksi di tab **Web Custom**.

Di menu **Integrasi**, tab kanal terbuka setelah Anda menyetujui **Syarat & Ketentuan**. Langkah koneksi lengkapnya ada di [Integrasi](../13-integrasi/README.md).

### Persetujuan ketentuan

Mode **API** menampilkan kotak centang: "Saya telah membaca dan menyetujui Ketentuan Kerja Sama Mitra untuk mode integrasi." Kotak ini wajib dicentang sebelum media bisa disimpan. Tautan **Ketentuan Kerja Sama Mitra** membuka halaman ketentuan di tab baru. Saat Anda mengedit media bermode API, kotak ini sudah tercentang.

### Setelah memilih API

Media bermode API baru tampil di katalog klien setelah disetujui tim Tauthub dan situsnya tersambung. Selama belum tersambung, baris media menampilkan **Belum terhubung** dan ikon mata abu-abu. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

Mengubah mode atau platform pada media yang sudah **Aktif** membuat media kembali ke tinjauan. Lihat [Mengedit media dan status](05-mengedit-media-dan-status.md).

## Bagian Harga & kerja sama

Teks di bawah judul bagian: "Harga dan kesepakatan untuk kerja sama publikasi."

### Harga Pengajuan (IDR)

Kolom ini wajib diisi dengan angka lebih dari 0. Ketik angkanya saja, misalnya `50000`. Kolom menambahkan pemisah ribuan otomatis dan membuang huruf atau tanda baca. Harga maksimal Rp1.000.000.000. Bila lebih, penyimpanan ditolak dengan pesan "Invalid price".

Bila kolom kosong atau 0, pesan merah muncul di bawahnya: "Harga wajib diisi. Media tanpa harga tidak akan tampil di katalog klien dan tidak bisa dipesan." Tombol simpan juga nonaktif.

Harga ditentukan oleh Anda. Tidak ada negosiasi harga di layar ini.

### Kotak Kesepakatan

Kotak **Kesepakatan** menghitung langsung dari harga yang Anda ketik. Isinya perkiraan penghasilan Anda per artikel dan bagian Tauthub, lengkap dengan persentasenya. Angka yang sama tampil di kolom **Harga** pada daftar media. Aturan pembagian hasil dijelaskan di [Penghasilan](../09-penghasilan.md).

### Mengubah harga setelah media Aktif

- Harga baru diajukan ke tim Tauthub. Media tetap tayang dengan harga lama sampai ada keputusan.
- Menurunkan harga hingga di bawah separuh harga yang berlaku dianggap berisiko salah ketik. Dalam kasus ini seluruh media kembali ke status **Menunggu review** dan sementara tidak tampil di katalog.
- Harga tidak bisa dikosongkan lewat **Edit**. Pesannya: "Harga media wajib lebih dari 0. Media tanpa harga tidak bisa dipesan klien."

> **Catatan:** Bila tim Tauthub sudah menetapkan harga sebagai final, penyimpanan ditolak dengan pesan "Harga sudah final (disepakati). Hubungi Admin untuk perubahan selanjutnya." Hubungi tim Tauthub lewat [Dukungan](../12-dukungan.md).

## Secure Direct

Secure Direct adalah mode terbit otomatis. Teks di layar menjelaskan: bila dicentang pada sebuah media, artikel dari klien langsung diproses dan terbit ke situs media tanpa menunggu persetujuan Anda. Bila tidak dicentang, Anda harus menyetujui dulu setiap artikel.

Fitur ini punya dua lapis. Lapis pertama adalah izin untuk akun Anda, yang diberikan tim Tauthub setelah Anda mengajukan. Lapis kedua adalah kotak centang pada tiap media.

### Cara mengajukan izin Secure Direct

1. Buka menu **Daftar Media**, tab **Kelola Media**.
2. Klik **Ajukan Secure Direct**. Dialog dengan judul yang sama terbuka.
3. Baca ringkasan **Cara kerjanya**. Isinya: kotak **Secure Direct** dicentang pada media pilihan, dan setiap pengaktifan ditinjau tim Tauthub lebih dulu untuk menjaga mutu distribusi.
4. Klik **Kirim pengajuan**.

Pesan hijau di dalam dialog menyatakan pengajuan terkirim. Anda menerima notifikasi di aplikasi setelah mode ini diaktifkan.

Selama pengajuan ditinjau:

- Tombol **Ajukan Secure Direct** menampilkan titik kuning.
- Dialog menampilkan "Menunggu persetujuan" dan hanya punya tombol **Tutup**, sehingga Anda tidak bisa mengajukan dua kali.

Bila pengajuan ditolak, dialog menampilkan "Pengajuan kamu sebelumnya ditolak." beserta catatan tim Tauthub bila ada. Anda boleh mengajukan lagi dari dialog yang sama.

### Setelah izin aktif

Tombol **Ajukan Secure Direct** hilang. Kotak centang **Secure Direct** muncul di kolom **Mode Integrasi** pada tabel, atau di bagian bawah kartu pada HP.

1. Centang kotak **Secure Direct** pada media yang ingin diterbitkan otomatis.
2. Pesan hijau "Mode otomatis diaktifkan untuk" disertai nama media muncul.
3. Hapus centang untuk mematikannya. Pesannya "Mode otomatis dimatikan untuk" disertai nama media.

Perubahan ini berlaku langsung tanpa tinjauan. Kotak nonaktif sebentar selama perubahan disimpan. Ikon tanda seru di sebelah kotak menampilkan penjelasan mode ini.

Klien yang memesan di media ber-Secure Direct tidak lagi menunggu persetujuan Anda. Di detail media, klien melihat jalur publikasi **Tauthub Secure Direct™**. Media tanpa Secure Direct tampil sebagai **Editorial Review (SLA Tracked)**.

> **Penting:** Secure Direct bekerja lewat pengiriman otomatis ke situs Anda. Aktifkan hanya pada media bermode **API**. Pada media **Manual**, sistem tidak mengirim artikel ke situs Anda.

## Pertanyaan yang sering diajukan

**Mengapa tombol Ajukan Secure Direct tidak ada di halaman saya?**
Tombol hanya muncul di tab **Kelola Media**, dan hilang bila izin sudah aktif di akun Anda.

**Mengapa kotak Secure Direct tidak muncul di baris media?**
Kotak itu baru muncul setelah tim Tauthub mengaktifkan izin untuk akun Anda.

**Apakah media Manual saya harus tersambung di Integrasi?**
Tidak. Koneksi di menu **Integrasi** hanya diperlukan untuk media bermode **API**.

## Lihat juga

- [Menambah media dan identitas media](02-menambah-media-dan-identitas.md)
- [Mengedit media dan status](05-mengedit-media-dan-status.md)
- [Publikasi](../03-publikasi/README.md)
- [Integrasi](../13-integrasi/README.md)
