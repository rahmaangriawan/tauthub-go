# Transparansi Data dan detail media

> Tab Transparansi Data menampilkan angka media Anda sama persis dengan yang dilihat klien di katalog. Jendela Detail merangkum aturan penerbitan media Anda. Halaman ini menjelaskan arti tiap angka dan apa yang bisa Anda lakukan kalau angkanya belum terisi.

**Lokasi:** menu **Daftar Media**, tab **Transparansi Data**
**Untuk:** Mitra

## Gambaran singkat

Tab **Transparansi Data** hanya untuk dibaca. Tidak ada tombol untuk mengubah angka di sini, karena sistem yang mengisinya. Tujuannya supaya Anda tahu bagaimana media Anda terlihat di mata klien. Di atas tabel tertulis: "Metrik di bawah ini sama dengan yang dilihat klien di katalog Tauthub."

Kotak pencarian dan dua penyaring di bagian atas halaman juga berlaku di tab ini. Tombol **Impor XLSX**, **Ajukan Secure Direct**, dan **Tambah Media** tidak ada di tab ini.

Tab ini memuat sampai 500 media sekaligus dan menampilkan 10 baris per halaman. Di bawah tabel tertulis, misalnya, "Menampilkan 1–10 dari 25 media". Tombol **Sebelumnya** dan **Berikutnya** hanya muncul kalau ada lebih dari satu halaman.

Tab ini menampilkan semua media sesuai penyaring, termasuk yang belum **Aktif**. Media yang belum Aktif tetap tidak tampil di katalog klien.

## Kolom tabel di komputer

| Kolom | Isi |
|---|---|
| **Media** | Favicon (gambar kecil logo situs) dan domain. Kalau media punya lebih dari satu kategori, di bawahnya tertulis "+N kategori lain". |
| **Kategori** | Kategori pertama media. |
| **Trafik** | Perkiraan jumlah kunjungan per bulan. |
| **DA** | Skor Domain Authority. |
| **AI Trust** | Skor AI Trust. |
| **SLA** | Rata-rata waktu sampai artikel terbit. |
| **Distribusi** | Jumlah artikel yang sudah terbit lewat media Anda. |
| **Rating** | Rata-rata nilai dari klien. |
| **Kesehatan** | Skor kesehatan media. |
| **Artikel/minggu** | Jumlah artikel yang terbit di situs Anda dalam seminggu. |
| **Aksi** | Tombol **Detail**. |

Di HP, setiap media tampil sebagai kartu dengan delapan angka yang sama. Ketuk kartu untuk membuka detailnya.

Kalau belum ada media, tab ini menampilkan "Belum ada media untuk ditampilkan." Selama memuat, tertulis "Memuat metrik media…".

## Arti tiap angka

Beberapa istilah di tabel ini perlu penjelasan. DA (Domain Authority) adalah nilai kekuatan nama situs di mata mesin pencari. AI Trust adalah nilai kepercayaan situs bagi mesin pencari berbasis AI (kecerdasan buatan). SLA di sini adalah rata-rata waktu Anda menerbitkan artikel.

| Angka | Artinya | Kalau belum ada nilai |
|---|---|---|
| **Trafik** | Perkiraan kunjungan per bulan, ditulis seperti "± 25 rb / bulan" atau "± 1,2M / bulan". Kalau ada data kunjungan dari penyedia data trafik, data itu yang dipakai. Kalau tidak ada, yang dipakai adalah perkiraan yang tersimpan di Tauthub. Di bawah angka tertulis **Via SimilarWeb**. | Tertulis 0. |
| **DA** | Skor Domain Authority situs Anda. Sistem memeriksanya otomatis. | **Belum diukur** |
| **AI Trust** | Skor 0 sampai 100 yang menilai tanda kepercayaan situs Anda bagi mesin pencari berbasis AI. Makin tinggi skornya, makin hijau warnanya. | **Belum diukur** |
| **SLA** (di detail: **SLA terbit**) | Rata-rata waktu dari artikel masuk ke media Anda sampai terbit, ditulis seperti "± <1 jam", "± 6 jam", atau "± 4,2 hari". Publikasi yang baru terbit lebih dari 7 hari setelah masuk tidak dihitung. Pada media yang punya platform tujuan, angka yang tampil paling lama ± 3 hari. | **Belum ada riwayat** |
| **Distribusi** (di detail: **Artikel terbit**) | Jumlah artikel yang sudah terbit lewat media Anda di seluruh Tauthub, ditulis "N artikel". | **Belum ada distribusi** atau **Belum ada** |
| **Rating** (di detail: **Rating klien**) | Rata-rata nilai kecepatan terbit dari klien dengan skala 5, misalnya "4,5/5". Di detail, jumlah klien yang menilai tertulis dalam kurung. | **Belum ada penilaian** |
| **Kesehatan** | Skor 0 sampai 100. Penjelasannya ada di bagian berikut. | **Belum Diskor** |
| **Artikel/minggu** (di detail: **Artikel per minggu**) | Jumlah artikel yang terdeteksi terbit di situs Anda dalam 7 hari terakhir. Tauthub membacanya dari sitemap atau feed situs. Sitemap adalah daftar semua halaman situs. Feed adalah daftar artikel terbaru. | Lihat bagian Artikel/minggu |

