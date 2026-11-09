# Menyetujui atau menolak publikasi

> Setiap pesanan baru menunggu keputusan Anda. Anda punya 3×24 jam untuk menyetujui atau menolaknya. Bila waktu habis, pesanan batal sendiri dan saldo kembali ke klien.

**Lokasi:** menu **Publikasi** › tab **Menunggu**
**Untuk:** Mitra

## Gambaran singkat

Klien mengirim artikel ke media Anda. Publikasi itu berstatus **Menunggu persetujuan**, dan Anda menerima notifikasi **Publikasi menunggu persetujuan**. Anda punya tiga pilihan: menyetujui artikel, menolaknya dengan alasan, atau membiarkannya sampai batas waktu lewat.

Media Anda memakai **Secure Direct**? Maka langkah ini dilewati. Artikel klien langsung masuk **Antrean** tanpa menunggu persetujuan Anda. Lihat [Cara artikel terbit di situs Anda](07-cara-artikel-terbit.md).

## Batas waktu persetujuan 3×24 jam

- Batas 3×24 jam dihitung dari jadwal terbit yang dipilih klien. Klien tidak memilih jadwal? Maka batas dihitung dari saat pesanan dibuat. Waktunya tidak bisa ditambah.
- Sisa waktu tampil di bawah lencana status: **Sisa N hari**, **Sisa kurang dari 1 hari**, atau **Melewati SLA — dibatalkan otomatis**. SLA di sini artinya batas waktu kerja Anda.
- Jadwal terbit tidak menerbitkan artikel yang belum Anda setujui. Artikel tetap menunggu keputusan Anda sampai batas 3×24 jam lewat.
- Untuk publikasi yang punya jadwal, Tauthub mengirim pengingat **Publikasi membutuhkan persetujuan** (notifikasi dan email). Pengingat dikirim mulai sekitar 2 jam sebelum jadwal terbit, bila artikel masih menunggu.

### Bila Anda tidak menanggapi

Setelah batas lewat, sistem membatalkan pesanan sendiri. Sistem memeriksa secara berkala, jadi pembatalan terjadi segera setelah batas terlewati. Akibatnya:

- Status berubah menjadi **Dibatalkan** dengan alasan "Dibatalkan otomatis: media tidak merespons melebihi SLA 3×24 jam." Publikasi ini hanya terlihat di tab **Semua**.
- Anda menerima notifikasi **Pesanan dibatalkan (SLA)**.
- Klien menerima notifikasi **Pesanan dibatalkan, saldo dikembalikan**. Uang pesanan yang tadinya ditahan kembali ke saldo klien.

## Sebelum memutuskan

Pakai alat berikut agar keputusan Anda tepat:

- **Pratinjau Artikel** untuk membaca seluruh isi artikel. Lihat [Pratinjau artikel dan pemeriksaan konten](05-pratinjau-artikel.md).
- Hasil pemeriksaan isi otomatis di dialog Detail.
- **Catatan klien** di dialog Detail, bila klien menulis catatan.
- **Edit draf** untuk merapikan judul, isi, gambar, atau kategori sebelum Anda menyetujui. Lihat [Mengedit draf artikel](06-mengedit-draf-artikel.md).
- **Hubungi Klien** untuk menanyakan hal yang belum jelas. Lihat [Menghubungi klien dari publikasi](08-hubungi-klien.md).

## Cara menyetujui

### Dari daftar

1. Buka menu **Publikasi** dan klik tab **Menunggu**.
2. Cari baris berstatus **Menunggu persetujuan**.
3. Klik ikon pesawat kertas **Setuju & Terbitkan**.
4. Baca keterangan di jendela konfirmasi. Untuk media **Manual**, keterangannya menyebut bahwa Anda akan diminta menempel URL artikel yang sudah tayang. Untuk media **API**, keterangannya menyebut bahwa artikel langsung dikirim ke situs lewat integrasi.
5. Klik **Ya, Setuju & Terbitkan**. Selama diproses, tombol berubah menjadi **Menerbitkan…**.
6. Pada media **Manual**, jendela **Kirim URL Artikel** terbuka. Ikuti langkahnya di [Cara artikel terbit di situs Anda](07-cara-artikel-terbit.md).

Tauthub menolak permintaan Anda? Maka jendela konfirmasi tetap terbuka, dan pesan galat tampil di dalamnya.

### Dari dialog Detail

1. Klik baris artikel untuk membuka **Detail publikasi**.
2. Klik **Setuju & Terbitkan** (media **API**) atau **Setujui** (media **Manual**) di bagian bawah dialog.

