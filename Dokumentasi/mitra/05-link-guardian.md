# Link Guardian

> Link Guardian menunjukkan apakah artikel Klien yang terbit di media Anda masih bisa dibuka, dan apakah domain media Anda sehat. Pakai halaman ini untuk menemukan artikel atau domain bermasalah, lalu memperbaikinya.

**Lokasi:** menu **Link Guardian** (grup **MEDIA & PUBLIKASI**)
**Untuk:** Mitra

## Gambaran singkat

Link Guardian memeriksa alamat (URL) setiap artikel Klien yang sudah terbit di media Anda, lalu mencatat hasilnya: halaman masih tayang, hilang (404), atau tidak terjangkau. Klien melihat hasil yang sama untuk artikel mereka di [Link Guardian sisi Klien](../klien/06-link-guardian.md).

Halaman ini juga memantau kesehatan domain setiap media Anda: apakah situs bisa dibuka dan apakah kepemilikan domain sudah terverifikasi.

Halaman ini hanya menampilkan data. Tidak ada tombol yang mengubah artikel, saldo, atau status publikasi.

### Artikel yang dipantau

Sebuah artikel ikut dipantau bila semua syarat ini terpenuhi:

- Artikel sudah terbit dan **URL Terbit**-nya terisi. URL terisi otomatis untuk pengiriman lewat WordPress dan Blogspot, serta lewat Web Custom bila situs Anda mengembalikan alamat artikel. Untuk media Manual, Anda mengirim URL sendiri. Untuk integrasi lain, lengkapi lewat **Perbarui URL** di menu **Publikasi**.
- Artikel masih dalam masa pemantauan. Masa ini mengikuti pengaturan **Masa berlaku tautan (backlink)** pada media Anda di [Daftar Media](04-daftar-media/README.md). Pilihan **Permanen** berarti dipantau terus. Pilihan **Jangka Waktu** berarti dipantau selama 12, 24, atau 36 bulan sejak tanggal terbit.
- Artikel tidak terbit sebagai draf di website Anda. Draf belum tampil untuk publik, jadi tidak diperiksa.

### Kapan pemeriksaan berjalan

- **Artikel:** pemeriksaan pertama berjalan tak lama setelah artikel terbit, lalu diulang kira-kira setiap 30 hari.
- **Domain media:** diperiksa kira-kira seminggu sekali, selama media punya alamat website.
- **Pemeriksaan tambahan:** Klien pemilik artikel dapat menjalankannya dari Link Guardian mereka (dibatasi sekali sehari). Tim Tauthub juga dapat menjalankannya. Di Dashboard Mitra tidak ada tombol pemeriksaan manual.
- **Cara memeriksa:** pemeriksa membuka alamat artikel dengan identitas peramban yang memuat nama `TauthubLinkCheckBot`, menunggu paling lama 15 detik, dan mengikuti pengalihan (redirect) sampai 5 kali. Yang dinilai adalah halaman akhirnya.

## Yang tampil di layar

### Panel ringkasan

- **Diawasi:** jumlah artikel yang dipantau. Di bawahnya ada bilah berwarna dan tiga angka: **Tidak bermasalah** (hijau), **Tidak terjangkau** (kuning), dan **Hilang (404)** (merah). Angka **Tidak bermasalah** dihitung dari **Diawasi** dikurangi dua angka lainnya, jadi artikel yang belum pernah diperiksa ikut terhitung di sini.
- **Domain down:** jumlah media dengan domain berstatus **Tidak terjangkau**. Angka berwarna merah bila lebih dari 0.
- **Perlu perhatian:** jumlah media dengan kesehatan domain **Belum diperiksa** atau **Token hilang**. Angka berwarna kuning bila lebih dari 0.

Angka ringkasan dimuat saat halaman dibuka dan tidak berubah ketika Anda memakai filter.

### Tab Tautan Artikel

Tab ini terbuka lebih dulu. Daftarnya memuat kolom berikut.

| Kolom | Isi |
|---|---|
| **Artikel** | Gambar kecil artikel (bila ada) dan judul |
| **Media** | Nama media, lalu jenis dan kategori media |
| **URL** | Alamat artikel. Klik untuk membuka di tab baru |
| **Status** | Lencana status tautan |
| **Diperiksa** | Tanggal pemeriksaan terakhir, dengan keterangan seperti "hari ini" atau "3 hari lalu" |

Di komputer, arahkan kursor ke lencana **Status** untuk melihat kode HTTP dan jumlah gagal berturut-turut, misalnya "HTTP 404 · 2 gagal berturut-turut". Artikel yang paling baru terdeteksi bermasalah tampil paling atas. Setiap halaman memuat 20 baris, dengan tulisan "Menampilkan … dari … tautan" dan tombol **Sebelumnya** serta **Berikutnya**.

Alat penyaring di atas daftar:

- Kolom pencarian **Cari judul, media, atau URL…**. Ketik sebagian judul, nama media, atau alamat, lalu tekan Enter.
- **Status tautan:** **Bermasalah (404 / tidak terjangkau)** (bawaan), **Semua status**, **Hilang (404)**, **Tidak terjangkau**, **Tayang**, atau **URL belum ada**.
- **Filter media:** **Semua media** atau satu media. Pilihannya mengikuti daftar yang sedang tampil di tab **Kesehatan Domain**, jadi media berdomain sehat bisa belum muncul. Pakai kolom pencarian bila nama media tidak ada di pilihan.
- **Rentang waktu:** **Semua waktu**, **7 hari terakhir**, **30 hari terakhir**, atau **90 hari terakhir**. Rentang ini dihitung dari tanggal pemeriksaan terakhir, bukan tanggal terbit.
- Tombol **Reset filter** mengosongkan pencarian dan ketiga pilihan di atas.

Bila daftar kosong, termasuk saat filter Anda tidak menemukan hasil, layar menampilkan "Tidak ada tautan bermasalah. Semua artikel sehat."

### Tab Kesehatan Domain

Tab ini memuat satu baris per media. Pilihan **Kesehatan domain** menyaring daftar: **Perlu perhatian (belum diperiksa / token hilang / down)** (bawaan), **Aktif**, **Tidak terjangkau**, **Belum diperiksa**, atau **Token hilang**.

| Kolom | Isi |
|---|---|
| **Media** | Nama media |
| **Domain** | Alamat website media, atau tanda — bila belum diisi |
| **Kesehatan** | Lencana kesehatan domain |
| **Verifikasi** | **Terverifikasi** atau **Belum** |
| **Gagal** | Jumlah pemeriksaan domain yang gagal berturut-turut. Angka kembali ke 0 saat domain berhasil diverifikasi |
| **Diperiksa** | Tanggal pemeriksaan domain terakhir |

Bila tidak ada media yang perlu perhatian, layar menampilkan "Tidak ada domain yang perlu perhatian."

### Perbedaan HP dan komputer

Di layar lebar, kedua tab berbentuk tabel. Di layar HP, setiap artikel dan setiap domain tampil sebagai kartu dengan data yang sama. Keterangan kode HTTP di lencana **Status** hanya muncul di perangkat yang punya kursor.

## Cara menggunakan

### Cara melihat artikel bermasalah

1. Klik menu **Link Guardian**.
2. Baca panel ringkasan. Angka **Hilang (404)** dan **Tidak terjangkau** di atas 0 berarti ada artikel yang perlu dicek.
3. Tetap di tab **Tautan Artikel**. Daftar bawaan hanya memuat artikel **Hilang (404)** dan **Tidak terjangkau**.
4. Klik alamat di kolom **URL** untuk membuka artikel dan melihat kondisinya.

### Cara melihat semua artikel, termasuk yang sehat

1. Buka tab **Tautan Artikel**.
2. Buka pilihan **Status tautan**.
3. Pilih **Semua status** untuk melihat semuanya, atau **Tayang** untuk melihat artikel yang sehat saja.

### Cara memeriksa kesehatan domain

1. Klik tab **Kesehatan Domain**.
2. Lihat kolom **Kesehatan**, **Verifikasi**, dan **Gagal** pada media Anda.
3. Ubah **Kesehatan domain** ke **Aktif** bila Anda ingin melihat media yang sehat.

### Cara menindaklanjuti artikel Hilang (404)

1. Klik alamat di kolom **URL** untuk memastikan halamannya memang tidak ada.
2. Cari penyebabnya di website Anda: artikel terhapus, dipindah ke draf, atau alamatnya berubah.
3. Pulihkan artikel, atau kembalikan alamat lamanya. Pengalihan (redirect) dari alamat lama ke alamat baru juga cukup, selama halaman akhirnya terbuka normal.
4. Bila alamat artikel sengaja berubah, buka detail publikasinya di menu **Publikasi**, klik **Perbarui URL**, isi alamat baru, lalu klik **Simpan URL**.
5. Tunggu pemeriksaan berikutnya. Status di halaman ini diperbarui setelah artikel diperiksa lagi.

### Cara menindaklanjuti artikel Tidak terjangkau

1. Buka alamat artikel di peramban Anda.
2. Bila halaman terbuka normal, kemungkinan firewall atau CDN situs menolak pemeriksa. Izinkan permintaan yang User-Agent-nya memuat `TauthubLinkCheckBot`.
3. Bila halaman tidak terbuka, periksa hosting atau server Anda.
4. Tunggu pemeriksaan berikutnya.

## Status dan artinya

### Status tautan artikel

| Status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **Tayang** | Halaman menjawab dengan sukses (kode HTTP 2xx). | Tidak perlu tindakan. |
| **Hilang (404)** | Halaman menjawab 404 atau 410, yaitu tidak ditemukan atau sudah dihapus. | Pulihkan artikel atau alamatnya, lalu perbarui URL di **Publikasi** bila alamat berubah. |
| **Tidak terjangkau** | Halaman tidak bisa dibuka saat pemeriksaan: situs menolak (misalnya kode 403 atau 429), server error (5xx), atau koneksi gagal. Belum tentu artikel dihapus. | Pastikan situs hidup dan izinkan pemeriksa di firewall. |
| **URL belum ada** | URL Terbit artikel kosong. | Isi URL lewat **Perbarui URL** di **Publikasi**. |
| **Belum diperiksa** | Artikel belum pernah diperiksa. Status ini hanya tampil bila **Status tautan** diatur ke **Semua status**. | Tunggu pemeriksaan pertama. |