### Kesehatan

Sistem menghitung skor ini otomatis dari riwayat publikasi di media Anda. Yang dihitung: berapa persen yang berhasil terbit, seberapa cepat terbit, berapa yang Anda tolak, dan apakah media berstatus **Aktif**. Tim Tauthub juga bisa menambahkan penilaian mutu tulisan yang ikut memengaruhi skor. Sistem menghitung ulang skor secara berkala. Media yang belum pernah menerima publikasi mendapat skor 0 sampai ada publikasi pertama.

Warna angka menunjukkan tingkatnya.

| Skor | Warna | Sebutan yang dilihat klien |
|---|---|---|
| 70 sampai 100 | Hijau | Sangat Sehat |
| 40 sampai 69 | Kuning | Perlu Perhatian |
| Di bawah 40 | Merah | Kondisi Rendah |

Di kolom **Kualitas** pada tab **Kelola Media**, skor yang sama tampil sebagai angka tanpa warna.

### Artikel/minggu

Kolom ini bisa menampilkan:

| Tulisan | Artinya | Langkah Anda |
|---|---|---|
| "N artikel" | Jumlah artikel yang terdeteksi dalam 7 hari terakhir. | Tidak ada. |
| "N artikel (belum diperbarui)" | Angka lama. Pembacaan terakhir sudah lebih dari seminggu lalu atau tidak berhasil. | Tunggu pembacaan berikutnya. |
| **Diblokir situs** | Situs menolak dibaca oleh Tauthub. | Buka menu **Akses Crawler** untuk memberi akses. |
| **Tak ada sitemap/feed** | Tauthub tidak menemukan sitemap atau feed di situs Anda. | Sediakan sitemap atau feed di situs. |
| **Belum ada data** | Belum ada hasil pembacaan. | Tunggu. |

Di tampilan klien, angka ini bernama **Content Velocity** (kecepatan terbitnya konten). Kalau datanya masih baru, klien juga melihat lencana **PRODUKTIF** (minimal satu artikel) atau **STAGNAN** (nol artikel).

## Jendela Detail

Klik **Detail** pada baris media (di HP, ketuk kartu) untuk membuka jendela profil media. Jendela ini hanya untuk dibaca dan menampilkan data yang sedang tayang. Perubahan yang masih menunggu persetujuan belum tampil.

Bagian atas jendela menampilkan nama media, domain sebagai tautan ke situs, jenis media (**Blogger** atau **Mainstream**), dan kategori pertama. Teks pembukanya: "Ini tampilan media Anda di katalog klien Tauthub."

### Ringkasan

Ada delapan kotak angka: **Trafik per bulan**, **Domain Authority**, **AI Trust**, **Rating klien**, **SLA terbit**, **Artikel terbit**, **Kesehatan** (ditulis "skor/100"), dan **Artikel per minggu**. Di bawah **Artikel per minggu** bisa tertulis "Data per" disertai tanggal pembacaan terakhir.

Kalau data kesehatan ada, tiga indikator persentase tampil di bawah kotak. Indikator yang mencapai 100 persen diberi tanda centang.

| Indikator | Artinya |
|---|---|
| **Fulfillment** | Seberapa besar bagian publikasi yang berhasil terbit. |
| **Retensi tautan** | Turun kalau ada publikasi yang ditolak oleh Mitra. |
| **Uptime** | Seberapa sering media tersedia. Penuh kalau media berstatus **Aktif**. |

### Ketentuan publikasi

