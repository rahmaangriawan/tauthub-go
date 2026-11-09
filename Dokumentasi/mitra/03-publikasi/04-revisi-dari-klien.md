# Revisi dari klien

> Klien bisa meminta perubahan pada artikel yang sudah terbit di media Anda. Permintaan itu muncul sebagai status **Perlu Revisi**. Anda yang memutuskan untuk menerima atau menolaknya.

**Lokasi:** menu **Publikasi** › tab **Revisi**
**Untuk:** Mitra

## Gambaran singkat

Revisi artinya perbaikan. Klien meminta revisi atas artikel yang sudah **Terbit**. Maka status publikasi berubah menjadi **Perlu Revisi**, dan Anda menerima notifikasi **Revisi artikel dari klien**. Notifikasi itu berisi catatan dari klien.

Artikel yang sudah tayang di situs Anda tidak berubah sampai Anda menerima revisinya. Anda bisa menerima revisi, atau menolaknya dengan alasan. Di dialog **Detail publikasi**, nama statusnya tertulis **Revisi dari klien**.

## Yang tampil di layar

Buka **Detail publikasi** pada baris berstatus **Perlu Revisi**. Di sana muncul bagian **Permintaan Revisi Klien**:

- Setiap catatan revisi tampil dengan isi catatan, waktunya, dan tautan lampiran (ikon penjepit kertas) bila klien melampirkan berkas.
- Klien tidak menulis catatan? Maka tertulis "Klien meminta revisi tanpa catatan tambahan."
- Di bawahnya ada petunjuk. Anda bisa menerima revisi untuk mengirim ulang artikel ke media Anda. Atau Anda bisa menolaknya dengan alasan.
- Anda pernah menolak? Maka tertulis "Sudah ditolak N/3 kali." Alasan penolakan Anda sebelumnya juga tampil di daftar catatan dengan awalan "Mitra menolak revisi:".

Ingin membaca isi artikel versi revisi? Klik **Pratinjau Artikel**. Lihat [Pratinjau artikel dan pemeriksaan konten](05-pratinjau-artikel.md). Perlu bertanya kepada klien? Pakai **Hubungi Klien** dengan alasan **Revisi**. Lihat [Menghubungi klien dari publikasi](08-hubungi-klien.md).

## Cara menggunakan

### Cara menerima revisi

1. Buka menu **Publikasi** dan klik tab **Revisi**.
2. Klik ikon centang **Terima Revisi** di baris artikel. Anda juga bisa membuka **Detail publikasi**, lalu klik **Terima Revisi**.
3. Baca pesan hijau "Revisi diterima. Artikel dikirim ulang ke media Anda."

Terima Revisi langsung berjalan tanpa jendela konfirmasi. Setelah itu:

- Status berubah menjadi **Antrean**, lalu mengikuti alur biasa sampai **Terbit** lagi.
- Catatan revisi yang tadi tampil ditandai selesai. Klien meminta revisi lagi? Maka catatan baru yang muncul.
- Hitungan penolakan Anda kembali ke 0.
- Klien menerima notifikasi **Revisi diterima mitra**.

Hasil di situs Anda bergantung pada mode media:

- **Media API:** Tauthub mengirim ulang artikel ke situs Anda. Pada integrasi WordPress, artikel yang sama diperbarui, bukan dibuat baru. Periksa artikel di situs Anda setelah status kembali **Terbit**.
- **Media Manual:** Tauthub tidak bisa mengubah artikel di situs Anda. Setelah Anda menerima revisi, status kembali **Terbit** tanpa ada pengiriman. Perbarui sendiri artikel di situs Anda dengan isi revisi. Untuk membaca dan menyalin isi terbaru, buka **Pratinjau Artikel** sebelum menerima revisi. Atau klik ikon pensil **Lihat & Salin Artikel** setelah status kembali **Terbit**.

### Cara menolak revisi

1. Buka tab **Revisi**.
2. Klik ikon silang **Tolak revisi** di baris artikel. Anda juga bisa membuka **Detail publikasi**, lalu klik **Tolak Revisi**.
3. Baca keterangan di jendela **Tolak revisi**. Isinya: revisi untuk media itu ditolak dan dikembalikan ke klien untuk diperbaiki. Artikel yang sudah tayang tidak diubah.
4. Tulis alasan di kolom **Alasan (min. 5 karakter)**.
5. Klik **Tolak revisi**. Tombol ini mati sampai alasan Anda berisi minimal 5 karakter.

Setelah Anda menolak:

- Muncul pesan hijau "Revisi ditolak. Klien akan memperbaiki artikelnya."
- Status tetap **Perlu Revisi**. Jadi publikasi tetap ada di tab **Revisi** sampai revisi diterima.
- Alasan Anda tersimpan sebagai catatan dengan awalan "Mitra menolak revisi:". Klien menerima notifikasi **Revisi ditolak mitra** yang berisi alasan itu.
- Artikel yang sudah tayang di situs Anda tidak berubah.

## Aturan dan batasan

- Anda boleh menolak revisi paling banyak **3 kali**. Hitungan kembali ke 0 setelah Anda menerima revisi. Setelah penolakan ketiga, tombol **Tolak revisi** dan **Tolak Revisi** tidak tampil lagi. Hanya **Terima Revisi** yang tersisa. Saat itu klien diberi tahu bahwa batas penolakan tercapai. Tim Tauthub juga diberi tahu.
- Alasan penolakan wajib diisi, minimal 5 dan paling banyak 500 karakter.
- Anda hanya bisa menerima atau menolak revisi saat status **Perlu Revisi**.
- Alasan penolakan tidak mengubah status menjadi **Ditolak**. Status itu hanya untuk penolakan pesanan baru. Lihat [Menyetujui atau menolak publikasi](03-menyetujui-dan-menolak.md).

## Pesan galat yang umum

| Pesan | Penyebab | Yang bisa Anda lakukan |
|---|---|---|
| "Batas penolakan revisi sudah tercapai. Publikasi ini hanya bisa diterima." | Anda sudah menolak revisi 3 kali | Klik **Terima Revisi**, atau hubungi klien lewat **Hubungi Klien** |
| "Publication status cannot transition from … to queued" | Status artikel bukan **Perlu Revisi** lagi, misalnya revisi sudah diterima di perangkat lain | Muat ulang halaman dan lihat status terbaru |
| "Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi." | Tombol ditekan terlalu sering | Tunggu satu menit, lalu ulangi |

## Pertanyaan yang sering diajukan

**Apakah revisi memengaruhi penghasilan saya?**
Penghasilan sebuah publikasi dicatat sekali. Pengiriman ulang setelah revisi tidak mencatat penghasilan kedua.

**Bisakah saya mengedit artikel sendiri saat statusnya Perlu Revisi?**
Tidak. Ikon **Edit draf** tidak ada pada status ini. Klien yang mengubah isi. Anda hanya menerima atau menolaknya.

**Di mana saya melihat catatan klien yang sudah selesai?**
Catatan yang sudah ditandai selesai setelah Anda menerima revisi tidak tampil lagi di dialog Detail.

## Lihat juga

- [Membaca daftar Publikasi](01-daftar-publikasi.md)
- [Detail publikasi](02-detail-publikasi.md)
- [Cara artikel terbit di situs Anda](07-cara-artikel-terbit.md)
- [Status publikasi dan penghasilan](09-status-dan-penghasilan.md)
