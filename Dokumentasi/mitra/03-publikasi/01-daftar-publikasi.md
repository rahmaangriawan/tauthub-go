# Membaca daftar Publikasi

> Halaman **Publikasi** menampilkan setiap artikel klien yang dikirim ke media Anda, lengkap dengan status, jadwal terbit, dan penghasilannya. Panduan ini menjelaskan tab status, pencarian, ekspor CSV, kolom daftar, dan tombol aksi di tiap baris.

**Lokasi:** menu **Publikasi**
**Untuk:** Mitra

## Gambaran singkat

Di bawah judul **Publikasi** tertulis keterangan "N permintaan artikel pada media Anda". Angka N mengikuti filter yang sedang aktif, jadi angkanya berubah saat Anda berpindah tab atau memakai filter media.

Daftar diurutkan dari publikasi terbaru. Setiap halaman memuat 20 publikasi. Klik sebuah baris (di HP: ketuk kartunya) untuk membuka dialog **Detail publikasi**. Penjelasannya ada di [Detail publikasi](02-detail-publikasi.md).

## Yang tampil di layar

### Tab status

Tab status berada di atas daftar. Setiap tab memuat ulang daftar sesuai status yang dipilih.

| Tab | Status yang ditampilkan |
|---|---|
| **Semua** | Semua status, termasuk **Dibatalkan** |
| **Menunggu** | **Menunggu persetujuan**, **Disetujui** (**Menunggu URL**), **Antrean**, **Diproses**, **Coba ulang** |
| **Revisi** | **Perlu Revisi** |
| **Sudah Terbit** | **Terbit** |
| **Gagal** | **Gagal** |
| **Ditolak Mitra** | **Ditolak** |

Di HP, tab diganti tombol ikon **Filter status publikasi**. Ketuk tombol itu untuk membuka menu pilihan; tanda centang menunjukkan filter yang aktif. Setiap kali Anda mengganti tab, daftar kembali ke halaman pertama.

### Filter media

Jika Anda membuka Publikasi dari lencana **N menunggu** di [Daftar Media](../04-daftar-media/README.md), muncul label **Media: nama media** di sebelah tab, dan daftar hanya memuat publikasi media itu pada tab **Menunggu**. Klik tanda silang (**Hapus filter media**) untuk kembali melihat semua media.

> **Catatan:** Angka pada lencana **N menunggu** hanya menghitung publikasi yang menunggu persetujuan. Tab **Menunggu** memuat lebih banyak status, jadi jumlah barisnya bisa lebih besar.

### Kolom pencarian

Kolom **Cari judul, media, atau nomor order…** mencari di judul artikel, nama media, dan nomor order. Huruf besar dan kecil tidak dibedakan. Pencarian hanya menyaring 20 baris yang sedang tampil di halaman itu. Jika artikel yang Anda cari tidak muncul, pindah halaman dengan **Berikutnya** atau pakai tab status untuk mempersempit daftar.

### Tombol Ekspor CSV

Tombol **Ekspor CSV** mengunduh file dengan nama `publikasi-mitra-` diikuti tanggal. Aturan isinya:

- Isi file mengikuti tab status yang aktif. Pencarian dan filter media tidak memengaruhi isi file.
- File memuat hingga 5.000 publikasi terbaru.
- Kolomnya: No. Order, Status, Klien, Artikel, Media, Kategori Media, Integrasi, Harga Klien, Jadwal, Terbit, URL Terbit, Status Verifikasi, Compliance, Sentiment, Confidence, Message Score, Catatan, Alasan Penolakan, dan Dibuat.
- Kolom **Status** di file berisi kode, misalnya `published` atau `waiting_partner`. Padanannya ada di [Status publikasi dan penghasilan](09-status-dan-penghasilan.md).
- Kolom **Harga Klien** adalah harga yang dibayar klien, bukan penghasilan Anda.

Jika pengunduhan gagal, muncul pesan "Gagal mengekspor CSV." Coba lagi beberapa saat kemudian.

### Kolom di layar lebar

| Kolom | Isi |
|---|---|
| **Artikel** | Gambar sampul kecil (bila ada), judul maksimal dua baris, dan nomor order, misalnya `ORD-202611-00123`. Artikel tanpa judul tertulis "Tanpa judul". |
| **Mode Integrasi** | **API** atau **Manual**. Arahkan kursor ke ikon info di judul kolom untuk membaca penjelasannya. |
| **Media** | Nama domain situs media, tautan **Lihat Artikel** bila artikel sudah punya URL, dan nama platform integrasi, misalnya WordPress. |
| **Penghasilan** | Penghasilan bersih Anda. Klik nominalnya untuk melihat rinciannya. |
| **Status** | Lencana status, ditambah keterangan seperti jadwal dan sisa waktu persetujuan. |
| **Aksi** | Deretan ikon tombol. |