Bagian ini merangkum aturan yang Anda isi di formulir. Untuk mengubahnya, buka **Edit**.

| Baris | Isi | Diatur di |
|---|---|---|
| **Garansi tayang** | "Permanen, tautan tidak dilepas", atau jumlah bulan. | [Ketentuan redaksi dan tautan](03-ketentuan-redaksi-dan-tautan.md), masa berlaku tautan |
| **Batas tautan keluar** | Misalnya "2 tautan Dofollow". Kalau belum diatur, tertulis "Maks. 2 tautan". | Ketentuan redaksi dan tautan, jumlah backlink dan jenis tautan |
| **Panjang artikel** | Misalnya "Sesuai Permintaan Klien" atau "300–700 kata". | Ketentuan redaksi dan tautan, jumlah kata |
| **Subjudul (H2/H3)** | Misalnya "Tanpa subjudul (bawaan media berita)". | Ketentuan redaksi dan tautan, subjudul |
| **Embed video/maps** | **Diizinkan** atau **Tidak diizinkan**. | Ketentuan redaksi dan tautan, support embed |
| **Platform** | Platform media. **Web Custom** ditulis **Custom**. | [Membaca Daftar Media](01-membaca-daftar-media.md), kolom Platform |
| **Contoh artikel** | Tautan **Lihat contoh**. Baris ini hanya ada kalau contoh artikel sudah diisi. | [Menambah media dan identitas media](02-menambah-media-dan-identitas.md) |
| **Label sponsor** | "Organik, tanpa label" atau "Dengan label sponsor". | Ketentuan redaksi dan tautan, label sponsor |
| **Topik dilarang** | Daftar topik, atau "Tidak ada". | Ketentuan redaksi dan tautan, topik terlarang |

Tutup jendela dengan tombol **Tutup**, tanda ×, tombol Esc, atau klik di luar jendela.

## Yang dilihat klien tetapi tidak ada di jendela Anda

Klien membuka jendela detail yang lebih lengkap. Selain angka di atas, jendela klien memuat:

- Kartu **Jalur Publikasi**: **Tauthub Secure Direct™** atau **Editorial Review (SLA Tracked)**, sesuai pengaturan di [Integrasi, harga, dan Secure Direct](04-integrasi-harga-dan-secure-direct.md).
- **Indikator Kinerja Mitra** dengan empat kotak: **Fulfillment Rate**, **SLA Responsiveness**, **Link Permanence**, dan **Server Uptime**, ditambah **Content Velocity**.
- **Parameter AI & Redaksi**, yaitu aturan penerbitan media Anda.
- **Pembatasan Gatekeeper**, yaitu topik yang Anda larang bersama pembatasan umum Tauthub.
- Biaya publikasi dan tombol untuk memesan.
- Tombol **Bagikan** dan **Ekspor Info**.

**Ekspor Info** mengunduh laporan PDF yang meringkas media. Tombol ini hanya ada di tampilan klien. Dashboard Mitra tidak punya tombol unduh PDF. Lihat [Katalog Media (tampilan Klien)](../../klien/02-katalog-media/README.md).

## Pertanyaan yang sering diajukan

**Mengapa DA atau AI Trust media saya tertulis Belum diukur?**
Sistem yang mengisinya setelah memeriksa situs Anda. Anda tidak bisa mengisinya sendiri. Pastikan **Website / Domain** pada media terisi dengan benar.

**Mengapa skor Kesehatan media baru saya 0?**
Sistem menghitung skor dari riwayat publikasi. Selama belum ada publikasi, skornya 0. Skor naik setelah publikasi pertama terbit.

**Bagaimana meningkatkan skor Kesehatan?**
Terbitkan artikel dengan cepat, hindari menolak artikel tanpa alasan kuat, dan jaga agar media tetap **Aktif**.

**Mengapa Artikel/minggu tertulis Diblokir situs?**
Situs Anda menolak dibaca oleh Tauthub. Atur aksesnya di menu **Akses Crawler**.

**Apakah angka yang saya lihat sama dengan yang dilihat klien?**
Ya. Tab ini memakai data yang sama dengan katalog klien.

## Lihat juga

- [Membaca Daftar Media](01-membaca-daftar-media.md)
- [Tampil di katalog klien](06-tampil-di-katalog-klien.md)
- [Katalog Media (tampilan Klien)](../../klien/02-katalog-media/README.md)
