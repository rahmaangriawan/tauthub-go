# Verifikasi domain dan pengajuan Dewan Pers

> Verifikasi domain membuktikan bahwa Anda mengelola situs media. Pengajuan Dewan Pers mendaftarkan nomor sertifikat Dewan Pers Anda untuk diperiksa. Keduanya menambah kepercayaan klien dan memengaruhi garansi tautan serta Impor XLSX.

**Lokasi:** menu **Daftar Media**, kolom **Verifikasi Kepemilikan** (ikon di baris media)
**Untuk:** Mitra

## Gambaran singkat

Anda mengerjakan kedua verifikasi di satu kotak bernama **Verifikasi Kepemilikan**. Kotak ini punya dua tab: **Verifikasi Domain** dan **Dewan Pers**. Nama media tampil di bawah judul kotak. Domain adalah alamat situs Anda, misalnya arusberita.com.

| Verifikasi | Yang dibuktikan | Dampaknya |
|---|---|---|
| Domain | Anda mengelola situs media. Sistem memeriksanya otomatis. | Klien melihat tanda **Terverifikasi Tauthub** dan bisa menyaring dengan **Domain terverifikasi**. Klien hanya bisa mengajukan klaim garansi Link Guardian untuk domain yang terverifikasi. Impor XLSX terbuka kalau ada media Aktif yang domainnya terverifikasi. |
| Dewan Pers | Media punya nomor sertifikat Dewan Pers. Tim Tauthub memeriksanya. | Klien melihat tanda **Terverifikasi Dewan Pers** dan bisa menyaring dengan **Dewan Pers**. |

Verifikasi domain tidak menjadi syarat tampil di katalog. Media tanpa verifikasi tetap tampil kalau syarat lain terpenuhi. Tetapi klien melihat tulisan **Tidak Digaransi** pada klaim Link Guardian untuk media itu. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

## Membuka kotak verifikasi

Klik ikon di kolom **Verifikasi Kepemilikan** pada baris media. Kotak membuka bagian yang masih kurang lebih dulu:

- Kalau domain belum terverifikasi, tab **Verifikasi Domain** terbuka.
- Kalau domain sudah terverifikasi, tab **Dewan Pers** terbuka.

Anda bisa pindah tab kapan saja. Tutup kotak dengan tombol ×, tombol Esc, atau klik di luar kotak. Tab **Dewan Pers** juga punya tombol **Batal**.

## Verifikasi domain

Isi **Website / Domain** pada media lebih dulu. Kalau kosong, verifikasi ditolak dengan pesan "Isi Website / Domain media terlebih dahulu". Kalau Anda mengubah alamat situs pada media yang sudah **Aktif**, media kembali ke pemeriksaan. Lihat [Mengedit media dan status](05-mengedit-media-dan-status.md).

### Cara memverifikasi domain dengan meta tag

Meta tag adalah satu baris kode kecil yang tidak terlihat pengunjung. Anda menempelkannya di situs sebagai bukti Anda pemilik situs.

1. Klik ikon di kolom **Verifikasi Kepemilikan**.
2. Baca petunjuk di kotak: tempel meta tag di bagian `head` (bagian kepala kode) halaman utama situs, lalu klik **Verifikasi Sekarang**.
3. Klik **Salin meta tag**. Tombol berubah menjadi **Tersalin** sebentar.
4. Tempel meta tag di bagian `head` halaman utama situs Anda. Kalau Anda tidak bisa membuka kode situs, minta bantuan pengelola situs atau penyedia hosting.
5. Klik **Verifikasi Sekarang**. Selama pemeriksaan, tombol berubah menjadi "Memeriksa…".
6. Baca hasilnya di sebelah tombol.

Meta tag selalu berbentuk seperti contoh berikut. Kode di dalamnya khusus untuk media Anda dan tetap sama setiap kali Anda membuka kotak ini.

`<meta name="tauthub-verification" content="KODE_ANDA" />`

Tombol **Salin token** menyalin kodenya saja. Kedua tombol salin tidak bisa diklik sampai kode selesai dimuat. Kalau penyalinan gagal, layar menampilkan "Gagal menyalin. Salin manual dari kotak teks."

### Cara lain lewat DNS TXT

DNS adalah buku alamat internet yang mengarahkan nama situs ke komputer yang menyimpannya. Cara ini cocok kalau Anda sulit mengubah kode situs. Cara ini juga cocok kalau situs memblokir pemeriksaan otomatis dengan firewall (WAF, penjaga yang menolak pengunjung otomatis). Tambahkan TXT record di DNS domain utama tanpa `www`. Kotak menampilkan baris yang harus Anda tambahkan, dengan bentuk seperti ini:

