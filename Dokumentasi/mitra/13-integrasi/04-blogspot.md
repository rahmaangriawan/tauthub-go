# Integrasi Blogspot

> Panduan menghubungkan akun Google Anda ke Tauthub, memilih blog Blogspot tujuan, dan menguji koneksinya, agar artikel Klien terkirim otomatis ke blog Anda sebagai draf atau langsung terbit.

**Lokasi:** menu **Integrasi** › tab **Blogspot**
**Untuk:** Mitra (tab terbuka setelah Syarat & Ketentuan Integrasi disetujui)

## Gambaran singkat

Pada platform **Blogspot**, Anda tidak mengisi kata sandi di Tauthub. Anda memberi izin lewat akun Google, lalu memilih satu blog tujuan untuk setiap media. Koneksi dibuat per media, jadi dua media Blogspot bisa memakai dua blog yang berbeda.

Setelah tersambung, setiap artikel yang Anda setujui dikirim ke blog tujuan. Gambar artikel disimpan ke Google Drive Anda, sehingga tampilannya tidak bergantung pada server Tauthub.

## Prasyarat

- Media Blogspot sudah dibuat di [Daftar Media](../04-daftar-media/README.md) dengan mode **API** dan **Platform Tujuan** **Blogspot**, lalu disetujui tim Tauthub. Bila belum ada, klik **Buat media Blogspot** di tab ini.
- **Syarat & Ketentuan Integrasi** sudah disetujui. Lihat [Integrasi](README.md).
- Anda masuk ke akun Google yang mengelola blog tujuan.
- Tauthub sudah punya pengaturan otorisasi Google. Bila layar menyebut konfigurasi Google belum siap, hubungi [Dukungan](../12-dukungan.md).

## Cara menyambungkan

### Cara menghubungkan akun Google

1. Klik menu **Integrasi**, lalu buka tab **Blogspot**.
2. Cari kotak media Anda. Status awalnya **Belum terhubung**.
3. Klik **Hubungkan Blogspot**. Tombol berubah menjadi "Menghubungkan…", lalu Anda dibawa ke halaman persetujuan Google.
4. Pilih akun Google yang mengelola blog.
5. Setujui izin yang diminta. Izin itu mencakup Blogger, dan Google Drive yang dipakai Tauthub untuk menyimpan gambar artikel.
6. Tunggu sampai Anda dibawa kembali ke Tauthub.

Setelah kembali, dua hal bisa terjadi:

- **Akun hanya punya satu blog.** Blog itu dipilih otomatis, dan Anda kembali ke tab **Blogspot** dengan pesan `Blog "…" sudah terhubung.`
- **Akun punya lebih dari satu blog, atau tidak ada blog.** Anda dibawa ke halaman **Pilih Blog**. Lanjutkan ke bagian berikut.

### Cara memilih blog

Halaman **Pilih Blog** berjudul "Pilih Blog" dengan keterangan "Pilih blog Blogger yang ingin diintegrasikan dengan Tauthub."

1. Tunggu daftar blog tampil. Setiap blog menampilkan nama, jumlah postingan, deskripsi, dan alamatnya.
2. Klik blog yang Anda inginkan. Bila hanya ada satu blog, blog itu sudah terpilih.
3. Klik **Hubungkan Blog**. Tombol berubah menjadi "Menyimpan…".
4. Tunggu sampai Anda kembali ke tab **Blogspot**.

Bila Anda berubah pikiran, klik **Batal**. Anda kembali ke Dashboard Mitra, dan kredensial Google yang sudah tersimpan tetap ada. Selesaikan pilihan blog nanti dengan **Ganti blog** di tab **Blogspot**.

Bila akun tidak punya blog, halaman ini menampilkan "Tidak ada blog ditemukan di akun Google ini." Buat blog di Blogger atau pakai akun Google lain.

### Cara mengganti blog

1. Buka tab **Blogspot**.
2. Pada kotak media, klik **Ganti blog**. Jendela **Pilih blog** terbuka dengan keterangan "Blog tujuan untuk media …".
3. Klik blog yang Anda inginkan.
4. Klik **Gunakan blog ini**. Layar menampilkan `Blog "…" dipilih. Koneksi Blogspot aktif.`

