# Revisi dari klien

> Klien dapat meminta perubahan pada artikel yang sudah terbit di media Anda. Permintaan itu muncul sebagai status **Perlu Revisi**, dan Anda memutuskan untuk menerima atau menolaknya.

**Lokasi:** menu **Publikasi** › tab **Revisi**
**Untuk:** Mitra

## Gambaran singkat

Ketika klien meminta revisi atas artikel yang sudah **Terbit**, status publikasi berubah menjadi **Perlu Revisi** dan Anda menerima notifikasi **Revisi artikel dari klien**. Notifikasi memuat catatan dari klien.

Artikel yang sudah tayang di situs Anda tidak berubah sampai Anda menerima revisinya. Anda dapat menerima revisi atau menolaknya dengan alasan. Di dialog **Detail publikasi**, nama statusnya tertulis **Revisi dari klien**.

## Yang tampil di layar

Buka **Detail publikasi** pada baris berstatus **Perlu Revisi**. Di sana muncul bagian **Permintaan Revisi Klien**:

- Setiap catatan revisi tampil dengan isi catatan, waktunya, dan tautan lampiran (ikon penjepit kertas) bila klien melampirkan berkas.
- Jika klien tidak menulis catatan, tertulis "Klien meminta revisi tanpa catatan tambahan."
- Di bawahnya ada petunjuk bahwa Anda dapat menerima revisi untuk mengirim ulang artikel ke media Anda, atau menolaknya dengan alasan.
- Jika Anda pernah menolak, tertulis "Sudah ditolak N/3 kali." Alasan penolakan Anda sebelumnya juga ikut tampil di daftar catatan dengan awalan "Mitra menolak revisi:".

Untuk membaca isi artikel versi revisi, klik **Pratinjau Artikel**. Lihat [Pratinjau artikel dan pemeriksaan konten](05-pratinjau-artikel.md). Bila perlu bertanya kepada klien, pakai **Hubungi Klien** dengan alasan **Revisi**. Lihat [Menghubungi klien dari publikasi](08-hubungi-klien.md).

## Cara menggunakan

### Cara menerima revisi

1. Buka menu **Publikasi** dan klik tab **Revisi**.
2. Klik ikon centang **Terima Revisi** di baris artikel. Anda juga dapat membuka **Detail publikasi** lalu klik **Terima Revisi**.
3. Baca pesan hijau "Revisi diterima. Artikel dikirim ulang ke media Anda."

Terima Revisi langsung berjalan tanpa jendela konfirmasi. Setelah itu:

- Status berubah menjadi **Antrean**, lalu mengikuti alur biasa sampai **Terbit** kembali.
- Catatan revisi yang tadi tampil ditandai selesai. Jika klien meminta revisi lagi, catatan baru yang muncul.
- Hitungan penolakan Anda kembali ke 0.
- Klien menerima notifikasi **Revisi diterima mitra**.

Hasil di situs Anda bergantung pada mode media:

- **Media API:** Tauthub mengirim ulang artikel ke situs Anda. Pada integrasi WordPress, artikel yang sama diperbarui, bukan dibuat ulang. Periksa artikel di situs Anda setelah status kembali **Terbit**.
- **Media Manual:** Tauthub tidak dapat mengubah artikel di situs Anda. Setelah Anda menerima revisi, status kembali **Terbit** tanpa pengiriman apa pun. Perbarui sendiri artikel di situs Anda dengan isi revisi. Buka **Pratinjau Artikel** sebelum menerima revisi, atau ikon pensil **Lihat & Salin Artikel** setelah status kembali **Terbit**, untuk membaca dan menyalin isi terbaru.

### Cara menolak revisi

1. Buka tab **Revisi**.
2. Klik ikon silang **Tolak revisi** di baris artikel. Anda juga dapat membuka **Detail publikasi** lalu klik **Tolak Revisi**.
3. Baca keterangan di jendela **Tolak revisi**: revisi untuk media tersebut ditolak dan dikembalikan ke klien untuk diperbaiki, dan artikel yang sudah tayang tidak diubah.
4. Tulis alasan di kolom **Alasan (min. 5 karakter)**.
5. Klik **Tolak revisi**. Tombol ini nonaktif sampai alasan Anda berisi minimal 5 karakter.

Setelah Anda menolak:

- Muncul pesan hijau "Revisi ditolak. Klien akan memperbaiki artikelnya."
- Status tetap **Perlu Revisi**, sehingga publikasi tetap berada di tab **Revisi** sampai revisi diterima.
- Alasan Anda tersimpan sebagai catatan dengan awalan "Mitra menolak revisi:" dan klien menerima notifikasi **Revisi ditolak mitra** berisi alasan itu.
- Artikel yang sudah tayang di situs Anda tidak berubah.

## Aturan dan batasan

- Anda dapat menolak revisi paling banyak **3 kali**. Hitungan kembali ke 0 setelah Anda menerima revisi. Setelah penolakan ketiga, tombol **Tolak revisi** dan **Tolak Revisi** tidak lagi tampil dan hanya **Terima Revisi** yang tersisa. Pada saat itu klien diberi tahu bahwa batas penolakan tercapai, dan tim Tauthub juga diberi tahu.
- Alasan penolakan wajib diisi, minimal 5 dan maksimal 500 karakter.
- Menerima atau menolak revisi hanya bisa dilakukan saat status **Perlu Revisi**.
- Alasan penolakan tidak mengubah status menjadi **Ditolak**; status itu hanya untuk penolakan pesanan baru. Lihat [Menyetujui atau menolak publikasi](03-menyetujui-dan-menolak.md).

## Pesan galat yang umum

| Pesan | Penyebab | Yang dapat Anda lakukan |
|---|---|---|
| "Batas penolakan revisi sudah tercapai. Publikasi ini hanya bisa diterima." | Anda sudah menolak revisi 3 kali | Klik **Terima Revisi**, atau hubungi klien lewat **Hubungi Klien** |
| "Publication status cannot transition from … to queued" | Status artikel bukan **Perlu Revisi** lagi, misalnya revisi sudah diterima di perangkat lain | Muat ulang halaman dan periksa status terbaru |
| "Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi." | Tombol ditekan terlalu sering | Tunggu satu menit, lalu ulangi |

## Pertanyaan yang sering diajukan

**Apakah revisi memengaruhi penghasilan saya?**
Penghasilan sebuah publikasi dicatat sekali. Pengiriman ulang setelah revisi tidak mencatat penghasilan kedua.

**Bisakah saya mengedit artikel sendiri saat statusnya Perlu Revisi?**
Tidak. Ikon **Edit draf** tidak tersedia pada status ini. Perubahan isi dilakukan klien; Anda hanya menerima atau menolaknya.

**Di mana saya melihat catatan klien yang sudah selesai?**
Catatan yang sudah ditandai selesai setelah Anda menerima revisi tidak tampil lagi di dialog Detail.

## Lihat juga

- [Membaca daftar Publikasi](01-daftar-publikasi.md)
- [Detail publikasi](02-detail-publikasi.md)
- [Cara artikel terbit di situs Anda](07-cara-artikel-terbit.md)
- [Status publikasi dan penghasilan](09-status-dan-penghasilan.md)
