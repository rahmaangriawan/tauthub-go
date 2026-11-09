# Verifikasi domain dan pengajuan Dewan Pers

> Verifikasi domain membuktikan Anda mengelola situs media, sedangkan pengajuan Dewan Pers mendaftarkan nomor sertifikat Dewan Pers untuk ditinjau. Keduanya menambah kepercayaan klien dan memengaruhi garansi tautan serta Impor XLSX.

**Lokasi:** menu **Daftar Media**, kolom **Verifikasi Kepemilikan** (ikon di baris media)
**Untuk:** Mitra

## Gambaran singkat

Kedua verifikasi dikerjakan di satu dialog bernama **Verifikasi Kepemilikan**. Dialog ini punya dua tab: **Verifikasi Domain** dan **Dewan Pers**. Nama media tampil di bawah judul dialog.

| Verifikasi | Yang dibuktikan | Dampaknya |
|---|---|---|
| Domain | Anda mengelola situs media. Diperiksa otomatis oleh sistem. | Klien melihat tanda **Terverifikasi Tauthub** dan bisa menyaring dengan **Domain terverifikasi**. Klaim garansi Link Guardian oleh klien hanya berlaku untuk domain terverifikasi. Impor XLSX terbuka bila ada media Aktif yang domainnya terverifikasi. |
| Dewan Pers | Media punya nomor sertifikat Dewan Pers. Ditinjau tim Tauthub. | Klien melihat tanda **Terverifikasi Dewan Pers** dan bisa menyaring dengan **Dewan Pers**. |

Verifikasi domain tidak menjadi syarat tampil di katalog. Media tanpa verifikasi tetap tampil bila syarat lain terpenuhi, tetapi klien melihat **Tidak Digaransi** pada klaim Link Guardian untuk media itu. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

## Membuka dialog verifikasi

Klik ikon di kolom **Verifikasi Kepemilikan** pada baris media. Dialog membuka bagian yang masih kurang lebih dulu:

- Bila domain belum terverifikasi, tab **Verifikasi Domain** terbuka.
- Bila domain sudah terverifikasi, tab **Dewan Pers** terbuka.

Anda bisa berpindah tab kapan saja. Tutup dialog dengan tombol ×, tombol Esc, atau klik di luar kotak dialog. Tab **Dewan Pers** juga punya tombol **Batal**.

## Verifikasi domain

Isi **Website / Domain** pada media lebih dulu. Bila kosong, verifikasi ditolak dengan pesan "Isi Website / Domain media terlebih dahulu". Mengubah alamat situs pada media yang sudah **Aktif** membuat media kembali ke tinjauan. Lihat [Mengedit media dan status](05-mengedit-media-dan-status.md).

### Cara memverifikasi domain dengan meta tag

1. Klik ikon di kolom **Verifikasi Kepemilikan**.
2. Baca petunjuk di dialog: tempel meta tag di bagian `head` halaman utama situs, lalu klik **Verifikasi Sekarang**.
3. Klik **Salin meta tag**. Tombol berubah menjadi **Tersalin** sebentar.
4. Tempel meta tag di bagian `head` halaman utama situs Anda. Minta bantuan pengelola situs atau penyedia hosting bila Anda tidak punya akses ke kode situs.
5. Klik **Verifikasi Sekarang**. Tombol berubah menjadi "Memeriksa…" selama pemeriksaan.
6. Baca hasilnya di sebelah tombol.

Bentuk meta tag selalu seperti contoh berikut. Kode di dalamnya unik untuk media Anda dan tetap sama setiap kali dialog dibuka.

`<meta name="tauthub-verification" content="KODE_ANDA" />`

Tombol **Salin token** menyalin kodenya saja. Kedua tombol salin nonaktif sampai kode selesai dimuat. Bila penyalinan gagal, layar menampilkan "Gagal menyalin. Salin manual dari kotak teks."

### Alternatif lewat DNS TXT

Bila Anda sulit mengubah kode situs, atau situs memblokir pemeriksaan otomatis dengan firewall (WAF), tambahkan TXT record di DNS domain utama tanpa `www`. Dialog menampilkan baris yang harus ditambahkan, dengan bentuk seperti ini:

`_tauthub-verify   TXT   "KODE_ANDA"`