`_tauthub-verify   TXT   "KODE_ANDA"`

Perubahan DNS bisa butuh waktu sampai 24 jam sebelum sistem bisa membacanya. Setelah itu klik **Verifikasi Sekarang**. Kalau kode tidak ada di halaman, pemeriksaan memakai TXT record sebagai cadangan.

### Hasil pemeriksaan

| Tulisan di layar | Artinya | Langkah Anda |
|---|---|---|
| Berhasil, domain terverifikasi. | Kode ditemukan. Ikon di kolom berubah menjadi perisai penuh. | Tidak ada. |
| Meta tag belum ditemukan di halaman utama. Pastikan sudah diunggah lalu coba lagi. | Halaman terbaca, tetapi kode belum ada. | Pastikan meta tag atau TXT record sudah terpasang, lalu coba lagi. |
| Status verifikasi: token_missing | Halaman memuat penanda verifikasi, tetapi kodenya berbeda. | Salin ulang kode terbaru dari kotak dan ganti yang lama. |
| Status verifikasi: unreachable | Server Tauthub tidak bisa membuka situs. | Pastikan situs aktif dan alamatnya benar. |
| Status verifikasi: blocked_by_waf | Situs aktif, tetapi menolak akses dari server Tauthub. | Pakai cara DNS TXT. |
| Status verifikasi: tls_error | Sertifikat HTTPS (kunci pengaman) situs bermasalah. | Perbaiki sertifikat situs, atau pakai DNS TXT. |
| Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi. | Anda mengeklik **Verifikasi Sekarang** lebih dari 5 kali dalam satu menit. | Tunggu satu menit. |
| Verifikasi domain gagal. | Pemeriksaan tidak berhasil dijalankan. | Coba lagi. Kalau terjadi lagi, hubungi [Dukungan](../12-dukungan.md). |

Kalau domain sudah terverifikasi, kotak menampilkan "Domain sudah terverifikasi." dan tombol **Verifikasi Sekarang** tidak ada.

### Setelah domain terverifikasi

- Jangan menghapus meta tag atau TXT record dari situs.
- Tauthub memeriksa ulang domain secara berkala. Kalau pemeriksaan gagal tiga kali berturut-turut pada domain yang sudah terverifikasi, Tauthub mencabut status verifikasinya. Anda perlu memverifikasi lagi.
- Pencabutan juga mengunci kembali **Impor XLSX** kalau tidak ada media Aktif lain yang domainnya terverifikasi. Lihat [Impor XLSX](08-impor-xlsx.md).

## Pengajuan Dewan Pers

Ajukan kalau media Anda punya sertifikat verifikasi Dewan Pers. Tim Tauthub memeriksa pengajuannya.

### Format nomor sertifikat

Nomor harus berbentuk `nomor/DP-Verifikasi/K/angka Romawi/tahun`, misalnya `123/DP-Verifikasi/K/X/2025`. Angka Romawi berkisar I sampai XII, dan tahun terdiri atas empat angka. Kalau format salah, layar menampilkan "Format nomor sertifikat tidak valid." disertai contoh format.

### Cara mengajukan Dewan Pers

1. Klik ikon di kolom **Verifikasi Kepemilikan**, lalu buka tab **Dewan Pers** kalau belum terbuka.
2. Isi **Nomor sertifikat**. Kolom ini wajib, minimal 3 karakter.
3. Unggah dokumen sertifikat kalau ada. Tarik dan lepas berkas ke kotak yang tersedia, atau klik kotak itu untuk memilih berkas. Dokumen ini boleh tidak diisi.
4. Klik **Ajukan Verifikasi**. Tombol tidak bisa diklik selama nomor kurang dari 3 karakter atau saat pengiriman berjalan ("Mengirim…").

Pesan hijau "Pengajuan verifikasi Dewan Pers terkirim dan menunggu review admin." muncul, lalu kotak menutup.

Aturan dokumen: format PDF, JPG, atau PNG, ukuran paling besar 10 MB. Kalau berkas tidak memenuhi syarat, layar menampilkan "Format dokumen harus PDF, JPG, atau PNG." atau "Ukuran file maksimal 10 MB." Klik **Hapus** di samping nama berkas untuk membatalkan pilihan.

Kalau nomor tersimpan tetapi dokumen gagal terunggah, layar menampilkan "Sertifikat tersimpan, tetapi unggah dokumen gagal." Tutup kotak dan buka lagi, hapus pengajuan, lalu ajukan lagi bersama dokumennya.

