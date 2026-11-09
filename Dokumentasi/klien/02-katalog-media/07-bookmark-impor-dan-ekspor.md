# Bookmark, impor, dan ekspor

> Simpan media favorit dengan Bookmark. Pilih banyak media sekaligus dari daftar domain (alamat situs) lewat Impor List. Unduh pilihan Anda sebagai proposal berformat CSV (file tabel sederhana) lewat Ekspor Proposal.

**Lokasi:** menu **RUANG KERJA** › **Katalog Media**
**Untuk:** Klien

## Gambaran singkat

Tiga fitur ini membantu Anda mengatur daftar media tanpa mencari ulang:

| Fitur | Fungsi |
|---|---|
| **Bookmark** | Menandai media favorit supaya mudah ditemukan lagi |
| **Impor List** | Mencentang banyak media sekaligus dari daftar domain atau file proposal |
| **Ekspor Proposal** | Mengunduh media terpilih sebagai file CSV untuk dibagikan atau disimpan |

Ketiganya tidak membuat kampanye dan tidak memengaruhi saldo.

## Bookmark

**Bookmark** menyimpan media favorit di aplikasi penjelajah web Anda (misalnya Chrome). Bookmark tidak ikut pindah ke perangkat atau aplikasi penjelajah lain. Bookmark berlaku untuk semua brand yang Anda buka di aplikasi penjelajah yang sama. Bookmark hilang bila Anda menghapus data aplikasi penjelajah itu.

### Cara menyimpan media ke Bookmark

1. Cari media di tabel.
2. Klik ikon bookmark di kolom **Aksi**. Keterangan yang muncul saat kursor diarahkan ke ikon itu bernama **Simpan ke Bookmark**.
3. Lihat ikon yang kini terisi. Di layar lebar, tombol **Bookmark** di deretan alat menunjukkan angka jumlah media tersimpan.

Mengklik ikon bookmark tidak membuka panel detail dan tidak mencentang media.

### Cara melihat media yang disimpan

1. Klik **Bookmark** di deretan alat. Di HP, klik tombol corong, lalu klik **Bookmark** di panel **Alat Katalog**.
2. Lihat banner (kotak info) **Menampilkan N media tersimpan** di atas tabel. Tabel kini hanya memuat media yang Anda simpan.
3. Klik **Tampilkan semua** pada banner, atau klik **Bookmark** sekali lagi, untuk kembali ke seluruh daftar.

Filter Bookmark bisa dipakai bersama pencarian, kategori, dan filter lanjutan. Angka N pada banner menghitung semua media yang Anda simpan, termasuk yang tidak tampil di daftar saat ini.

### Cara menghapus media dari Bookmark

1. Cari media itu di tabel, atau nyalakan filter **Bookmark**.
2. Klik ikon bookmark di kolom **Aksi** sekali lagi. Keterangannya sekarang bernama **Hapus dari Bookmark**.

## Impor List

**Impor List** memilih media dari daftar yang sudah Anda punya. Media yang cocok ditambahkan ke pilihan Anda. Pilihan yang sudah ada tidak dihapus.

### Format daftar yang dikenali

- Satu media per baris, ditulis sebagai domain (misalnya `contoh.id`) atau nama media (misalnya `Detik.com`).
- Awalan `https://`, `www.`, dan garis miring di akhir tidak dihiraukan. Huruf besar dan kecil dianggap sama.
- Bila satu baris punya beberapa kolom yang dipisah titik koma, koma, atau tab, hanya kolom pertama yang dibaca.
- File hasil **Ekspor Proposal** bisa Anda impor kembali, karena kolom pertamanya adalah nama media.

### Cara mengimpor daftar media

1. Kosongkan kotak pencarian di katalog.
2. Klik **Impor List**. Jendela **Impor Daftar Media** terbuka.
3. Pilih salah satu cara memasukkan daftar:
   - klik **Pilih file proposal (.csv / .txt)**, lalu pilih file dari perangkat Anda; atau
   - tempel teks daftar di kolom di bawah tulisan **Atau Tempel Teks**.
4. Klik **Terapkan**.
5. Lihat bar seleksi di bagian bawah layar. Media yang cocok kini tercentang.

Klik **Batal** atau area di luar jendela untuk menutupnya tanpa mengimpor.

> **Penting:** Tauthub mencocokkan daftar Anda dengan media yang sedang dimuat di halaman. Bila kotak pencarian masih berisi teks, hanya media hasil pencarian itu yang bisa cocok. Kosongkan pencarian sebelum Anda mengimpor.