Perubahan DNS bisa memerlukan waktu sampai 24 jam sebelum terbaca sistem. Setelah itu klik **Verifikasi Sekarang**. Pemeriksaan memakai TXT record sebagai cadangan bila kode tidak ditemukan di halaman.

### Hasil pemeriksaan

| Tulisan di layar | Artinya | Langkah Anda |
|---|---|---|
| Berhasil, domain terverifikasi. | Kode ditemukan. Ikon di kolom berubah menjadi perisai penuh. | Tidak ada. |
| Meta tag belum ditemukan di halaman utama. Pastikan sudah diunggah lalu coba lagi. | Halaman terbaca, tetapi kode belum ada. | Pastikan meta tag atau TXT record sudah terpasang, lalu coba lagi. |
| Status verifikasi: token_missing | Halaman memuat penanda verifikasi, tetapi kodenya berbeda. | Salin ulang kode terbaru dari dialog dan ganti yang lama. |
| Status verifikasi: unreachable | Situs tidak bisa dibuka dari server Tauthub. | Pastikan situs aktif dan alamatnya benar. |
| Status verifikasi: blocked_by_waf | Situs aktif, tetapi menolak akses dari server Tauthub. | Gunakan alternatif DNS TXT. |
| Status verifikasi: tls_error | Sertifikat HTTPS situs bermasalah. | Perbaiki sertifikat situs, atau gunakan DNS TXT. |
| Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi. | Anda mengeklik **Verifikasi Sekarang** lebih dari 5 kali dalam satu menit. | Tunggu satu menit. |
| Verifikasi domain gagal. | Pemeriksaan tidak berhasil dijalankan. | Coba lagi, lalu hubungi [Dukungan](../12-dukungan.md) bila berulang. |

Bila domain sudah terverifikasi, dialog menampilkan "Domain sudah terverifikasi." dan tombol **Verifikasi Sekarang** tidak ada.

### Setelah domain terverifikasi

- Jangan menghapus meta tag atau TXT record dari situs.
- Tauthub memeriksa ulang domain secara berkala. Bila pemeriksaan gagal tiga kali berturut-turut pada domain yang sudah terverifikasi, status verifikasi dicabut dan Anda perlu memverifikasi lagi.
- Pencabutan juga mengunci kembali **Impor XLSX** bila tidak ada media Aktif lain yang domainnya terverifikasi. Lihat [Impor XLSX](08-impor-xlsx.md).

## Pengajuan Dewan Pers

Ajukan bila media Anda punya sertifikat verifikasi Dewan Pers. Tim Tauthub meninjau pengajuannya.

### Format nomor sertifikat

Nomor harus berbentuk `nomor/DP-Verifikasi/K/angka Romawi/tahun`, misalnya `123/DP-Verifikasi/K/X/2025`. Angka Romawi berkisar I sampai XII dan tahun terdiri atas empat angka. Bila format salah, layar menampilkan "Format nomor sertifikat tidak valid." disertai contoh format.

### Cara mengajukan Dewan Pers

1. Klik ikon di kolom **Verifikasi Kepemilikan**, lalu buka tab **Dewan Pers** bila belum terbuka.
2. Isi **Nomor sertifikat**. Kolom ini wajib dan minimal 3 karakter.
3. Unggah dokumen sertifikat bila ada. Tarik dan lepas berkas ke kotak yang tersedia, atau klik kotak untuk memilih berkas. Dokumen bersifat opsional.
4. Klik **Ajukan Verifikasi**. Tombol nonaktif selama nomor kurang dari 3 karakter atau saat pengiriman berjalan ("Mengirim…").

Pesan hijau "Pengajuan verifikasi Dewan Pers terkirim dan menunggu review admin." muncul, dan dialog menutup.

Aturan dokumen: format PDF, JPG, atau PNG, ukuran maksimal 10 MB. Bila berkas tidak memenuhi syarat, layar menampilkan "Format dokumen harus PDF, JPG, atau PNG." atau "Ukuran file maksimal 10 MB." Klik **Hapus** di samping nama berkas untuk membatalkan pilihan.

Bila nomor tersimpan tetapi dokumen gagal terunggah, layar menampilkan "Sertifikat tersimpan, tetapi unggah dokumen gagal." Tutup dialog dan buka lagi, hapus pengajuan, lalu ajukan lagi bersama dokumennya.

