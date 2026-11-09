# Membaca Daftar Media

> Daftar Media menampilkan semua media Anda dalam tabel di komputer, atau dalam kartu di HP. Halaman ini menjelaskan arti tiap kolom, cara mencari dan menyaring, dan cara membaca status serta tanda di setiap baris.

**Lokasi:** menu **Daftar Media**, tab **Kelola Media**
**Untuk:** Mitra

## Gambaran singkat

Di bawah judul **Daftar Media** ada jumlah media Anda, misalnya "12 media terdaftar pada akun mitra Anda." Angka ini tidak berubah saat Anda mencari atau menyaring. Angka ini juga tidak menghitung media berstatus **Diarsipkan**.

Halaman ini punya dua tab:

- **Kelola Media**: tabel untuk mengelola media. Tab inilah yang dibahas di halaman ini.
- **Transparansi Data**: angka yang sama dengan yang dilihat klien. Lihat [Transparansi Data dan detail media](09-transparansi-data-dan-detail-media.md).

Media terbaru ada di urutan paling atas.

## Tombol di bagian atas

Tombol berikut hanya ada di tab **Kelola Media**.

| Tombol | Fungsi |
|---|---|
| **Impor XLSX** | Menambah banyak media sekaligus dari satu berkas. Tombol ini terkunci sampai syaratnya terpenuhi. Lihat [Impor XLSX](08-impor-xlsx.md). |
| **Ajukan Secure Direct** | Meminta izin memakai mode terbit otomatis. Tombol ini hilang setelah fitur aktif di akun Anda. Lihat [Integrasi, harga, dan Secure Direct](04-integrasi-harga-dan-secure-direct.md). |
| **Tambah Media** | Membuka formulir media baru. Lihat [Menambah media](02-menambah-media-dan-identitas.md). |

## Mencari dan menyaring

Tiga kotak di bawah tombol juga berlaku di tab **Transparansi Data**.

- **Cari nama media atau domain…**: mencari di semua media Anda, bukan hanya di halaman yang sedang tampil. Hasil muncul sesaat setelah Anda berhenti mengetik. Domain adalah alamat situs, misalnya arusberita.com.
- **Status media**: pilihannya **Semua status (tanpa arsip)**, **Aktif**, **Menunggu review**, **Ditolak**, **Nonaktif**, dan **Diarsipkan**. Media yang diarsipkan hanya tampil kalau Anda memilih **Diarsipkan**.
- **Status Dewan Pers**: pilihannya **Semua status Dewan Pers**, **Terverifikasi Dewan Pers**, dan **Belum terverifikasi**. Pilihan terakhir mencakup media yang belum mengajukan, sedang menunggu, ditolak, atau berstatus konflik.

## Pindah halaman

Tabel menampilkan 50 media per halaman. Di bawahnya ada tulisan seperti "Menampilkan 50 dari 120 media". Klik **Berikutnya** atau **Sebelumnya** untuk pindah halaman. Di halaman pertama, **Sebelumnya** tidak bisa diklik. Di halaman terakhir, **Berikutnya** tidak bisa diklik.

## Kolom tabel di komputer

Tabel muncul di layar selebar 768 piksel atau lebih. Urutan kolomnya: **Media**, **Verifikasi Kepemilikan**, **Kategori**, **Mode Integrasi**, **Harga**, **Kualitas**, **DA**, **Platform**, **Status**, **Order**, dan **Aksi**. Kalau layar kurang lebar, Anda bisa menggeser tabel ke samping.

### Media

Kolom ini menampilkan favicon (gambar kecil logo situs), lalu domain dengan huruf pertama kapital, misalnya "Arusberita.com". Klik domain untuk membuka situs di tab baru. Kalau media tidak punya alamat situs, yang tampil adalah nama media. Nama media yang Anda isi di formulir tidak tampil di tabel ini.

Di sebelah domain ada ikon mata. Ikon ini menunjukkan apakah media terlihat di katalog klien. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

Di bawah domain ada tautan **Contoh artikel** (kalau sudah diisi) atau **+ Tambah contoh artikel**. Cara mengisinya ada di [Menambah media dan identitas media](02-menambah-media-dan-identitas.md).

### Verifikasi Kepemilikan