### Kartu di HP

Di layar kecil, setiap publikasi tampil sebagai kartu. Bagian atas kartu memuat gambar sampul, judul, nomor order, dan ikon aksi. Bagian bawahnya memuat **Mode Integrasi**, **Media** (domain dengan ikon tautan bila artikel sudah punya URL), **Penghasilan**, dan **Status**.

### Keterangan tambahan di bawah status

Selain lencana status, sebuah baris dapat menampilkan:

- **Persentase** (misalnya 60%): kemajuan pengiriman untuk media **API** yang sedang **Antrean**, **Diproses**, atau **Coba ulang**.
- **Alasan penolakan** dalam huruf merah, jika artikel ditolak.
- **Dibatalkan otomatis:** atau **Dibatalkan klien:** diikuti alasannya, untuk publikasi **Dibatalkan**.
- **Dijadwalkan** disertai tanggal dan jam, jika klien memilih jadwal terbit. Keterangan ini muncul pada status **Menunggu persetujuan**, **Disetujui**, **Antrean**, **Diproses**, dan **Coba ulang**.
- Sisa waktu persetujuan pada status **Menunggu persetujuan**: **Sisa N hari**, **Sisa kurang dari 1 hari**, atau **Melewati SLA — dibatalkan otomatis**. Arahkan kursor ke keterangan itu untuk membaca aturannya. Penjelasan batas 3×24 jam ada di [Menyetujui atau menolak publikasi](03-menyetujui-dan-menolak.md).
- **URL belum terisi**, untuk artikel **Terbit** yang belum punya URL.
- **Diimport mitra** atau **Terbit di situs mitra**: penanda yang hanya muncul bila situs Anda memakai Feed RSS dan melaporkan statusnya ke Tauthub.

Arahkan kursor ke lencana **Antrean** pada artikel berjadwal untuk membaca "Terjadwal terbit" disertai tanggalnya, atau "Menunggu antrean penerbitan" bila jadwalnya sudah lewat.

### Rincian penghasilan

Nominal di kolom **Penghasilan** adalah pendapatan bersih setelah potongan platform. Klik nominal (ada ikon info kecil di sebelahnya) untuk membuka panel **Rincian penghasilan**:

| Baris | Artinya |
|---|---|
| **Harga diajukan** | Nilai sebelum potongan platform |
| **Potongan platform (N%)** | Persentase dan nominal yang dipotong. Saat ini potongannya 25%, jadi Anda menerima 75%. |
| **Pendapatan Anda** | Yang menjadi penghasilan Anda |

Klik di luar panel untuk menutupnya. Di HP, panel muncul di bagian bawah layar dengan tombol tutup. Pada baris lama yang tidak punya data potongan, nilai potongan tertulis "—" dan pendapatan sama dengan angka yang tercatat.

> **Penting:** Pada publikasi yang belum terbit, ditolak, dibatalkan, atau gagal, nominal ini hanya perkiraan. Tidak ada penghasilan yang masuk sampai artikel berstatus **Terbit**.

### Tombol aksi

Arahkan kursor ke sebuah ikon untuk membaca namanya. Ikon yang tampil bergantung pada status.

| Ikon (nama) | Fungsi | Muncul pada |
|---|---|---|
| Mata (**Detail**) | Membuka dialog Detail | Semua status |
| Buku (**Pratinjau Artikel**) | Membaca isi artikel | **Menunggu persetujuan**, **Disetujui**, **Antrean**, **Diproses**, **Coba ulang**, **Perlu Revisi** |
| Balon pesan (**Hubungi Klien**) | Memulai obrolan dengan klien | **Menunggu persetujuan**, **Disetujui**, **Perlu Revisi** |
| Pesawat kertas (**Setuju & Terbitkan**) | Menyetujui artikel, setelah konfirmasi | **Menunggu persetujuan** |
| Centang (**Terima Revisi**) | Menerima revisi klien, tanpa konfirmasi | **Perlu Revisi** |
| Pensil (**Edit draf**) | Membuka editor draf | **Menunggu persetujuan**; pada artikel **Terbit** di media **Manual**, editor hanya untuk membaca dan menyalin |
| Tautan (**Kirim URL**) | Mengirim URL artikel yang sudah tayang | **Disetujui** (**Menunggu URL**) |
| Panah keluar (**Lihat Langsung**) | Membuka artikel di tab baru | Artikel yang sudah punya URL terbit |
| Silang (**Tolak publikasi** atau **Tolak revisi**) | Menolak, dengan alasan | **Menunggu persetujuan**; **Perlu Revisi** selama penolakan revisi belum mencapai 3 kali |

