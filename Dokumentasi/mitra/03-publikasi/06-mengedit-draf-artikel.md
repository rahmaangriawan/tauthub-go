# Mengedit draf artikel

> Sebelum menyetujui, Anda dapat merapikan judul, isi, gambar utama, meta deskripsi, dan kategori artikel lewat editor draf, serta meminta AI menulis ulang artikel hingga 2 kali tanpa biaya. Setelah artikel terbit, editor hanya untuk membaca dan menyalin isinya.

**Lokasi:** menu **Publikasi** › ikon pensil di baris artikel (tooltip **Edit draf**)
**Untuk:** Mitra

## Gambaran singkat

Editor draf membuka dialog besar berjudul **Isi Artikel**. Suntingan Anda disimpan sebagai versi draf milik Anda. Dokumen asli klien tidak berubah. Saat artikel dikirim ke media Anda, Tauthub memakai versi suntingan Anda.

Ikon pensil muncul pada dua keadaan:

- **Menunggu persetujuan**: editor bisa diubah dan disimpan.
- **Terbit** pada media **Manual**: editor hanya untuk membaca dan menyalin (tooltip menambahkan "atau salin isi artikel", dan nama ikon untuk pembaca layar adalah **Lihat & Salin Artikel**). Dialog menampilkan keterangan "Artikel sudah terbit — isi hanya bisa dibaca dan disalin."

Pada status lain, ikon pensil tidak tampil.

## Yang tampil di layar

Di layar lebar, dialog terbagi dua: editor di kiri dan pengaturan artikel di kanan. Di HP, dialog memenuhi layar dan kedua bagian bertumpuk.

### Header

Header memuat judul **Isi Artikel**, judul artikel di bawahnya, tombol **Rewrite AI**, dan tombol **Tutup**. Tombol **Rewrite AI** tidak tampil bila artikel terkunci (sudah terbit).

### Kolom kiri: judul dan isi

- **Judul Artikel**: kolom judul dengan hitungan karakter "N/200". Hitungan berwarna merah saat mencapai 200.
- **Editor isi** dengan toolbar (tabel di bawah).
- Di bawah editor tertulis jumlah kata.

Toolbar editor:

| Tombol | Fungsi |
|---|---|
| **Tebal**, **Miring**, **Garis bawah** | Memformat teks |
| **Judul bagian**, **Sub-judul** | Menandai teks sebagai judul bagian (H2) atau sub-judul (H3) |
| **Kutipan** | Membuat blok kutipan |
| **Daftar poin**, **Daftar bernomor** | Membuat daftar |
| **Sisipkan tabel** | Menampilkan kisi ukuran tabel; klik ukuran yang diinginkan. Klik kanan di dalam tabel untuk menambah atau menghapus baris, kolom, atau tabel. |
| **Urungkan**, **Ulangi** | Membatalkan atau mengulang perubahan |
| **Sisipkan gambar** | Membuka jendela unggah. Klik, tarik berkas, atau tempel gambar. Mendukung JPG, PNG, dan WebP, dan gambar dikompres otomatis. |
| **Sisipkan tautan** | Membuka jendela tautan: **Teks Tautan (anchor)**, **URL**, dan **Jenis Tautan (rel)** (**Dofollow**, **Nofollow**, atau **Sponsored**) |
| **Sisipkan peta (Google Maps)**, **Sisipkan video (YouTube/Vimeo)** | Menyematkan peta atau video lewat tautan atau kode sematan |
| `</>` (**Edit HTML mentah**) | Beralih ke tampilan HTML. Klik lagi (**Kembali ke editor visual**) untuk kembali |

Pada tautan di artikel, klik kanan untuk memilih **Buka**, **Edit**, atau **Hapus**. Klik gambar di artikel untuk memunculkan tombol **Hapus gambar**. Klik peta atau video yang sudah tersemat untuk memunculkan **Ganti tautan** dan **Hapus embed**.

Jika media Anda tidak mengizinkan embed, tombol peta dan video terkunci dengan tooltip "Media ini tidak mengizinkan embed."

### Kolom kanan: pengaturan artikel

- **Gambar Utama**: gambar sampul artikel. Keterangannya: gambar ditampilkan di Tauthub dan dikirim ke media. Jika belum ada, tampil "Belum ada gambar utama" dan tombol **Tambah gambar**. Jika sudah ada, tersedia tombol **Ganti gambar** dan **Hapus gambar**.
- **Meta Deskripsi**: kolom ringkasan untuk mesin pencari, dengan hitungan "N/500".
- **Kategori**: pilihan kategori. Daftarnya memuat kategori dari media Anda dan kategori yang dipilih klien. Di dasar daftar ada kolom **Kategori baru…** dan tombol **Tambah**.