### Cara menguji koneksi

1. Pada kotak media, klik **Uji koneksi**. Tombol nonaktif bila belum ada blog yang dipilih.
2. Baca pesan di bagian atas kartu.

### Cara menghubungkan ulang atau memutuskan

1. Klik **Hubungkan ulang** untuk mengulang persetujuan Google. Pakai ini bila kredensial tidak valid, atau bila artikel terbit dengan gambar yang masih bertaut ke Tauthub.
2. Klik **Putuskan** untuk menghapus kredensial dan blog tujuan. Konfirmasi pada pertanyaan `Putuskan koneksi Blogspot untuk "…"? Artikel berikutnya tidak akan terkirim.`
3. Setelah diputus, layar menampilkan "Koneksi Blogspot diputuskan." Media tidak tersambung lagi dan hilang dari Katalog Media sampai Anda menghubungkannya kembali.

> **Penting:** selesaikan satu proses persetujuan Google sebelum memulai yang lain. Memulai proses baru membatalkan proses sebelumnya yang belum selesai.

## Yang tampil di layar

Kartu **Blogspot** menampilkan jumlah media. Setiap kotak media memuat:

- Nama media, titik status, nama blog tujuan, dan "Dicek" beserta tanggal dan jam bila pernah diuji.
- Tombol sesuai keadaan. Bila kredensial belum ada: **Hubungkan Blogspot**. Bila kredensial sudah ada: **Ganti blog**, **Uji koneksi**, **Hubungkan ulang**, dan **Putuskan**.
- Catatan tambahan, misalnya pesan galat berwarna merah saat status gagal.

| Status | Artinya |
|---|---|
| **Terhubung** (hijau) | Koneksi aktif. Status ini sudah muncul begitu kredensial tersimpan, sebelum Anda memilih blog. |
| **Koneksi gagal** (merah) | Uji koneksi terakhir gagal. Pesan alasannya tampil di kotak media. |
| **Menguji…** (kuning) | Uji koneksi sedang berjalan. |
| **Belum terhubung** (abu-abu) | Belum ada kredensial, atau koneksi sudah diputus. |

Bila belum ada media Blogspot, kartu menampilkan "Belum ada media berplatform Blogspot. Buat media dengan platform Blogspot di halaman Media; setelah disetujui admin, hubungkan di sini" beserta tombol **Buat media Blogspot**. Tombol itu membuka formulir media dengan platform Blogspot sudah terpilih.

Bila kredensial sudah ada tetapi blog belum dipilih, kotak media menampilkan "Kredensial tersimpan. Pilih Ganti blog untuk menyelesaikan koneksi". Status bisa sudah menunjukkan **Terhubung**, tetapi koneksi baru lengkap setelah blog dipilih.

## Arti pesan hasil uji

Uji koneksi memeriksa bahwa kredensial Google masih berlaku dan bahwa blog tujuan masih bisa diakses dengan akun itu.

| Pesan | Artinya | Langkah Anda berikutnya |
|---|---|---|
| "Blogger terhubung" | Kredensial berlaku dan blog bisa diakses. | Tidak perlu tindakan. |
| "Belum ada blog dipilih — hubungkan dan pilih blog terlebih dahulu." | Blog tujuan belum dipilih. | Klik **Ganti blog** dan pilih blog. |
| "Kredensial Blogger belum tersimpan — hubungkan terlebih dahulu." | Belum ada kredensial. | Klik **Hubungkan Blogspot**. |
| "Kredensial Blogger tidak valid — perlu hubungkan ulang." | Izin Google kedaluwarsa atau dicabut. | Klik **Hubungkan ulang**. |
| "Blog tidak ditemukan atau tidak dapat diakses dengan akun ini." | Blog dihapus, atau akun Google tidak lagi mengelolanya. | Pilih blog lain atau hubungkan ulang dengan akun yang benar. |
| "Konfigurasi integrasi Google belum siap. Hubungi admin Tauthub." | Pengaturan di sisi Tauthub belum siap. | Hubungi [Dukungan](../12-dukungan.md). |

