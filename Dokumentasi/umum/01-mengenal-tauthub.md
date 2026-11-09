# Mengenal Tauthub

> Tauthub adalah platform distribusi konten yang menghubungkan Klien, yang ingin artikelnya terbit di media online, dengan Mitra, pemilik media yang menerbitkannya. Halaman ini menjelaskan siapa saja penggunanya, perjalanan satu artikel dari pemesanan sampai terbit, dan letak penjelasan tiap fitur.

**Untuk:** Klien dan Mitra

## Apa itu Tauthub

Tauthub mengatur kerja sama dua pihak. Klien membuat kampanye, memilih media, menyiapkan artikel, lalu memesan penerbitannya. Mitra menyetujui pesanan itu dan menerbitkan artikelnya di situs media miliknya. Di antara keduanya, Tauthub mengurus pemesanan, persetujuan, penerbitan, pemantauan tautan, sampai pembayaran penghasilan Mitra.

Semua pekerjaan dilakukan di browser dengan satu akun. Setiap akun punya satu peran, yaitu Klien atau Mitra, dan setiap peran punya dashboard sendiri.

Tauthub juga memiliki halaman publik **Direktori Media Tauthub** yang bisa dibuka tanpa akun. Halaman itu memuat daftar media aktif beserta DA, estimasi trafik bulanan, harga per artikel, dan aturan backlink. Tombol **Pesan lewat Tauthub** pada halaman detail media membawa Anda ke halaman masuk.

## Siapa pengguna Tauthub

| Pengguna | Siapa | Yang dilakukan di Tauthub | Dashboard |
|---|---|---|---|
| Klien | Perusahaan, tim, atau individu yang ingin menggunakan layanan Tauthub | Membuat kampanye, memilih media, menyiapkan artikel, dan memesan penerbitan | Dashboard Klien |
| Mitra | Media partner, penerbit, atau pemilik media yang ingin bekerja sama dengan Tauthub | Mendaftarkan media, menyetujui pesanan, menerbitkan artikel, dan mencairkan penghasilan | Dashboard Mitra |

Tim Tauthub bekerja di balik layar, misalnya meninjau media baru sebelum tampil di **Katalog Media**, menjawab tiket dukungan Mitra, memproses penarikan penghasilan, dan mengirim pengumuman. Anda melihat dampaknya lewat status, notifikasi, atau pengumuman.

> **Catatan:** Satu alamat email hanya bisa dipakai untuk satu akun dengan satu peran. Untuk memakai Tauthub sebagai Klien sekaligus Mitra, daftarkan akun kedua dengan email lain. Mitra tingkat VVIP bisa membuka Dashboard Klien dari akun Mitranya. Caranya ada di [Akun, peran, dan pindah dashboard](03-akun-peran-dan-pindah-dashboard.md).

## Istilah dasar

| Istilah | Artinya |
|---|---|
| Brand | Ruang kerja Klien. Kampanye, artikel, dan Saldo disimpan per brand. Akun Klien baru otomatis memiliki satu brand yang bernama sama dengan nama akun. |
| Media | Situs milik Mitra tempat artikel diterbitkan. Media tampil di Katalog Media setelah ditinjau tim Tauthub. |
| Publikasi | Satu artikel yang dikirim ke satu media. |
| Saldo | Dompet rupiah milik brand. Diisi lewat **Top Up**, lalu dipakai untuk biaya publikasi dan fitur berbayar seperti penulisan artikel dengan bantuan AI. |
| Penghasilan | Pendapatan Mitra dari artikel yang terbit di medianya. |

Daftar istilah lengkap ada di [Glosarium](05-glosarium.md).

## Perjalanan sebuah artikel

Berikut alur umum dari pemesanan sampai pembayaran penghasilan. Rincian tiap langkah ada di panduan masing-masing dashboard.

