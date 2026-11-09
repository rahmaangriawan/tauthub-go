# Publikasi

> Menu **Publikasi** menampung semua pesanan artikel dari klien untuk media Anda. Di sini Anda membaca artikel, menyetujui atau menolaknya, memastikan artikel tayang di situs Anda, lalu memantau penghasilannya.

**Lokasi:** menu **Publikasi** di sidebar Dashboard Mitra (grup **MEDIA & PUBLIKASI**)
**Untuk:** Mitra (akun Mitra yang berstatus aktif)

## Gambaran singkat

Satu **publikasi** adalah satu artikel yang dikirim klien ke satu media milik Anda. Misalkan klien mengirim artikel yang sama ke tiga media Anda. Maka daftar berisi tiga publikasi. Masing-masing punya nomor order, status, dan penghasilan sendiri.

Tiap ada publikasi baru untuk media Anda, menu **Publikasi** menampilkan angka merah. Angka itu hilang begitu Anda membuka halamannya. Aturan lengkapnya ada di [Membaca daftar Publikasi](01-daftar-publikasi.md).

Hanya akun Mitra yang aktif yang bisa membuka halaman ini. Halaman dan tombol di bagian ini sama untuk semua peran dan tingkat VIP.

## Alur dari permintaan klien sampai terbayar

1. Klien memilih media Anda lewat Katalog Media atau kampanye Taut Pintar, lalu menyiapkan artikel dan mengirimnya. Saat itu Tauthub menahan saldo klien sebesar nilai pesanan.
2. Publikasi muncul di menu **Publikasi** dengan status **Menunggu persetujuan**, dan Anda menerima notifikasi. Media Anda memakai **Secure Direct**? Maka langkah persetujuan dilewati dan publikasi langsung masuk **Antrean**.
3. Anda membaca artikel lewat **Pratinjau Artikel**, hasil pemeriksaan isi otomatis, dan catatan klien. Bila perlu, Anda mengubahnya lewat **Edit draf** atau bertanya lewat **Hubungi Klien**.
4. Anda menyetujui atau menolak dalam 3×24 jam. Bila Anda menolak, saldo klien kembali. Bila Anda tidak menjawab, pesanan batal sendiri dan saldo kembali ke klien.
5. Setelah Anda menyetujui, artikel terbit dengan salah satu dari dua cara (lihat bagian berikutnya).
6. Saat artikel berstatus **Terbit**, saldo klien yang ditahan menjadi terpakai. Tauthub juga memeriksa URL (alamat) artikel itu.
7. Penghasilan dari artikel yang terbit dicatat di halaman **Penghasilan**. Beda antara media **API** dan **Manual** dijelaskan di [Status publikasi dan penghasilan](09-status-dan-penghasilan.md). Setelah masa tunggu selesai, Anda bisa mencairkannya lewat **Invoice**.
8. Klien bisa meminta revisi atas artikel yang sudah terbit. Artikel itu berstatus **Perlu Revisi** sampai Anda menerima atau menolak permintaannya.

## Dua cara artikel terbit

Kolom **Mode Integrasi** di daftar menunjukkan siapa yang menerbitkan artikel. Anda mengatur mode ini untuk tiap media di [Daftar Media](../04-daftar-media/README.md).

| Mode Integrasi | Yang terjadi setelah Anda menyetujui |
|---|---|
| **API** | Tauthub mengirim artikel ke situs Anda lewat integrasi, yaitu sambungan antara situs Anda dan Tauthub ([Integrasi](../13-integrasi/README.md)). Status berjalan dari **Antrean** ke **Diproses**, lalu **Terbit**. |
| **Manual** | Anda menempel artikel di situs sendiri, lalu mengirim URL artikelnya ke Tauthub. Status menjadi **Menunggu URL** sampai URL terkirim, lalu **Terbit**. |

Panduan lengkapnya ada di [Cara artikel terbit di situs Anda](07-cara-artikel-terbit.md).

> **Catatan:** Seberapa cepat Anda menyetujui dan menerbitkan memengaruhi rata-rata waktu terbit media Anda. Angka itu tampil sebagai **SLA** di Katalog Media yang dilihat klien. SLA adalah janji waktu kerja.

## Hal yang perlu diingat

- Batas persetujuan adalah **3×24 jam**, tanpa tambahan waktu. Pesanan yang lewat batas batal sendiri.
- Klien bisa memilih jadwal terbit. Tetapi jadwal itu tidak menerbitkan artikel yang belum Anda setujui.
- Pada media **Manual**, status **Terbit** muncul begitu Anda mengirim URL. Jadi pastikan URL-nya sudah benar.
- Obrolan dengan klien dari publikasi berlaku **3×24 jam** sejak pesan pertama.
- Jumlah di kolom **Penghasilan** adalah penghasilan bersih setelah dipotong biaya platform. Uang itu baru benar-benar masuk setelah artikel terbit.

## Isi bagian ini

- [Membaca daftar Publikasi](01-daftar-publikasi.md): tab status, pencarian, ekspor CSV (simpan daftar sebagai berkas tabel), kolom, tombol aksi, dan angka merah di menu.
- [Detail publikasi](02-detail-publikasi.md): isi jendela Detail, URL terbit, pemeriksaan, dan riwayat pengiriman.
- [Menyetujui atau menolak publikasi](03-menyetujui-dan-menolak.md): batas 3×24 jam, alasan penolakan, dan akibatnya pada saldo klien.
- [Revisi dari klien](04-revisi-dari-klien.md): menerima atau menolak permintaan revisi.
- [Pratinjau artikel dan pemeriksaan konten](05-pratinjau-artikel.md): membaca artikel sebelum terbit dan memahami hasil pemeriksaan otomatis.
- [Mengedit draf artikel](06-mengedit-draf-artikel.md): editor judul, isi, gambar utama, meta deskripsi, kategori, dan Rewrite AI.
- [Cara artikel terbit di situs Anda](07-cara-artikel-terbit.md): Mode API dan Manual, status Antrean, Secure Direct, dan pengiriman URL.
- [Menghubungi klien dari publikasi](08-hubungi-klien.md): alasan obrolan, catatan, dan sesi 3×24 jam.
- [Status publikasi dan penghasilan](09-status-dan-penghasilan.md): tabel status lengkap, notifikasi, dan kapan penghasilan tercatat.

## Lihat juga

- [Daftar Media](../04-daftar-media/README.md): mengatur mode integrasi, Secure Direct, dan aturan media.
- [Integrasi](../13-integrasi/README.md): menyambungkan situs agar artikel bisa dikirim otomatis.
- [Penghasilan](../09-penghasilan.md) dan [Invoice dan pencairan](../10-invoice-dan-pencairan.md): memantau dan mencairkan penghasilan.
- [Pesan](../11-pesan.md): membaca dan membalas obrolan dengan klien.
- [Notifikasi](../15-notifikasi.md): pengaturan pemberitahuan.
- [Ringkasan Dashboard Mitra](../01-ringkasan.md) dan [Mengenal Tauthub](../../umum/01-mengenal-tauthub.md).
