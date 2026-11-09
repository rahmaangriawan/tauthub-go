# Editor teks naskah

> Editor teks di tab Editor Post dipakai untuk menyunting isi artikel: format huruf, judul bagian, daftar, tabel, tautan, gambar, serta peta dan video. Halaman ini menjelaskan setiap tombol dan perilakunya.

**Lokasi:** menu **Kampanye** › pilih kampanye › **Penyedia Konten** › **Pengelola Naskah** › tab **Editor Post** › kotak **Isi Artikel (Konten)**
**Untuk:** Klien dengan peran Writer atau lebih tinggi pada brand

## Gambaran singkat

Editor bekerja seperti pengolah kata sederhana. Semua perubahan baru tersimpan ke artikel setelah Anda mengklik **Simpan Artikel**. Editor yang sama dipakai di halaman **Editor Artikel** (lihat [Editor Artikel](27-editor-artikel.md)).

Penghitung kata tampil di sisi kanan label **Isi Artikel (Konten)**. Jumlah tautan tampil sebagai chip "N link" di header jendela. Penjelasan chip ada di [Pengelola Naskah](04-pengelola-naskah.md).

Pada artikel terkunci atau kampanye yang sudah selesai, editor hanya menampilkan isi dan semua tombolnya nonaktif.

## Yang tampil di layar

Toolbar memuat tombol berikut, berurutan dari kiri:

| Tombol | Nama | Fungsi |
|---|---|---|
| B, I, U | **Tebal**, **Miring**, **Garis bawah** | Memformat teks terpilih. Tanpa teks terpilih, format berlaku untuk kata di posisi kursor |
| H2 | **Judul bagian** | Mengubah paragraf menjadi judul bagian |
| H3 | **Sub-judul** | Mengubah paragraf menjadi sub-judul |
| Tanda kutip | **Kutipan** | Membuat blok kutipan |
| Butir | **Daftar poin** | Membuat daftar tanpa urutan |
| Angka | **Daftar bernomor** | Membuat daftar berurutan |
| Tabel | **Sisipkan tabel** | Menyisipkan tabel |
| Panah putar | **Urungkan**, **Ulangi** | Membatalkan atau mengulang suntingan |
| Gambar | **Sisipkan gambar** | Membuka Pustaka Gambar AI |
| Rantai | **Sisipkan tautan** | Membuka dialog tautan |
| Pin peta | **Sisipkan peta (Google Maps)** | Menyisipkan peta |
| Segitiga | **Sisipkan video (YouTube/Vimeo)** | Menyisipkan video |
| Kode | **Edit HTML mentah** | Beralih antara editor visual dan HTML |

Tombol yang sedang aktif di posisi kursor diberi warna berbeda. Chip **Tabel** muncul di toolbar saat kursor berada di dalam tabel. Judul bagian di dalam artikel dimulai dari H2, sedangkan judul artikel diisi di kolom **Judul Artikel**.

## Cara menggunakan

### Cara menyisipkan tautan

1. Blok teks yang akan dijadikan tautan, lalu klik **Sisipkan tautan**.
2. Periksa **Teks Tautan (anchor)**. Kolom ini terisi dari teks yang Anda blok dan bisa diubah.
3. Isi **URL** lengkap dengan `https://`. Editor tidak menambahkannya otomatis.
4. Pilih **Jenis Tautan (rel)**: **Dofollow**, **Nofollow**, atau **Sponsored**. Keterangan di bawah pilihan menjelaskan efeknya untuk SEO.
5. Klik **Simpan**.

Bila tidak ada teks terblok dan **Teks Tautan** kosong, muncul pesan "Blok teks di artikel dulu, atau isi Teks Tautan." Mengosongkan **URL** lalu klik **Simpan** melepas tautan dari teks terpilih.

Cara cepat: tempel sebuah URL saat teks terblok, dan teks itu langsung menjadi tautan dofollow tanpa dialog.

Mitra menentukan jenis tautan (Dofollow, Nofollow, atau Sponsored) dan jumlah maksimal untuk tiap media. AI menulis tautan sesuai aturan itu. Untuk tautan yang Anda sisipkan sendiri, pilih jenis yang sama dan jaga jumlahnya. Lihat [Cek Kualitas Naskah](05-cek-kualitas-naskah.md).

### Cara mengubah atau menghapus tautan

1. Klik kanan pada tautan di dalam artikel. Menu menampilkan domain tujuan.
2. Pilih **Buka** untuk membuka tautan di tab baru, **Edit** untuk membuka dialog **Edit Tautan**, atau **Hapus** untuk melepas tautan dan menyisakan teksnya.

Klik biasa pada tautan hanya memilih teksnya dan tidak membuka halaman tujuan.

### Cara menyisipkan tabel

1. Arahkan kursor ke tombol **Sisipkan tabel**. Kotak ukuran muncul dengan label seperti "3 × 2" (kolom × baris).
2. Klik sel untuk memilih ukuran, sampai 8 kolom dan 6 baris. Klik tombol tabel untuk menahan kotak ukuran tetap terbuka.