Dari dialog, persetujuan langsung berjalan tanpa jendela konfirmasi.

### Setelah Anda menyetujui

- Muncul pesan hijau "Artikel disetujui dan diterbitkan." di atas daftar. Pada media **Manual**, pesan ini belum berarti artikel tayang. Anda masih harus mengirim URL.
- Media **API**: status menjadi **Antrean**. Klien memilih jadwal? Maka artikel menunggu sampai waktu jadwal, lalu dikirim ke situs Anda. Jadwalnya sudah lewat atau tidak ada jadwal? Maka pengiriman dimulai segera.
- Media **Manual**: status menjadi **Menunggu URL** sampai Anda mengirim URL artikel.
- Klien menerima notifikasi **Publikasi disetujui mitra**.
- Persetujuan tidak bisa dibatalkan dari halaman ini. Tidak ada tombol untuk membatalkannya.

## Cara menolak

1. Cari baris berstatus **Menunggu persetujuan**.
2. Klik ikon silang **Tolak publikasi**. Anda juga bisa membuka **Detail publikasi**, lalu klik **Tolak**.
3. Baca keterangan di jendela **Tolak publikasi**: "Publikasi ke (nama media) oleh (nama klien) akan ditolak dan dibatalkan."
4. Tulis alasan di kolom **Alasan (min. 5 karakter)**. Contohnya: `Topik di luar niche media`.
5. Klik **Tolak publikasi**. Tombol ini mati sampai alasan Anda berisi minimal 5 karakter.

Aturan penolakan:

- Alasan wajib diisi, minimal 5 dan paling banyak 500 karakter.
- Muncul pesan hijau "Artikel ditolak." Status menjadi **Ditolak**, dan publikasi pindah ke tab **Ditolak Mitra**. Alasan Anda tampil dengan huruf merah di bawah lencana status.
- Saldo klien yang ditahan untuk pesanan itu dikembalikan.
- Klien menerima notifikasi **Publikasi ditolak mitra** yang berisi nama media dan alasan Anda. Notifikasi itu menyebut nama media, bukan nama perusahaan Anda.
- Penolakan tidak bisa dibatalkan.

> **Catatan:** Riwayat penolakan dan kegagalan pengiriman ikut dihitung dalam skor mutu media. Lama waktu dari pesanan dibuat sampai artikel terbit menentukan angka SLA media yang dilihat klien di Katalog Media.

## Pesan galat yang umum

| Pesan | Penyebab | Yang bisa Anda lakukan |
|---|---|---|
| "Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi." | Anda menekan tombol keputusan terlalu sering dalam waktu singkat | Tunggu satu menit, lalu ulangi |
| "Terjadi kesalahan internal. Coba lagi." | Status artikel berubah saat Anda menekan tombol, misalnya sudah batal sendiri. Bisa juga ada gangguan sementara | Muat ulang halaman dan lihat status terbaru. Hubungi menu **Dukungan** bila pesan ini berulang |
| "Permintaan gagal." | Tauthub tidak memberi rincian penyebab | Coba lagi beberapa saat kemudian |
| "String must contain at least 5 character(s)" | Alasan penolakan terlalu pendek, atau lebih dari 500 karakter | Tulis alasan sepanjang 5 sampai 500 karakter |

## Pertanyaan yang sering diajukan

**Bisakah saya menyetujui sebagian isi artikel?**
Tidak. Anda bisa mengubah draf sebelum menyetujui lewat **Edit draf**. Tetapi keputusan setuju atau tolak berlaku untuk seluruh artikel.

**Apakah penolakan merugikan klien?**
Tidak. Saldo klien yang ditahan untuk pesanan itu dikembalikan penuh, dan klien menerima alasan penolakan Anda.

**Apa yang terjadi bila jadwal terbit klien lewat sebelum saya memutuskan?**
Tidak ada artikel yang terbit tanpa persetujuan Anda. Pesanan tetap menunggu sampai Anda memutuskan, atau sampai batas 3×24 jam lewat.

**Di mana saya melihat pesanan yang batal sendiri?**
Di tab **Semua**, dengan status **Dibatalkan** dan alasan yang diawali "Dibatalkan otomatis:".

## Lihat juga

- [Membaca daftar Publikasi](01-daftar-publikasi.md)
- [Revisi dari klien](04-revisi-dari-klien.md)
- [Menghubungi klien dari publikasi](08-hubungi-klien.md)
- [Status publikasi dan penghasilan](09-status-dan-penghasilan.md)
- [Notifikasi](../15-notifikasi.md)