Pesan lain berasal dari Google, misalnya "permintaan Blogger gagal (403)". Hubungi [Dukungan](../12-dukungan.md) bila pesan itu menetap.

## Yang dikirim ke Blogspot

- **Judul** dan **isi artikel** dalam HTML.
- **Label:** satu label berisi kategori yang Anda pilih saat mengedit draf, atau kategori media Anda. Blogger memakai label bebas, jadi kategori baru langsung tampil.
- **Gambar utama:** diletakkan di bagian paling atas isi artikel.
- **Gambar:** gambar utama dan gambar di isi artikel disimpan ke Google Drive Anda, di folder yang namanya diawali "Tauthub - " diikuti nama media. Tauthub membuka akses baca publik pada gambar itu supaya pengunjung blog bisa melihatnya. Format yang didukung adalah JPG, JPEG, PNG, WebP, dan GIF. Sampai 20 gambar di isi artikel disimpan per artikel.
- **Status:** draf atau terbit, sesuai pilihan saat artikel disetujui di **Publikasi**. Alamat artikel yang dikembalikan Blogger langsung menjadi **URL Terbit**.

## Aturan dan batasan

- Setiap media memakai satu blog.
- Koneksi Blogspot dianggap tersambung (dan media tampil di Katalog Media) bila statusnya **Terhubung**. Status ini sudah dipasang sejak kredensial tersimpan, sebelum blog dipilih, jadi pilih blog segera.
- Bila persetujuan Google dibatalkan atau gagal, Anda dikembalikan ke Dashboard Mitra tanpa pesan sebab dan tanpa koneksi baru. Ulangi dari **Hubungkan Blogspot**.
- Koneksi lama yang dibuat sebelum izin Google Drive ditambahkan tetap bisa mengirim artikel, tetapi gambarnya masih bertaut ke Tauthub. Klik **Hubungkan ulang** agar gambar disimpan ke Google Drive Anda.
- Bila gambar utama gagal disimpan ke Google Drive, pengiriman berhenti dengan pesan "Gambar utama tidak bisa dititipkan ke Google Drive mitra". Gambar di isi artikel yang gagal tidak menghentikan pengiriman.

## Kendala umum

**Setelah menyetujui Google, saya kembali ke Ringkasan tanpa koneksi.**
Proses persetujuan gagal atau dibatalkan, atau ada proses lain yang dimulai sesudahnya. Buka tab **Blogspot** dan klik **Hubungkan Blogspot** lagi. Setujui semua izin yang diminta.

**Artikel gagal terkirim dengan pesan "Media Blogger belum terhubung atau belum memilih blog."**
Pilih blog lewat **Ganti blog**. Pesan galat pengiriman ada di bagian **Riwayat** pada detail publikasi. Lihat [Publikasi](../03-publikasi/README.md).

**Artikel gagal dengan pesan "Kredensial Blogger tidak valid — Perlu Hubungkan Ulang."**
Klik **Hubungkan ulang** di tab **Blogspot**.

**Artikel gagal dengan pesan "Akses ditolak Blogger — periksa izin akun."**
Akun Google yang terhubung tidak lagi punya izin menulis di blog. Hubungkan ulang dengan akun yang mengelola blog.

**Gambar di artikel masih bertaut ke Tauthub.**
Koneksi belum punya izin Google Drive. Klik **Hubungkan ulang**.

**Media Blogspot hilang dari Katalog Media.**
Status koneksi bukan **Terhubung**, atau koneksi sudah diputus. Hubungkan kembali, lalu jalankan **Uji koneksi**.

## Lihat juga

- [Integrasi](README.md): alur umum dan syarat tampil di Katalog Media.
- [Daftar Media](../04-daftar-media/README.md): membuat media Blogspot.
- [Publikasi](../03-publikasi/README.md)
- [Dukungan](../12-dukungan.md)