1. **Klien mengisi Saldo.** Klien menambah Saldo lewat **Top Up**. Saldo harus cukup untuk biaya publikasi di media yang dipilih.
2. **Klien membuat kampanye.** Di menu **Kampanye**, klien mengklik **Kampanye Baru**, lalu **Katalog Media** terbuka. Klien memilih media yang cocok dan mengklik **Buat Kampanye** untuk menyimpannya sebagai kampanye. Pemilihan media juga bisa dimulai langsung dari menu **Katalog Media**. Membuat kampanye tidak memotong Saldo.
3. **Klien menyiapkan artikel.** Di tahap **Penyedia Konten**, artikel dibuat dengan bantuan AI atau diunggah sendiri, lalu disunting di **Pengelola Naskah**. Tauthub memeriksa kualitas naskah sebelum artikel boleh dikirim.
4. **Klien mengirim pesanan.** Lewat panel **Terbitkan Kampanye**, klien memilih **Terbitkan Sekarang** atau **Terbitkan & Jadwalkan**. Saldo untuk tiap media ditahan, dan pesanan masuk ke Mitra dengan status **Menunggu persetujuan media** (di **Beranda** status yang sama tampil sebagai **Menunggu Mitra**). Tahap **Ringkasan** lalu terbuka untuk memantau hasilnya.
5. **Mitra menyetujui atau menolak.** Mitra mendapat notifikasi dan punya waktu 3×24 jam untuk menyetujui atau menolak. Bila Mitra menolak, Saldo yang ditahan langsung kembali ke brand Klien. Bila Mitra tidak menanggapi sampai batas waktu, pesanan dibatalkan otomatis dan Saldo juga kembali.
6. **Artikel diterbitkan.** Bila media tersambung lewat menu **Integrasi** (WordPress, EmDash, website custom, atau Blogspot), artikel dikirim otomatis ke situs Mitra, sebagai draf atau langsung terbit. Bila tidak tersambung, Mitra menerbitkan sendiri lalu mengisi URL artikelnya. Setelah publikasi berhasil, Saldo yang ditahan menjadi terpakai, dan Klien serta Mitra menerima notifikasi.
7. **Tautan dipantau.** **Link Guardian** memeriksa secara berkala apakah artikel masih tayang dan tautan di dalamnya masih utuh. Klien memantau statusnya di **Status Publikasi** dan bisa membagikan **Berita Acara** kampanye lewat tautan yang dapat dibuka tanpa masuk.
8. **Mitra menerima penghasilan.** Penghasilan tercatat setelah artikel terbit dan berstatus **Masa tunggu**. Setelah masa tunggu 2×24 jam, penghasilan menjadi **Siap ditarik**. Mitra mengajukan penarikan lewat **Tarik Saldo**, tim Tauthub meninjaunya, dan invoice pengajuan tampil di menu **Invoice**.

> **Penting:** Saldo punya tiga keadaan. Saldo ditahan saat pesanan dikirim, terpakai setelah publikasi berhasil, dan kembali ke brand bila pesanan ditolak atau dibatalkan. Saldo brand yang tidak memiliki transaksi saldo maupun publikasi selama 12 bulan akan hangus. Penghangusan bersifat final dan tidak bisa dibatalkan, jadi lakukan transaksi atau publikasi sebelum batas itu.

> **Catatan:** Batas 3×24 jam dihitung sejak jadwal terbit. Bila pesanan tidak dijadwalkan, batas dihitung sejak pesanan dibuat. Biaya, tarif, dan persentase bagi hasil bisa berubah, jadi angkanya selalu mengikuti yang tampil di layar Anda.

## Modul utama

### Dashboard Klien

| Menu atau fitur | Fungsi singkat | Penjelasan lengkap |
|---|---|---|
| **Beranda** | Ringkasan saldo dan publikasi terbaru, serta pintasan ke Katalog Media, Kampanye, dan top up | [Beranda](../klien/01-beranda.md) |
| **Katalog Media** | Daftar media yang bisa dipilih beserta harga dan detailnya | [Katalog Media](../klien/02-katalog-media/README.md) |
| **Kampanye** | Membuat kampanye dari pemilihan media sampai pengiriman pesanan | [Kampanye](../klien/03-kampanye/README.md) |
| Tombol **Profiler** di halaman **Kampanye** | Menyusun brief brand lewat Chat Brief atau Instan Brief | [Profiler dan Taut Brief](../klien/04-profiler-dan-taut-brief/README.md) |
| **Status Publikasi** | Memantau pesanan yang sudah dikirim ke media | [Status Publikasi](../klien/05-status-publikasi.md) |
| **Link Guardian** | Memantau tautan artikel yang sudah terbit | [Link Guardian](../klien/06-link-guardian.md) |
| **Analisis Kinerja** dan **Audit Publikasi** | Belum tersedia (Segera hadir) | [Analisis dan audit](../klien/07-analisis-dan-audit.md) |
| **Saldo** | Saldo, **Top Up**, dan riwayat transaksi | [Saldo dan top up](../klien/08-saldo-dan-top-up.md) |
| **Invoice & Pajak** | Daftar invoice brand | [Invoice dan pajak](../klien/09-invoice-dan-pajak.md) |
| **Profil & Akun** | Identitas akun, preferensi notifikasi, dan keamanan | [Profil dan akun](../klien/10-profil-dan-akun.md) |
| **Pesan** | Obrolan dengan Mitra per artikel | [Pesan](../klien/11-pesan.md) |
| **Apa yang Baru** | Catatan pembaruan fitur | [Apa yang baru](../klien/13-apa-yang-baru.md) |

