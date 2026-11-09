# Penyedia Konten

> Penyedia Konten adalah tahap kedua kampanye di Taut Pintar. Di sini Anda menyiapkan satu naskah untuk tiap media pilihan. Lalu Anda memeriksa mutunya, memantau tagihan, dan menerbitkan kampanye.

**Lokasi:** menu **Kampanye** › pilih kampanye › tahap **Penyedia Konten**. Halaman ini juga terbuka sendiri setelah Anda klik **Buat Kampanye** di **Katalog Media**.
**Untuk:** Klien dengan peran Writer atau lebih tinggi pada brand. Peran Viewer dan Reviewer tidak bisa memakai halaman ini.

## Gambaran singkat

Halaman ini menampilkan satu kartu untuk setiap media yang Anda pilih. Tiap kartu memuat satu atau beberapa baris artikel. Setiap artikel butuh naskah, gambar utama (cover), dan kategori. Artikel juga harus lolos QC sebelum kampanye bisa diterbitkan. QC adalah pemeriksaan mutu naskah oleh sistem.

Penanda tahap tampil di bar atas: **Katalog Media**, **Penyedia Konten**, dan **Ringkasan**. Tahap **Ringkasan** muncul setelah Anda pernah sampai di sana. Klik tahap yang pernah Anda capai untuk pindah ke tahap itu.

Judul banner mengikuti jalur kampanye:

- **Dapur Produksi AI (N Media)** untuk kampanye jalur AI, termasuk kampanye dari Katalog Media. Teks di bawahnya menyebut biaya menulis satu artikel dengan The Writer.
- **Dapur Produksi (N Media)** untuk kampanye jalur Manual. Teks di bawahnya mengajak Anda mengunggah naskah sendiri, atau membuat naskah dengan AI lewat tombol di tiap media.

Bila sebuah media punya lebih dari satu artikel, jumlah artikel ikut tampil, misalnya "Dapur Produksi AI (3 Media · 5 Artikel)".

## Yang tampil di layar

### Banner dan kartu informasi

Banner memuat judul, teks biaya, tombol **Generate Semua**, dan label nama kampanye. Dua kartu informasi bisa muncul di bawahnya:

- **Kutipan Narasumber (dari The Profiler)** muncul bila kampanye punya narasumber. AI memakai kutipan itu di artikel semua media terpilih, paling sedikit satu kutipan lengkap dengan nama dan jabatan.
- **Mode Pelacakan & Anchor Text** muncul bila kampanye punya target tautan utama. Lihat bagian khusus di bawah.

### Kartu media

| Elemen | Fungsi |
|---|---|
| Kotak centang | Memilih semua artikel media itu sekaligus |
| Nama domain media | Nama media (alamat websitenya), huruf pertama kapital |
| Chip **Mainstream** atau **Blogger** | Jenis media. Jenis ini menentukan gaya tulisan AI dan aturan editor |
| Baris status | "N artikel · semua sudah ada naskahnya", "Belum ada artikel", atau "N artikel belum dibuat" |
| Tombol panah | Menutup atau membuka daftar artikel. Klik area kosong di header juga bisa |
| **Buat N artikel** | Membuat semua artikel kosong lewat satu jendela. Hanya muncul bila media punya lebih dari satu artikel dan masih ada yang belum dibuat |
| Tombol **−** dan **+** | Mengubah jumlah artikel media itu |
| Ikon tong sampah | Melepas media dari kampanye |

### Baris artikel

Setiap baris menampilkan kotak centang, gambar kecil cover (ikon gambar bila belum ada), chip status, chip tautan, judul, jumlah kata, dan nomor artikel ("Artikel 1"). Chip adalah label kecil berbentuk pil. Chip **Putaran N** muncul bila kampanye diulang. Chip **Terkunci** muncul bila artikel sudah punya publikasi, yaitu sudah dikirim untuk terbit atau sudah terjadwal. Di sisi kanan ada status cover dan tombol aksi.

| Chip status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **Menunggu Artikel** | Naskah belum dibuat. Chip yang sama juga tampil bila QC meminta revisi | Klik **Generate Konten** atau **Tulis Sendiri**. Untuk revisi, buka **Analisis QC** |
| **Draft Tersedia** | Naskah sudah ada, tetapi belum diperiksa QC | Simpan lewat **Edit Artikel** atau jalankan **Analisis QC** |
| **Memindai QC…** | QC sedang berjalan | Tunggu sampai selesai |
| **Siap Terbit** | Artikel lolos QC | Tidak perlu tindakan |
| **Diblokir** | Pemeriksaan isi menolak artikel ini | Lihat [Cek Kualitas Naskah](05-cek-kualitas-naskah.md) |

Chip tautan memberi gambaran jumlah dan jenis tautan. Sebelum naskah ada, chip memuat batas media, misalnya "Dofollow (Maks. 2)" atau "Sponsored (Maks. 2)". Dofollow, Nofollow, dan Sponsored adalah jenis tautan yang menentukan bagaimana mesin pencari memperlakukan tautan itu. Setelah naskah ada, chip menghitung tautan yang sebenarnya: "1 Dofollow", "2 Nofollow", "1 Sponsored", atau "0 Tautan". Chip **Tidak Tersedia Backlink** berarti media itu tidak menerima tautan ke website Anda.