### Footer

Footer memuat keterangan sumber ("Sumber: dokumen klien", atau "Sumber: draf WordPress"), waktu suntingan terakhir ("diedit" diikuti tanggal), serta total berat gambar di artikel, misalnya "412 KB". Jika berat gambar tidak dapat diukur, tertulis jumlah gambarnya, misalnya "±3 gambar". Tombol di footer: **Salin Artikel**, **Tutup**, dan **Simpan Draf** (tidak tampil bila artikel terkunci).

## Cara menggunakan

### Cara mengedit dan menyimpan draf

1. Klik ikon pensil **Edit draf** di baris artikel berstatus **Menunggu persetujuan**.
2. Ubah judul dan isi artikel.
3. Atur **Gambar Utama**, **Meta Deskripsi**, dan **Kategori** di kolom kanan bila perlu.
4. Klik **Simpan Draf**. Tombol berubah menjadi **Menyimpan draf…**.
5. Cari pesan "Perubahan draf tersimpan." Di layar lebar pesan itu muncul di footer dialog. Di HP, pesan itu muncul sebagai pesan hijau di atas daftar setelah dialog ditutup.

> **Catatan:** Mengklik **Tutup** atau di luar dialog tidak menyimpan suntingan. Klik **Simpan Draf** lebih dulu.

### Cara mengganti gambar utama

1. Di kolom **Gambar Utama**, klik **Tambah gambar** atau **Ganti gambar**.
2. Pilih berkas JPG, PNG, atau WebP dari perangkat Anda.
3. Tunggu sampai gambar baru tampil. Klik **Hapus gambar** bila ingin mengosongkannya.
4. Klik **Simpan Draf**.

Berkas JPG dan PNG otomatis dikonversi ke WebP dengan target maksimal 150 KB. Rasio 16:9 disarankan. Ukuran berkas yang diunggah maksimal 10 MB.

### Cara memilih atau menambah kategori

1. Klik daftar **Kategori**.
2. Pilih kategori yang tersedia, atau ketik nama baru di kolom **Kategori baru…**.
3. Klik **Tambah**. Kategori baru langsung terpilih di daftar. Pesan "Kategori baru tersimpan dan siap dipakai." tampil di atas daftar Publikasi setelah dialog ditutup.
4. Klik **Simpan Draf**.

Pada media **API**, kategori yang Anda simpan ikut dikirim bersama artikel ke situs Anda. Kategori baru juga diajukan sebagai perubahan pada media Anda di [Daftar Media](../04-daftar-media/README.md). Untuk media yang sudah aktif, perubahan itu dapat menunggu persetujuan tim Tauthub dan tampil sebagai **Perubahan menunggu persetujuan**. Kategori yang Anda pilih untuk artikel ini tetap tersimpan lewat **Simpan Draf**.

### Cara memakai Rewrite AI

1. Pastikan artikel berstatus **Menunggu persetujuan**.
2. Klik **Rewrite AI**. Tombolnya menampilkan "gratis" dan sisa jatah, misalnya "sisa 2/2".
3. Tunggu sampai tombol selesai berputar. Judul dan isi di editor diganti dengan hasil tulis ulang, dan sisa jatah berkurang satu.
4. Baca hasilnya dengan teliti. Pesan "Rewrite selesai (gratis)…" yang mengingatkan Anda untuk meninjau sebelum menyimpan tampil di atas daftar Publikasi setelah dialog ditutup.
5. Klik **Simpan Draf** bila Anda puas. Jika tidak, ubah sendiri atau tutup dialog tanpa menyimpan.

Aturan Rewrite AI:

- Gratis, maksimal 2 kali per artikel. Jatah dihitung per artikel, bukan per akun.
- AI The Writer menulis ulang judul dan isi mengikuti sudut pandang (angle) yang dipilih klien di kampanyenya. Meta deskripsi dan gambar tidak berubah.
- Rewrite bekerja pada isi yang sedang ada di editor, termasuk suntingan Anda yang belum disimpan. Hasilnya belum tersimpan sebelum Anda mengklik **Simpan Draf**.
- Jika AI gagal memberi hasil yang valid, jatah dikembalikan sehingga Anda dapat mencoba lagi.
- Setelah jatah habis, tombol nonaktif dengan tooltip "Batas Rewrite AI tercapai — maksimal 2 kali per artikel."