### Dashboard Mitra

| Menu | Fungsi singkat | Penjelasan lengkap |
|---|---|---|
| **Ringkasan** | Gambaran media, pesanan yang menunggu, penghasilan, dan pesan | [Ringkasan](../mitra/01-ringkasan.md) |
| **VIP Dashboard** | Khusus Mitra VVIP | [VIP Dashboard](../mitra/02-vip-dashboard.md) |
| **Publikasi** | Menyetujui, menolak, dan menerbitkan pesanan artikel | [Publikasi](../mitra/03-publikasi/README.md) |
| **Daftar Media** | Mendaftarkan dan mengelola media | [Daftar Media](../mitra/04-daftar-media/README.md) |
| **Link Guardian** | Memantau tautan artikel dan kesehatan domain media | [Link Guardian](../mitra/05-link-guardian.md) |
| **Akses Crawler** | Kredensial agar pemeriksa Tauthub bisa membaca situs | [Akses Crawler](../mitra/06-akses-crawler.md) |
| **SEO & Performa** | Analisis kualitas dan performa situs media | [SEO dan performa](../mitra/07-seo-dan-performa.md) |
| **Referral Mitra** | Tautan referral dan komisinya | [Referral Mitra](../mitra/08-referral-mitra.md) |
| **Penghasilan** | Penghasilan dan penarikan saldo | [Penghasilan](../mitra/09-penghasilan.md) |
| **Invoice** | Invoice dan status pencairan | [Invoice dan pencairan](../mitra/10-invoice-dan-pencairan.md) |
| **Pesan** | Obrolan dengan Klien per artikel | [Pesan](../mitra/11-pesan.md) |
| **Dukungan** | Tiket ke tim Tauthub | [Dukungan](../mitra/12-dukungan.md) |
| **Integrasi** | Menyambungkan situs media agar artikel terkirim otomatis | [Integrasi](../mitra/13-integrasi/README.md) |
| **Konfigurasi** | Profil, rekening penarikan, notifikasi, dan keamanan | [Profil dan konfigurasi](../mitra/14-profil-dan-konfigurasi/README.md) |
| **Apa yang Baru** | Catatan pembaruan fitur | [Apa yang baru](../mitra/16-apa-yang-baru.md) |

## Fitur pendukung

### Link Guardian

Link Guardian memantau artikel yang sudah terbit secara berkala. Klien melihat hasilnya di halaman Link Guardian dan di Status Publikasi. Mitra memakainya untuk memantau tautan artikel yang hilang dan kesehatan domain media. Penjelasan ada di [Link Guardian untuk Klien](../klien/06-link-guardian.md) dan [Link Guardian untuk Mitra](../mitra/05-link-guardian.md).

### Pesan

Pesan memungkinkan Klien dan Mitra mengobrol langsung per artikel. Sesi obrolan berlangsung 3×24 jam sejak pesan pertama. Setelah itu riwayat tetap bisa dibaca, tetapi pesan baru tidak bisa dikirim. Lihat [Pesan untuk Klien](../klien/11-pesan.md) dan [Pesan untuk Mitra](../mitra/11-pesan.md).

### Saldo dan Penghasilan

Saldo adalah sisi Klien: diisi lewat **Top Up**, ditahan saat pesanan dikirim, dan terpakai setelah publikasi berhasil. Penghasilan adalah sisi Mitra: bertambah saat artikel terbit di media, melewati masa tunggu, lalu bisa ditarik. Penjelasannya ada di [Saldo dan top up](../klien/08-saldo-dan-top-up.md), [Penghasilan](../mitra/09-penghasilan.md), dan [Invoice dan pencairan](../mitra/10-invoice-dan-pencairan.md).

### Notifikasi

Tauthub memberi kabar lewat lonceng di dashboard, email, WhatsApp, dan notifikasi perangkat. Lihat [Saluran notifikasi](04-saluran-notifikasi.md).

## Lihat juga

- [Daftar dan masuk](02-daftar-dan-masuk.md)
- [Akun, peran, dan pindah dashboard](03-akun-peran-dan-pindah-dashboard.md)
- [Saluran notifikasi](04-saluran-notifikasi.md)
- [Glosarium](05-glosarium.md)
