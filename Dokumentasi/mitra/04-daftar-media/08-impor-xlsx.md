# Impor XLSX: menambah banyak media sekaligus

> Impor XLSX membuat Anda bisa menambah banyak media dari satu berkas spreadsheet. XLSX adalah jenis berkas tabel, seperti yang dibuat Excel. Halaman ini menjelaskan syarat membuka fitur, isi kolom berkas, langkah pratinjau dan impor, serta pesan kesalahan yang sering muncul.

**Lokasi:** menu **Daftar Media**, tab **Kelola Media**, tombol **Impor XLSX**
**Untuk:** Mitra yang sudah punya media Aktif dengan domain terverifikasi

## Gambaran singkat

Anda mengisi satu baris spreadsheet untuk tiap media, mengunggahnya, memeriksa pratinjau, lalu mengonfirmasi. Tidak ada data yang tersimpan sebelum Anda mengonfirmasi. Semua media hasil impor berstatus **Menunggu review**, sama seperti media yang Anda tambah lewat formulir.

Batas berkas: format XLSX, ukuran paling besar 5 MB, dan paling banyak 500 baris data. Sistem hanya membaca lembar pertama. Judul kolom harus ada di baris 1.

## Syarat membuka Impor XLSX

Tombol **Impor XLSX** terkunci (ada ikon gembok dan tombolnya tidak bisa diklik) sampai dua hal terpenuhi:

1. Anda punya media yang sudah **Aktif**.
2. Domain media itu sudah terverifikasi. Domain adalah alamat situs Anda.

Arahkan kursor ke tombol untuk membaca penjelasannya: "Impor massal terbuka setelah media pertama Anda disetujui admin dan domainnya terverifikasi." Selama terkunci, kotak "Impor massal belum terbuka" tampil di atas tabel dengan dua langkah: verifikasi domain pada baris media, lalu Impor XLSX aktif dengan sendirinya.

Tombol juga tidak bisa diklik, tanpa gembok, sebentar saat daftar media sedang dimuat.

Sistem menghitung syarat ini setiap kali. Kalau verifikasi domain dicabut dan tidak ada media Aktif lain yang terverifikasi, tombol terkunci lagi. Cara memverifikasi domain ada di [Verifikasi domain dan Dewan Pers](07-verifikasi-domain-dan-dewan-pers.md).

Impor baru terbuka setelah Anda punya media. Jadi nomor WhatsApp dan izin untuk dihubungi juga harus sudah terisi. Pratinjau tidak memeriksanya, tetapi konfirmasi impor ditolak kalau belum lengkap. Lihat [Menambah media dan identitas media](02-menambah-media-dan-identitas.md).

## Cara mengimpor media

1. Klik **Impor XLSX**. Kotak **Impor Media Massal** terbuka.
2. Klik **Buka Template Spreadsheet**. Template (contoh berkas kosong untuk Anda isi) terbuka di tab baru.
3. Isi satu baris untuk tiap media. Isi kolom sesuai bagian berikutnya.
4. Simpan sebagai XLSX. Di Google Spreadsheet, pilih File, Download, lalu Microsoft Excel.
5. Kembali ke kotak tadi. Tarik dan lepas berkas ke kotak "Tarik & lepas berkas XLSX di sini", atau klik kotak itu untuk memilih berkas.
6. Kotak menutup dan kartu **Pratinjau Impor** tampil. Periksa isinya.
7. Kalau tidak ada baris bermasalah, klik **Impor N media**.

Setelah impor, judul kartu berganti menjadi **Hasil Impor** dan pesan hijau "N media berhasil diimpor." muncul. Media baru tampil di daftar dengan status **Menunggu review**. Tim Tauthub menerima pemberitahuan untuk tiap media.

Kalau tim Tauthub belum menyediakan template, kotak menampilkan pesan bahwa template spreadsheet belum diatur. Hubungi [Dukungan](../12-dukungan.md).

### Membaca pratinjau

Kartu **Pratinjau Impor** menampilkan ringkasan, misalnya "20 baris · 18 valid · 2 bermasalah". Tabelnya punya kolom **Baris**, **Nama**, **Kategori**, **Harga**, dan **Catatan**. Kolom **Catatan** berisi salah satu dari tiga hal:

- Teks merah: kesalahan pada baris itu. Kalau ada beberapa kesalahan, tiap kesalahan dipisah titik koma.
- Teks abu-abu: peringatan. Barisnya tetap sah.
- **OK** berwarna hijau: baris sah tanpa catatan.

Tombol **Impor N media** hanya muncul kalau tidak ada baris bermasalah. Kalau ada, perbaiki berkas di spreadsheet lalu unggah ulang. Pada tahap ini Anda tidak bisa mengimpor sebagian baris yang sah saja.

Klik tanda × di kartu untuk menutupnya tanpa menyimpan apa pun.

## Kolom berkas

Tulis judul kolom di baris 1. Huruf besar-kecil dan spasi ganda tidak berpengaruh. Sistem juga mengenali beberapa nama yang sama artinya, misalnya **Website**, **Domain**, **Nama**, dan **Harga**. Kalau satu judul muncul dua kali, berkas ditolak dengan pesan "Header duplikat". Kalau kolom wajib hilang, pesannya "Header wajib tidak ditemukan" disertai nama kolomnya.

| Kolom | Wajib | Isi yang diterima |
|---|---|---|
| **Website / Domain** | Ya | Alamat situs, misalnya `https://medianasional.example`. Awalan `https://` ditambahkan otomatis kalau tidak ada. Paling banyak 500 karakter. |
| **Kategori** | Ya | Satu kategori, paling banyak 15 karakter. Huruf dirapikan otomatis, misalnya "berita" menjadi "Berita". |
| **Harga Pengajuan (IDR)** | Ya | Angka, misalnya `50000` atau `50.000`. Tidak boleh negatif dan paling besar 1.000.000.000. |
| **Nama Media** | Tidak | 2 sampai 160 karakter. Kalau kosong, nama dibuat dari domain (`arus-berita.co.id` menjadi "Arus Berita") dan baris mendapat peringatan abu-abu. |
| **Jenis Media** | Tidak | **Media Mainstream** atau **Media Blogger**. Kata yang sama artinya seperti `berita`, `blogger`, dan `blog` juga diterima. Kosong berarti Mainstream. |
| **Platform / Integrasi** | Tidak | `WordPress`, `Blogspot` (juga `Blogger`), atau `Manual`. Kosong berarti WordPress, atau Blogspot kalau Jenis Media adalah Blogger. |
| **Mode Publikasi** | Tidak | `DRAFT` untuk Manual atau `PUBLISH` untuk API. Kosong berarti DRAFT. |
| **Bahasa Artikel** | Tidak | **Indonesia**, **Inggris**, **China**, atau **Arab**. Pisahkan dengan koma kalau lebih dari satu. Kosong berarti Indonesia. |
| **Sifat Harga** | Tidak | `NETT` atau `NEGO`. Kosong berarti NETT. |
| **Nomor Sertifikat Dewan Pers** | Tidak | Nomor sertifikat. Tulis `tidak` atau kosongkan kalau belum punya. |
| **Estimasi Trafik** | Tidak | Angka dari 0 sampai 10.000.000.000. |
| **Deskripsi** dan **Catatan** | Tidak | Paling banyak 2000 karakter untuk masing-masing. |

Pengaturan lain tidak ada di berkas. Pengaturan itu adalah jumlah backlink (tautan di artikel yang menuju situs klien), masa berlaku tautan, jenis tautan, jumlah kata, subjudul, contoh artikel, topik terlarang, dan label sponsor. Media hasil impor memakai nilai awal dari sistem untuk pengaturan itu. Setelah impor, periksa dan sesuaikan lewat **Edit**.

### Hal yang perlu diperhatikan

