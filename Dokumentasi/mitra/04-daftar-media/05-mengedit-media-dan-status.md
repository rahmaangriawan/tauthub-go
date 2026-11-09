# Mengedit media dan memahami status

> Halaman ini menjelaskan cara mengedit media, perubahan mana yang langsung berlaku atau perlu persetujuan tim Tauthub, arti setiap status media, serta cara mengajukan kembali, mengarsipkan, dan memulihkan media.

**Lokasi:** menu **Daftar Media** › ikon titik tiga di baris media › **Edit**; kolom **Status**
**Untuk:** Mitra

## Gambaran singkat

Menu **Edit** membuka formulir yang sama dengan **Tambah media**, dengan tambahan kartu **Kesimpulan Website**. Hasil penyimpanan bergantung pada status media dan bagian yang Anda ubah. Media yang sedang tayang dilindungi: sebagian besar perubahan menunggu persetujuan, dan media tetap tayang dengan data lama.

## Cara mengedit media

1. Klik ikon titik tiga di baris media, lalu klik **Edit**.
2. Ubah isian yang perlu.
3. Klik **Simpan perubahan**.
4. Bila ada perubahan, dialog **Konfirmasi perubahan media** muncul. Baca isinya, lalu klik **Simpan & Ajukan Review**. Klik **Batal** untuk kembali ke formulir.

Dialog konfirmasi muncul untuk hampir semua perubahan, termasuk jumlah kata, embed, dan contoh artikel yang sebenarnya berlaku langsung. Mengubah izin subjudul saja disimpan tanpa dialog.

> **Penting:** Klik di luar kotak formulir atau tekan Esc menutup formulir tanpa menyimpan.

Bila ada perubahan yang sedang menunggu persetujuan, formulir menampilkan nilai yang diajukan, bukan nilai yang sedang tayang.

## Apa yang terjadi pada media Aktif

| Yang Anda ubah | Hasilnya | Media di katalog klien |
|---|---|---|
| Jumlah kata, support embed, subjudul, contoh artikel, kotak **Secure Direct** | Berlaku langsung | Tetap tampil |
| Nama media, kategori, bahasa artikel, topik terlarang, label sponsor, jenis tautan, jumlah backlink, masa berlaku tautan, harga | Menunggu persetujuan tim Tauthub, maksimal 2×24 jam | Tetap tampil dengan data lama |
| Jenis media, Website / Domain, mode **Manual** atau **API**, platform tujuan, atau harga turun di bawah separuh harga yang berlaku | Media kembali ke status **Menunggu review** | Tidak tampil sampai disetujui |

Bila satu penyimpanan memuat perubahan dari baris ketiga, seluruh perubahan ikut masuk tinjauan penuh.

### Perubahan yang menunggu persetujuan

Setelah menyimpan, pesan hijau menyatakan: "Perubahan tersimpan dan menunggu persetujuan Admin (maksimal 2×24 jam). Media tetap tayang dengan data lama selama menunggu."

- Kolom **Status** menampilkan **Perubahan menunggu persetujuan**. Arahkan kursor ke teks itu untuk melihat daftarnya dalam bentuk "Nama kolom: nilai lama → nilai baru".
- Pesanan yang masuk selama menunggu memakai data lama.
- Bila disetujui, Anda menerima notifikasi "Perubahan media disetujui" dan data baru berlaku.
- Bila ditolak, Anda menerima notifikasi "Perubahan media ditolak" disertai alasan. Media tetap tayang dengan data lama.
- Bila belum diputuskan dalam 2×24 jam, pengajuan dibatalkan otomatis. Anda menerima notifikasi "Pengajuan perubahan media kedaluwarsa".
- Setelah ditolak atau kedaluwarsa, ajukan lagi lewat **Edit**.

Untuk membatalkan pengajuan lebih awal, buka **Edit**, kembalikan isian ke nilai yang sedang tayang, lalu simpan.

### Perubahan yang membuat media kembali ke tinjauan

