# Link Guardian

> Alat yang memeriksa artikel terbit Anda secara berkala. Ia melihat apakah artikel masih bisa dibuka dan apakah tautan ke situs Anda masih ada di dalamnya. Bila artikel hilang atau tautan dihapus, Anda bisa mengajukan klaim garansi.

**Lokasi:** menu **Link Guardian** (grup **RUANG KERJA**)

**Untuk:** Klien. Semua peran di brand bisa melihat halaman ini. Untuk **Crawl Sekarang** dan **Ajukan Klaim**, peran Anda harus **Writer** atau lebih tinggi.

## Gambaran singkat

Link Guardian memeriksa empat hal pada setiap artikel yang sudah terbit:

1. **Halaman artikel**: apakah alamatnya masih bisa dibuka.
2. **Tautan di dalam artikel**: apakah tautan ke situs Anda masih ada di isi artikel.
3. **Atribut tautan**: apakah tautan utama masih sesuai kebijakan media. Untuk media yang menjanjikan **Dofollow**, tautan yang berubah menjadi nofollow, sponsored, atau ugc dihitung sebagai perubahan.
4. **Noindex**: apakah artikel ditandai agar tidak muncul di Google.

Yang diperiksa hanya artikel berstatus **Terbit** yang punya alamat terbit, masih dalam masa garansi media, dan bukan draf (naskah yang belum tayang). Artikel yang dikirim sebagai draf baru diperiksa setelah mitra mempublikasikannya.

Tauthub memeriksa artikel sendiri setiap hari selama 30 hari pertama setelah terbit, lalu seminggu sekali selama masa garansi. Anda juga bisa meminta pemeriksaan sendiri, paling banyak sekali sehari.

Garansi berjalan otomatis. Bila masalah terbaca pada dua pemeriksaan berturut-turut, Tauthub membuka klaim untuk Anda dan memberi tahu lewat notifikasi. Anda tidak perlu mengajukannya.

Garansi hanya berlaku untuk **Media Terverifikasi**. Ini media yang kepemilikan domain (alamat situs)-nya sudah dipastikan oleh Tauthub. Di [Katalog Media](02-katalog-media/README.md), media seperti itu bertanda centang biru "Domain Terverifikasi". Anda bisa menyaringnya lewat **Domain terverifikasi**.

Lama garansi beda-beda untuk tiap media: **Permanen**, atau sekian bulan sejak tanggal terbit. Label kebijakan tautan di baris media pada Katalog Media menunjukkannya, misalnya **2 Dofollow • 12 Bln SLA**. SLA di sini berarti lama garansi. Setelah masa garansi habis, artikel hilang dari halaman ini.

## Yang tampil di layar

### Bagian atas

- Tombol **Crawl Sekarang** untuk memasukkan permintaan pemeriksaan manual ke antrean.
- Tulisan **Cek Manual Maks. 1x Sehari**.
- **Terakhir sinkronisasi**, misalnya "Hari ini, 08:30 WIB". Ini waktu pemeriksaan terbaru atas artikel Anda.

### Empat kartu

| Kartu | Isi |
|---|---|
| **Total Dipantau** | Jumlah artikel yang diperiksa ("Dalam masa garansi aktif") |
| **Tayang & Aman** | Halaman masih bisa dibuka (200 OK) |
| **Tak Terjangkau** | Situs menolak akses atau sedang bermasalah. Keterangannya "Sistem keamanan media memblokir bot" |
| **Bermasalah** | Artikel tidak ditemukan atau 404 ("404 atau Tautan dihapus (SLA Klaim)") |

Setiap kartu memuat angka dan persen dari **Total Dipantau**. Grafik gelombang kecil hanya hiasan arah kecenderungan. Pegangan Anda adalah angka dan persennya.

> **Catatan:** angka di kartu mengikuti status halaman. Bila halaman artikel masih tayang tetapi tautan Anda sudah dihapus, artikel itu masuk hitungan **Tayang & Aman**. Di tabel, artikel itu tetap tampil merah sebagai **Tautan dihapus**, dan Anda bisa mengklaimnya.