- **Platform bawaan adalah WordPress.** Baris tanpa isian **Platform / Integrasi** menjadi media berplatform WordPress, walau **Mode Publikasi** berisi DRAFT. Media seperti itu baru tampil di katalog setelah situsnya tersambung di menu **Integrasi**, dan barisnya menampilkan **Belum terhubung**. Kalau Anda menerbitkan artikel sendiri tanpa sambungan, tulis `Manual` di kolom **Platform / Integrasi**. Nilai `Custom Web`, `Custom`, `Webhook`, dan `API` juga dianggap Manual.
- **Blogspot hanya untuk Media Blogger.** Kalau **Platform / Integrasi** berisi Blogspot dan **Jenis Media** kosong, jenis media otomatis menjadi Media Blogger. Kalau **Jenis Media** berisi Mainstream, baris ditolak.
- **Harga harus lebih dari 0.** Harga 0 lolos pratinjau. Tetapi media tanpa harga tidak bisa disetujui dan tidak tampil di katalog. Isi harganya lewat **Edit**.
- **Sertifikat Dewan Pers diajukan otomatis.** Setelah media dibuat, nomor yang valid diajukan untuk diperiksa. Kalau pengajuan gagal, baris tetap berhasil dibuat dan hasil impor mencatat kesalahannya. Lihat [Verifikasi domain dan Dewan Pers](07-verifikasi-domain-dan-dewan-pers.md).
- **Impor berjalan baris demi baris.** Baris yang gagal disimpan dilaporkan di **Hasil Impor** sebagai "Baris N: …". Baris lain tetap masuk.

## Pesan kesalahan yang sering muncul

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Gagal membaca file XLSX. Pastikan file adalah hasil download Google Spreadsheet (XLSX) dengan header di baris 1. | Berkas bukan XLSX, rusak, atau lembar pertama tidak punya judul kolom di baris 1. | Simpan ulang sebagai XLSX dan pastikan judul kolom ada di baris 1 lembar pertama. |
| File terlalu besar. Maksimal 5 MB. | Ukuran berkas melebihi batas. | Bagi data menjadi beberapa berkas. |
| File tidak berisi data. Isi minimal 1 baris data di bawah header. | Hanya ada judul kolom. | Isi minimal satu baris. |
| Jumlah baris melebihi batas 500. Bagi menjadi beberapa file. | Ada lebih dari 500 baris data. | Bagi menjadi beberapa berkas. |
| Website / Domain wajib diisi. | Kolom wajib kosong. | Isi alamat situs. |
| Website / Domain tidak valid. | Alamat tidak berbentuk domain. | Pakai bentuk `https://medianasional.example`. |
| Kategori maksimal 15 karakter. | Nama kategori terlalu panjang. | Pendekkan kategori. |
| Harga harus angka. | Kolom harga berisi huruf atau simbol. | Tulis angka saja, misalnya `50000`. |
| Platform harus WordPress, Blogspot, atau Custom Web. | Isian **Platform / Integrasi** tidak dikenali. | Isi `WordPress`, `Blogspot`, atau `Manual`. |
| Mode harus DRAFT atau PUBLISH. | Isian **Mode Publikasi** tidak dikenali. | Isi `DRAFT` atau `PUBLISH`. |
| Format Nomor Sertifikat tidak valid. | Nomor tidak sesuai format Dewan Pers. | Perbaiki nomor, atau tulis `tidak`. |
| Impor massal belum terbuka. Ajukan satu media dulu lewat "Tambah Media"… | Syarat membuka impor belum terpenuhi. | Ikuti bagian syarat di atas. |
| Menambah media kedua dan seterusnya wajib mengisi nomor WhatsApp dan mencentang izin dihubungi… | Nomor WhatsApp atau izin dihubungi belum lengkap. | Isi di menu **Konfigurasi**, lalu ulangi impor. |
| Gagal: slug "…" sudah dipakai media lain. | Nama media sama dengan media lain, atau hanya beda huruf besar-kecil dan tanda baca. | Ubah **Nama Media** di berkas, lalu impor ulang baris itu. |

## Pertanyaan yang sering diajukan

**Apakah pratinjau menyimpan data?**
Tidak. Data baru tersimpan setelah Anda mengklik **Impor N media**.

**Mengapa tombol Impor N media tidak muncul?**
Masih ada baris bermasalah. Perbaiki baris yang bercatatan merah, lalu unggah ulang berkasnya.

**Apakah media hasil impor langsung tampil di katalog klien?**
Tidak. Media berstatus **Menunggu review**. Media baru tampil setelah tim Tauthub menyetujuinya dan syarat lain terpenuhi. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

## Lihat juga

- [Menambah media dan identitas media](02-menambah-media-dan-identitas.md)
- [Verifikasi domain dan Dewan Pers](07-verifikasi-domain-dan-dewan-pers.md)
- [Konfigurasi](../14-profil-dan-konfigurasi/README.md)