Hanya kode 404 dan 410 yang dihitung **Hilang (404)**. Kode lain yang gagal masuk **Tidak terjangkau**, supaya artikel tidak dianggap dihapus tanpa bukti.

### Kesehatan domain

| Status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **Aktif** | Pemeriksaan terakhir membuktikan situs hidup: domain berhasil diverifikasi, atau situs merespons walau menolak akses dari server Tauthub atau punya masalah sertifikat. | Tidak perlu tindakan. |
| **Tidak terjangkau** | Pemeriksaan tidak bisa membuka situs, misalnya domain tidak ditemukan atau halaman utama menjawab error. | Buka situs, pulihkan hosting atau DNS, lalu tunggu pemeriksaan berikutnya. |
| **Belum diperiksa** | Belum ada hasil yang menandai situs **Aktif** atau **Tidak terjangkau**. Umum pada media baru, media tanpa alamat website, dan domain yang belum terverifikasi. | Isi alamat website di Daftar Media dan verifikasi domain. |
| **Token hilang** | Token verifikasi domain tidak ditemukan di situs. | Pasang ulang token verifikasi (lihat Daftar Media). |

Bila kolom **Diperiksa** sudah terisi tetapi **Kesehatan** masih **Belum diperiksa**, biasanya situs bisa dibuka namun token verifikasi belum terpasang. Cara memverifikasi domain (lewat meta tag atau catatan TXT DNS) dijelaskan di [Daftar Media](04-daftar-media/README.md).

## Aturan dan batasan

- **Verifikasi domain bisa dicabut.** Bila domain yang sudah terverifikasi gagal diperiksa tiga kali berturut-turut, status verifikasinya dicabut dan kolom **Verifikasi** kembali menjadi **Belum**. Pemeriksaan yang hanya ditolak firewall tidak dihitung gagal.
- **Garansi tayang untuk Klien.** Garansi mengikuti masa pemantauan media dan berlaku untuk media berdomain terverifikasi. Klien dapat mengajukan klaim untuk artikel yang berstatus 404 atau yang tautan Klien di dalam artikelnya dihapus. Tim Tauthub memeriksa klaim itu dan dapat menghubungi pemilik domain. Klaim dan hasilnya tidak ditampilkan di halaman ini.
- **Tautan Klien di dalam artikel.** Selain status halaman, pemeriksa membaca isi artikel dan membandingkannya dengan tautan Klien yang seharusnya ada. Bila tautan itu dihapus, sisi Klien menampilkan **Tautan dihapus**, sedangkan halaman Anda tetap **Tayang**. Jangan menghapus atau mengubah tautan Klien pada artikel yang sudah terbit.
- **Domain tanpa alamat website** tidak diperiksa dan tetap berstatus **Belum diperiksa**.
- **Notifikasi domain.** Bila situs Anda diblokir firewall atau gagal diakses tiga kali berturut-turut, Anda menerima notifikasi berjudul "Situs … tidak bisa diakses crawler Tauthub", paling sering sekali per media setiap 30 hari. Lihat [Akses Crawler](06-akses-crawler.md).

## Pertanyaan yang sering diajukan

**Mengapa daftar kosong padahal artikel saya sudah terbit?**
Daftar bawaan hanya memuat artikel bermasalah. Ubah **Status tautan** ke **Semua status**. Artikel juga tidak tampil bila URL Terbitnya belum terisi, masa pemantauannya sudah lewat, atau terbit sebagai draf.

**Mengapa artikel saya berstatus Tidak terjangkau padahal bisa dibuka di peramban?**
Firewall atau CDN situs mungkin menolak permintaan dari server Tauthub. Izinkan User-Agent yang memuat `TauthubLinkCheckBot`, lalu tunggu pemeriksaan berikutnya.

**Kapan status diperbarui setelah saya memperbaiki artikel?**
Pada pemeriksaan berikutnya, kira-kira setiap 30 hari untuk tiap artikel. Klien atau tim Tauthub dapat menjalankan pemeriksaan tambahan lebih cepat. Anda tidak bisa memicunya dari Dashboard Mitra.

**Apakah halaman ini memengaruhi Penghasilan saya?**
Halaman ini hanya menampilkan data. Bila Anda ingin tahu status klaim Klien atau dampaknya, hubungi [Dukungan](12-dukungan.md).

## Lihat juga

- [Daftar Media](04-daftar-media/README.md): pengaturan **Masa berlaku tautan (backlink)** dan verifikasi domain.
- [Publikasi](03-publikasi/README.md): **Perbarui URL** dan riwayat pengiriman artikel.
- [Akses Crawler](06-akses-crawler.md): mengizinkan crawler Tauthub di balik firewall.
- [Integrasi](13-integrasi/README.md): menyambungkan website agar artikel terkirim otomatis.
- [Link Guardian sisi Klien](../klien/06-link-guardian.md)
- [Dukungan](12-dukungan.md)
