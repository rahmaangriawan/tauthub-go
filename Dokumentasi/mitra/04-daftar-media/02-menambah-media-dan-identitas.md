# Menambah media dan mengisi identitas media

> Gunakan tombol Tambah Media untuk mendaftarkan situs Anda sebagai media di Tauthub. Halaman ini menjelaskan syarat, langkah menyimpan, pesan galat yang umum, dan cara mengisi bagian Identitas media pada formulir.

**Lokasi:** menu **Daftar Media** › **Tambah Media**
**Untuk:** Mitra

## Gambaran singkat

Formulir **Tambah media** memiliki lima bagian. Di layar lebar, formulir terbagi dua kolom. Di HP, semua bagian tersusun ke bawah.

| Bagian | Isi | Dibahas di |
|---|---|---|
| **Identitas media** | Nama, alamat situs, jenis media, bahasa, kategori, contoh artikel | Halaman ini |
| **Ketentuan redaksi** | Jumlah kata, embed, topik terlarang, label sponsor, subjudul | [Ketentuan redaksi dan tautan](03-ketentuan-redaksi-dan-tautan.md) |
| **Integrasi & publikasi** | Manual atau API, platform tujuan | [Integrasi, harga, dan Secure Direct](04-integrasi-harga-dan-secure-direct.md) |
| **Backlink & SEO** | Jenis tautan, jumlah backlink, masa berlaku tautan | [Ketentuan redaksi dan tautan](03-ketentuan-redaksi-dan-tautan.md) |
| **Harga & kerja sama** | Harga pengajuan | [Integrasi, harga, dan Secure Direct](04-integrasi-harga-dan-secure-direct.md) |

Setelah disimpan, media berstatus **Menunggu review** dan belum tampil di katalog klien sampai tim Tauthub menyetujuinya.

## Sebelum mulai

- Siapkan harga per artikel. Harga wajib diisi.
- Siapkan alamat situs lengkap, misalnya `https://medianasional.example`. Alamat ini tidak wajib di formulir, tetapi dibutuhkan untuk verifikasi domain, deteksi platform, dan koneksi WordPress atau EmDash.
- Mulai dari media kedua, Anda wajib mengisi nomor WhatsApp. Lihat bagian di bawah.
- Untuk mode **API**, Anda perlu mencentang persetujuan ketentuan kerja sama di formulir.

## Cara menambah media

1. Buka menu **Daftar Media**.
2. Klik **Tambah Media**.
3. Isi bagian **Identitas media**. Nama media, minimal satu kategori, dan minimal satu bahasa wajib diisi.
4. Isi empat bagian lain pada formulir.
5. Klik **Tambah media** di bagian bawah formulir.

Pesan hijau "Media diajukan untuk review admin." muncul di atas daftar, dan media baru tampil dengan status **Menunggu review**.

Tombol **Tambah media** nonaktif bila harga kosong atau 0, bila mode **API** dipilih tanpa persetujuan, atau selama proses simpan berjalan ("Menyimpan…"). Bila harga kosong, arahkan kursor ke tombol untuk melihat sebabnya.

Pesan galat tampil dalam kotak merah di bagian atas formulir. Formulir menggulir ke atas otomatis supaya pesan itu terlihat.

> **Penting:** Klik di luar kotak formulir atau tekan Esc menutup formulir tanpa menyimpan. Isian yang belum disimpan hilang.

> **Tips:** Di menu **Integrasi**, tab **Blogspot**, tombol **Buat media Blogspot** membuka formulir ini dengan **Media Blogger**, mode **API**, dan platform **Blogspot** yang sudah terpilih.

## Nomor WhatsApp untuk media kedua dan seterusnya

Media pertama bebas. Mulai media kedua, Tauthub meminta nomor WhatsApp dan izin untuk dihubungi. Semua media di akun Anda ikut dihitung, termasuk yang **Ditolak** atau **Diarsipkan**.

Bila syarat ini belum terpenuhi, penyimpanan ditolak dengan pesan: "Menambah media kedua dan seterusnya wajib mengisi nomor WhatsApp dan mencentang izin dihubungi." Di bawah pesan ada tautan **Isi nomor WhatsApp sekarang**.

1. Klik **Isi nomor WhatsApp sekarang**. Anda dibawa ke menu **Konfigurasi**.
2. Isi nomor di kartu **Nomor WhatsApp** pada tab **Profil**.
3. Centang kotak persetujuan.
4. Klik **Simpan nomor WhatsApp**.
5. Kembali ke **Daftar Media**, lalu simpan media lagi.

Nomor yang sama juga bisa diisi di tab **Notifikasi**. Lihat [Profil dan konfigurasi](../14-profil-dan-konfigurasi/README.md).

## Mengisi bagian Identitas media

Teks di bawah judul bagian: "Yang dilihat klien di katalog: nama, domain, jenis, kategori, dan contoh artikel."

### Nama media

Wajib diisi, maksimal 160 karakter. Contoh isian: `Media Nasional Indonesia`.

### Website / Domain

Isi alamat lengkap dengan `https://`. Ikon tautan di ujung kolom membuka alamat itu di tab baru.

### Jenis media

Pilih salah satu kartu. Pilihan awalnya **Media Mainstream**.

- **Media Blogger**: "Blog personal / Blogger yang digunakan sebagai media publikasi."
- **Media Mainstream**: "Media berita / publisher profesional."

