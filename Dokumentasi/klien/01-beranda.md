# Beranda Klien

> Beranda adalah halaman pertama Dashboard Klien. Di sini Anda melihat kampanye yang sedang berjalan, saldo, kinerja bulan ini, dan publikasi terbaru, lalu membuka menu yang Anda perlukan. Halaman ini juga menjelaskan cara membaca menu samping, pil saldo, lonceng, dan menu akun.

**Lokasi:** menu **MENU UTAMA** › **Beranda**
**Untuk:** Klien (semua anggota brand yang dapat melihat brand tersebut)

## Gambaran singkat

Setelah masuk, Tauthub membawa Anda langsung ke Beranda brand pertama Anda. Jika Anda membuka halaman awal Dashboard Klien tanpa menyebut brand (misalnya dari penanda halaman di browser), Anda dibawa ke brand yang terakhir Anda buka di browser itu, atau ke brand pertama jika belum ada. Selama proses itu tampil halaman singkat bertajuk **Dashboard Klien** dengan tulisan "Mengarahkan Anda ke dashboard brand…". Jika brand gagal dimuat, muncul pesan "Gagal memuat brand. Silakan muat ulang halaman atau coba lagi nanti." Muat ulang halaman, lalu coba lagi.

Tidak ada halaman pemilih brand. Lihat [Profil & Akun](10-profil-dan-akun.md) untuk penjelasan tambahan.

Beranda dimuat sekali saat halaman dibuka dan tidak memperbarui dirinya sendiri. Muat ulang halaman untuk melihat angka terbaru.

## Yang tampil di layar

Dari atas ke bawah, Beranda berisi bagian berikut.

### Sapaan dan kartu kampanye

Bagian paling atas menyapa Anda dengan "Selamat datang kembali" dan nama tampilan Anda. Ada dua tombol:

- **Buka Katalog Media** membuka menu **Katalog Media**.
- **Buat Kampanye** membuka daftar **Kampanye**. Kampanye baru dibuat dari halaman itu, bukan dari Beranda.

Di sisi kanan (di HP: di bawah tombol) ada kartu kampanye. Kartu ini menampilkan kampanye terbaru yang masih berjalan.

| Bagian kartu | Isinya |
|---|---|
| Label | **Kampanye berjalan**. Jika semua kampanye sudah berstatus **Terjadwal** atau **Terkirim**, labelnya **Kampanye terakhir** |
| Tahap | Nama tahap kampanye, sama dengan yang tampil di daftar **Kampanye** |
| Progres | Angka seperti 2 / 5 dengan keterangan **media terbit**, dan bilah progres. Angka kiri adalah jumlah publikasi yang sudah terbit. Angka kanan adalah jumlah artikel yang disiapkan di kampanye itu (atau jumlah publikasi terbit jika lebih besar) |
| Tautan | **Buka Kampanye** membuka halaman kampanye tersebut |

Jika kampanye belum punya artikel atau publikasi terbit, kartu menulis **Belum ada media dipilih**. Jika Anda belum punya kampanye, kartu menulis **Belum ada kampanye** beserta keterangan bahwa kampanye Anda akan tampil di sini.

### Empat kartu angka

| Kartu | Isinya | Keterangan di layar |
|---|---|---|
| **Saldo Tersedia** | Saldo yang siap dipakai | Siap digunakan untuk publikasi |
| **Kredit Tertahan** | Dana yang ditahan untuk pesanan yang masih berjalan | Mengamankan pesanan aktif |
| **Estimasi Jangkauan (Bulan Ini)** | Angka estimasi jangkauan bulan berjalan | Perbandingan dengan bulan lalu: "Naik N% dari bulan lalu", "Turun N% dari bulan lalu", atau "Sama seperti bulan lalu". Tanda "—" muncul jika bulan lalu belum punya data |
| **Total Klik (Smart Link)** | Jumlah klik yang tercatat pada bulan berjalan | Konversi tautan artikel aktif |

> **Catatan:** Dua kartu pertama menulis angka dengan satuan **Kredit**. Pil saldo di bagian atas layar dan halaman **Saldo** menulis nominal dalam rupiah. Jadikan keduanya acuan nominal Anda. Penjelasan lengkap ada di [Saldo dan Top Up](08-saldo-dan-top-up.md).

### Tauthub AI Insights

Kartu ini menampilkan dua pesan ringkas yang bergantian otomatis, dengan lencana **AGENT 2 & 4**. Teks pesannya tetap. Angka di dalamnya bukan hasil hitungan dari akun Anda. Untuk data sebenarnya, gunakan empat kartu angka dan tabel publikasi di halaman yang sama.

### Akses Cepat Keuangan

- **Total Saldo Aktif** menampilkan saldo tersedia.
- **Top Up Saldo** membuka jendela Top Up.
- **Riwayat Transaksi** membuka halaman **Saldo**.
- **Klaim PPh 23** membuka halaman **Invoice & Pajak**. Halaman itu saat ini hanya memuat daftar invoice. Lihat [Invoice dan Pajak](09-invoice-dan-pajak.md).

### Antrean & Status Publikasi Terkini

