# Beranda Klien

> Beranda adalah halaman pertama Dashboard Klien. Dashboard adalah halaman kerja Anda. Di Beranda Anda melihat kampanye yang berjalan, saldo, kinerja bulan ini, dan publikasi terbaru, lalu membuka menu yang Anda perlukan. Halaman ini juga menjelaskan cara membaca menu samping, kotak saldo, lonceng, dan menu akun.

**Lokasi:** menu **MENU UTAMA** › **Beranda**
**Untuk:** Klien (semua anggota brand yang boleh melihat brand tersebut)

## Gambaran singkat

Setelah Anda masuk, Tauthub langsung membuka Beranda brand pertama Anda. Anda bisa membuka halaman awal Dashboard Klien tanpa menyebut brand, misalnya dari penanda halaman (bookmark) di browser. Dalam kasus itu, Tauthub membuka brand yang terakhir Anda buka di browser tersebut. Kalau belum ada, Tauthub membuka brand pertama. Selama proses itu, muncul halaman singkat bernama **Dashboard Klien** dengan tulisan "Mengarahkan Anda ke dashboard brand…". Kalau brand gagal dimuat, muncul pesan "Gagal memuat brand. Silakan muat ulang halaman atau coba lagi nanti." Muat ulang halaman, lalu coba lagi.

Tidak ada halaman untuk memilih brand. Lihat [Profil & Akun](10-profil-dan-akun.md) untuk penjelasan tambahan.

Beranda hanya dimuat sekali saat halaman dibuka. Beranda tidak memperbarui dirinya sendiri. Muat ulang halaman untuk melihat angka terbaru.

## Yang tampil di layar

Dari atas ke bawah, Beranda berisi bagian berikut.

### Sapaan dan kartu kampanye

Bagian paling atas menyapa Anda dengan "Selamat datang kembali" dan nama tampilan Anda. Ada dua tombol:

- **Buka Katalog Media** membuka menu **Katalog Media**.
- **Buat Kampanye** membuka daftar **Kampanye**. Anda membuat kampanye baru dari halaman itu, bukan dari Beranda.

Di sisi kanan ada kartu kampanye. Di HP, kartu ini ada di bawah tombol. Kartu ini menampilkan kampanye terbaru yang masih berjalan.

| Bagian kartu | Isinya |
|---|---|
| Label | **Kampanye berjalan**. Kalau semua kampanye sudah berstatus **Terjadwal** atau **Terkirim**, labelnya **Kampanye terakhir** |
| Tahap | Nama tahap kampanye, sama dengan yang tampil di daftar **Kampanye** |
| Progres | Angka seperti 2 / 5 dengan keterangan **media terbit**, dan bilah progres. Angka kiri adalah jumlah publikasi yang sudah terbit. Angka kanan adalah jumlah artikel yang disiapkan di kampanye itu. Kalau jumlah publikasi terbit lebih besar, angka kanan memakai jumlah itu |
| Tautan | **Buka Kampanye** membuka halaman kampanye tersebut |

Kalau kampanye belum punya artikel atau publikasi terbit, kartu menulis **Belum ada media dipilih**. Kalau Anda belum punya kampanye, kartu menulis **Belum ada kampanye**. Di bawahnya ada keterangan bahwa kampanye Anda akan tampil di sini.

### Empat kartu angka

| Kartu | Isinya | Keterangan di layar |
|---|---|---|
| **Saldo Tersedia** | Saldo yang siap dipakai | Siap digunakan untuk publikasi |
| **Kredit Tertahan** | Dana yang disisihkan untuk pesanan yang masih berjalan | Mengamankan pesanan aktif |
| **Estimasi Jangkauan (Bulan Ini)** | Perkiraan jumlah orang yang terjangkau bulan ini | Perbandingan dengan bulan lalu: "Naik N% dari bulan lalu", "Turun N% dari bulan lalu", atau "Sama seperti bulan lalu". Kalau bulan lalu belum punya data, yang tampil hanya tanda strip |
| **Total Klik (Smart Link)** | Jumlah klik yang tercatat pada bulan ini | Konversi tautan artikel aktif |