Status cover tampil sebagai **Cover ada**, **Cover tidak ada**, atau **Cover belum ada** (naskah belum dibuat).

### Tombol aksi per artikel

Tombol hanya berisi ikon. Namanya muncul saat kursor diarahkan ke tombol, atau saat tombol dipilih lewat keyboard.

| Ikon | Tombol | Fungsi |
|---|---|---|
| Kilau | **Generate Konten** | Membuka **Pengelola Naskah** di tab **Generate dengan AI**. Tampil bila naskah belum ada |
| Pensil dengan garis | **Tulis Sendiri** | Membuka **Pengelola Naskah** di tab **Unggah Dokumen**. Tampil bila naskah belum ada. Pembaruan lama menyebutnya "Unggah Artikel" |
| Pensil | **Edit Artikel** | Membuka naskah untuk diubah. Menjadi **Lihat Artikel** bila artikel terkunci |
| Perisai bercentang | **Cek Validasi** | Menampilkan ringkasan indikator artikel |
| Daftar bercentang | **Analisis QC** | Membuka panel hasil QC. Nonaktif sebelum naskah ada |
| Tiga titik | **Aksi lain** | Menu **Hapus artikel N**. Hanya tampil bila media punya lebih dari satu artikel |

Titik kecil di pojok tombol **Analisis QC** menandai hasil terakhir. Hijau berarti lulus. Merah berarti sudah diperiksa tetapi belum lulus.

## Cara menggunakan

### Cara menyiapkan satu artikel

1. Buka kartu media yang ingin Anda kerjakan.
2. Klik **Generate Konten** untuk menulis dengan AI. Atau klik **Tulis Sendiri** untuk mengunggah dokumen atau mengetik naskah.
3. Lengkapi naskah, gambar utama, dan kategori di **Pengelola Naskah**.
4. Klik **Simpan Artikel**. QC berjalan sendiri.
5. Tunggu chip berubah menjadi **Siap Terbit**.
6. Ulangi untuk artikel lain, lalu klik **Lanjut Tinjau & Terbit**.

Panduan lengkap ada di [Pengelola Naskah](04-pengelola-naskah.md). Langkah menerbitkan ada di [Meninjau dan menerbitkan kampanye](22-meninjau-dan-menerbitkan-kampanye.md).

### Cara menambah media

1. Klik **Kembali** di bar bawah. Anda masuk ke **Katalog Media**. Media yang sudah dipilih tetap tercentang.
2. Centang media baru.
3. Klik **Tambahkan ke Kampanye**. Anda kembali ke Penyedia Konten.

Media baru tampil sebagai kartu tanpa naskah. Lihat juga [Katalog Media](../02-katalog-media/README.md).

### Cara melepas media

1. Klik ikon tong sampah di kartu media.
2. Baca jendela **Hapus Media dari Kampanye?**, lalu klik **Ya, Lepas Media**.

Media hanya dilepas dari kampanye. Media tidak dihapus selamanya. Bila media punya beberapa artikel, jendela itu menyebut bahwa semuanya ikut dilepas. Bila Anda memilih media itu lagi lewat Katalog Media, media masuk sebagai artikel baru. Jendela itu menyebut "The Matchmaker". Maksudnya tahap pertama, yaitu **Katalog Media**.

### Cara memilih beberapa artikel

Centang kotak di baris artikel atau di header media. Bar **N media dipilih** muncul dengan tombol **Pilih Semua** atau **Batal Pilih Semua**. Tombol **Generate Semua** di banner aktif bila Anda memilih sedikitnya dua artikel. Fitur ini belum tersedia. Bila Anda mengklik tombol itu, yang muncul hanya pemberitahuan bahwa pembuatan artikel di semua media terpilih masih dikerjakan. Buat artikel satu per satu, atau pakai **Buat N artikel** untuk satu media.

## Bar bawah

Bar bawah selalu terlihat. Isinya:

- **Kembali** ke Katalog Media.
- Kotak **TOTAL TAGIHAN AKHIR**. Arahkan kursor atau klik untuk melihat **Rincian biaya**. Penjelasannya ada di [Tagihan di Penyedia Konten](20-tagihan-di-penyedia-konten.md).
- Pesan status QC (hanya di layar lebar): "QC sudah lulus" atau "QC belum lulus. Klik Lanjut Tinjau & Terbit untuk melihat apa yang kurang."
- Tombol **Lanjut Tinjau & Terbit**.

## Mode Pelacakan & Anchor Text

Kartu ini mengatur tautan utama kampanye. Kartu muncul bila kampanye punya target tautan, misalnya dari tahap Profiler atau Brief (lihat [Profiler dan Taut Brief](../04-profiler-dan-taut-brief/README.md)). Anchor text adalah kata di artikel yang bisa diklik untuk menuju tautan Anda.

