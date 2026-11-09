# Status publikasi dan penghasilan

> Halaman ini berisi tabel lengkap status publikasi beserta langkah Anda berikutnya, daftar notifikasi yang Anda terima, dan penjelasan kapan penghasilan dari sebuah publikasi tercatat.

**Lokasi:** menu **Publikasi**, kolom **Status**, dan dialog **Detail publikasi**
**Untuk:** Mitra

## Gambaran singkat

Setiap publikasi berpindah status mengikuti perjalanan artikelnya. Daftar dan dialog **Detail publikasi** memakai nama yang sedikit berbeda untuk beberapa status. Berkas ekspor CSV (berkas tabel yang bisa dibuka di Excel) memakai kode berbahasa Inggris. Tabel di bawah memuat ketiganya.

## Status dan artinya

| Status di daftar | Nama di dialog Detail | Kode di CSV | Artinya | Langkah Anda berikutnya |
|---|---|---|---|---|
| **Menunggu** | **Pending** | `pending` | Status sementara saat pesanan baru dibuat. Segera pindah ke **Menunggu persetujuan** atau **Antrean**. Hanya terlihat di tab **Semua**. | Tidak ada |
| **Menunggu persetujuan** | **Menunggu** | `waiting_partner` | Artikel menunggu keputusan Anda | Baca artikelnya, lalu setujui atau tolak sebelum batas 3×24 jam. Lihat [Menyetujui atau menolak publikasi](03-menyetujui-dan-menolak.md) |
| **Disetujui**, atau **Menunggu URL** pada media **Manual** yang belum punya URL | **Disetujui** | `partner_approved` | Anda sudah menyetujui. Pada media **API**, status ini hanya sesaat sebelum **Antrean**. Pada media **Manual**, artikel menunggu URL dari Anda | Media **Manual**: terbitkan di situs Anda, lalu klik **Kirim URL**. Lihat [Cara artikel terbit di situs Anda](07-cara-artikel-terbit.md) |
| **Antrean** | **Diantrekan** | `queued` | Artikel siap dikirim ke situs Anda dan menunggu giliran atau jadwal terbit | Tunggu. Arahkan kursor ke lencana untuk melihat jadwalnya |
| **Diproses** | **Diproses** | `processing` | Artikel sedang dikirim ke situs Anda | Tunggu |
| **Coba ulang** | **Mencoba ulang** | `retrying` | Pengiriman gagal dan akan dicoba lagi otomatis | Tunggu. Bila berulang, baca **Riwayat** dan periksa [Integrasi](../13-integrasi/README.md) |
| **Terbit** | **Terbit** | `published` | Artikel sudah tayang | Periksa URL terbit dan hasil pemeriksaannya di dialog Detail |
| **Gagal** | **Gagal** | `failed` | Pengiriman gagal setelah percobaan terakhir, atau dihentikan oleh pemeriksaan keamanan isi artikel. Saldo klien dikembalikan | Baca **Riwayat**, perbaiki penyebabnya, lalu minta klien mengirim ulang |
| **Ditolak** (tab **Ditolak Mitra**) | **Ditolak** | `partner_rejected` | Anda menolak publikasi ini | Tidak ada. Alasan Anda tampil di bawah status |
| **Dibatalkan** | **Dibatalkan** | `cancelled` | Pesanan batal sendiri karena lewat 3×24 jam, atau dibatalkan klien atau tim Tauthub | Tidak ada. Baca alasan pembatalan di bawah status |
| **Perlu Revisi** (tab **Revisi**) | **Revisi dari klien** | `revision` | Klien meminta perubahan pada artikel yang sudah terbit | Terima atau tolak revisi. Lihat [Revisi dari klien](04-revisi-dari-klien.md) |

> **Catatan:** Klien bisa mengembalikan publikasi yang gagal atau dibatalkan ke tahap persetujuan. Publikasi itu kembali berstatus **Menunggu persetujuan**, dan Anda menerima notifikasi baru.

Tab di atas daftar menyaring status seperti ini. **Menunggu** berisi **Menunggu persetujuan**, **Disetujui**, **Antrean**, **Diproses**, dan **Coba ulang**. **Revisi**, **Sudah Terbit**, **Gagal**, dan **Ditolak Mitra** masing-masing berisi satu status. **Semua** berisi semuanya, termasuk **Dibatalkan** dan **Menunggu**.

## Notifikasi untuk Anda

Notifikasi (pemberitahuan) tampil di lonceng. Pilih notifikasi tentang sebuah publikasi, maka dialog **Detail publikasi** artikel itu terbuka. Sebagian kejadian juga dikirim lewat email atau WhatsApp. Atur pilihannya di [Notifikasi](../15-notifikasi.md).