> **Catatan:** Dua kartu pertama menulis angka dengan satuan **Kredit**. Kotak saldo di bagian atas layar dan halaman **Saldo** menulis jumlah uang dalam rupiah. Jadikan keduanya patokan jumlah uang Anda. Penjelasan lengkap ada di [Saldo dan Top Up](08-saldo-dan-top-up.md).

### Tauthub AI Insights

Kartu ini menampilkan dua pesan singkat yang berganti sendiri, dengan lencana **AGENT 2 & 4**. Isi pesannya selalu sama. Angka di dalamnya bukan hasil hitungan dari akun Anda. Untuk data yang sebenarnya, lihat empat kartu angka dan tabel publikasi di halaman yang sama.

### Akses Cepat Keuangan

- **Total Saldo Aktif** menampilkan saldo tersedia.
- **Top Up Saldo** membuka jendela Top Up.
- **Riwayat Transaksi** membuka halaman **Saldo**.
- **Klaim PPh 23** membuka halaman **Invoice & Pajak**. Saat ini halaman itu hanya berisi daftar invoice. Lihat [Invoice dan Pajak](09-invoice-dan-pajak.md).

### Antrean & Status Publikasi Terkini

Tabel ini memuat enam publikasi terbaru dari semua status. Yang terbaru ada di atas.

| Kolom | Isinya |
|---|---|
| **Nama Kampanye / Artikel** | Judul artikel |
| **Media** | Media tujuan |
| **Status (QC)** | Status publikasi dengan penanda warna |
| **Biaya** | Harga publikasi dalam rupiah |

Klik satu baris untuk membuka publikasi itu di menu **Status Publikasi**. Beranda tidak punya tombol "lihat semua". Buka menu **Status Publikasi** untuk melihat daftar lengkap. Kalau belum ada publikasi, tampil "Belum ada publikasi aktif."

| Status | Warna | Artinya |
|---|---|---|
| **Menunggu** | Kuning | Publikasi baru dibuat dan belum diproses |
| **Menunggu Mitra** | Kuning | Menunggu keputusan media. Kalau media tidak menjawab dalam 3×24 jam, publikasi batal sendiri dan dana kembali ke saldo |
| **Disetujui Mitra** | Hijau | Media menyetujui artikel, dan artikel masuk antrean untuk dikirim |
| **Ditolak Mitra** | Merah | Media menolak artikel |
| **Dalam Antrean** | Abu-abu | Menunggu giliran dikirim |
| **Diproses** | Biru | Artikel sedang dikirim ke media |
| **Terbit** | Hijau | Pengiriman berhasil (artikel terbit atau tersimpan sebagai draf di media) |
| **Gagal** | Merah | Pengiriman gagal |
| **Mencoba Ulang** | Kuning | Pengiriman gagal dan sedang dicoba lagi |
| **Dibatalkan** | Abu-abu | Publikasi dibatalkan |
| **Revisi** | Abu-abu | Publikasi sedang diperbaiki |

Langkah yang perlu Anda lakukan untuk tiap status ada di [Status Publikasi](05-status-publikasi.md). Di HP, tabel tampil sebagai kartu yang bertumpuk, tanpa baris judul kolom.

### Panduan & Bantuan

Ada dua kartu, **Cara Kerja Tauthub** dan **Standar Gatekeeper**. Keduanya membuka jendela **Panduan & Bantuan**. Jendela itu punya dua tab dengan nama yang sama. Tab **Cara Kerja Tauthub** menjelaskan enam langkah: isi saldo, siapkan artikel, lolos Gatekeeper, pilih media lalu sebarkan, mitra menerbitkan artikel, serta cek, pantau, dan laporan. Tab **Standar Gatekeeper** menjelaskan pemeriksaan otomatis dan aturan isi artikel di tiap media. Artikel yang tidak lolos pemeriksaan bertanda **Hard gate** tidak bisa dikirim ke media. Penjelasan rincinya ada di [Cek kualitas naskah](03-kampanye/05-cek-kualitas-naskah.md).

