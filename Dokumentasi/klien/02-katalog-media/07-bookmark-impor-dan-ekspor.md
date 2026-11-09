# Bookmark, impor, dan ekspor

> Simpan media favorit dengan Bookmark, pilih banyak media sekaligus dari daftar domain lewat Impor List, dan unduh pilihan Anda sebagai proposal berformat CSV lewat Ekspor Proposal.

**Lokasi:** menu **RUANG KERJA** › **Katalog Media**
**Untuk:** Klien

## Gambaran singkat

Tiga fitur ini membantu Anda mengelola daftar media tanpa mengulang pencarian:

| Fitur | Fungsi |
|---|---|
| **Bookmark** | Menandai media favorit agar mudah ditemukan lagi |
| **Impor List** | Mencentang banyak media sekaligus dari daftar domain atau file proposal |
| **Ekspor Proposal** | Mengunduh media terpilih sebagai file CSV untuk dibagikan atau diarsipkan |

Tidak satu pun dari ketiganya membuat kampanye atau memengaruhi saldo.

## Bookmark

**Bookmark** menyimpan media favorit di peramban Anda. Bookmark tidak ikut berpindah ke perangkat atau peramban lain, dan berlaku untuk semua brand yang Anda buka di peramban yang sama. Bookmark hilang bila Anda menghapus data peramban.

### Cara menyimpan media ke Bookmark

1. Cari media di tabel.
2. Klik ikon bookmark di kolom **Aksi**. Tooltipnya bernama **Simpan ke Bookmark**.
3. Lihat ikon terisi. Pada layar lebar, tombol **Bookmark** di toolbar menunjukkan angka jumlah media tersimpan.

Mengklik ikon bookmark tidak membuka panel detail dan tidak mencentang media.

### Cara melihat media yang disimpan

1. Klik **Bookmark** di toolbar. Di HP, klik tombol corong, lalu klik **Bookmark** di panel **Alat Katalog**.
2. Lihat banner **Menampilkan N media tersimpan** di atas tabel. Tabel kini hanya memuat media yang Anda simpan.
3. Klik **Tampilkan semua** pada banner, atau klik **Bookmark** sekali lagi, untuk kembali ke seluruh daftar.

Filter Bookmark bisa digabung dengan pencarian, kategori, dan filter lanjutan. Angka N pada banner menghitung semua media yang Anda simpan, termasuk yang tidak tampil di daftar saat ini.

### Cara menghapus media dari Bookmark

1. Cari media itu di tabel, atau aktifkan filter **Bookmark**.
2. Klik ikon bookmark di kolom **Aksi** sekali lagi. Tooltipnya sekarang bernama **Hapus dari Bookmark**.

## Impor List

**Impor List** memilih media dari daftar yang sudah Anda punya. Media yang cocok ditambahkan ke pilihan Anda, dan pilihan yang sudah ada tidak dihapus.

### Format daftar yang dikenali

- Satu media per baris, ditulis sebagai domain (misalnya `contoh.id`) atau nama media (misalnya `Detik.com`).
- Awalan `https://`, `www.`, dan garis miring di akhir diabaikan. Huruf besar dan kecil dianggap sama.
- Bila satu baris memuat beberapa kolom yang dipisah titik koma, koma, atau tab, hanya kolom pertama yang dibaca.
- File hasil **Ekspor Proposal** bisa diimpor kembali karena kolom pertamanya adalah nama media.

### Cara mengimpor daftar media

1. Kosongkan kotak pencarian di katalog.
2. Klik **Impor List**. Jendela **Impor Daftar Media** terbuka.
3. Pilih salah satu cara memasukkan daftar:
   - klik **Pilih file proposal (.csv / .txt)** lalu pilih file dari perangkat Anda; atau
   - tempel teks daftar di kolom di bawah tulisan **Atau Tempel Teks**.
4. Klik **Terapkan**.
5. Lihat bar seleksi di bagian bawah layar. Media yang cocok kini tercentang.

Klik **Batal** atau area di luar jendela untuk menutupnya tanpa mengimpor.

> **Penting:** Daftar dicocokkan dengan media yang sedang dimuat di halaman. Bila kotak pencarian masih berisi teks, hanya media hasil pencarian itu yang bisa cocok. Kosongkan pencarian sebelum mengimpor.

### Pesan galat saat mengimpor