### Cara menyalin artikel untuk situs Anda

1. Buka editor draf artikel.
2. Klik **Salin Artikel**. Anda juga bisa memakai tombol ini pada artikel **Terbit** di media **Manual**.
3. Tempel isinya di editor situs Anda. Pesan "Artikel disalin. Tempel ke editor website Anda." tampil di atas daftar Publikasi setelah dialog ditutup.

Isi disalin dalam bentuk teks berformat dengan cadangan teks biasa. Periksa tampilan artikel, termasuk gambar, di situs Anda sebelum menerbitkan.

## Aturan media yang berlaku pada artikel

Aturan ini diatur per media di [Daftar Media](../04-daftar-media/README.md):

- **Batas tautan keluar.** Saat artikel dikirim, Tauthub menghitung ulang tautan keluarnya. Jika jumlahnya melebihi kebijakan media, pengiriman gagal dengan pesan yang menyebut jumlah tautan dan batasnya. Hapus kelebihan tautan di editor sebelum menyetujui.
- **Media yang hanya menerima nofollow.** Tautan keluar diberi atribut nofollow otomatis saat dikirim, sehingga artikel tidak ditolak.
- **Subjudul.** Artikel dari Taut Pintar ditulis mengikuti izin subjudul media Anda. Tombol **Judul bagian** dan **Sub-judul** tetap ada di editor.
- **Embed.** Tombol peta dan video terkunci pada media yang tidak mengizinkan embed.

## Aturan dan batasan

- Draf hanya dapat disimpan selama status masih **Menunggu persetujuan**. Setelah Anda menyetujui, editor tidak tersedia untuk mengubah isi. Jika status berubah saat editor terbuka, penyimpanan ditolak.
- Judul maksimal 200 karakter, meta deskripsi maksimal 500 karakter, dan isi artikel maksimal 200.000 karakter termasuk kode HTML.
- Format gambar: JPG, PNG, atau WebP.
- Di HP, pesan sukses di footer dan pesan galat unggah gambar utama tidak ditampilkan. Pastikan gambar utama sudah tampil di kotak **Gambar Utama**, lalu cek pesan "Perubahan draf tersimpan." setelah dialog ditutup.

## Pesan galat yang umum

| Pesan | Penyebab | Yang dapat Anda lakukan |
|---|---|---|
| "Format gambar harus JPG, PNG, atau WebP." | Berkas bukan JPG, PNG, atau WebP | Pilih berkas dengan format yang diizinkan |
| "Ukuran gambar maksimal 10 MB." | Berkas terlalu besar | Perkecil gambar lalu unggah lagi |
| "Gagal mengunggah gambar. Periksa koneksi lalu coba lagi." | Koneksi terputus atau format tidak diterima | Periksa koneksi, lalu ulangi |
| "Batas Rewrite AI tercapai — maksimal 2 kali per artikel." | Jatah Rewrite AI sudah habis | Ubah artikel secara manual |
| "AI tidak mengembalikan hasil yang valid. Silakan coba lagi." | AI gagal menyusun hasil; jatah dikembalikan | Klik **Rewrite AI** lagi |
| "Artikel yang sudah terbit tidak bisa diedit." atau "Edit hanya untuk artikel yang menunggu atau sudah terkirim ke media Anda." | Status artikel berubah saat editor terbuka | Tutup dialog dan muat ulang halaman |
| "Gagal menyalin artikel. Blok teks lalu salin manual." | Browser menolak akses clipboard | Blok isi artikel, lalu salin dengan Ctrl+C atau Cmd+C |

## Pertanyaan yang sering diajukan

**Apakah suntingan saya mengubah dokumen klien?**
Tidak. Suntingan disimpan sebagai versi draf milik Anda dan dokumen klien tetap seperti aslinya. Artikel yang dikirim ke media Anda memakai versi suntingan Anda.

**Apakah pratinjau menampilkan suntingan saya?**
Tidak. Pratinjau menampilkan isi dari klien. Lihat suntingan Anda di editor draf.

**Apakah saya bisa mengedit artikel yang sudah terbit?**
Tidak. Pada media **Manual** Anda masih bisa membaca dan menyalin isinya, tetapi tidak mengubahnya.

## Lihat juga

- [Pratinjau artikel dan pemeriksaan konten](05-pratinjau-artikel.md)
- [Menyetujui atau menolak publikasi](03-menyetujui-dan-menolak.md)
- [Cara artikel terbit di situs Anda](07-cara-artikel-terbit.md)
- [Daftar Media](../04-daftar-media/README.md)
