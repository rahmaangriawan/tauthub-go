# Cara artikel terbit di situs Anda

> Setelah Anda menyetujui, artikel terbit lewat salah satu dari dua jalur: Tauthub mengirimnya otomatis ke situs Anda (**API**) atau Anda menerbitkannya sendiri lalu mengirim URL-nya (**Manual**). Panduan ini juga menjelaskan status **Antrean**, **Secure Direct**, dan pesan galat pengiriman.

**Lokasi:** menu **Publikasi** › kolom **Mode Integrasi**, ikon **Kirim URL**, dan bagian **Riwayat** di dialog **Detail publikasi**
**Untuk:** Mitra

## Gambaran singkat

Kolom **Mode Integrasi** menunjukkan jalur yang dipakai media tiap publikasi. Mode ini diatur per media di [Daftar Media](../04-daftar-media/README.md), dan sambungan ke situs diatur di [Integrasi](../13-integrasi/README.md).

| Mode Integrasi | Siapa yang menerbitkan | Status setelah Anda menyetujui |
|---|---|---|
| **API** | Tauthub, lewat integrasi WordPress, EmDash, Blogspot, atau Custom Web | **Antrean**, lalu **Diproses**, lalu **Terbit** |
| **Manual** | Anda, di situs Anda sendiri | **Menunggu URL**, lalu **Terbit** setelah URL terkirim |

## Mode API

### Alur status

1. Anda menyetujui artikel. Status sesaat menjadi **Disetujui**, lalu **Antrean**.
2. Saat giliran tiba, status menjadi **Diproses** dan Tauthub mengirim artikel ke situs Anda, lengkap dengan gambar utama, kategori, dan isi.
3. Setelah situs Anda menerima artikel, status menjadi **Terbit**. Pada sebagian besar integrasi, URL terbit terisi otomatis dari alamat artikel yang dikembalikan situs.

Di baris daftar, persentase kecil di bawah status menunjukkan kemajuan pengiriman. Daftar menyegarkan dirinya sendiri, jadi Anda tidak perlu memuat ulang halaman.

### Status Antrean

**Antrean** berarti artikel sudah boleh terbit dan menunggu giliran dikirim ke situs Anda. Status ini muncul:

- setelah Anda menyetujui artikel pada media **API**;
- langsung setelah klien mengirim artikel, bila media memakai **Secure Direct**;
- setelah Anda menerima revisi klien, atau setelah klien mencoba ulang pengiriman yang gagal.

Jika klien memilih jadwal terbit, artikel menunggu di **Antrean** sampai waktu jadwal. Arahkan kursor ke lencana **Antrean** untuk membaca "Terjadwal terbit" disertai tanggal dan jam, atau "Menunggu antrean penerbitan" bila jadwalnya sudah lewat. Pada publikasi tanpa jadwal, pengiriman dimulai segera.

### Bila pengiriman bermasalah

Tauthub mencoba mengirim hingga 3 kali dalam waktu singkat. Saat percobaan gagal tetapi masih ada kesempatan, status menjadi **Coba ulang**. Jika percobaan ketiga gagal, status menjadi **Gagal**, saldo klien yang ditahan dikembalikan, dan Anda menerima notifikasi **Publikasi gagal** yang memuat penyebabnya. Beberapa penyebab, seperti pemeriksaan keamanan konten, langsung membuat status **Gagal** tanpa percobaan ulang. Setelah penyebabnya diperbaiki, klien dapat mengirim ulang dari Dashboard Klien. Hubungi menu **Dukungan** bila Anda tidak dapat memperbaikinya sendiri.

Setiap percobaan tercatat di **Riwayat** pada dialog **Detail publikasi**. Pesan penyebab tertulis dengan huruf merah. Pesan yang sering muncul:

| Pesan (awal kalimat) | Artinya | Yang dapat Anda lakukan |
|---|---|---|
| "Tautan keluar melebihi kebijakan media: artikel memuat …" | Artikel memuat tautan keluar lebih banyak dari yang diizinkan media | Hapus kelebihan tautan lewat **Edit draf** sebelum menyetujui. Jika sudah **Gagal**, klien perlu memperbaiki artikel lalu mengirim ulang |
| "Konten tidak dapat diproses karena terindikasi tidak sesuai dengan kebijakan publikasi …" | Pemeriksaan keamanan konten menghentikan pengiriman | Klien perlu memeriksa materi artikel. Hubungi **Dukungan** bila Anda menilai ini keliru |
| "Konten sedang menunggu pemeriksaan sebelum dapat diproses." | Pengiriman ditahan sampai tim Tauthub selesai memeriksa | Tunggu. Hubungi **Dukungan** bila terlalu lama |
| "No active … connection for media … — hubungkan situs … di dashboard mitra" | Situs Anda belum tersambung atau sambungannya tidak aktif | Hubungkan situs di menu **Integrasi**, lalu minta klien mengirim ulang |
| "Media Blogger belum terhubung atau belum memilih blog." atau "Kredensial Blogger tidak valid — Perlu Hubungkan Ulang." | Sambungan Blogspot belum lengkap atau kedaluwarsa | Hubungkan ulang akun Google dan pilih blog di **Integrasi** |
| "Konfigurasi Google OAuth belum diatur Admin." | Pengaturan di sisi Tauthub belum lengkap | Hubungi **Dukungan** |
| "… endpoint URL belum diatur" atau "… API key belum dikonfigurasi" | Pengaturan Custom Web belum lengkap | Lengkapi pengaturannya di **Integrasi** |
| "WordPress returned …" diikuti kode | WordPress menolak permintaan. Umumnya kode 401 atau 403 berarti akun yang tersambung ditolak | Periksa akun dan izinnya di **Integrasi**, lalu minta klien mengirim ulang |
| "Gambar cover artikel tidak bisa diunggah ke WordPress …" | Gambar utama gagal diunggah ke situs | Periksa gambar utama di **Edit draf**, lalu minta klien mengirim ulang |

> **Catatan:** Pesan galat ditulis oleh sistem dan sebagian berbahasa Inggris. Jika pesan Anda tidak ada di tabel, salin pesannya saat menghubungi **Dukungan**.

Jika akun WordPress yang tersambung tidak boleh membuat kategori baru, artikel tetap terbit tanpa kategori itu dan Anda menerima notifikasi **Kategori artikel belum bisa dibuat di (nama media)** berisi cara mengatasinya. Notifikasi ini dikirim paling banyak sekali per media setiap 14 hari.

## Mode Manual

Pada media **Manual**, Tauthub tidak mengirim apa pun ke situs Anda. Setelah persetujuan, status menjadi **Menunggu URL** dan Anda yang menerbitkan.

### Cara menerbitkan artikel manual

1. Buka ikon pensil **Edit draf** pada artikel berstatus **Menunggu persetujuan**, rapikan isinya, lalu klik **Salin Artikel**. Lihat [Mengedit draf artikel](06-mengedit-draf-artikel.md).
2. Tempel artikel di editor situs Anda dan terbitkan. Salin alamat (URL) artikel yang sudah tayang.
3. Setujui publikasi di Tauthub. Lihat [Menyetujui atau menolak publikasi](03-menyetujui-dan-menolak.md).
4. Pada jendela **Kirim URL Artikel** yang terbuka otomatis, tempel URL di kolom **URL artikel terbit**.
5. Klik **Kirim URL**. Tombol berubah menjadi **Mengirim…**.
6. Baca pesan hijau "URL artikel tersimpan — artikel ditandai terbit."

> **Penting:** Setelah disetujui, ikon pensil **Edit draf** tidak tersedia pada artikel Manual sampai artikelnya berstatus **Terbit**. Salin artikel sebelum menyetujui. Setelah disetujui, Anda masih bisa membaca isinya lewat **Pratinjau Artikel**.

Jika Anda belum menerbitkan artikel di situs saat jendela terbuka, klik **Batal**. Publikasi tetap **Menunggu URL**. Nanti, klik ikon tautan **Kirim URL** di baris artikel untuk membuka jendela yang sama.