Jenis media memengaruhi gaya naskah yang disusun otomatis untuk media Anda. Media Mainstream ditulis bergaya berita. Media Blogger ditulis bergaya blog yang lebih santai. Jenis media juga menentukan bawaan izin subjudul, seperti dijelaskan di [Ketentuan redaksi dan tautan](03-ketentuan-redaksi-dan-tautan.md). Klien bisa menyaring katalog berdasarkan jenis media.

### Bahasa artikel

Centang bahasa yang diterima media Anda: **Indonesia** (pilihan awal), **Inggris**, **China**, atau **Arab**. Minimal satu bahasa harus dicentang.

> **Catatan:** Klien melihat semua bahasa yang Anda centang. Namun naskah yang disusun otomatis ditulis dalam satu bahasa saja, yaitu bahasa yang paling dulu tersimpan di daftar bahasa media Anda.

### Kategori media

Minimal satu kategori, maksimal 20.

1. Ketik nama kategori di kolom **Cari kategori…**.
2. Klik **＋ Tambah kategori** atau tekan Enter.
3. Ulangi untuk kategori lain.

Di bawah kolom, tombol saran muncul dalam bentuk **＋ nama kategori** (paling banyak 8). Saran berasal dari kategori media Anda yang lain. Bila teks yang Anda ketik belum ada di saran, muncul tombol **＋ Buat kategori baru** dengan teks tersebut. Klik tanda × pada kategori untuk menghapusnya. Setelah 20 kategori, tertulis "Maksimal 20 kategori per media."

Saat disimpan, nama kategori dirapikan otomatis: setiap kata diawali huruf kapital ("berita" menjadi "Berita"), dan singkatan berhuruf kapital seperti "SEO" dipertahankan. Tanda koma di dalam satu isian memecahnya menjadi beberapa kategori.

### Contoh artikel

Contoh artikel adalah tautan satu artikel karya Anda yang terbit di media ini. Teks di layar menyebut contoh artikel membuat media lebih dipercaya klien. Klien melihatnya sebagai tautan **Lihat contoh**.

Dari formulir:

1. Klik **Tambah contoh artikel**. Dialog **Contoh Artikel** terbuka.
2. Isi **URL artikel**, misalnya `https://media.example/artikel-contoh`.
3. Klik **Simpan**. Tautan baru masuk ke formulir dan tersimpan bersama media.

Setelah tautan ada, formulir menampilkan lencana **Tersedia** dan tombol **Ganti** serta **Hapus**. **Hapus** meminta konfirmasi "Hapus contoh artikel?" dengan pilihan **Batal** atau **Ya, hapus**.

Dari tabel: klik **+ Tambah contoh artikel** pada baris media, isi **URL artikel**, lalu klik **Simpan**. Pesan "Contoh artikel tersimpan." muncul dan perubahan berlaku langsung tanpa tinjauan.

> **Penting:** Pastikan artikel yang Anda tautkan tidak terikat perjanjian kerahasiaan (NDA). Tauthub mendorong mitra untuk menjaga kerahasiaan kerja sama dengan klien.

> **Catatan:** Pada versi saat ini, mengosongkan tautan lewat **Hapus** lalu **Simpan perubahan** belum menghapus tautan yang sudah tersimpan. Untuk mengubahnya, gunakan **Ganti**.

## Pesan galat yang umum

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Nama media wajib diisi. | Nama kosong. | Isi nama media. |
| Tambahkan minimal 1 kategori media. | Belum ada kategori. | Tambahkan satu kategori atau lebih. |
| Pilih minimal 1 bahasa artikel. | Semua bahasa tidak dicentang. | Centang minimal satu bahasa. |
| Harga media wajib diisi (lebih dari 0) sebelum diajukan untuk review. | Harga kosong atau 0. | Isi harga di bagian **Harga & kerja sama**. |
| Centang persetujuan Ketentuan Kerja Sama Mitra terlebih dahulu sebelum menyimpan media integrasi. | Mode **API** dipilih tanpa persetujuan. | Centang kotak persetujuan di bagian **Integrasi & publikasi**. |
| Menambah media kedua dan seterusnya wajib mengisi nomor WhatsApp… | Nomor WhatsApp atau izin dihubungi belum lengkap. | Ikuti langkah di bagian nomor WhatsApp di atas. |
| Brand slug, domain, or membership already exists | Nama media sama dengan media lain yang sudah terdaftar, atau hanya berbeda huruf besar-kecil dan tanda baca. | Ubah nama, misalnya tambahkan kata pembeda seperti nama kota. |
| Invalid max_links | Jumlah backlink 0 atau 3 dipilih. | Pilih **1 Link** atau **2 Link**. Lihat [Ketentuan redaksi dan tautan](03-ketentuan-redaksi-dan-tautan.md). |
| Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi. | Anda menyimpan terlalu sering dalam waktu singkat. | Tunggu satu menit, lalu coba lagi. |

## Setelah media disimpan

Tim Tauthub menerima pemberitahuan pengajuan Anda dan meninjaunya. Selama menunggu, Anda tetap bisa memverifikasi domain, mengisi contoh artikel, dan membuka **Edit**. Pemeriksaan DA dan AI Trust juga dimulai otomatis sejak pengajuan.

Keputusan tim Tauthub Anda terima sebagai notifikasi. Penjelasan status dan langkah berikutnya ada di [Mengedit media dan status](05-mengedit-media-dan-status.md). Syarat agar media tampil di katalog klien ada di [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

## Lihat juga

- [Membaca Daftar Media](01-membaca-daftar-media.md)
- [Impor XLSX](08-impor-xlsx.md) untuk menambah banyak media sekaligus
- [Integrasi](../13-integrasi/README.md)
