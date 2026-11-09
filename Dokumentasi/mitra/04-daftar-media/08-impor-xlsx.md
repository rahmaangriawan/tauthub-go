# Impor XLSX: menambah banyak media sekaligus

> Impor XLSX memungkinkan Anda menambah banyak media dari satu berkas spreadsheet. Halaman ini menjelaskan syarat membuka fitur, isi kolom berkas, langkah pratinjau dan impor, serta pesan galat yang umum.

**Lokasi:** menu **Daftar Media**, tab **Kelola Media**, tombol **Impor XLSX**
**Untuk:** Mitra yang sudah punya media Aktif dengan domain terverifikasi

## Gambaran singkat

Anda mengisi satu baris spreadsheet untuk tiap media, mengunggahnya, memeriksa pratinjau, lalu mengonfirmasi. Tidak ada data yang tersimpan sebelum Anda mengonfirmasi. Semua media hasil impor berstatus **Menunggu review**, sama seperti media yang ditambah lewat formulir.

Batas berkas: format XLSX, ukuran maksimal 5 MB, maksimal 500 baris data. Sistem hanya membaca lembar pertama, dan header kolom harus ada di baris 1.

## Syarat membuka Impor XLSX

Tombol **Impor XLSX** terkunci (ada ikon gembok dan tombolnya nonaktif) sampai dua hal terpenuhi:

1. Anda punya media yang sudah **Aktif**.
2. Domain media itu sudah terverifikasi.

Arahkan kursor ke tombol untuk membaca penjelasannya: "Impor massal terbuka setelah media pertama Anda disetujui admin dan domainnya terverifikasi." Selama terkunci, kotak "Impor massal belum terbuka" tampil di atas tabel dengan dua langkah: verifikasi domain pada baris media, lalu Impor XLSX aktif dengan sendirinya.

Tombol juga nonaktif tanpa gembok sebentar saat daftar media sedang dimuat.

Syarat ini dihitung ulang setiap kali. Bila verifikasi domain dicabut dan tidak ada media Aktif lain yang terverifikasi, tombol terkunci lagi. Cara memverifikasi domain ada di [Verifikasi domain dan Dewan Pers](07-verifikasi-domain-dan-dewan-pers.md).

Karena impor baru terbuka setelah Anda punya media, nomor WhatsApp dan izin dihubungi juga harus sudah terisi. Pratinjau tidak memeriksanya, tetapi konfirmasi impor akan ditolak bila belum lengkap. Lihat [Menambah media dan identitas media](02-menambah-media-dan-identitas.md).

## Cara mengimpor media

1. Klik **Impor XLSX**. Dialog **Impor Media Massal** terbuka.
2. Klik **Buka Template Spreadsheet**. Template terbuka di tab baru.
3. Isi satu baris untuk tiap media. Isi kolom sesuai bagian berikutnya.
4. Simpan sebagai XLSX. Di Google Spreadsheet, pilih File, Download, lalu Microsoft Excel.
5. Kembali ke dialog. Tarik dan lepas berkas ke kotak "Tarik & lepas berkas XLSX di sini", atau klik kotak itu untuk memilih berkas.
6. Dialog menutup dan kartu **Pratinjau Impor** tampil. Periksa isinya.
7. Bila tidak ada baris bermasalah, klik **Impor N media**.

Setelah impor, kartu berganti judul menjadi **Hasil Impor** dan pesan hijau "N media berhasil diimpor." muncul. Media baru tampil di daftar dengan status **Menunggu review**, dan tim Tauthub menerima pemberitahuan untuk tiap media.

Bila template belum disediakan tim Tauthub, dialog menampilkan pesan bahwa template spreadsheet belum diatur. Hubungi [Dukungan](../12-dukungan.md).

### Membaca pratinjau

Kartu **Pratinjau Impor** menampilkan ringkasan, misalnya "20 baris · 18 valid · 2 bermasalah". Tabelnya punya kolom **Baris**, **Nama**, **Kategori**, **Harga**, dan **Catatan**. Kolom **Catatan** berisi salah satu dari tiga hal:

- Teks merah: galat pada baris itu. Bila ada beberapa galat, dipisah titik koma.
- Teks abu-abu: peringatan. Baris tetap sah.
- **OK** berwarna hijau: baris sah tanpa catatan.

Tombol **Impor N media** hanya muncul bila tidak ada baris bermasalah. Bila ada, perbaiki berkas di spreadsheet lalu unggah ulang. Anda tidak bisa mengimpor sebagian baris yang sah saja pada tahap ini.

Klik tanda × di kartu untuk menutupnya tanpa menyimpan apa pun.

## Kolom berkas

Header ditulis di baris 1. Huruf besar-kecil dan spasi ganda tidak berpengaruh. Sistem juga mengenali beberapa nama setara, misalnya **Website**, **Domain**, **Nama**, dan **Harga**. Header yang sama dua kali ditolak dengan pesan "Header duplikat". Bila kolom wajib hilang, pesannya "Header wajib tidak ditemukan" disertai nama kolomnya.

| Kolom | Wajib | Isi yang diterima |
|---|---|---|
| **Website / Domain** | Ya | Alamat situs, misalnya `https://medianasional.example`. Awalan `https://` ditambahkan otomatis bila tidak ada. Maksimal 500 karakter. |
| **Kategori** | Ya | Satu kategori, maksimal 15 karakter. Huruf dirapikan otomatis, misalnya "berita" menjadi "Berita". |
| **Harga Pengajuan (IDR)** | Ya | Angka, misalnya `50000` atau `50.000`. Tidak boleh negatif dan maksimal 1.000.000.000. |
| **Nama Media** | Tidak | 2 sampai 160 karakter. Bila kosong, nama dibuat dari domain (`arus-berita.co.id` menjadi "Arus Berita") dan baris mendapat peringatan abu-abu. |
| **Jenis Media** | Tidak | **Media Mainstream** atau **Media Blogger**. Kata setara seperti `berita`, `blogger`, dan `blog` juga diterima. Kosong berarti Mainstream. |
| **Platform / Integrasi** | Tidak | `WordPress`, `Blogspot` (juga `Blogger`), atau `Manual`. Kosong berarti WordPress, atau Blogspot bila Jenis Media adalah Blogger. |
| **Mode Publikasi** | Tidak | `DRAFT` untuk Manual atau `PUBLISH` untuk API. Kosong berarti DRAFT. |
| **Bahasa Artikel** | Tidak | **Indonesia**, **Inggris**, **China**, atau **Arab**. Pisahkan dengan koma bila lebih dari satu. Kosong berarti Indonesia. |
| **Sifat Harga** | Tidak | `NETT` atau `NEGO`. Kosong berarti NETT. |
| **Nomor Sertifikat Dewan Pers** | Tidak | Nomor sertifikat. Tulis `tidak` atau kosongkan bila belum punya. |
| **Estimasi Trafik** | Tidak | Angka dari 0 sampai 10.000.000.000. |
| **Deskripsi** dan **Catatan** | Tidak | Maksimal 2000 karakter masing-masing. |

Pengaturan lain seperti jumlah backlink, masa berlaku tautan, jenis tautan, jumlah kata, subjudul, contoh artikel, topik terlarang, dan label sponsor tidak ada di berkas. Media hasil impor memakai nilai awal sistem untuk pengaturan itu. Periksa dan sesuaikan lewat **Edit** setelah impor.

### Hal yang perlu diperhatikan