Status **Gagal**, **Ditolak**, dan **Dibatalkan** hanya memiliki ikon **Detail** (ditambah **Lihat Langsung** bila ada URL).

### Halaman, daftar kosong, dan pembaruan otomatis

- Teks "Menampilkan N dari M publikasi" menunjukkan jumlah baris di halaman ini dan total sesuai filter. Tombol **Sebelumnya** dan **Berikutnya** nonaktif di halaman pertama dan terakhir.
- Saat daftar sedang dimuat, tertulis "Memuat publikasi…". Jika pemuatan gagal, muncul pesan merah "Gagal memuat publikasi." Muat ulang halaman untuk mencoba lagi.
- Jika tidak ada baris yang cocok, muncul keterangan seperti "Tidak ada publikasi dengan status Menunggu." atau "Tidak ada publikasi yang cocok dengan pencarian" disertai kata yang Anda ketik. Akun yang belum menerima pesanan melihat "Tidak ada publikasi dengan status Semua."
- Selama ada artikel **Disetujui**, **Antrean**, **Diproses**, atau **Coba ulang** di halaman yang tampil, daftar menyegarkan dirinya setiap beberapa detik. Anda tidak perlu memuat ulang.

### Angka merah di menu Publikasi

Angka merah di sebelah menu **Publikasi** menghitung publikasi yang dibuat setelah terakhir kali Anda membuka halaman Publikasi, apa pun statusnya. Angka ini bukan jumlah artikel yang menunggu keputusan.

- Angka hilang segera setelah Anda membuka halaman Publikasi.
- Angka muncul lagi ketika ada publikasi baru, dan diperbarui secara berkala saat Anda berada di halaman mana pun di Dashboard Mitra.
- Tanda "sudah dibaca" tersimpan di akun Anda, bukan di perangkat. Membuka halaman di satu perangkat menghapus angka di perangkat lain pada pembaruan berikutnya.

## Cara menggunakan

### Cara melihat artikel yang perlu keputusan Anda

1. Klik menu **Publikasi**.
2. Klik tab **Menunggu**.
3. Cari baris berstatus **Menunggu persetujuan**. Baris ini menunggu keputusan Anda.
4. Perhatikan keterangan sisa waktu di bawah lencana status.

### Cara mencari artikel

1. Klik kolom **Cari judul, media, atau nomor order…**.
2. Ketik sebagian judul, nama media, atau nomor order.
3. Baca hasilnya. Jika kosong, klik **Berikutnya** atau pilih tab status lain.

### Cara mengekspor daftar ke CSV

1. Klik tab status yang ingin Anda ekspor.
2. Klik **Ekspor CSV**.
3. Buka file hasil unduhan di Excel atau Google Sheets.

### Cara melihat rincian penghasilan satu artikel

1. Cari baris artikelnya di kolom **Penghasilan**.
2. Klik nominalnya.
3. Baca baris **Harga diajukan**, **Potongan platform**, dan **Pendapatan Anda**.

## Pertanyaan yang sering diajukan

**Kenapa pencarian tidak menemukan artikel yang saya yakin ada?**
Pencarian hanya menyaring baris di halaman yang sedang tampil. Pindah ke halaman lain atau persempit daftar dengan tab status.

**Kenapa tidak ada ikon Setuju pada baris tertentu?**
Ikon itu hanya muncul pada status **Menunggu persetujuan**. Untuk status lain, lihat tabel tombol aksi di atas.

**Kenapa nominal di kolom Penghasilan berbeda dari harga media saya?**
Kolom itu menampilkan pendapatan bersih setelah potongan platform. Klik nominalnya untuk melihat rinciannya.

## Lihat juga

- [Detail publikasi](02-detail-publikasi.md)
- [Menyetujui atau menolak publikasi](03-menyetujui-dan-menolak.md)
- [Status publikasi dan penghasilan](09-status-dan-penghasilan.md)
- [Daftar Media](../04-daftar-media/README.md)
- [Penghasilan](../09-penghasilan.md)