Kolom ini berisi dua ikon bulat. Ikon pertama menunjukkan verifikasi domain. Ikon kedua menunjukkan pengajuan Dewan Pers.

| Ikon | Artinya |
|---|---|
| Perisai penuh | Domain sudah terverifikasi. |
| Perisai bergaris putus | Domain belum terverifikasi. |
| Centang | Dewan Pers terverifikasi. |
| Tanda seru | Pengajuan Dewan Pers berstatus Menunggu, Ditolak, atau Konflik. |
| Tanda silang abu-abu | Belum ada pengajuan Dewan Pers. |

Arahkan kursor ke ikon (atau ketuk di HP) untuk melihat ringkasan, misalnya "Domain: terverifikasi · Dewan Pers: belum diajukan". Klik ikon untuk membuka kotak verifikasi. Lihat [Verifikasi domain dan Dewan Pers](07-verifikasi-domain-dan-dewan-pers.md).

### Kategori

Kolom ini menampilkan kategori media, dipisah koma. Klik kategori untuk membuka kotak **Kelola Kategori**. Lihat [Mengedit media dan status](05-mengedit-media-dan-status.md).

### Mode Integrasi

Lencana **API** (sambungan otomatis antara Tauthub dan situs Anda) berarti artikel dikirim otomatis ke situs Anda. Lencana **Manual** berarti Anda menempel sendiri artikelnya, lalu mengirim alamat artikel yang sudah terbit. Ikon tanda seru di judul kolom menampilkan penjelasan yang sama.

Kalau Secure Direct aktif di akun Anda, di bawah lencana ada kotak centang **Secure Direct**.

### Harga

Angka di kolom ini adalah penghasilan bersih Anda per artikel, setelah potongan platform. Angka ini bukan harga yang Anda ajukan.

Klik angkanya untuk membuka **Rincian penghasilan**. Isinya tiga baris: **Harga diajukan**, **Potongan platform**, dan **Pendapatan Anda**. Aturan pembagian hasil ada di [Penghasilan](../09-penghasilan.md).

### Kualitas dan DA

Kolom **Kualitas** menampilkan skor kesehatan media dari 0 sampai 100. Kalau skor belum pernah dihitung, tertulis **Belum dihitung**. Kolom **DA** menampilkan skor Domain Authority, yaitu nilai kekuatan nama situs di mata mesin pencari. Kalau belum diukur, yang tampil tanda strip.

Sistem mengisi kedua kolom ini. Anda tidak bisa mengubahnya di halaman ini. Arti lengkapnya ada di [Transparansi Data dan detail media](09-transparansi-data-dan-detail-media.md).

### Platform

Kolom ini menunjukkan jenis platform situs media, yaitu aplikasi yang menjalankan situs Anda.

- Pada media yang punya platform tujuan, yang tampil adalah platform itu: **WordPress**, **EmDash**, **Blogspot**, atau **Web Custom**. Anda memilih platform tujuan di formulir saat memakai mode **API**.
- Pada media tanpa platform tujuan (mode **Manual** di formulir), Tauthub membaca halaman depan situs Anda dan menentukan platformnya sendiri: **WordPress**, **Blogspot**, atau **Custom**. Tanda strip berarti platform belum terdeteksi. Misalnya karena alamat situs belum diisi, atau halamannya tidak bisa dibaca. Tauthub mengulang pembacaan ini otomatis, dan memulainya lagi kalau Anda mengubah alamat situs.

Anda tidak bisa mengubah hasil deteksi sendiri. Di tab **Transparansi Data** dan di sisi klien, **Web Custom** ditulis **Custom**.

Teks merah **Belum terhubung** di bawah nama platform muncul pada media yang punya platform tujuan, tetapi situsnya belum tersambung. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

### Status

Lencana status menunjukkan posisi media dalam proses pemeriksaan: **Aktif**, **Menunggu review**, **Ditolak**, **Nonaktif**, **Draf**, atau **Diarsipkan**. Di bawahnya bisa ada teks tambahan:

- **Perubahan menunggu persetujuan**: Anda mengajukan perubahan pada media Aktif, dan tim Tauthub belum memutuskannya. Arahkan kursor ke teks itu untuk melihat daftar perubahannya.
- **Alasan: …**: alasan penolakan pada media berstatus **Ditolak**.