## Cara menggunakan

### Cara memulai kampanye dari Beranda

1. Klik **Buat Kampanye** di bagian sapaan.
2. Ikuti langkah di [Membuat dan mengelola kampanye](03-kampanye/01-membuat-dan-mengelola-kampanye.md).

### Cara melanjutkan kampanye yang berjalan

1. Cari kartu kampanye di bagian sapaan.
2. Klik **Buka Kampanye**.

### Cara mengisi saldo dari Beranda

1. Klik **Top Up Saldo** pada kartu **Akses Cepat Keuangan**.
2. Pilih jumlahnya, lalu klik **Lanjutkan ke Pembayaran →**.
3. Klik **Bayar Sekarang** pada tampilan "Pesanan dibuat".
4. Selesaikan pembayaran di tab yang terbuka.

Kalau pembayaran gagal dibuat dari Beranda, layar hanya menulis "Terjadi kesalahan." tanpa penjelasan. Kalau Anda ingin pesan yang lebih jelas, lakukan Top Up dari kotak saldo atau dari halaman **Saldo**. Langkah lengkapnya ada di [Saldo dan Top Up](08-saldo-dan-top-up.md).

### Cara melihat publikasi terbaru

1. Cari tabel **Antrean & Status Publikasi Terkini**.
2. Klik baris publikasi yang ingin Anda buka.

## Mengenal tampilan dashboard klien

Semua halaman Dashboard Klien punya kerangka yang sama: menu samping, bar atas, dan area isi.

### Menu samping

| Grup | Menu | Fungsi |
|---|---|---|
| MENU UTAMA | **Beranda** | Ringkasan brand |
| RUANG KERJA | **Katalog Media** | Memilih media. Lihat [Katalog Media](02-katalog-media/README.md) |
| RUANG KERJA | **Kampanye** | Membuat dan mengatur kampanye. Lihat [Kampanye](03-kampanye/README.md) |
| RUANG KERJA | **Status Publikasi** | Memantau tiap artikel. Lihat [Status Publikasi](05-status-publikasi.md) |
| RUANG KERJA | **Link Guardian** | Memantau tautan artikel yang sudah terbit. Lihat [Link Guardian](06-link-guardian.md) |
| ANALITIK & PELAPORAN | **Analisis Kinerja**, **Audit Publikasi** | Belum tersedia. Lihat [Analisis dan audit](07-analisis-dan-audit.md) |
| KEUANGAN | **Saldo** | [Saldo dan Top Up](08-saldo-dan-top-up.md) |
| KEUANGAN | **Invoice & Pajak** | [Invoice dan Pajak](09-invoice-dan-pajak.md) |
| PENGATURAN | **Profil & Akun** | [Profil dan Akun](10-profil-dan-akun.md) |
| PENGATURAN | **Pesan** | [Pesan](11-pesan.md) |
| PENGATURAN | **Apa yang Baru** | [Apa yang Baru](13-apa-yang-baru.md) |

Menu yang sedang terbuka diberi warna dan huruf yang lebih tebal. Di komputer, klik tombol **Ciutkan sidebar** di sebelah logo untuk mengecilkan menu samping. Klik **Perluas sidebar** untuk melebarkannya lagi. Tombol pintas **Ctrl+B** (**⌘B** di Mac) punya fungsi yang sama saat Anda tidak sedang mengetik. Saat menu mengecil, hanya ikon yang tampil. Nama menu muncul saat Anda menaruh kursor di atasnya. Menu kembali lebar setiap kali Anda memuat ulang halaman.

### Bar atas

- Di kiri, halaman kampanye menampilkan deretan tahap kampanye. Di HP ada tombol **Buka navigasi**.
- Di kanan ada pilihan bahasa (**Indonesia** atau **English**), kotak saldo berbentuk pil, dan lonceng.