Dialog konfirmasi memberi peringatan: "Perubahan ini akan memengaruhi visibilitas media Anda di sisi Klien." Setelah disimpan, media berstatus **Menunggu review** dan tampil abu-abu: klien tidak dapat memilihnya sampai tim Tauthub menyetujui perubahan.

## Media dengan status lain

- **Menunggu review** dan **Nonaktif**: perubahan tersimpan langsung dan status tidak berubah.
- **Ditolak**: perubahan tersimpan langsung, tetapi status tetap **Ditolak**. Kirim ke tinjauan lewat **Ajukan Kembali**.

> **Catatan:** Untuk media **Ditolak**, dialog konfirmasi tetap menyebut media akan berstatus Menunggu Review Admin. Status baru berubah setelah Anda memilih **Ajukan Kembali**.

## Mengelola kategori dari tabel

Di komputer, Anda bisa mengubah kategori tanpa membuka formulir.

1. Klik kategori di kolom **Kategori**. Kotak **Kelola Kategori** terbuka.
2. Klik tanda × untuk menghapus kategori, atau ketik di kolom **Tambah atau ganti kategori…** lalu klik **Tambah**.
3. Klik **Simpan**.

Angka di bawah kotak, misalnya "3/20", menunjukkan jumlah kategori dari batas 20. **Simpan** nonaktif bila tidak ada kategori atau proses simpan berjalan. Pesan yang mungkin muncul: "Maksimal 20 kategori per media." dan "Kategori sudah ada."

Pesan hijau "Kategori diperbarui untuk" disertai nama media muncul setelah disimpan. Pada media **Aktif**, perubahan kategori diperlakukan seperti perubahan lain di tabel atas: menunggu persetujuan. Kolom **Kategori** tetap menampilkan kategori lama sampai disetujui.

Kotak ini tidak tersedia di HP. Gunakan **Edit**.

## Kesimpulan Website

Kartu **Kesimpulan Website** hanya muncul saat **Edit**. Kartu ini hanya untuk dibaca. Di komputer letaknya di kolom kiri, di HP di bagian paling bawah formulir.

Isinya ringkasan situs Anda yang dibuat otomatis oleh AI, lengkap dengan tanggal pembuatan. Taut Pintar memakainya saat mencocokkan kampanye. Periksa isinya, dan laporkan lewat [Dukungan](../12-dukungan.md) bila keliru.

Sebelum ringkasan ada, kartu menampilkan penjelasan singkat. Baris di bawahnya menunjukkan proses pengambilan data:

| Tulisan | Artinya |
|---|---|
| AI sudah menarik data dari: … | Ringkasan dibuat dari halaman situs yang disebutkan. |
| Kesimpulan dibuat dari data mitra. Web media belum dapat diakses AI saat ini. | Ringkasan dibuat dari data yang Anda isi. |
| AI sedang menarik data dari web media… | Pengambilan data sedang berjalan. |
| Menunggu antrean global AI. atau Masuk antrean global AI. | Permintaan menunggu giliran. |
| Pengambilan data gagal, sistem akan mencoba ulang otomatis. | Tidak ada tindakan yang perlu Anda lakukan. |

## Status media dan artinya

| Status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **Menunggu review** | Pengajuan baru, pengajuan ulang, atau media yang dipulihkan sedang ditinjau tim Tauthub. Belum tampil di katalog. | Tunggu keputusan. Anda tetap bisa memverifikasi domain dan mengedit. |
| **Aktif** | Media disetujui. Tampil di katalog bila syarat lain terpenuhi. | Periksa ikon mata. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md). |
| **Ditolak** | Tim Tauthub menolak media. Alasannya tampil di baris sebagai "Alasan: …". | Perbaiki lewat **Edit**, lalu pilih **Ajukan Kembali**. |
| **Nonaktif** | Media belum aktif. Tidak tampil di katalog. | Hubungi [Dukungan](../12-dukungan.md) bila status ini tidak Anda harapkan. |
| **Draf** | Media belum aktif. Tidak tampil di katalog. | Hubungi [Dukungan](../12-dukungan.md) bila status ini tidak Anda harapkan. |
| **Diarsipkan** | Anda mengarsipkan media. Tidak tampil di katalog. | Klik **Pulihkan** bila ingin memakainya lagi. |

