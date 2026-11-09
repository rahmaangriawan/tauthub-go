# Link Guardian

> Pemantau berkala yang memeriksa apakah artikel terbit Anda masih bisa dibuka dan apakah tautan ke situs Anda masih ada di dalamnya. Bila artikel hilang atau tautan dihapus, Anda dapat mengajukan klaim garansi.

**Lokasi:** menu **Link Guardian** (grup **RUANG KERJA**)

**Untuk:** Klien. Semua peran brand dapat melihat halaman. **Crawl Sekarang** dan **Ajukan Klaim** memerlukan peran **Writer** ke atas.

## Gambaran singkat

Link Guardian memeriksa dua hal pada setiap artikel yang sudah terbit:

1. **Halaman artikel**: apakah alamatnya masih bisa dibuka.
2. **Tautan di dalam artikel**: apakah tautan ke situs Anda masih ada di isi artikel.

Yang dipantau hanya artikel berstatus **Terbit** yang punya tautan terbit, masih dalam masa garansi media, dan bukan draf. Artikel yang dikirim sebagai draf mulai dipantau setelah mitra mempublikasikannya.

Tauthub memeriksa artikel secara otomatis sekitar sebulan sekali. Anda juga dapat memicu pengecekan sendiri, maksimal sekali sehari.

Garansi berlaku khusus pada **Media Terverifikasi**, yaitu media yang kepemilikan domainnya sudah diverifikasi Tauthub. Di [Katalog Media](02-katalog-media/README.md), media seperti itu bertanda centang biru "Domain Terverifikasi" dan dapat difilter lewat **Domain terverifikasi**.

Lama garansi berbeda untuk tiap media: **Permanen**, atau sekian bulan sejak tanggal terbit. Badge kebijakan tautan pada baris media di Katalog Media menunjukkannya, misalnya **2 Dofollow • 12 Bln SLA**. Setelah masa garansi habis, artikel keluar dari halaman ini.

## Yang tampil di layar

### Bagian atas

- Tombol **Crawl Sekarang** untuk mengantre pengecekan manual.
- Keterangan **Cek Manual Maks. 1x Sehari**.
- **Terakhir sinkronisasi**, misalnya "Hari ini, 08:30 WIB", yaitu waktu pengecekan terbaru atas artikel Anda.

### Empat kartu

| Kartu | Isi |
|---|---|
| **Total Dipantau** | Jumlah artikel yang dipantau ("Dalam masa garansi aktif") |
| **Tayang & Aman** | Halaman masih bisa dibuka (200 OK) |
| **Tak Terjangkau** | Situs menolak akses atau sedang bermasalah. Keterangannya "Sistem keamanan media memblokir bot" |
| **Bermasalah** | Artikel 404 ("404 atau Tautan dihapus (SLA Klaim)") |

Setiap kartu memuat angka dan persentase dari **Total Dipantau**. Grafik gelombang kecil adalah ilustrasi kecenderungan. Acuan Anda adalah angka dan persentasenya.

> **Catatan:** angka kartu mengikuti status halaman. Artikel yang halamannya masih tayang tetapi tautan Anda sudah dihapus dihitung pada **Tayang & Aman**. Artikel itu tetap tampil merah sebagai **Tautan dihapus** di tabel dan dapat diklaim.

### Pencarian dan filter

- Kolom pencarian ("Cari artikel, media, atau URL…") mencari judul artikel, nama media, dan URL. Tekan **Enter** untuk menjalankannya.
- Pilihan status: **Semua Status**, **Bermasalah saja (SLA Klaim)**, **Tak Terjangkau**, **Tayang & Aman**, dan **Belum ada URL**. **Bermasalah saja (SLA Klaim)** menampilkan artikel **Hilang (404)**.
- Tombol **Filter** membuka panel berisi **Media**, **Tanggal** (tanggal tayang: **7 Hari Terakhir**, **30 Hari Terakhir**, **3 Bulan Terakhir**), **Reset filter**, dan **Terapkan**. Pilihan berlaku langsung. **Terapkan** hanya menutup panel. Angka kecil pada tombol menunjukkan jumlah filter aktif.
- Pilihan **Media** hanya berisi media pada halaman yang sedang terbuka.

### Tabel artikel