### Pencarian dan filter

- Kolom pencarian ("Cari artikel, media, atau URL…") mencari judul artikel, nama media, dan alamat. Tekan **Enter** untuk mulai mencari.
- Pilihan status: **Semua Status**, **Bermasalah saja (SLA Klaim)**, **Tak Terjangkau**, **Tayang & Aman**, dan **Belum ada URL**. Pilihan **Bermasalah saja (SLA Klaim)** menampilkan artikel **Hilang (404)**.
- Tombol **Filter** membuka panel berisi **Media**, **Tanggal** (tanggal tayang: **7 Hari Terakhir**, **30 Hari Terakhir**, **3 Bulan Terakhir**), **Reset filter**, dan **Terapkan**. Pilihan Anda langsung berlaku. **Terapkan** hanya menutup panel. Angka kecil di tombol menunjukkan jumlah filter yang aktif.
- Pilihan **Media** hanya berisi media yang ada di halaman yang sedang terbuka.

### Tabel artikel

| Kolom | Isi |
|---|---|
| **Informasi Artikel & Tautan** | Gambar kecil, alamat artikel (klik untuk membuka di tab baru), judul, dan "Anchor:" bila diketahui. Anchor adalah kata yang diklik untuk menuju situs Anda |
| **Publisher** | Nama media dan **Tgl Tayang** |
| **Status Verifikasi** | **Terverifikasi** atau **Belum terverifikasi** |
| **Status Deteksi** | Hasil pemeriksaan (lihat tabel di bawah) |
| **Waktu Cek** | Waktu relatif ("3 hari yang lalu") dan tanggal lengkap, atau "Belum pernah" |
| **Aksi Garansi** | Tombol atau status klaim |

Klik judul kolom **Informasi Artikel & Tautan** atau **Waktu Cek** untuk mengurutkan. Urutan hanya berlaku di halaman yang terbuka. Tanpa Anda ubah, baris berurutan menurut judul dari Z ke A.

Di bawah tabel ada tulisan "Menampilkan 1–10 dari N artikel", pilihan **10 per halaman**, **20 per halaman**, atau **50 per halaman**, serta tombol **Sebelumnya** dan **Berikutnya**.

Bila daftar kosong, layar menulis "Tidak ada masalah pada artikel Anda". Tulisan yang sama juga muncul bila belum ada artikel yang diperiksa, atau filter tidak menemukan hasil.

### Tampilan di HP

Di layar sempit, tabel berubah menjadi satu kartu untuk tiap artikel. Setiap bagian punya label sendiri, jadi Anda tidak perlu menggeser ke samping. Kartu ringkasan tersusun ke bawah. Bilah pencarian menempel di bawah bilah atas saat Anda menggulir. Tombol **Filter** hanya berupa ikon.

## Status dan artinya

### Status Deteksi

| Status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **200 OK (aman)** | Halaman artikel bisa dibuka | Tidak perlu berbuat apa pun |
| **Hilang (404)** | Halaman tidak ada di alamatnya (HTTP 404 atau 410, yaitu kode "halaman tidak ditemukan"). Sebabnya biasanya artikel dihapus atau ditarik, atau alamatnya berganti | Buka alamatnya untuk memastikan. Ajukan klaim bila memenuhi syarat |
| **Tautan dihapus** | Halaman masih tayang, tetapi tautan ke situs Anda sudah tidak ada di isi artikel | Periksa artikelnya. Ajukan klaim bila memenuhi syarat |
| **Noindex** | Halaman tayang, tetapi ditandai noindex sehingga tidak muncul di Google | Minta mitra menghapus tanda itu, atau pilih penyelesaian garansi |
| **Atribut tautan berubah** | Media menjanjikan Dofollow, tetapi tautan sekarang nofollow, sponsored, atau ugc | Pilih penyelesaian garansi |
| **Tak terjangkau** | Situs menolak akses atau sedang bermasalah, atau memberi jawaban lain yang bukan tayang, 404, maupun 410. Artikel belum tentu hilang | Buka alamatnya sendiri. Tunggu pemeriksaan berikutnya |
| **Belum ada URL** | Belum ada hasil pemeriksaan, atau belum ada alamat terbit yang bisa diperiksa. Artikel yang baru terbit dan belum pernah diperiksa juga tampil begini | Tunggu pemeriksaan berikutnya atau klik **Crawl Sekarang** |