| Kejadian | Judul notifikasi |
|---|---|
| Pesanan baru menunggu keputusan Anda | **Publikasi menunggu persetujuan** |
| Artikel berjadwal masih menunggu persetujuan dan jadwalnya sudah dekat (sekitar 2 jam) | **Publikasi membutuhkan persetujuan** |
| Batas 3×24 jam lewat dan pesanan dibatalkan | **Pesanan dibatalkan (SLA)** |
| Klien menyetujui artikel | **Artikel disetujui klien** |
| Klien meminta revisi atas artikel yang sudah terbit | **Revisi artikel dari klien** |
| Artikel berhasil terbit | **Artikel berhasil diterbitkan** |
| Pengiriman gagal | **Publikasi gagal** |
| Klien meminta URL artikel | **Permintaan URL publikasi** |
| Tim Tauthub membatalkan pesanan | **Pesanan dibatalkan oleh admin** |
| Akun WordPress yang tersambung tidak bisa membuat kategori | **Kategori artikel belum bisa dibuat di (nama media)** |
| Klien mengirim pesan di obrolan | **Pesan baru dari (nama brand klien)**. Lihat [Pesan](../11-pesan.md) |

## Kapan penghasilan tercatat

Jumlah di kolom **Penghasilan** pada daftar hanya perkiraan sampai artikel berstatus **Terbit**. Pada publikasi yang ditolak, dibatalkan, atau gagal, tidak ada penghasilan yang masuk.

Hitungannya sama dengan panel **Rincian penghasilan**: **Harga diajukan** dikurangi **Potongan platform** (saat ini 25%) sama dengan **Pendapatan Anda**. Baris lama kadang tidak punya data potongan. Maka daftar menampilkan angka yang tercatat tanpa potongan.

- **Media API:** penghasilan tercatat sendiri tepat saat pengiriman berhasil dan status menjadi **Terbit**. Penghasilan hanya dicatat sekali per publikasi. Pengiriman ulang setelah revisi tidak mencatatnya lagi.
- **Media Manual:** periksa halaman **Penghasilan** setelah status berubah menjadi **Terbit**. Jumlahnya belum muncul di sana? Hubungi menu **Dukungan** dan sebutkan nomor order publikasi.
- Penghasilan yang baru tercatat punya masa tunggu. Uangnya baru bisa ditarik 2 hari setelah artikel terbit, sesuai keterangan di halaman **Penghasilan**. Setelah itu Anda bisa mencairkannya lewat [Invoice dan pencairan](../10-invoice-dan-pencairan.md).

Lihat [Penghasilan](../09-penghasilan.md) untuk rincian saldo dan riwayat penghasilan.

### Efek ke saldo klien

Saldo klien ikut berubah mengikuti status publikasi:

| Kejadian | Saldo klien |
|---|---|
| Klien mengirim artikel dan pesanan dibuat | Nilai pesanan ditahan |
| Artikel berstatus **Terbit** | Nilai yang ditahan menjadi terpakai |
| Anda menolak publikasi | Dikembalikan ke klien |
| Pesanan **Dibatalkan** (otomatis, oleh klien, atau oleh tim Tauthub) | Dikembalikan ke klien |
| Pengiriman **Gagal** setelah percobaan terakhir | Dikembalikan ke klien |

## Pertanyaan yang sering diajukan

**Kenapa jumlah penghasilan sudah tampil padahal artikel belum terbit?**
Itu perkiraan berdasarkan harga dan potongan platform saat ini. Penghasilan baru tercatat setelah artikel **Terbit**.

**Kenapa publikasi yang sudah Antrean kembali ke Menunggu persetujuan?**
Klien bisa mengembalikan publikasi ke tahap persetujuan. Anda menerima notifikasi **Publikasi menunggu persetujuan** lagi. Batas 3×24 jam tetap dihitung dari jadwal terbit atau dari saat pesanan dibuat, jadi sisa waktunya bisa pendek. Baca keterangan sisa waktu di bawah lencana status.

**Kenapa publikasi yang saya tolak masih terlihat?**
Publikasi yang ditolak tetap tersimpan sebagai riwayat. Buka tab **Ditolak Mitra** untuk melihatnya.

## Lihat juga

- [Membaca daftar Publikasi](01-daftar-publikasi.md)
- [Detail publikasi](02-detail-publikasi.md)
- [Penghasilan](../09-penghasilan.md)
- [Invoice dan pencairan](../10-invoice-dan-pencairan.md)
- [Notifikasi](../15-notifikasi.md)