Arti tiap status dan langkah Anda berikutnya ada di [Mengedit media dan status](05-mengedit-media-dan-status.md).

### Order

Kolom ini menampilkan jumlah artikel klien yang menunggu konfirmasi Anda, misalnya "3 menunggu". Kalau angkanya lebih dari 0, angka itu menjadi tautan. Klik tautan itu untuk membuka menu **Publikasi**, tab **Menunggu**. Daftarnya sudah disaring untuk media itu. Kalau tidak ada artikel yang menunggu, tertulis "0 menunggu" berwarna abu-abu. Cara menyetujui atau menolak artikel ada di [Publikasi](../03-publikasi/README.md).

### Aksi

Klik ikon titik tiga di ujung baris untuk membuka menu aksi. Menu menutup sendiri kalau Anda menggulir halaman atau mengubah ukuran jendela.

| Menu | Fungsi |
|---|---|
| **Ajukan Kembali** | Hanya untuk media **Ditolak**. Mengirim ulang media untuk diperiksa. |
| **Edit** | Membuka formulir edit media. |
| **Test Koneksi** | Hanya muncul pada media dengan jenis integrasi API lama yang diatur tim Tauthub. Menguji sambungan dan menampilkan hasilnya di bagian atas halaman. |
| **Hapus** | Memindahkan media ke arsip. Media tidak dihapus permanen. |

Pada media berstatus **Diarsipkan**, kolom ini menampilkan lencana **Terarsip** dan tombol **Pulihkan**. Cara kerjanya ada di [Mengedit media dan status](05-mengedit-media-dan-status.md).

## Tampilan di HP

Di layar yang lebih sempit dari 768 piksel, setiap media tampil sebagai kartu.

- Bagian atas kartu berisi domain, ikon mata, status, jumlah **menunggu**, kategori, alasan penolakan (kalau ada), tautan contoh artikel, dan tombol titik tiga.
- Bagian tengah berisi **Harga**, **Kualitas**, **DA**, dan **Platform**.
- Bagian bawah berisi **Verifikasi Kepemilikan**, **Mode Integrasi**, dan kotak centang **Secure Direct** kalau fitur itu aktif.

Kotak **Kelola Kategori** tidak ada di kartu. Untuk mengubah kategori dari HP, pakai menu **Edit**. Ketuk ikon untuk melihat penjelasan yang di komputer muncul saat kursor diarahkan.

## Tampilan kosong dan pesan umum

| Tampilan | Artinya |
|---|---|
| Memuat media… | Daftar sedang diambil. |
| Belum ada media. Tambahkan media pertama Anda. | Akun Anda belum punya media. Klik **Tambah Media**. |
| Tidak ada media yang cocok dengan filter ini. | Pencarian atau status yang Anda pilih tidak menemukan media. Ubah atau kosongkan penyaringnya. |
| Gagal memuat daftar media. | Daftar tidak berhasil diambil. Muat ulang halaman. Kalau terjadi lagi, hubungi [Dukungan](../12-dukungan.md). |

Pesan hijau di atas daftar berarti aksi Anda berhasil. Pesan merah berarti aksi gagal, lengkap dengan sebabnya.

## Pertanyaan yang sering diajukan

**Mengapa nama media yang saya isi tidak tampil di tabel?**
Tabel menampilkan domain situs. Nama media hanya tampil kalau alamat situs kosong.

**Mengapa angka di kolom Harga lebih kecil dari harga yang saya isi?**
Kolom itu menampilkan penghasilan bersih Anda setelah potongan platform. Klik angkanya untuk melihat rinciannya.

**Mengapa media Aktif saya tidak muncul di katalog klien?**
Ada beberapa syarat selain status Aktif. Arahkan kursor ke ikon mata di baris media untuk melihat sebabnya. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

**Mengapa media yang saya arsipkan tidak ada di daftar?**
Pilihan **Semua status (tanpa arsip)** menyembunyikan media yang diarsipkan. Pilih **Diarsipkan** di penyaring status untuk melihatnya.

## Lihat juga

- [Menambah media dan identitas media](02-menambah-media-dan-identitas.md)
- [Mengedit media dan status](05-mengedit-media-dan-status.md)
- [Katalog Media (tampilan Klien)](../../klien/02-katalog-media/README.md)