Bila sudah ada pengajuan, bagian atas tab menampilkan statusnya, tanggal pengajuan, dan tombol **Lihat dokumen** untuk membuka dokumen yang tersimpan. Mengunggah berkas baru menggantikan dokumen lama.

### Status Dewan Pers dan artinya

| Status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **Menunggu** | Pengajuan sedang ditinjau tim Tauthub. | Tunggu. Untuk mengganti nomor, hapus pengajuan lalu ajukan lagi. |
| **Terverifikasi** | Pengajuan disetujui. Klien melihat tanda Dewan Pers. | Tidak ada. Anda tidak bisa menghapusnya sendiri. |
| **Ditolak** | Tim Tauthub menolak. Alasannya dikirim lewat notifikasi dan email. | Perbaiki nomor atau dokumen, lalu ajukan lagi. |
| **Konflik** | Nomor yang sama sudah dipakai media lain yang menunggu atau terverifikasi. Perlu pemeriksaan tim Tauthub. | Periksa nomor Anda. Hapus pengajuan bila salah ketik. |

Bila belum ada pengajuan, ringkasan di kolom menyebut "Dewan Pers: belum diajukan". Keputusan tim Tauthub datang sebagai notifikasi berjudul "Verifikasi Dewan Pers disetujui" atau "Verifikasi Dewan Pers ditolak".

### Mencabut pengajuan

Anda bisa mencabut pengajuan yang berstatus **Menunggu**, **Ditolak**, atau **Konflik**.

1. Buka tab **Dewan Pers** pada dialog verifikasi.
2. Klik **Hapus pengajuan**. Tombol berubah menjadi **Ya, hapus pengajuan**.
3. Klik **Ya, hapus pengajuan** untuk mengonfirmasi.

Pesan hijau menyatakan pengajuan dihapus dan Anda bisa mengajukan ulang kapan saja. Bila tidak ada pengajuan lain yang tersisa, nomor sertifikat dan tanda Dewan Pers pada media ikut hilang.

Tombol **Hapus pengajuan** tidak muncul untuk sertifikat **Terverifikasi**. Pencabutan sertifikat yang sudah sah dilakukan oleh tim Tauthub lewat [Dukungan](../12-dukungan.md).

> **Penting:** Mengajukan nomor yang berbeda pada media yang sudah **Terverifikasi** membuat statusnya kembali **Menunggu** sampai nomor baru disetujui. Selama itu tanda Dewan Pers tidak tampil di katalog. Mengajukan nomor yang sama tidak mengubah apa pun.

### Pesan galat Dewan Pers

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Format nomor sertifikat tidak valid. | Nomor tidak sesuai format. | Salin nomor persis dari sertifikat. |
| Nomor sertifikat ini sudah digunakan oleh media lain dan memerlukan pemeriksaan Admin. | Nomor sudah terdaftar pada media lain. | Periksa nomor. Tunggu pemeriksaan tim Tauthub, atau hapus pengajuan. |
| Verifikasi Dewan Pers sedang menunggu review Admin. Anda belum bisa mengubah nomor sertifikat. | Masih ada pengajuan berstatus **Menunggu**. | Hapus pengajuan lebih dulu, lalu ajukan lagi. |
| Pengajuan Dewan Pers gagal. | Pengiriman tidak berhasil. | Coba lagi dan hubungi [Dukungan](../12-dukungan.md) bila berulang. |

Mitra yang memakai [Impor XLSX](08-impor-xlsx.md) juga bisa mengisi kolom **Nomor Sertifikat Dewan Pers**. Nomor dalam berkas diajukan otomatis setelah media dibuat.

## Pertanyaan yang sering diajukan

**Apakah saya perlu memverifikasi domain sebelum media disetujui?**
Tidak. Verifikasi domain bisa dikerjakan kapan saja, termasuk saat media masih **Menunggu review**.

**Bolehkah saya memakai meta tag dan DNS TXT sekaligus?**
Boleh. Pemeriksaan menerima salah satunya.

**Berapa lama tim Tauthub meninjau Dewan Pers?**
Layar tidak menyebut batas waktu. Anda menerima notifikasi begitu ada keputusan.

## Lihat juga

- [Membaca Daftar Media](01-membaca-daftar-media.md)
- [Link Guardian](../05-link-guardian.md)
- [Katalog Media (tampilan Klien)](../../klien/02-katalog-media/README.md)