### Aturan pengiriman URL

- URL wajib diawali `http://` atau `https://`, tidak boleh kosong, dan maksimal 1.000 karakter. Pesan yang tampil bila salah: "URL artikel tidak boleh kosong." atau "URL harus diawali http:// atau https://".
- Tombol **Kirim URL** nonaktif selama kolom kosong.
- URL hanya bisa dikirim untuk artikel yang sudah disetujui (mode manual) atau yang sudah terbit. Pada status lain, muncul pesan galat berwarna merah di jendela.
- Status langsung menjadi **Terbit** begitu URL tersimpan, sebelum verifikasi selesai. Pastikan URL sudah benar.
- Tauthub lalu memeriksa URL di belakang layar dan menampilkan hasilnya sebagai lencana di dialog Detail. Pemeriksaan melihat apakah judul artikel dan nama brand klien tampil di halaman, dan apakah URL berada di domain media Anda. Lihat [Detail publikasi](02-detail-publikasi.md).
- Klien menerima notifikasi **Artikel terbit** yang memuat URL Anda.
- Sistem tidak membatalkan otomatis publikasi yang berstatus **Menunggu URL**, tetapi klien menunggu artikel Anda tayang. Lama waktu dari pesanan sampai terbit ikut menentukan angka SLA media Anda.

Jika artikel sudah **Terbit** tetapi URL-nya belum terisi, di daftar muncul keterangan **URL belum terisi**. Jika artikel yang **Terbit** belum punya URL setelah sekitar 7 hari, klien dapat meminta URL tersebut dan Anda menerima notifikasi **Permintaan URL publikasi**. Isi URL lewat **Perbarui URL** di dialog Detail.

## Secure Direct

**Secure Direct** adalah mode terbit otomatis. Artikel klien langsung diproses dan terbit ke situs media tanpa menunggu persetujuan Anda per artikel.

- Publikasi baru dari media ber-Secure Direct tidak melewati status **Menunggu persetujuan**. Statusnya langsung **Antrean**, lalu mengikuti alur Mode API.
- Tidak ada tombol menolak untuk artikel baru tersebut. Pemeriksaan keamanan konten tetap berjalan sebelum pengiriman.
- Dialog Detail menampilkan kotak biru: "Media ini memakai Secure Direct: artikel terbit otomatis tanpa persetujuan per artikel. Kalau ingin meninjau dulu, nonaktifkan Secure Direct pada media ini."
- Secure Direct diatur per media di [Daftar Media](../04-daftar-media/README.md). Tombol **Ajukan Secure Direct** di sana mengirim pengajuan ke tim Tauthub untuk ditinjau. Setelah akun Anda diizinkan, kotak centang **Secure Direct** tampil di tiap media.
- Untuk kembali meninjau tiap artikel, hapus centang **Secure Direct** pada media itu.

## Pertanyaan yang sering diajukan

**Kenapa artikel saya masih Antrean?**
Kemungkinan klien memilih jadwal terbit. Arahkan kursor ke lencana untuk melihat jadwalnya. Jika jadwal sudah lewat, artikel menunggu giliran dikirim.

**Apakah saya bisa mengirim ulang artikel yang Gagal?**
Tidak ada tombol kirim ulang di Dashboard Mitra. Tombolnya ada di Dashboard Klien. Perbaiki penyebabnya, lalu minta klien mengirim ulang.

**Kenapa status langsung Terbit padahal saya baru mengirim URL?**
Pada media **Manual**, status berubah begitu URL tersimpan. Verifikasi berjalan sesudahnya dan hasilnya tampil sebagai lencana.

## Lihat juga

- [Detail publikasi](02-detail-publikasi.md)
- [Menyetujui atau menolak publikasi](03-menyetujui-dan-menolak.md)
- [Revisi dari klien](04-revisi-dari-klien.md)
- [Integrasi](../13-integrasi/README.md)
- [Daftar Media](../04-daftar-media/README.md)