### Pil saldo dan Top Up cepat

Pil saldo menampilkan saldo tersedia dalam rupiah dengan ikon koin hijau. Dana yang ditahan dan tagihan yang belum dibayar tidak ikut dihitung. Klik pil untuk membuka jendela **Top Up Saldo** dari halaman mana pun. Angka di pil ikut berubah setelah pembayaran selesai.

### Lonceng

Klik lonceng untuk melihat daftar notifikasi terbaru. Lihat [Notifikasi](12-notifikasi.md).

### Menu akun

Di bagian bawah menu samping ada tombol akun. Isinya inisial Anda, nama tampilan, dan label **Klien**. Klik tombol itu untuk membuka menu:

- **Portal Mitra**, hanya untuk akun Mitra yang juga memakai Dashboard Klien. Lihat [Akun, peran, dan pindah dashboard](../umum/03-akun-peran-dan-pindah-dashboard.md).
- **Profil akun**, sama dengan menu **Profil & Akun**.
- **Keluar**, untuk keluar dari akun dan kembali ke halaman masuk.

### Tampilan di HP

Pada layar yang lebarnya di bawah 768 piksel, menu samping berubah menjadi laci yang bisa dibuka dan ditutup. Klik **Buka navigasi** untuk membukanya. Untuk menutupnya, klik **Tutup navigasi**, tekan tombol **Esc**, atau klik area di luar laci. Tombol akun ada di bagian bawah laci. Pilihan bahasa dan saldo pindah ke tombol ikon kotak bernama **Menu lainnya**. Di dalamnya ada pilihan bahasa dan baris saldo dengan tulisan **Top Up**.

### Angka merah penanda

| Lokasi | Yang dihitung | Kapan hilang |
|---|---|---|
| Menu **Status Publikasi** | Notifikasi publikasi yang belum dibaca: disetujui mitra, terbit, atau terkirim sebagai draf | Saat Anda membuka menu **Status Publikasi** |
| Menu **Pesan** | Pesan dari mitra yang belum dibaca di semua percakapan | Saat Anda membuka percakapannya |
| Lonceng | Semua notifikasi yang belum dibaca, umum dan pengumuman. Angka di atas 9 tampil sebagai 9+ | Saat Anda membaca notifikasinya |

Angka merah berubah sendiri. Kadang jedanya sampai sekitar satu menit.

## Aturan dan batasan

- Semua anggota brand bisa melihat Beranda. Untuk Top Up dan mengirim pesan, Anda perlu hak akses penulis ke atas. Lihat [Saldo dan Top Up](08-saldo-dan-top-up.md).
- Pengumuman dari tim Tauthub yang belum Anda baca muncul sebagai jendela **Pengumuman** beberapa saat setelah halaman dibuka. Lihat [Notifikasi](12-notifikasi.md).

## Pertanyaan yang sering diajukan

**Mengapa saldo di Beranda tidak sama tampilannya dengan pil saldo?**
Kartu Beranda memakai satuan **Kredit**, sedangkan pil saldo memakai rupiah. Selain itu, kartu Beranda baru berubah setelah Anda memuat ulang halaman.

**Saldo sudah bertambah di pil, tetapi kartu Beranda belum berubah.**
Muat ulang halaman.

**Bagaimana berpindah ke brand lain?**
Dashboard Klien tidak punya menu untuk berpindah brand. Setelah masuk, Anda langsung dibawa ke brand pertama Anda.

**Kartu kampanye menampilkan "Kampanye terakhir", bukan "Kampanye berjalan". Mengapa?**
Semua kampanye Anda sudah berstatus **Terjadwal** atau **Terkirim**. Buat kampanye baru dari menu **Kampanye**.

## Lihat juga

- [Saldo dan Top Up](08-saldo-dan-top-up.md)
- [Status Publikasi](05-status-publikasi.md)
- [Kampanye](03-kampanye/README.md)
- [Notifikasi](12-notifikasi.md)
- [Glosarium](../umum/05-glosarium.md)