- **Platform bawaan adalah WordPress.** Baris tanpa isian **Platform / Integrasi** menjadi media berplatform WordPress, walau **Mode Publikasi** berisi DRAFT. Media seperti itu baru tampil di katalog setelah situsnya tersambung di menu **Integrasi**, dan barisnya menampilkan **Belum terhubung**. Bila Anda menerbitkan artikel sendiri tanpa koneksi, tulis `Manual` di kolom **Platform / Integrasi**. Nilai `Custom Web`, `Custom`, `Webhook`, dan `API` juga diperlakukan sebagai Manual.
- **Blogspot hanya untuk Media Blogger.** Bila **Platform / Integrasi** berisi Blogspot dan **Jenis Media** kosong, jenis media otomatis menjadi Media Blogger. Bila **Jenis Media** berisi Mainstream, baris ditolak.
- **Harga harus lebih dari 0.** Harga 0 lolos pratinjau, tetapi media tanpa harga tidak bisa disetujui dan tidak tampil di katalog. Isi harganya lewat **Edit**.
- **Sertifikat Dewan Pers diajukan otomatis.** Nomor yang valid diajukan ke tinjauan setelah media dibuat. Bila pengajuan gagal, baris tetap berhasil dibuat dan hasil impor mencatat galatnya. Lihat [Verifikasi domain dan Dewan Pers](07-verifikasi-domain-dan-dewan-pers.md).
- **Impor berjalan baris demi baris.** Baris yang gagal disimpan dilaporkan di **Hasil Impor** sebagai "Baris N: …", sedangkan baris lain tetap masuk.

## Pesan galat yang umum

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Gagal membaca file XLSX. Pastikan file adalah hasil download Google Spreadsheet (XLSX) dengan header di baris 1. | Berkas bukan XLSX, rusak, atau lembar pertama tidak berisi header di baris 1. | Simpan ulang sebagai XLSX dan pastikan header ada di baris 1 lembar pertama. |
| File terlalu besar. Maksimal 5 MB. | Ukuran berkas melebihi batas. | Bagi data menjadi beberapa berkas. |
| File tidak berisi data. Isi minimal 1 baris data di bawah header. | Hanya ada header. | Isi minimal satu baris. |
| Jumlah baris melebihi batas 500. Bagi menjadi beberapa file. | Lebih dari 500 baris data. | Bagi menjadi beberapa berkas. |
| Website / Domain wajib diisi. | Kolom wajib kosong. | Isi alamat situs. |
| Website / Domain tidak valid. | Alamat tidak berbentuk domain. | Gunakan bentuk `https://medianasional.example`. |
| Kategori maksimal 15 karakter. | Nama kategori terlalu panjang. | Pendekkan kategori. |
| Harga harus angka. | Kolom harga memuat huruf atau simbol. | Tulis angka saja, misalnya `50000`. |
| Platform harus WordPress, Blogspot, atau Custom Web. | Isian **Platform / Integrasi** tidak dikenali. | Isi `WordPress`, `Blogspot`, atau `Manual`. |
| Mode harus DRAFT atau PUBLISH. | Isian **Mode Publikasi** tidak dikenali. | Isi `DRAFT` atau `PUBLISH`. |
| Format Nomor Sertifikat tidak valid. | Nomor tidak sesuai format Dewan Pers. | Perbaiki nomor, atau tulis `tidak`. |
| Impor massal belum terbuka. Ajukan satu media dulu lewat "Tambah Media"… | Syarat membuka impor belum terpenuhi. | Ikuti bagian syarat di atas. |
| Menambah media kedua dan seterusnya wajib mengisi nomor WhatsApp dan mencentang izin dihubungi… | Nomor WhatsApp atau izin dihubungi belum lengkap. | Isi di menu **Konfigurasi**, lalu ulangi impor. |
| Gagal: slug "…" sudah dipakai media lain. | Nama media sama dengan media lain, atau hanya berbeda huruf besar-kecil dan tanda baca. | Ubah **Nama Media** di berkas, lalu impor ulang baris itu. |

## Pertanyaan yang sering diajukan

**Apakah pratinjau menyimpan data?**
Tidak. Data baru tersimpan setelah Anda mengklik **Impor N media**.

**Mengapa tombol Impor N media tidak muncul?**
Masih ada baris bermasalah. Perbaiki baris yang bercatatan merah, lalu unggah ulang berkasnya.

**Apakah media hasil impor langsung tampil di katalog klien?**
Tidak. Media berstatus **Menunggu review** dan baru tampil setelah disetujui tim Tauthub dan memenuhi syarat lain. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

## Lihat juga

- [Menambah media dan identitas media](02-menambah-media-dan-identitas.md)
- [Verifikasi domain dan Dewan Pers](07-verifikasi-domain-dan-dewan-pers.md)
- [Konfigurasi](../14-profil-dan-konfigurasi/README.md)