| Pesan | Artinya | Langkah Anda |
|---|---|---|
| **Tidak ada baris teks yang terbaca.** | Kolom teks kosong atau hanya berisi spasi | Tempel daftar atau pilih file |
| **Tidak ada media yang cocok dengan daftar Anda.** | Tidak satu pun baris sama dengan media di katalog | Periksa ejaan domain, dan pastikan pencarian kosong |
| **Gagal membaca file.** | File tidak bisa dibaca | Pilih file `.csv` atau `.txt` lain, atau tempel teksnya |

Baris yang tidak cocok dengan media mana pun dilewati tanpa pemberitahuan. Periksa jumlah media di bar seleksi setelah impor untuk memastikan semuanya masuk.

## Ekspor Proposal

**Ekspor Proposal** mengunduh daftar media terpilih sebagai file CSV. Tombol ini ada di bar seleksi pada layar lebar. Tombolnya tidak tersedia di HP.

### Cara mengunduh proposal

1. Centang media yang ingin dimasukkan ke proposal.
2. Klik **Ekspor Proposal** di bar seleksi.
3. Buka file yang terunduh. Namanya berpola **proposal-media-** diikuti tanggal hari itu, misalnya `proposal-media-2026-11-08.csv`.

File memakai pemisah titik koma agar kolomnya terbaca rapi di Excel atau Google Sheets berbahasa Indonesia. Kolomnya:

| Kolom | Isi |
|---|---|
| **Nama Media** | Nama media |
| **Website** | Alamat situs media |
| **Kategori** | Kategori utama. Tertulis Umum bila kosong |
| **SLA Terbit** | Rata-rata waktu terbit seperti di tabel, atau — bila belum ada data |
| **DA** | Skor DA, atau — bila belum diperiksa |
| **AI Trust / GEO** | Skor AI Trust, tanda garis tegak, lalu **GEO:Ya**, **GEO:Tidak**, atau **GEO:—** |
| **Harga IDR** | Harga satu publikasi dalam rupiah, ditulis sebagai angka tanpa titik pemisah |

Pada kolom **AI Trust / GEO**, **GEO:Ya** berarti media terdeteksi dikutip AI, **GEO:Tidak** berarti tidak, dan **GEO:—** berarti belum bisa ditentukan. Informasi ini tidak ditampilkan di tabel katalog.

> **Catatan:** File hanya memuat media terpilih yang sedang dimuat di halaman. Bila Anda memakai pencarian, media terpilih yang tidak ada di hasil pencarian tidak ikut diekspor. Kosongkan pencarian sebelum mengekspor.

### Cara mengunduh ringkasan satu media

Untuk ringkasan PDF satu media, pakai tombol **Ekspor Info** di panel detail. Langkahnya ada di [Panel detail media](03-detail-media.md).

## Aturan dan batasan

- **Bookmark**, pilihan media, dan filter bekerja pada media yang dimuat di halaman, paling banyak 250.
- **Impor List** menambah pilihan. Fitur ini tidak mengganti atau menghapus pilihan yang sudah ada.
- **Ekspor Proposal** hanya menyusun file di perangkat Anda. Tidak ada pesanan, perubahan saldo, atau pemberitahuan ke pihak lain.
- Angka di file proposal adalah angka saat file dibuat. Harga dan metrik bisa berubah kemudian.

## Pertanyaan yang sering diajukan

**Mengapa bookmark saya tidak ada di laptop lain?** Bookmark disimpan di peramban, bukan di akun. Simpan ulang di perangkat itu, atau pakai **Ekspor Proposal** dan **Impor List** untuk memindahkan daftar.

**Bagaimana memindahkan pilihan dari satu perangkat ke perangkat lain?** Pilih media di perangkat pertama, klik **Ekspor Proposal**, lalu impor file itu di perangkat kedua lewat **Impor List**.

**Mengapa jumlah media yang tercentang kurang dari jumlah baris di daftar saya?** Sebagian baris tidak cocok dengan media di katalog, atau media itu belum tampil di katalog. Periksa ejaan, dan cari media itu secara manual.

## Lihat juga

- [Memilih media dan bar seleksi](04-memilih-media-dan-bar-seleksi.md)
- [Panel detail media](03-detail-media.md)
- [Menjelajah dan menyaring media](01-menjelajah-dan-menyaring-media.md)