Tabel ini memuat enam publikasi terbaru dari semua status, yang terbaru di atas.

| Kolom | Isinya |
|---|---|
| **Nama Kampanye / Artikel** | Judul artikel |
| **Media** | Media tujuan |
| **Status (QC)** | Status publikasi dengan penanda warna |
| **Biaya** | Harga publikasi dalam rupiah |

Klik satu baris untuk membuka publikasi itu di menu **Status Publikasi**. Beranda tidak punya tombol "lihat semua"; buka menu **Status Publikasi** untuk daftar lengkap. Jika belum ada publikasi, tampil "Belum ada publikasi aktif."

| Status | Warna | Artinya |
|---|---|---|
| **Menunggu** | Kuning | Publikasi baru dibuat dan belum diproses |
| **Menunggu Mitra** | Kuning | Menunggu keputusan media. Jika media tidak merespons dalam 3×24 jam, publikasi dibatalkan otomatis dan dana kembali ke saldo |
| **Disetujui Mitra** | Hijau | Media menyetujui artikel dan artikel diantrekan untuk dikirim |
| **Ditolak Mitra** | Merah | Media menolak artikel |
| **Dalam Antrean** | Abu-abu | Menunggu giliran dikirim |
| **Diproses** | Biru | Artikel sedang dikirim ke media |
| **Terbit** | Hijau | Pengiriman berhasil (artikel terbit atau tersimpan sebagai draf di media) |
| **Gagal** | Merah | Pengiriman gagal |
| **Mencoba Ulang** | Kuning | Pengiriman gagal dan sedang dicoba lagi |
| **Dibatalkan** | Abu-abu | Publikasi dibatalkan |
| **Revisi** | Abu-abu | Publikasi sedang direvisi |

Langkah lanjutan untuk tiap status ada di [Status Publikasi](05-status-publikasi.md). Di HP, tabel tampil sebagai kartu bertumpuk tanpa baris judul kolom.

### Panduan & Bantuan

Dua kartu, **Cara Kerja Tauthub** dan **Standar Gatekeeper**, membuka jendela **Panduan & Bantuan**. Jendela itu punya dua tab dengan nama yang sama. Tab **Cara Kerja Tauthub** menjelaskan enam langkah: isi saldo, siapkan artikel, lolos Gatekeeper, pilih media lalu distribusi, mitra menerbitkan artikel, serta verifikasi, pantau, dan laporan. Tab **Standar Gatekeeper** menjelaskan pemeriksaan otomatis dan standar konten tiap media. Artikel yang tidak lolos pemeriksaan bertanda **Hard gate** tidak bisa didistribusikan. Detailnya ada di [Cek kualitas naskah](03-kampanye/05-cek-kualitas-naskah.md).

## Cara menggunakan

### Cara memulai kampanye dari Beranda

1. Klik **Buat Kampanye** di bagian sapaan.
2. Ikuti langkah di [Membuat dan mengelola kampanye](03-kampanye/01-membuat-dan-mengelola-kampanye.md).

### Cara melanjutkan kampanye yang berjalan

1. Cari kartu kampanye di bagian sapaan.
2. Klik **Buka Kampanye**.

### Cara mengisi saldo dari Beranda

1. Klik **Top Up Saldo** pada kartu **Akses Cepat Keuangan**.
2. Pilih nominal, lalu klik **Lanjutkan ke Pembayaran →**.
3. Klik **Bayar Sekarang** pada tampilan "Pesanan dibuat".
4. Selesaikan pembayaran di tab yang terbuka.

Dari Beranda, kegagalan saat membuat pembayaran tampil sebagai "Terjadi kesalahan." tanpa rincian. Untuk melihat pesan yang lebih rinci, lakukan Top Up dari pil saldo atau dari halaman **Saldo**. Langkah lengkap ada di [Saldo dan Top Up](08-saldo-dan-top-up.md).

### Cara melihat publikasi terbaru

1. Cari tabel **Antrean & Status Publikasi Terkini**.
2. Klik baris publikasi yang ingin Anda buka.

## Mengenal tampilan dashboard klien

Semua halaman Dashboard Klien memakai kerangka yang sama: menu samping, bar atas, dan area isi.

### Menu samping

| Grup | Menu | Fungsi |
|---|---|---|
| MENU UTAMA | **Beranda** | Ringkasan brand |
| RUANG KERJA | **Katalog Media** | Memilih media. Lihat [Katalog Media](02-katalog-media/README.md) |
| RUANG KERJA | **Kampanye** | Membuat dan mengelola kampanye. Lihat [Kampanye](03-kampanye/README.md) |
| RUANG KERJA | **Status Publikasi** | Memantau tiap artikel. Lihat [Status Publikasi](05-status-publikasi.md) |
| RUANG KERJA | **Link Guardian** | Memantau tautan artikel terbit. Lihat [Link Guardian](06-link-guardian.md) |
| ANALITIK & PELAPORAN | **Analisis Kinerja**, **Audit Publikasi** | Belum tersedia. Lihat [Analisis dan audit](07-analisis-dan-audit.md) |
| KEUANGAN | **Saldo** | [Saldo dan Top Up](08-saldo-dan-top-up.md) |
| KEUANGAN | **Invoice & Pajak** | [Invoice dan Pajak](09-invoice-dan-pajak.md) |
| PENGATURAN | **Profil & Akun** | [Profil dan Akun](10-profil-dan-akun.md) |
| PENGATURAN | **Pesan** | [Pesan](11-pesan.md) |
| PENGATURAN | **Apa yang Baru** | [Apa yang Baru](13-apa-yang-baru.md) |

