# Editor teks naskah

> Editor teks di tab Editor Post dipakai untuk menyunting isi artikel. Anda bisa mengatur huruf, judul bagian, daftar, tabel, tautan, gambar, peta, dan video. Halaman ini menjelaskan setiap tombol dan cara kerjanya.

**Lokasi:** menu **Kampanye** › pilih kampanye › **Penyedia Konten** › **Pengelola Naskah** › tab **Editor Post** › kotak **Isi Artikel (Konten)**
**Untuk:** Klien dengan peran Writer atau lebih tinggi pada brand

## Gambaran singkat

Editor ini bekerja seperti program Word yang sederhana. Perubahan Anda baru tersimpan ke artikel setelah Anda klik **Simpan Artikel**. Halaman **Editor Artikel** memakai editor yang sama (lihat [Editor Artikel](27-editor-artikel.md)).

Penghitung kata ada di sisi kanan label **Isi Artikel (Konten)**. Jumlah tautan tampil sebagai label "N link" di bagian atas jendela. Penjelasan label itu ada di [Pengelola Naskah](04-pengelola-naskah.md).

Pada artikel yang terkunci atau kampanye yang sudah selesai, editor hanya menampilkan isi. Semua tombolnya tidak bisa diklik.

## Yang tampil di layar

Deretan tombol (toolbar) memuat tombol berikut, berurutan dari kiri:

| Tombol | Nama | Fungsi |
|---|---|---|
| B, I, U | **Tebal**, **Miring**, **Garis bawah** | Mengatur bentuk teks yang Anda blok. Bila tidak ada teks terblok, pengaturan berlaku untuk kata di posisi kursor |
| H2 | **Judul bagian** | Mengubah paragraf menjadi judul bagian |
| H3 | **Sub-judul** | Mengubah paragraf menjadi sub-judul |
| Tanda kutip | **Kutipan** | Membuat blok kutipan |
| Butir | **Daftar poin** | Membuat daftar tanpa urutan |
| Angka | **Daftar bernomor** | Membuat daftar berurutan |
| Tabel | **Sisipkan tabel** | Menyisipkan tabel |
| Panah putar | **Urungkan**, **Ulangi** | Membatalkan atau mengulang suntingan |
| Gambar | **Sisipkan gambar** | Membuka Pustaka Gambar AI |
| Rantai | **Sisipkan tautan** | Membuka kotak tautan |
| Pin peta | **Sisipkan peta (Google Maps)** | Menyisipkan peta |
| Segitiga | **Sisipkan video (YouTube/Vimeo)** | Menyisipkan video |
| Kode | **Edit HTML mentah** | Berpindah antara editor biasa dan tampilan kode HTML. HTML adalah bahasa kode yang membentuk halaman web |

Tombol yang sedang berlaku di posisi kursor berwarna lain. Tombol **Tabel** muncul di toolbar saat kursor ada di dalam tabel. Judul bagian di dalam artikel dimulai dari H2. Judul artikel Anda isi di kolom **Judul Artikel**.

## Cara menggunakan

### Cara menyisipkan tautan

1. Blok teks yang ingin dijadikan tautan, lalu klik **Sisipkan tautan**.
2. Periksa **Teks Tautan (anchor)**. Anchor adalah kata yang bisa diklik. Kolom ini terisi dari teks yang Anda blok, dan bisa Anda ubah.
3. Isi **URL** lengkap dengan `https://`. URL adalah alamat situs tujuan. Editor tidak menambahkan `https://` sendiri.
4. Pilih **Jenis Tautan (rel)**: **Dofollow**, **Nofollow**, atau **Sponsored**. Keterangan di bawah pilihan menjelaskan pengaruhnya untuk SEO (peringkat di mesin pencari).
5. Klik **Simpan**.

Bila tidak ada teks terblok dan **Teks Tautan** kosong, muncul pesan "Blok teks di artikel dulu, atau isi Teks Tautan." Bila Anda mengosongkan **URL** lalu klik **Simpan**, tautan lepas dari teks terpilih.

Cara cepat: blok teks, lalu tempel sebuah URL. Teks itu langsung menjadi tautan dofollow tanpa kotak isian.

Mitra (pemilik media) menentukan jenis tautan (Dofollow, Nofollow, atau Sponsored) dan jumlah paling banyak untuk tiap media. AI menulis tautan sesuai aturan itu. Untuk tautan yang Anda sisipkan sendiri, pilih jenis yang sama dan jaga jumlahnya. Lihat [Cek Kualitas Naskah](05-cek-kualitas-naskah.md).

### Cara mengubah atau menghapus tautan

1. Klik kanan pada tautan di dalam artikel. Menu menampilkan situs tujuannya.
2. Pilih **Buka** untuk membuka tautan di tab baru. Pilih **Edit** untuk membuka kotak **Edit Tautan**. Pilih **Hapus** untuk melepas tautan dan menyisakan tulisannya.

Bila Anda hanya mengklik tautan, teksnya terpilih dan halaman tujuan tidak terbuka.

### Cara menyisipkan tabel

1. Arahkan kursor ke tombol **Sisipkan tabel**. Kotak ukuran muncul dengan label seperti "3 × 2" (kolom × baris).
2. Klik sel untuk memilih ukuran, sampai 8 kolom dan 6 baris. Klik tombol tabel supaya kotak ukuran tetap terbuka.