### Pesan galat saat mengimpor

| Pesan | Artinya | Langkah Anda |
|---|---|---|
| **Tidak ada baris teks yang terbaca.** | Kolom teks kosong atau hanya berisi spasi | Tempel daftar atau pilih file |
| **Tidak ada media yang cocok dengan daftar Anda.** | Tidak satu pun baris sama dengan media di katalog | Periksa ejaan domain, dan pastikan pencarian kosong |
| **Gagal membaca file.** | File tidak bisa dibaca | Pilih file `.csv` atau `.txt` lain, atau tempel teksnya |

Baris yang tidak cocok dengan media mana pun dilewati tanpa pemberitahuan. Setelah impor, lihat jumlah media di bar seleksi untuk memastikan semuanya masuk.

## Ekspor Proposal

**Ekspor Proposal** mengunduh daftar media terpilih sebagai file CSV. Tombol ini ada di bar seleksi pada layar lebar. Di HP, tombol ini tidak ada.

### Cara mengunduh proposal

1. Centang media yang ingin Anda masukkan ke proposal.
2. Klik **Ekspor Proposal** di bar seleksi.
3. Buka file yang terunduh. Namanya berpola **proposal-media-** diikuti tanggal hari itu, misalnya `proposal-media-2026-11-08.csv`.

File memakai pemisah titik koma, supaya kolomnya tampil rapi di Excel atau Google Sheets berbahasa Indonesia. Kolomnya:

| Kolom | Isi |
|---|---|
| **Nama Media** | Nama media |
| **Website** | Alamat situs media |
| **Kategori** | Kategori utama. Tertulis Umum bila kosong |
| **SLA Terbit** | Rata-rata waktu tayang seperti di tabel, atau tanda strip (-) bila belum ada data |
| **DA** | Skor DA, atau tanda strip (-) bila belum diperiksa |
| **AI Trust / GEO** | Skor AI Trust, tanda garis tegak, lalu **GEO:Ya**, **GEO:Tidak**, atau **GEO:—** |
| **Harga IDR** | Harga satu publikasi dalam rupiah, ditulis sebagai angka tanpa titik pemisah |

Pada kolom **AI Trust / GEO**, **GEO:Ya** berarti media terdeteksi dikutip AI. **GEO:Tidak** berarti tidak dikutip. **GEO:—** berarti belum bisa ditentukan. Tabel katalog tidak menampilkan informasi ini.

> **Catatan:** File hanya memuat media terpilih yang sedang dimuat di halaman. Bila Anda memakai pencarian, media terpilih yang tidak ada di hasil pencarian tidak ikut diekspor. Kosongkan pencarian sebelum Anda mengekspor.

### Cara mengunduh ringkasan satu media

Untuk ringkasan PDF satu media, pakai tombol **Ekspor Info** di panel detail. Langkahnya ada di [Panel detail media](03-detail-media.md).

## Aturan dan batasan

- **Bookmark**, pilihan media, dan filter bekerja pada media yang dimuat di halaman, paling banyak 250.
- **Impor List** menambah pilihan. Fitur ini tidak mengganti atau menghapus pilihan yang sudah ada.
- **Ekspor Proposal** hanya menyusun file di perangkat Anda. Tidak ada pesanan, perubahan saldo, atau pemberitahuan ke pihak lain.
- Angka di file proposal adalah angka saat file dibuat. Harga dan angka lain bisa berubah kemudian.

## Pertanyaan yang sering diajukan

**Mengapa bookmark saya tidak ada di laptop lain?**
Bookmark disimpan di aplikasi penjelajah web, bukan di akun. Simpan ulang di perangkat itu. Atau pakai **Ekspor Proposal** dan **Impor List** untuk memindahkan daftar.

**Bagaimana memindahkan pilihan dari satu perangkat ke perangkat lain?**
Pilih media di perangkat pertama dan klik **Ekspor Proposal**. Lalu impor file itu di perangkat kedua lewat **Impor List**.

**Mengapa jumlah media yang tercentang kurang dari jumlah baris di daftar saya?**
Sebagian baris tidak cocok dengan media di katalog, atau media itu belum tampil di katalog. Periksa ejaan, dan cari media itu sendiri.

## Lihat juga

- [Memilih media dan bar seleksi](04-memilih-media-dan-bar-seleksi.md)
- [Panel detail media](03-detail-media.md)
- [Menjelajah dan menyaring media](01-menjelajah-dan-menyaring-media.md)