Menu yang sedang dibuka ditandai dengan warna dan huruf yang lebih tebal. Di desktop, klik tombol **Ciutkan sidebar** di sebelah logo untuk mengecilkan menu samping, dan **Perluas sidebar** untuk melebarkannya lagi. Pintasan **Ctrl+B** (**⌘B** di Mac) melakukan hal yang sama saat Anda tidak sedang mengetik. Saat menu dikecilkan, hanya ikon yang tampil dan nama menu muncul saat kursor diarahkan. Menu kembali lebar setiap halaman dimuat ulang.

### Bar atas

- Di kiri, halaman kampanye menampilkan deretan tahap kampanye. Di HP ada tombol **Buka navigasi**.
- Di kanan ada pemilih bahasa (**Indonesia** atau **English**), pil saldo, dan lonceng.

### Pil saldo dan Top Up cepat

Pil saldo menampilkan saldo tersedia dalam rupiah dengan ikon koin hijau. Dana tertahan dan tagihan yang belum dibayar tidak termasuk. Klik pil untuk membuka jendela **Top Up Saldo** dari halaman mana pun. Pil ikut diperbarui setelah pembayaran selesai.

### Lonceng

Lonceng membuka daftar notifikasi terbaru. Lihat [Notifikasi](12-notifikasi.md).

### Menu akun

Di bagian bawah menu samping ada tombol akun berisi inisial, nama tampilan, dan label **Klien**. Klik tombol itu untuk membuka menu:

- **Portal Mitra**, hanya untuk akun Mitra yang juga memakai Dashboard Klien. Lihat [Akun, peran, dan pindah dashboard](../umum/03-akun-peran-dan-pindah-dashboard.md).
- **Profil akun**, sama dengan menu **Profil & Akun**.
- **Keluar**, mengakhiri sesi dan membawa Anda ke halaman masuk.

### Tampilan di HP

Pada layar di bawah 768 piksel, menu samping berubah menjadi laci. Klik **Buka navigasi** untuk membukanya, dan **Tutup navigasi**, tombol **Esc**, atau area di luar laci untuk menutupnya. Tombol akun ada di bagian bawah laci. Pemilih bahasa dan saldo pindah ke tombol ikon kotak bernama **Menu lainnya**. Di dalamnya ada pilihan bahasa dan baris saldo dengan tulisan **Top Up**.

### Badge angka merah

| Lokasi | Yang dihitung | Kapan hilang |
|---|---|---|
| Menu **Status Publikasi** | Notifikasi publikasi yang belum dibaca: disetujui mitra, terbit, atau terkirim sebagai draf | Saat Anda membuka menu **Status Publikasi** |
| Menu **Pesan** | Pesan dari mitra yang belum dibaca di semua percakapan | Saat Anda membuka percakapannya |
| Lonceng | Semua notifikasi yang belum dibaca, umum dan pengumuman. Angka di atas 9 tampil sebagai 9+ | Saat notifikasi dibaca |

Badge diperbarui otomatis, dengan jeda hingga sekitar satu menit.

## Aturan dan batasan

- Semua anggota brand dapat melihat Beranda. Top Up dan pengiriman pesan memerlukan hak akses penulis ke atas. Lihat [Saldo dan Top Up](08-saldo-dan-top-up.md).
- Pengumuman dari tim Tauthub yang belum Anda baca muncul sebagai jendela **Pengumuman** beberapa saat setelah halaman dibuka. Lihat [Notifikasi](12-notifikasi.md).

## Pertanyaan yang sering diajukan

**Mengapa saldo di Beranda tidak sama tampilannya dengan pil saldo?**
Kartu Beranda memakai satuan **Kredit**, sedangkan pil saldo memakai rupiah. Selain itu, kartu Beranda baru berubah setelah halaman dimuat ulang.

**Saldo sudah bertambah di pil, tetapi kartu Beranda belum berubah.**
Muat ulang halaman.

**Bagaimana berpindah ke brand lain?**
Dashboard Klien tidak menyediakan menu untuk berpindah brand. Setelah masuk, Anda dibawa ke brand pertama Anda.

**Kartu kampanye menampilkan "Kampanye terakhir", bukan "Kampanye berjalan". Mengapa?**
Semua kampanye Anda sudah berstatus **Terjadwal** atau **Terkirim**. Buat kampanye baru dari menu **Kampanye**.

## Lihat juga

- [Saldo dan Top Up](08-saldo-dan-top-up.md)
- [Status Publikasi](05-status-publikasi.md)
- [Kampanye](03-kampanye/README.md)
- [Notifikasi](12-notifikasi.md)
- [Glosarium](../umum/05-glosarium.md)
