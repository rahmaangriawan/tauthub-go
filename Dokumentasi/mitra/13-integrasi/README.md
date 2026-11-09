# Integrasi

> Menu Integrasi menyambungkan website media Anda ke Tauthub. Dengan begitu artikel Klien terkirim sendiri lewat WordPress, EmDash, Web Custom, atau Blogspot. Halaman ini menjelaskan alur umum, tab-tab di layar, dan kapan media Anda tampil di Katalog Media.

**Lokasi:** menu **Integrasi** (grup **PENGATURAN**)
**Untuk:** Mitra

## Gambaran singkat

Setiap media Anda punya salah satu dari dua cara menerbitkan artikel Klien. Anda memilihnya di [Daftar Media](../04-daftar-media/README.md), pada bagian **Integrasi & publikasi**.

- **Manual:** Anda menempel artikel ke website sendiri, lalu mengirim URL artikelnya di Tauthub. URL adalah alamat artikel di internet. Status menjadi **Terbit** saat URL dikirim. Manual tidak perlu integrasi.
- **API:** Tauthub mengirim artikel ke website Anda secara otomatis setelah Anda menyetujuinya. API adalah jalur yang membuat dua sistem bisa saling kirim data tanpa campur tangan Anda. Media yang memakai **Secure Direct** mengirim tanpa menunggu persetujuan. Mode ini perlu integrasi yang tersambung, dan itulah isi menu **Integrasi**.

Setiap media API memakai satu platform tujuan: **WordPress**, **EmDash**, **Blogspot**, atau **Custom Web**. Platform di sini artinya jenis website atau blog tujuan. Setiap platform punya tab sendiri di halaman ini.

## Alur menyambungkan media

1. Klik **Tambah Media** di [Daftar Media](../04-daftar-media/README.md). Pada bagian **Integrasi & publikasi**, pilih **API**, lalu pilih **Platform Tujuan**.
2. Isi kolom **Website / Domain**, centang persetujuan **Ketentuan Kerja Sama Mitra**, lalu simpan. Kotak persetujuan wajib dicentang sebelum media bisa disimpan.
3. Tunggu tim Tauthub menyetujui media. Formulir menyarankan Anda menyambungkan integrasi setelah media disetujui.
4. Buka menu **Integrasi** dan setujui **Syarat & Ketentuan** (cukup sekali).
5. Buka tab platform media Anda dan ikuti panduannya.
6. Jalankan uji koneksi sampai berhasil.
7. Periksa ikon mata di Daftar Media. Media yang sudah tersambung dan memenuhi syarat lain tampil di Katalog Media.

## Yang tampil di layar

Bagian atas halaman berisi judul **Integrasi**, keterangan "Sambungkan website media Anda agar artikel klien terkirim otomatis lewat WordPress, EmDash, website custom, atau Blogspot", dan status syarat di sisi kanan. Status itu berbunyi "Syarat disetujui" disertai tanggal, atau "Syarat belum disetujui".

### Tab dan titik status

Baris tab berisi lima tab: **Syarat & Ketentuan**, **WordPress**, **EmDash**, **Web Custom**, dan **Blogspot**. Di layar HP, baris tab bisa digeser ke samping.

| Tampilan tab | Artinya |
|---|---|
| Ikon gembok | Syarat & Ketentuan belum disetujui. Tab kanal terkunci, dan teks bantuan menyebut bahwa tab terbuka setelah syarat disetujui. Kanal artinya jalur pengiriman, yaitu tab untuk tiap platform. |
| Abu-abu dan tidak bisa diklik | Belum ada media Anda yang memakai platform itu. Arahkan kursor ke tab untuk membaca petunjuknya: pilih platform itu saat menambah atau mengedit media. |
| Angka dan titik berwarna | Angka adalah jumlah koneksi aktif (tab **WordPress** dan **EmDash**) atau jumlah media (tab **Web Custom** dan **Blogspot**). Titik merangkum statusnya. |

Tab sebuah platform bisa dibuka kalau ada media Anda yang memakai platform itu. Untuk **WordPress** dan **EmDash**, koneksi aktif juga cukup. Kalau Anda belum punya media dengan platform apa pun, tab **WordPress** tetap bisa dibuka supaya halaman tidak kosong.

Titik status memakai warna yang sama di seluruh halaman:

| Warna | Artinya |
|---|---|
| Hijau | **Terhubung** |
| Merah | Gagal. Salah satu koneksi atau media di tab itu gagal diuji |
| Kuning | Sedang diuji |
| Abu-abu | Belum terhubung atau belum diuji |

Pada titik di tab, merah menang atas kuning, dan kuning menang atas hijau. Satu koneksi yang gagal sudah membuat titik tab menjadi merah.

### Tab Syarat & Ketentuan

Tab ini terbuka lebih dulu saat Anda membuka menu **Integrasi**. Pengecualiannya kalau Anda datang lewat tautan ke tab platform tertentu. Kartu **Syarat & Ketentuan Integrasi** berisi empat pokok:

- Dengan menyetujui, Anda mengizinkan Tauthub mengirim artikel Klien ke akun atau website media Anda lewat kanal yang Anda hubungkan.
- Anda bertanggung jawab memastikan data masuk (kredensial) yang Anda isi benar, dan akun yang terhubung milik media Anda. Tauthub tidak pernah mengubah artikel yang sudah terbit tanpa tindakan Anda.
- Waktu persetujuan Anda dicatat. Teks syarat menyebut bahwa Anda bisa mencabut persetujuan dengan memutus koneksi di tab yang bersangkutan.
- Akses ke media Anda di Tauthub hanya boleh diberikan kepada anggota berperan penulis, atau paling tinggi editor. Peran yang lebih tinggi tidak boleh mengelola media.

## Cara menggunakan

### Cara menyetujui Syarat & Ketentuan

1. Klik menu **Integrasi**. Tab **Syarat & Ketentuan** terbuka.
2. Baca kartu **Syarat & Ketentuan Integrasi**.
3. Centang "Saya menyetujui Syarat & Ketentuan Integrasi Tauthub di atas".
4. Klik **Setujui dan lanjutkan**. Tombol nonaktif sampai kotak dicentang.
5. Di dialog **Setujui Syarat & Ketentuan?**, klik **Ya, setujui**. Klik **Batal** kalau Anda berubah pikiran.

Setelah itu, bagian atas halaman menampilkan "Syarat disetujui" dengan tanggal, tab kanal terbuka, dan kartu syarat menampilkan "Disetujui pada" beserta tanggal dan jam. Klik **Buka kanal** untuk membuka tab pertama yang bisa dipakai.

> **Catatan:** Syarat ini juga dianggap disetujui saat Anda menyimpan profil di menu **Konfigurasi** › tab **Profil**, kalau syarat belum disetujui. Layar itu menampilkan peringatan "Syarat Integrasi Belum Disetujui" dengan keterangan "Dengan menyimpan profil ini, Anda menyatakan menyetujui syarat integrasi publikasi Tauthub." Lihat [Konfigurasi](../14-profil-dan-konfigurasi/README.md).

### Cara memilih kanal

| Kanal | Cocok untuk | Yang Anda perlukan | Panduan |
|---|---|---|---|
| **WordPress** | Website WordPress | Username dan Application Password | [Integrasi WordPress](01-wordpress.md) |
| **EmDash** | Website EmDash | Access Token pribadi | [Integrasi EmDash](02-emdash.md) |
| **Web Custom** | Website buatan sendiri | Alamat penerima artikel dan API key | [Integrasi Web Custom](03-web-custom.md) |
| **Blogspot** | Blog di Blogger | Akun Google pemilik blog | [Integrasi Blogspot](04-blogspot.md) |

Tab **Web Custom** juga punya kartu **Publishing API**. Penjelasannya ada di [Publishing API](05-publishing-api.md). Developer (orang yang membuat program) yang membangun penerima artikel bisa membaca [Webhook bertanda tangan](06-webhook-bertanda-tangan.md).

### Cara menuju tab yang tepat dari Daftar Media

1. Buka [Daftar Media](../04-daftar-media/README.md).
2. Cari media yang berlabel disembunyikan dari katalog karena belum tersambung.
3. Klik ikon mata di barisnya.
4. Tauthub membuka tab platform yang sesuai di **Integrasi**, dan media itu disorot. Di tab **WordPress** dan **EmDash**, formulir **Tambah koneksi** terbuka dengan media itu sudah terpilih.

Kalau media belum punya alamat website, layar menampilkan catatan agar Anda mengisinya dulu di Daftar Media.

## Kapan media tampil di Katalog Media

