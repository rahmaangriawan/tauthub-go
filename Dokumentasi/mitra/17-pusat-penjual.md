# Pusat Penjual

> Pusat Penjual adalah tempat harian Anda sebagai Mitra: apa yang perlu dikerjakan hari ini, pesanan yang menunggu, penghasilan, trafik dari Google, dan kondisi tiap media.

**Lokasi:** menu **RINGKASAN** › **Pusat Penjual**
**Untuk:** Mitra

## Gambaran singkat

Halaman ini disusun dari yang paling mendesak. Bagian paling atas, **Perlu dikerjakan**, berisi tugas yang kalau dibiarkan membuat Anda rugi, misalnya pesanan yang hampir dibatalkan otomatis atau media yang tidak tampil di katalog klien. Di bawahnya ada pesanan yang menunggu, penghasilan, trafik dari Google, dan kondisi tiap media.

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

### Kondisi media

Satu kartu per media berisi:

- **Tampil di katalog** atau alasan tidak tampil.
- DA dan PA, kunjungan bulanan, artikel tayang, order berjalan, rating kecepatan, dan rata-rata waktu tayang. Media yang belum pernah menerbitkan artikel ditulis **Belum ada riwayat**.
- **Harga Anda** dibandingkan dengan media sebaya.
- Status domain, Dewan Pers, dan sambungan ke Tauthub.

#### Pembanding harga sebaya

Harga Anda dibandingkan dengan harga media aktif lain. Pembanding dipilih bertahap: kategori dan jenis media yang sama dengan DA serupa (selisih paling banyak 10), lalu kategori dan jenis media yang sama, lalu jenis media yang sama. Tahap dipakai bila menghasilkan minimal 5 pembanding. Bila tidak ada tahap yang cukup, halaman menulis **Data pembanding belum cukup**.

Anda melihat kisaran (dari seperempat terbawah sampai seperempat teratas) dan median, serta apakah harga Anda **Di bawah**, **Dalam**, atau **Di atas** kisaran. Identitas media pembanding tidak pernah ditampilkan.

> **Penting:** Ini pembanding harga, bukan jaminan laku. Harga di atas kisaran bisa membuat order lebih sepi, tetapi situs yang memang lebih kuat boleh lebih mahal.

## Notifikasi dari Pusat Penjual

- **Pesanan hampir kedaluwarsa**: dikirim sekali, saat sisa waktu persetujuan tinggal sekitar 24 jam. Lewat lonceng, notifikasi perangkat, dan WhatsApp bila nomor Anda sudah diisi dan Anda mengizinkannya.
- **Trafik dari Google turun**: dikirim paling banyak sekali per pengambilan data, lewat lonceng dan notifikasi perangkat.

## Lihat juga

- [Ringkasan](01-ringkasan.md)
- [Daftar Media](04-daftar-media/README.md)
- [Link Guardian](05-link-guardian.md)
- [Penghasilan](09-penghasilan.md)