- Kotak **Tauthub Tracking Link** (tautan pendek dan statistik klik). Bila dimatikan, tautan dipasang apa adanya (**Direct URL (Pure SEO)**).
- **Tautan Utama (Prioritas 1)**: kolom **Target URL** dan **Anchor Text Induk** wajib diisi.
- **Tautan Cadangan (Prioritas 2)**: boleh tidak diisi. Bila diisi, kedua kolomnya harus terisi berpasangan. Keterangan di kartu menyebut AI memakainya secara dinamis, hanya pada media yang mengizinkan paling banyak 2 tautan.

Perubahan tersimpan sendiri. Status penyimpanan tampil di pojok kanan atas kartu: **Belum lengkap**, **Belum tersimpan**, **Menyimpan…**, **Tersimpan otomatis**, atau **Gagal tersimpan**. Pratinjau URL atau tautan pendek tampil di bawah kolom utama.

## Mode lihat saja

Kampanye berstatus **Terjadwal** atau **Terkirim** tidak bisa diubah lagi. Banner "Kampanye ini sudah selesai" muncul bersama keterangan mode lihat saja, dengan tautan **Buka Ringkasan**. Dalam mode ini:

- tombol **Generate Konten**, **Tulis Sendiri**, **Cek Validasi**, pengatur jumlah artikel, ikon tong sampah, dan **Lanjut Tinjau & Terbit** nonaktif;
- **Edit Artikel** berubah menjadi **Lihat Artikel**, dan naskah terbuka tanpa kolom yang bisa diubah;
- **Analisis QC** tetap bisa dibuka untuk melihat hasilnya;
- tahap **Katalog Media** terkunci.

Pada kampanye yang diulang, hanya artikel yang sudah punya publikasi yang terkunci (chip **Terkunci**). Artikel putaran baru tetap bisa Anda kerjakan.

## Tampilan HP

- Header kartu media memakai tata letak berpetak. Kontrol **Buat N artikel**, pengatur jumlah, dan tong sampah turun ke baris sendiri.
- Tombol aksi artikel turun ke bawah keterangan artikel dan lebih besar. HP tidak punya kursor, jadi nama tombol tidak muncul. Kenali tombol dari ikon pada tabel di atas.
- Di bar bawah, **Kembali** menjadi ikon panah. Tombol utama bertuliskan **Lanjut**. Pesan status QC tidak tampil.
- Gambar hiasan di banner disembunyikan.

## Pesan yang mungkin muncul

| Pesan | Artinya | Langkah Anda |
|---|---|---|
| "AI Writer belum dikonfigurasi…" (pita merah) | Fitur menulis dengan AI belum menyala | Tulis atau unggah naskah sendiri. Fitur AI berjalan setelah tim Tauthub menyalakannya |
| "Belum ada media terpilih…" | Kampanye belum punya media | Klik **Kembali** dan pilih media |
| "Gagal memuat data Writer." | Data halaman tidak terbaca | Muat ulang halaman |
| "Pemindaian QC masih berjalan di server…" | QC memakan waktu lebih lama dari biasanya | Tunggu. Hasil muncul sendiri |
| **Saldo Tidak Cukup** | Saldo kurang saat menerbitkan | Klik **Top Up Saldo**, lalu buka **Ringkasan** untuk memeriksa status publikasi |
| "Penerbitan gagal. Periksa panel Analisis QC lalu coba lagi." | Penerbitan ditolak | Klik **Buka Panel QC**, perbaiki, lalu ulangi |
| "Artikel sudah masuk jadwal terbit…" | Artikel terjadwal terkunci | Hubungi admin bila perlu revisi |

## Pertanyaan yang sering diajukan

**Kenapa tombol Generate Konten tidak ada di sebuah artikel?**
Tombol itu hanya tampil bila naskah belum ada. Untuk menulis ulang naskah yang sudah ada, klik **Edit Artikel** lalu buka tab **Generate dengan AI**.

**Apakah artikel yang saya ketik sendiri tetap diperiksa QC?**
Ya. Semua artikel, baik buatan AI maupun unggahan, melewati pemeriksaan yang sama.

**Bisakah saya menerbitkan sebagian media saja?**
Tidak. Semua artikel yang belum terbit harus siap. Lepaskan media yang tidak ingin Anda terbitkan.

## Lihat juga

- [Pengelola Naskah](04-pengelola-naskah.md)
- [Cek Kualitas Naskah](05-cek-kualitas-naskah.md)
- [Tagihan di Penyedia Konten](20-tagihan-di-penyedia-konten.md)
- [Beberapa artikel untuk satu media](21-beberapa-artikel-untuk-satu-media.md)
- [Meninjau dan menerbitkan kampanye](22-meninjau-dan-menerbitkan-kampanye.md)
- [Ringkasan dan distribusi](06-ringkasan-distribusi.md)
- [Alur dan tahap kampanye](README.md)