Klik kanan di dalam tabel untuk membuka menu **Kelola tabel**: **Tambah baris di atas**, **Tambah baris di bawah**, **Tambah kolom di kiri**, **Tambah kolom di kanan**, **Hapus baris**, **Hapus kolom**, dan **Hapus tabel**.

### Cara menyisipkan peta atau video

1. Klik **Sisipkan peta (Google Maps)** atau **Sisipkan video (YouTube/Vimeo)**.
2. Tempel tautan, atau seluruh kode sematan (iframe) dari menu Bagikan atau Sematkan. Kode sematan adalah kode yang memasang peta atau video di halaman lain.
3. Klik **Sisipkan**. Anda juga bisa memakai pintasan Ctrl/Cmd+Enter.

Editor mengenali tautan Google Maps (tautan penuh atau kode sematan), YouTube, dan Vimeo. Editor menentukan jenisnya dari isi tautan, bukan dari tombol yang Anda klik. Tautan pendek Google Maps (maps.app.goo.gl) tidak bisa dimuat. Pakai tautan penuh atau kode sematan. Tinggi sematan tetap, sehingga tampil selebar artikel.

Klik sematan di dalam artikel untuk memilihnya. Bar kecil muncul dengan **Ganti tautan** dan **Hapus embed**.

Bila media tidak mengizinkan embed (sematan), tombol peta dan video terkunci dengan keterangan "Media ini tidak mengizinkan embed."

| Pesan | Artinya |
|---|---|
| "Tempel tautan atau kode sematan terlebih dahulu." | Kolom masih kosong |
| "Tautan tidak valid. Pastikan diawali https:// …" | Format tautan salah. Tempel tautan lengkap atau seluruh kode sematan |
| "Hanya tautan https:// yang didukung." | Tautan tidak memakai https |
| "Tautan belum dikenali. Pakai tautan/kode sematan Google Maps, YouTube, atau Vimeo." | Layanan lain tidak bisa dipakai |

### Cara menyisipkan gambar

1. Klik **Sisipkan gambar**. **Pustaka Gambar AI** terbuka.
2. Unggah gambar, atau centang gambar yang sudah ada.
3. Klik **Sisipkan ke artikel**.

Langkah lengkapnya ada di [Gambar utama dan Pustaka Gambar](26-gambar-utama-dan-pustaka-gambar.md). Anda juga bisa menarik berkas gambar langsung ke isi artikel. Tauthub mengecilkan gambar ke format WebP, mengunggahnya, lalu menyisipkannya.

Klik sebuah gambar di dalam artikel untuk memilihnya. Tombol **Hapus gambar** muncul di atasnya. Anda juga bisa menekan Delete atau Backspace.

### Cara menyunting HTML mentah

1. Klik tombol **Edit HTML mentah** di ujung kanan toolbar. Kotak berubah menjadi teks kode HTML.
2. Ubah HTML sesuai kebutuhan.
3. Klik tombol itu lagi (sekarang berjudul **Kembali ke editor visual**) untuk kembali ke editor biasa.

Di mode HTML, tombol toolbar lain tidak bisa diklik. Tauthub membuang elemen berbahaya secara otomatis. Contohnya skrip, formulir, dan sematan dari situs selain Google Maps, YouTube, dan Vimeo.

### Cara menempel dari dokumen

Tempelan dari Google Docs atau Word web diubah menjadi HTML yang bersih. Subjudul (heading), tabel, daftar, tautan, serta huruf tebal, miring, garis bawah, dan coretan tetap ada. Font dan gaya bawaan dokumen dibuang. Bila **Judul Artikel** masih kosong, judul dokumen menjadi judul artikel dan tidak ikut masuk ke isi. Yang dihitung sebagai judul dokumen adalah gaya "Judul", atau "Judul 1" yang paling awal dan tidak lebih dari 200 karakter.

### Pintasan keyboard

| Pintasan | Fungsi |
|---|---|
| Ctrl/Cmd+Z | Urungkan |
| Ctrl/Cmd+Shift+Z atau Ctrl+Y | Ulangi |
| Delete atau Backspace | Menghapus gambar atau sematan yang terpilih |

Riwayat urungkan menyimpan sampai 80 langkah selama jendela terbuka.

## Aturan dan batasan

- Label "N link" menghitung semua tautan di isi artikel. Label menjadi merah dan **Simpan Artikel** tidak bisa diklik bila jumlahnya melebihi batas media.
- Bila media tidak menerima tautan, hapus semua tautan sebelum QC. QC adalah pemeriksaan kualitas naskah. Pemeriksaan Link Health akan menandai artikel itu **Gagal**.
- Saat Anda menyimpan, sistem membersihkan HTML sekali lagi. Sematan hanya boleh dari Google Maps, YouTube, dan Vimeo.

## Pertanyaan yang sering diajukan

**Kenapa tombol peta dan video terkunci?**
Media tujuan tidak mengizinkan embed. Mitra yang menetapkan pengaturan itu.

**Kenapa gambar yang saya tarik tidak muncul?**
Hanya berkas gambar yang diterima. Bila muncul "Gagal mengunggah gambar. Periksa koneksi lalu coba lagi.", cek koneksi dan ukuran berkas (paling besar 10 MB), lalu ulangi.

## Lihat juga

- [Pengelola Naskah](04-pengelola-naskah.md)
- [Gambar utama dan Pustaka Gambar](26-gambar-utama-dan-pustaka-gambar.md)
- [Cek Kualitas Naskah](05-cek-kualitas-naskah.md)