Media tampil di Katalog Media kalau semua syarat ini terpenuhi: media sudah disetujui dan aktif, tidak sedang ditangguhkan, harganya terisi, Anda tidak menyembunyikannya, dan, untuk media API, integrasinya tersambung. Syarat selain integrasi dijelaskan di [Daftar Media](../04-daftar-media/README.md).

Media API dianggap tersambung dalam keadaan berikut:

| Platform | Dianggap tersambung bila |
|---|---|
| **WordPress** dan **EmDash** | Ada koneksi aktif (belum dicabut) yang alamatnya sama dengan alamat website media. Awalan `www.` diabaikan. Hasil uji koneksi tidak ikut dinilai. |
| **Blogspot** | Status koneksi media adalah **Terhubung**. |
| **Custom Web** | Status koneksi media adalah **Terhubung**, artinya uji koneksi terakhir berhasil. Kalau Anda menyimpan perubahan alamat atau API key, status kembali ke **Belum terhubung**. |
| Media berjenis **Tulisio API** | Adapter URL media sudah terisi. Lihat [Publishing API](05-publishing-api.md). |
| Media **Manual** | Tidak perlu koneksi. |

Klien yang mencoba memesan media yang belum tersambung melihat pesan "Media ini belum tersambung ke sistem publikasi, jadi belum bisa dipesan. Silakan pilih media lain." Pesanan yang sudah berjalan tidak dibatalkan hanya karena media disembunyikan dari katalog.

## Aturan dan batasan

- Syarat & Ketentuan cukup disetujui sekali. Sebelum itu, tab kanal terkunci di layar.
- Setiap media memakai satu platform tujuan.
- Data masuk yang sudah tersimpan tidak ditampilkan lagi. Kolom Application Password, Access Token, dan API key menampilkan isinya dalam bentuk tersamar.
- Pesan galat pengiriman artikel tampil di bagian **Riwayat** pada detail publikasi di menu **Publikasi**. Lihat [Publikasi](../03-publikasi/README.md).
- Tautan lama ke halaman Koneksi Media, API Publishing, dan Blogger otomatis dialihkan ke tab yang sesuai.

## Pertanyaan yang sering diajukan

**Mengapa tab platform saya abu-abu?**
Belum ada media Anda yang memakai platform itu. Pilih platform tersebut di formulir media, atau buat media baru dengan platform itu.

**Mengapa tab terkunci?**
Syarat & Ketentuan belum disetujui. Setujui dulu di tab **Syarat & Ketentuan**.

**Mengapa media saya disembunyikan dari Katalog Media?**
Salah satu syarat tampil belum terpenuhi. Arahkan kursor ke ikon mata di Daftar Media untuk membaca alasannya. Kalau alasannya belum tersambung, klik ikon itu untuk menuju tab yang tepat.

**Di mana saya melihat artikel yang gagal terkirim?**
Di menu **Publikasi**. Buka detail publikasinya, lalu lihat bagian **Riwayat**.

**Apakah Tauthub mengubah artikel yang sudah terbit di website saya?**
Tidak, kalau Anda tidak melakukan sesuatu lebih dulu. Contohnya, saat Anda menerima revisi dari Klien, Tauthub mengirim ulang artikel itu.

## Isi bagian ini

- [Integrasi WordPress](01-wordpress.md): menyambungkan WordPress dengan Application Password, peran akun, dan pesan hasil uji.
- [Integrasi EmDash](02-emdash.md): menyambungkan EmDash dengan token akses pribadi dan memindahkan gambar ke media library.
- [Integrasi Web Custom](03-web-custom.md): menyambungkan website buatan sendiri lewat alamat penerima dan API key, beserta aturan Publisher Protocol untuk developer.
- [Integrasi Blogspot](04-blogspot.md): menghubungkan akun Google dan memilih blog tujuan.
- [Publishing API](05-publishing-api.md): kartu kunci API di tab Web Custom untuk media jenis lama, beserta aturan tanda tangannya.
- [Webhook bertanda tangan](06-webhook-bertanda-tangan.md): aturan terbuka untuk developer, berisi event, header tanda tangan, dan contoh isi pesan.

## Lihat juga

- [Daftar Media](../04-daftar-media/README.md)
- [Publikasi](../03-publikasi/README.md)
- [Link Guardian](../05-link-guardian.md): pemantauan artikel yang sudah terbit di website Anda.
- [Konfigurasi](../14-profil-dan-konfigurasi/README.md)
- [Dukungan](../12-dukungan.md)