Status **Nonaktif** dan **Draf** diatur tim Tauthub. Anda tidak bisa memilihnya sendiri.

## Alur tinjauan oleh tim Tauthub

Tim Tauthub meninjau pengajuan baru, pengajuan ulang, media yang dipulihkan, dan perubahan yang menunggu persetujuan.

- Bila disetujui, status menjadi **Aktif** dan Anda menerima notifikasi "Media disetujui".
- Bila ditolak, status menjadi **Ditolak**. Anda menerima notifikasi "Media ditolak" disertai alasan, dan alasan itu tampil di baris media.
- Keputusan juga bisa dikirim lewat email dan WhatsApp sesuai pengaturan notifikasi Anda. Lihat [Profil dan konfigurasi](../14-profil-dan-konfigurasi/README.md).

Tim Tauthub juga dapat menangguhkan media, misalnya karena ada masalah pada domain. Status media tetap **Aktif**, tetapi media tidak tampil di katalog dan ikon mata menampilkan "Disembunyikan dari katalog klien: media sedang ditangguhkan admin." Artikel yang belum terbit untuk media itu dibatalkan, sedangkan artikel yang sudah terbit tetap. Hubungi [Dukungan](../12-dukungan.md) untuk mengetahui sebabnya.

## Mengajukan kembali media yang ditolak

1. Baca alasan penolakan di baris media.
2. Klik ikon titik tiga, lalu **Edit**.
3. Perbaiki data, pastikan harga terisi, lalu klik **Simpan perubahan**.
4. Klik ikon titik tiga lagi, lalu **Ajukan Kembali**.

Pesan hijau "Media diajukan ulang untuk review admin." muncul. Status berubah menjadi **Menunggu review** dan alasan penolakan terhapus.

Bila harga belum terisi, muncul pesan merah: "Isi harga media dulu lewat tombol Edit sebelum mengajukannya kembali — media tanpa harga tidak bisa dipesan klien."

## Mengarsipkan dan memulihkan media

**Mengarsipkan.** Klik ikon titik tiga, lalu **Hapus**. Peramban menampilkan pertanyaan: "Arsipkan media "nama media"? Media yang diarsipkan tidak tampil di katalog klien." Klik **OK** untuk melanjutkan. Pesan "Media diarsipkan." muncul. Menu **Hapus** hanya mengarsipkan. Media tidak terhapus permanen dan Anda bisa memulihkannya.

**Memulihkan.** Pilih **Diarsipkan** di filter status. Klik **Pulihkan** pada baris media, lalu setujui pertanyaan konfirmasi. Pesan "Media dipulihkan dan menunggu review admin." muncul. Media berstatus **Menunggu review**, sehingga perlu disetujui lagi sebelum tampil di katalog.

## Pertanyaan yang sering diajukan

**Mengapa perubahan harga saya belum tampil di katalog?**
Pada media Aktif, harga baru menunggu persetujuan tim Tauthub, maksimal 2×24 jam. Selama menunggu, harga lama tetap berlaku.

**Mengapa media saya mendadak tampil abu-abu?**
Anda mengubah jenis media, alamat situs, mode, platform, atau menurunkan harga di bawah separuh. Media kembali ke tinjauan. Tunggu persetujuan tim Tauthub.

**Apakah menu Hapus menghapus riwayat publikasi?**
Tidak. Menu itu hanya mengubah status media menjadi **Diarsipkan**.

## Lihat juga

- [Membaca Daftar Media](01-membaca-daftar-media.md)
- [Tampil di katalog klien](06-tampil-di-katalog-klien.md)
- [Publikasi](../03-publikasi/README.md)