Tulisan kecil **Isi artikel belum bisa diperiksa** di bawah status berarti Tauthub belum bisa membaca isi halaman, atau artikel tidak punya tautan acuan. Arahkan penunjuk ke tulisan itu untuk melihat alasannya.

### Aksi Garansi

| Tampilan | Artinya |
|---|---|
| **Garansi aktif: pilih penyelesaian** | Masalah terbaca dua kali berturut-turut dan klaim sudah dibuka otomatis. Pilih **Artikel pengganti** atau **Kredit kembali** |
| **Mencarikan media pengganti** | Anda memilih artikel pengganti dan Tauthub sedang mencarikan media |
| **Artikel pengganti dibuat** | Artikel dibuatkan ulang di media lain tanpa biaya tambahan |
| Tombol **Ajukan Klaim** | Artikel **Hilang (404)** atau **Tautan dihapus**, media terverifikasi, masih dalam garansi, dan belum ada klaim |
| **Klaim diajukan** | Klaim sudah masuk dan menunggu tim Tauthub |
| **Sedang diperiksa** | Tim sedang memeriksa kondisi domain |
| **Menghubungi pemilik domain** | Tim sedang menghubungi pemilik media |
| **Dana dikembalikan** | Klaim disetujui. Nilai pembelian artikel kembali ke **Saldo** |
| **Klaim ditolak** | Klaim ditolak. Tidak ada tombol lanjutan |
| **Tidak Digaransi** | Media belum terverifikasi |
| Tanda strip | Tidak ada tindakan karena artikel sehat |

## Cara menggunakan

### Cara memeriksa kondisi tautan

1. Buka menu **Link Guardian**.
2. Baca empat kartu di bagian atas.
3. Cari artikel yang bertanda merah atau kuning di kolom **Status Deteksi**.
4. Klik alamat artikel untuk membukanya dan memastikan sendiri.

### Cara menjalankan pemeriksaan manual

1. Klik **Crawl Sekarang**. Tombol berubah menjadi **Mengantrekan…**.
2. Baca pesan hijau di atas tabel, misalnya "Antrean pengecekan dibuat: 12 artikel."
3. Beberapa saat kemudian, muat ulang halaman untuk melihat hasilnya.

Setelah satu pemeriksaan manual, tombol mati selama 24 jam. Arahkan penunjuk ke tombol untuk melihat kapan Anda bisa mencoba lagi. Jatah harian hanya terpakai bila ada artikel yang benar-benar masuk antrean. Sekali tekan, paling banyak 200 artikel diperiksa. Sisanya menyusul pada pemeriksaan otomatis berikutnya. Bila semua artikel baru saja diperiksa, layar menulis "Semua artikel baru saja diperiksa, belum ada yang perlu dicek ulang."

### Cara mencari dan menyaring artikel

1. Ketik kata kunci di kolom pencarian, lalu tekan **Enter**.
2. Pilih status di daftar sebelahnya.
3. Klik **Filter** untuk memilih **Media** atau **Tanggal**.
4. Klik **Reset filter** untuk mengembalikan semua pilihan.

### Cara memilih penyelesaian garansi

1. Buka notifikasi **Garansi tautan aktif**, atau buka menu **Link Guardian**.
2. Cari artikel dengan tulisan **Garansi aktif: pilih penyelesaian** di kolom **Aksi Garansi**.
3. Klik **Artikel pengganti** bila Anda tetap ingin artikelnya tayang. Tauthub memilih media lain yang cocok dengan harga tidak lebih tinggi dari pembelian awal. Atau klik **Kredit kembali** bila Anda lebih suka dananya kembali ke **Saldo**.
4. Konfirmasi pilihan Anda. Pilihan tidak bisa dibatalkan dan hanya bisa dibuat satu kali per artikel.

