# Pusat Penjual

> Pusat Penjual adalah tempat harian Anda sebagai Mitra: apa yang perlu dikerjakan hari ini, pesanan yang menunggu, penghasilan, trafik dari Google, dan minat klien pada tiap media.

**Lokasi:** menu **RINGKASAN** › **Pusat Penjual**
**Untuk:** Mitra

## Gambaran singkat

Halaman ini disusun dari yang paling mendesak. Bagian paling atas, **Perlu dikerjakan**, berisi tugas yang kalau dibiarkan membuat Anda rugi, misalnya pesanan yang hampir dibatalkan otomatis atau media yang tidak tampil di katalog klien. Di bawahnya ada pesanan yang menunggu, penghasilan, trafik dari Google, dan minat klien pada tiap media.

Semua angka dihitung dari data Anda di Tauthub. Yang belum ada datanya ditulis **Belum ada data**, bukan angka nol atau perkiraan.

## Yang tampil di layar

### Perlu dikerjakan

Setiap tugas punya tingkat dan tombol **Buka** yang membawa Anda langsung ke halaman yang tepat.

| Tingkat | Artinya |
|---|---|
| **Mendesak** | Ada batas waktu atau uang yang terancam, misalnya pesanan dengan sisa waktu kurang dari 24 jam atau artikel yang tautannya harus diperbaiki |
| **Perlu perhatian** | Menghalangi order, misalnya media belum tersambung, belum punya harga, atau situsnya tidak bisa diakses |
| **Saran** | Menambah kepercayaan klien, misalnya verifikasi domain dan verifikasi Dewan Pers |

Tugas yang mungkin muncul:

- **Setujui pesanan**: pesanan yang menunggu persetujuan Anda. Tenggatnya 72 jam sejak jadwal terbit pesanan, atau sejak pesanan masuk bila tidak dijadwalkan. Lewat dari itu pesanan dibatalkan otomatis dan tidak ada penghasilan.
- **Media tidak tampil di katalog**: belum tersambung, belum punya harga, Anda sembunyikan, ditangguhkan, atau belum aktif. Setiap alasan punya langkah perbaikannya.
- **Situs tidak bisa diakses** dan **crawler gagal membaca situs**.
- **Perbaiki tautan**: ada artikel yang tautannya bermasalah dan klaim garansi sudah terbuka. Lihat [Link Guardian](05-link-guardian.md).
- **Isi nomor WhatsApp** dan **isi rekening penarikan**.
- **Verifikasi domain** dan **verifikasi Dewan Pers**.

Bila tidak ada tugas, halaman menulis bahwa media Anda siap menerima order.

### Pesanan menunggu Anda

Daftar pesanan yang menunggu persetujuan, diurutkan dari yang tenggatnya paling dekat, dengan sisa waktunya. Di bawahnya:

- **Rata-rata waktu tayang** dari artikel yang sudah terbit.
- **Pesanan yang Anda tolak** sebagai persentase dari pesanan yang sudah Anda tangani.
- **Dibatalkan karena tidak direspons** dalam 30 hari terakhir.

### Penghasilan

Saldo yang bisa ditarik sekarang, penghasilan yang masih dalam masa tahan 48 jam, jumlah 30 hari terakhir, dan yang sudah dicairkan, lengkap dengan grafik batang per hari. Penarikan tetap dilakukan di halaman [Penghasilan](09-penghasilan.md).

### Trafik dari Google

Bagian ini memakai Google Search Console, alat gratis dari Google yang menunjukkan berapa kali situs Anda muncul dan diklik di hasil pencarian.

1. Klik **Hubungkan Search Console** dan setujui akses baca saja di akun Google Anda.
2. Tauthub mencocokkan situs di Search Console Anda dengan media Anda lewat nama domainnya.
3. Klik **Ambil data** untuk pengambilan pertama. Setelah itu data diambil otomatis seminggu sekali.

Yang tampil: **Klik**, **Tayangan**, dan **Posisi rata-rata** untuk 28 hari terakhir, serta perubahan klik dibanding pengambilan sebelumnya.

> **Catatan:** Search Console tidak memberi tahu halaman mana yang sudah terindeks lewat jalur ini, jadi Tauthub tidak menulis status terindeks. Bila situs Anda belum terdaftar di Search Console, tambahkan dan verifikasi dulu di sana, lalu hubungkan ulang.

Bila klik turun lebih dari 30 persen dari pengambilan sebelumnya (dengan dasar minimal 50 klik), Anda menerima notifikasi **Trafik dari Google turun**.

### Minat klien

Satu baris per media menunjukkan apakah klien benar-benar memilihnya, dihitung dari kampanye klien dalam 30 hari terakhir:

- **Dipilih klien**: berapa kali media Anda dimasukkan ke kampanye, dan dari berapa brand.
- **Jadi pesanan**: berapa di antaranya yang lanjut menjadi pesanan.
- **Dibatalkan**: berapa yang dibatalkan.

Garis tipis di bawahnya menunjukkan porsi yang jadi pesanan. Nama brand tidak ditampilkan.

Media yang belum tampil di katalog menulis alasannya (misalnya **Belum tersambung**), karena klien belum bisa memilihnya. Media yang tampil tetapi belum dipilih siapa pun menulis **Belum ada klien yang memilih media ini dalam 30 hari terakhir**.

> **Catatan:** Jumlah tayang di katalog dan penanda bookmark klien belum tercatat di server, jadi tidak ada di sini. Angka di bagian ini hanya yang benar-benar tercatat.

## Notifikasi dari Pusat Penjual

- **Pesanan hampir kedaluwarsa**: dikirim sekali, saat sisa waktu persetujuan tinggal sekitar 24 jam. Lewat lonceng, notifikasi perangkat, dan WhatsApp bila nomor Anda sudah diisi dan Anda mengizinkannya.
- **Trafik dari Google turun**: dikirim paling banyak sekali per pengambilan data, lewat lonceng dan notifikasi perangkat.

## Lihat juga

- [Ringkasan](01-ringkasan.md)
- [Daftar Media](04-daftar-media/README.md)
- [Link Guardian](05-link-guardian.md)
- [Penghasilan](09-penghasilan.md)