Kalau sudah ada pengajuan, bagian atas tab menampilkan statusnya, tanggal pengajuan, dan tombol **Lihat dokumen** untuk membuka dokumen yang tersimpan. Berkas baru yang Anda unggah menggantikan dokumen lama.

### Status Dewan Pers dan artinya

| Status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **Menunggu** | Tim Tauthub sedang memeriksa pengajuan. | Tunggu. Untuk mengganti nomor, hapus pengajuan lalu ajukan lagi. |
| **Terverifikasi** | Pengajuan disetujui. Klien melihat tanda Dewan Pers. | Tidak ada. Anda tidak bisa menghapusnya sendiri. |
| **Ditolak** | Tim Tauthub menolak. Alasannya dikirim lewat notifikasi dan email. | Perbaiki nomor atau dokumen, lalu ajukan lagi. |
| **Konflik** | Media lain yang menunggu atau terverifikasi sudah memakai nomor yang sama. Tim Tauthub perlu memeriksanya. | Periksa nomor Anda. Kalau salah ketik, hapus pengajuan. |

Kalau belum ada pengajuan, ringkasan di kolom berbunyi "Dewan Pers: belum diajukan". Keputusan tim Tauthub datang sebagai notifikasi berjudul "Verifikasi Dewan Pers disetujui" atau "Verifikasi Dewan Pers ditolak".

### Mencabut pengajuan

Anda bisa mencabut pengajuan yang berstatus **Menunggu**, **Ditolak**, atau **Konflik**.

1. Buka tab **Dewan Pers** di kotak verifikasi.
2. Klik **Hapus pengajuan**. Tombol berubah menjadi **Ya, hapus pengajuan**.
3. Klik **Ya, hapus pengajuan** untuk mengonfirmasi.

Pesan hijau menyatakan pengajuan dihapus, dan Anda bisa mengajukan ulang kapan saja. Kalau tidak ada pengajuan lain yang tersisa, nomor sertifikat dan tanda Dewan Pers pada media ikut hilang.

Tombol **Hapus pengajuan** tidak muncul untuk sertifikat **Terverifikasi**. Tim Tauthub yang mencabut sertifikat yang sudah sah, lewat [Dukungan](../12-dukungan.md).

> **Penting:** Kalau Anda mengajukan nomor yang berbeda pada media yang sudah **Terverifikasi**, statusnya kembali **Menunggu** sampai nomor baru disetujui. Selama itu tanda Dewan Pers tidak tampil di katalog. Kalau Anda mengajukan nomor yang sama, tidak ada yang berubah.

### Pesan kesalahan Dewan Pers

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Format nomor sertifikat tidak valid. | Nomor tidak sesuai format. | Salin nomor persis dari sertifikat. |
| Nomor sertifikat ini sudah digunakan oleh media lain dan memerlukan pemeriksaan Admin. | Media lain sudah memakai nomor ini. | Periksa nomor. Tunggu pemeriksaan tim Tauthub, atau hapus pengajuan. |
| Verifikasi Dewan Pers sedang menunggu review Admin. Anda belum bisa mengubah nomor sertifikat. | Masih ada pengajuan berstatus **Menunggu**. | Hapus pengajuan lebih dulu, lalu ajukan lagi. |
| Pengajuan Dewan Pers gagal. | Pengiriman tidak berhasil. | Coba lagi. Kalau terjadi lagi, hubungi [Dukungan](../12-dukungan.md). |

Kalau Anda memakai [Impor XLSX](08-impor-xlsx.md), Anda juga bisa mengisi kolom **Nomor Sertifikat Dewan Pers**. Setelah media dibuat, Tauthub mengajukan nomor dalam berkas itu otomatis.

## Pertanyaan yang sering diajukan

**Apakah saya perlu memverifikasi domain sebelum media disetujui?**
Tidak. Anda bisa memverifikasi domain kapan saja, termasuk saat media masih **Menunggu review**.

**Bolehkah saya memakai meta tag dan DNS TXT sekaligus?**
Boleh. Pemeriksaan menerima salah satunya.

**Berapa lama tim Tauthub memeriksa Dewan Pers?**
Layar tidak menyebut batas waktu. Anda menerima notifikasi begitu ada keputusan.

## Lihat juga

- [Membaca Daftar Media](01-membaca-daftar-media.md)
- [Link Guardian](../05-link-guardian.md)
- [Katalog Media (tampilan Klien)](../../klien/02-katalog-media/README.md)