Klik kanan di dalam tabel untuk membuka menu **Kelola tabel**: **Tambah baris di atas**, **Tambah baris di bawah**, **Tambah kolom di kiri**, **Tambah kolom di kanan**, **Hapus baris**, **Hapus kolom**, dan **Hapus tabel**.

### Cara menyisipkan peta atau video

1. Klik **Sisipkan peta (Google Maps)** atau **Sisipkan video (YouTube/Vimeo)**.
2. Tempel tautan, atau seluruh kode sematan (iframe) dari menu Bagikan atau Sematkan.
3. Klik **Sisipkan**. Pintasan Ctrl/Cmd+Enter juga bisa dipakai.

Editor mengenali tautan Google Maps (tautan penuh atau kode sematan), YouTube, dan Vimeo. Jenis ditentukan dari isi tautan, bukan dari tombol yang diklik. Tautan pendek Google Maps (maps.app.goo.gl) tidak bisa dimuat; pakai tautan penuh atau kode sematan. Sematan memakai tinggi tetap sehingga tampil penuh lebar artikel.

Klik sematan di dalam artikel untuk memilihnya. Bar kecil muncul dengan **Ganti tautan** dan **Hapus embed**.

Bila media tidak mengizinkan embed, tombol peta dan video terkunci dengan keterangan "Media ini tidak mengizinkan embed."

| Pesan | Artinya |
|---|---|
| "Tempel tautan atau kode sematan terlebih dahulu." | Kolom masih kosong |
| "Tautan tidak valid. Pastikan diawali https:// …" | Format tautan salah; tempel tautan lengkap atau seluruh kode sematan |
| "Hanya tautan https:// yang didukung." | Tautan tidak memakai https |
| "Tautan belum dikenali. Pakai tautan/kode sematan Google Maps, YouTube, atau Vimeo." | Layanan lain tidak didukung |

### Cara menyisipkan gambar

1. Klik **Sisipkan gambar**. **Pustaka Gambar AI** terbuka.
2. Unggah gambar, atau centang gambar yang sudah ada.
3. Klik **Sisipkan ke artikel**.

Langkah lengkap ada di [Gambar utama dan Pustaka Gambar](26-gambar-utama-dan-pustaka-gambar.md). Anda juga bisa menarik berkas gambar langsung ke isi artikel; gambar dikompres ke WebP, diunggah, lalu disisipkan.

Klik sebuah gambar di dalam artikel untuk memilihnya. Tombol **Hapus gambar** muncul di atasnya, atau tekan Delete atau Backspace.

### Cara menyunting HTML mentah

1. Klik tombol **Edit HTML mentah** di ujung kanan toolbar. Kotak berubah menjadi teks HTML.
2. Ubah HTML sesuai kebutuhan.
3. Klik tombol itu lagi (kini berjudul **Kembali ke editor visual**) untuk kembali ke editor visual.

Pada mode HTML, tombol toolbar lain nonaktif. Elemen berbahaya, seperti skrip, formulir, dan sematan dari situs selain Google Maps, YouTube, dan Vimeo, dibuang otomatis.

### Cara menempel dari dokumen

Tempelan dari Google Docs atau Word web dikonversi menjadi HTML bersih: heading, tabel, daftar, tautan, serta tebal, miring, garis bawah, dan coret dipertahankan, sedangkan font dan gaya bawaan dokumen dibuang. Bila **Judul Artikel** masih kosong, judul dokumen (gaya "Judul", atau "Judul 1" yang berada paling awal dan tidak lebih dari 200 karakter) menjadi judul artikel dan tidak ikut masuk ke isi.

### Pintasan keyboard

| Pintasan | Fungsi |
|---|---|
| Ctrl/Cmd+Z | Urungkan |
| Ctrl/Cmd+Shift+Z atau Ctrl+Y | Ulangi |
| Delete atau Backspace | Menghapus gambar atau sematan yang terpilih |

Riwayat urungkan menyimpan sampai 80 langkah selama jendela terbuka.

## Aturan dan batasan

- Chip "N link" menghitung semua tautan di isi artikel. Chip menjadi merah dan **Simpan Artikel** nonaktif bila jumlahnya melebihi batas media.
- Media yang tidak menerima tautan: hapus semua tautan sebelum QC, karena pemeriksaan Link Health akan menandainya **Gagal**.
- Saat disimpan, HTML dibersihkan lagi di sistem. Sematan hanya dari Google Maps, YouTube, dan Vimeo.

## Pertanyaan yang sering diajukan

**Kenapa tombol peta dan video terkunci?**
Media tujuan tidak mengizinkan embed. Pengaturan itu ditetapkan Mitra.

**Kenapa gambar yang saya tarik tidak muncul?**
Hanya berkas gambar yang diterima. Bila muncul "Gagal mengunggah gambar. Periksa koneksi lalu coba lagi.", cek koneksi dan ukuran berkas (maksimal 10 MB), lalu ulangi.

## Lihat juga

- [Pengelola Naskah](04-pengelola-naskah.md)
- [Gambar utama dan Pustaka Gambar](26-gambar-utama-dan-pustaka-gambar.md)
- [Cek Kualitas Naskah](05-cek-kualitas-naskah.md)