| Kolom | Isi |
|---|---|
| **Informasi Artikel & Tautan** | Gambar mini, alamat artikel (klik untuk membuka di tab baru), judul, dan "Anchor:" bila diketahui |
| **Publisher** | Nama media dan **Tgl Tayang** |
| **Status Verifikasi** | **Terverifikasi** atau **Belum terverifikasi** |
| **Status Deteksi** | Hasil pengecekan (tabel di bawah) |
| **Waktu Cek** | Waktu relatif ("3 hari yang lalu") dan tanggal lengkap, atau "Belum pernah" |
| **Aksi Garansi** | Tombol atau status klaim |

Klik judul kolom **Informasi Artikel & Tautan** atau **Waktu Cek** untuk mengurutkan. Pengurutan hanya berlaku pada halaman terbuka. Secara bawaan baris diurutkan menurut judul dari Z ke A.

Di bawah tabel ada keterangan "Menampilkan 1–10 dari N artikel", pilihan **10 per halaman**, **20 per halaman**, atau **50 per halaman**, serta tombol **Sebelumnya** dan **Berikutnya**.

Bila daftar kosong, tampil "Tidak ada masalah pada artikel Anda". Pesan yang sama muncul bila belum ada artikel yang dipantau atau filter tidak menemukan hasil.

### Tampilan di HP

Pada layar sempit, tabel berubah menjadi kartu per artikel. Tiap bagian diberi label sendiri, sehingga tidak perlu menggeser ke samping. Kartu ringkasan tersusun ke bawah. Bilah pencarian menempel di bawah bilah atas saat Anda menggulir, dan tombol **Filter** hanya berupa ikon.

## Status dan artinya

### Status Deteksi

| Status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **200 OK (aman)** | Halaman artikel bisa dibuka | Tidak ada tindakan |
| **Hilang (404)** | Halaman tidak ditemukan di alamatnya (HTTP 404 atau 410). Penyebab umum: artikel dihapus atau ditarik, atau alamatnya berubah | Buka alamatnya untuk memastikan. Ajukan klaim bila memenuhi syarat |
| **Tautan dihapus** | Halaman masih tayang, tetapi tautan ke situs Anda tidak ditemukan lagi di isi artikel | Periksa artikel. Ajukan klaim bila memenuhi syarat |
| **Tak terjangkau** | Situs menolak akses atau sedang bermasalah, atau memberi respons lain yang bukan tayang, 404, maupun 410. Belum tentu hilang | Buka alamatnya sendiri. Tunggu pengecekan berikutnya |
| **Belum ada URL** | Belum ada hasil pengecekan atau URL terbit yang bisa dicek. Artikel yang baru terbit dan belum pernah diperiksa juga tampil begini | Tunggu pengecekan berikutnya atau klik **Crawl Sekarang** |

Teks kecil **Isi artikel belum bisa diperiksa** di bawah status berarti Tauthub belum dapat membaca isi halaman atau artikel tidak memuat tautan acuan. Arahkan penunjuk ke teks itu untuk melihat alasannya.

### Aksi Garansi

| Tampilan | Artinya |
|---|---|
| Tombol **Ajukan Klaim** | Artikel **Hilang (404)** atau **Tautan dihapus**, media terverifikasi, masih dalam garansi, dan belum ada klaim |
| **Klaim diajukan** | Klaim masuk dan menunggu tim Tauthub |
| **Sedang diperiksa** | Tim memeriksa kondisi domain |
| **Menghubungi pemilik domain** | Tim menghubungi pemilik media |
| **Dana dikembalikan** | Klaim disetujui. Nilai pembelian artikel dikembalikan ke **Saldo** |
| **Klaim ditolak** | Klaim ditolak. Tidak ada tombol lanjutan |
| **Tidak Digaransi** | Media belum terverifikasi |
| Tanda — | Tidak ada tindakan karena artikel sehat |

## Cara menggunakan

### Cara memeriksa kondisi tautan

1. Buka menu **Link Guardian**.
2. Baca empat kartu di bagian atas.
3. Cari artikel yang bertanda merah atau kuning pada kolom **Status Deteksi**.
4. Klik alamat artikel untuk membukanya dan memastikan sendiri.

### Cara menjalankan pengecekan manual

1. Klik **Crawl Sekarang**. Tombol berubah menjadi **Mengantrekan…**.
2. Baca pesan hijau di atas tabel, misalnya "Antrean pengecekan dibuat: 12 artikel."
3. Muat ulang halaman setelah beberapa saat untuk melihat hasilnya.