Bila tidak ada media pengganti yang cocok, Anda diberi tahu dan bisa memilih **Kredit kembali**. Penghasilan mitra atas artikel yang bermasalah dibatalkan, jadi Tauthub tidak membayar artikel yang tidak bertahan.

### Cara mengajukan klaim garansi

1. Cari artikel berstatus **Hilang (404)** atau **Tautan dihapus**. Filter **Bermasalah saja (SLA Klaim)** hanya menampilkan yang **Hilang (404)**. Jadi, pakai daftar lengkap untuk mencari **Tautan dihapus**.
2. Pastikan kolom **Status Verifikasi** menunjukkan **Terverifikasi**.
3. Klik **Ajukan Klaim** di kolom **Aksi Garansi**. Tombol berubah menjadi **Mengirim…**.
4. Baca pesan "Klaim diajukan. Tim Tauthub akan memeriksa kondisi tautan dan menindaklanjuti."
5. Pantau kolom **Aksi Garansi**. Muat ulang halaman untuk melihat status terbaru.

Tim Tauthub memeriksa kondisi domain dan menghubungi pemilik media. Bila masalah tidak bisa diselesaikan, tim mengembalikan saldo sebesar harga pembelian artikel itu. Saat mengembalikan, sistem memeriksa lagi tiga hal: artikel masih dalam masa garansi, media terverifikasi, dan artikel memang 404 atau tautannya sudah dicabut. Setiap artikel hanya punya satu klaim.

## Aturan dan batasan

- Artikel yang sudah lewat masa garansi tidak tampil.
- Klaim hanya bisa diajukan untuk **Hilang (404)** atau **Tautan dihapus**. Status **Tak terjangkau** tidak bisa diklaim.
- Klaim yang ditolak tidak bisa diajukan lagi dari halaman ini.
- Hasil pemeriksaan hanya tampil di halaman ini, dan halaman tidak berubah sendiri. Buka halaman secara berkala dan muat ulang untuk melihat hasil terbaru.

## Pesan galat yang umum

| Pesan | Penyebab dan cara mengatasi |
|---|---|
| "Halaman tidak dapat dimuat" | Data gagal diambil, misalnya karena koneksi putus. Klik **Coba lagi**. |
| "Gagal memuat daftar artikel." | Daftar gagal dimuat. Klik **Coba lagi** di bawah pesan. |
| "Pengecekan manual dibatasi 1x sehari. Coba lagi nanti." | Jatah harian sudah terpakai. Tunggu 24 jam sejak pemeriksaan manual terakhir. |
| "Gagal mengantrekan pengecekan." atau "Gagal mengajukan klaim." | Permintaan gagal diproses. Coba lagi beberapa saat kemudian. |
| "Brand not found" | Peran Anda belum **Writer**, jadi **Crawl Sekarang** dan **Ajukan Klaim** ditolak. Minta admin brand menaikkan peran Anda. |

Selama satu klaim sedang dikirim, tombol **Ajukan Klaim** di baris lain mati sementara.

## Pertanyaan yang sering diajukan

**Apakah Tak terjangkau berarti artikel hilang?**
Belum tentu. Sistem keamanan sebagian media memblokir pemeriksa otomatis. Buka alamat artikel sendiri untuk memastikan.

**Mengapa artikel saya tidak ada di daftar?**
Artikel mungkin masih draf di situs mitra, belum punya alamat terbit, atau sudah lewat masa garansi media.

**Mengapa tombol Ajukan Klaim tidak muncul?**
Tombol hanya muncul untuk artikel bermasalah di media terverifikasi yang masih dalam garansi dan belum pernah diklaim.

**Di mana saya melihat hasil pemeriksaan lain, seperti nada berita artikel?**
Di [Status Publikasi](05-status-publikasi.md), pada baris publikasi yang sudah terbit.

## Lihat juga

- [Status Publikasi](05-status-publikasi.md)
- [Katalog Media](02-katalog-media/README.md)
- [Saldo dan Top Up](08-saldo-dan-top-up.md)
- [Analisis Kinerja dan Audit Publikasi](07-analisis-dan-audit.md)