Setelah satu pengecekan manual, tombol nonaktif selama 24 jam. Arahkan penunjuk ke tombol untuk melihat kapan Anda bisa mencoba lagi. Jatah harian hanya terpakai bila ada artikel yang benar-benar diantrekan. Sekali tekan memeriksa paling banyak 200 artikel, dan sisanya menyusul pada siklus otomatis berikutnya. Bila semua artikel baru saja diperiksa, tampil "Semua artikel baru saja diperiksa, belum ada yang perlu dicek ulang."

### Cara mencari dan menyaring artikel

1. Ketik kata kunci pada kolom pencarian, lalu tekan **Enter**.
2. Pilih status pada daftar di sebelahnya.
3. Klik **Filter** untuk memilih **Media** atau **Tanggal**.
4. Klik **Reset filter** untuk mengembalikan semua pilihan.

### Cara mengajukan klaim garansi

1. Temukan artikel berstatus **Hilang (404)** atau **Tautan dihapus**. Filter **Bermasalah saja (SLA Klaim)** hanya menampilkan yang **Hilang (404)**, jadi gunakan daftar penuh untuk **Tautan dihapus**.
2. Pastikan kolom **Status Verifikasi** menunjukkan **Terverifikasi**.
3. Klik **Ajukan Klaim** pada kolom **Aksi Garansi**. Tombol berubah menjadi **Mengirim…**.
4. Baca pesan "Klaim diajukan. Tim Tauthub akan memeriksa kondisi tautan dan menindaklanjuti."
5. Pantau kolom **Aksi Garansi**. Muat ulang halaman untuk melihat status terbaru.

Tim Tauthub memeriksa kondisi domain dan menghubungi pemilik media. Bila masalah tidak dapat diselesaikan, tim mengembalikan saldo senilai pembelian artikel itu. Saat pengembalian, sistem memeriksa ulang bahwa artikel masih dalam masa garansi, media terverifikasi, dan artikel benar-benar 404 atau tautannya dicabut. Setiap artikel hanya punya satu klaim.

## Aturan dan batasan

- Artikel yang sudah lewat masa garansi tidak tampil.
- Klaim hanya dapat diajukan untuk **Hilang (404)** atau **Tautan dihapus**. Status **Tak terjangkau** tidak dapat diklaim.
- Klaim yang ditolak tidak bisa diajukan ulang dari halaman ini.
- Hasil pengecekan hanya tampil di halaman ini, dan halaman tidak memperbarui diri. Buka halaman secara berkala dan muat ulang untuk melihat hasil terbaru.

## Pesan galat yang umum

| Pesan | Penyebab dan cara mengatasi |
|---|---|
| "Halaman tidak dapat dimuat" | Data gagal diambil, misalnya karena koneksi terputus. Klik **Coba lagi**. |
| "Gagal memuat daftar artikel." | Daftar gagal dimuat. Klik **Coba lagi** di bawah pesan. |
| "Pengecekan manual dibatasi 1x sehari. Coba lagi nanti." | Jatah harian sudah terpakai. Tunggu 24 jam sejak pengecekan manual terakhir. |
| "Gagal mengantrekan pengecekan." atau "Gagal mengajukan klaim." | Permintaan gagal diproses. Coba lagi beberapa saat kemudian. |
| "Brand not found" | Peran Anda belum **Writer**, sehingga **Crawl Sekarang** dan **Ajukan Klaim** ditolak. Minta admin brand menaikkan peran Anda. |

Selama satu klaim sedang dikirim, tombol **Ajukan Klaim** di baris lain nonaktif sementara.

## Pertanyaan yang sering diajukan

**Apakah Tak terjangkau berarti artikel hilang?**
Belum tentu. Sistem keamanan sebagian media memblokir pengecek otomatis. Buka alamat artikel sendiri untuk memastikan.

**Mengapa artikel saya tidak ada di daftar?**
Artikel mungkin masih draf di situs mitra, belum punya tautan terbit, atau sudah lewat masa garansi media.

**Mengapa tombol Ajukan Klaim tidak muncul?**
Tombol hanya muncul untuk artikel bermasalah pada media terverifikasi yang masih dalam garansi dan belum pernah diklaim.

**Di mana saya melihat hasil pemeriksaan lain, seperti sentimen artikel?**
Di [Status Publikasi](05-status-publikasi.md), pada baris publikasi yang sudah terbit.

## Lihat juga

- [Status Publikasi](05-status-publikasi.md)
- [Katalog Media](02-katalog-media/README.md)
- [Saldo dan Top Up](08-saldo-dan-top-up.md)
- [Analisis Kinerja dan Audit Publikasi](07-analisis-dan-audit.md)
