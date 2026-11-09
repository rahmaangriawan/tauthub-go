# Membaca Daftar Media

> Daftar Media menampilkan semua media Anda dalam tabel di komputer atau kartu di HP. Halaman ini menjelaskan arti tiap kolom, cara mencari dan menyaring, serta cara membaca status dan tanda di setiap baris.

**Lokasi:** menu **Daftar Media**, tab **Kelola Media**
**Untuk:** Mitra

## Gambaran singkat

Di bawah judul **Daftar Media** tertulis jumlah media Anda, misalnya "12 media terdaftar pada akun mitra Anda." Angka ini tidak berubah ketika Anda mencari atau menyaring, dan tidak menghitung media berstatus **Diarsipkan**.

Halaman memiliki dua tab:

- **Kelola Media**: tabel untuk mengelola media. Tab inilah yang dibahas di sini.
- **Transparansi Data**: metrik yang sama dengan yang dilihat klien. Lihat [Transparansi Data dan detail media](09-transparansi-data-dan-detail-media.md).

Media terbaru berada di urutan paling atas.

## Tombol di bagian atas

Tombol berikut hanya tampil di tab **Kelola Media**.

| Tombol | Fungsi |
|---|---|
| **Impor XLSX** | Menambah banyak media sekaligus dari satu berkas. Tombol terkunci sampai syaratnya terpenuhi. Lihat [Impor XLSX](08-impor-xlsx.md). |
| **Ajukan Secure Direct** | Meminta izin mode terbit otomatis. Tombol hilang setelah fitur aktif di akun Anda. Lihat [Integrasi, harga, dan Secure Direct](04-integrasi-harga-dan-secure-direct.md). |
| **Tambah Media** | Membuka formulir media baru. Lihat [Menambah media](02-menambah-media-dan-identitas.md). |

## Mencari dan menyaring

Tiga kontrol di bawah tombol juga berlaku di tab **Transparansi Data**.

- **Cari nama media atau domain…**: mencari di semua media Anda, bukan hanya halaman yang sedang tampil. Hasil muncul sesaat setelah Anda berhenti mengetik.
- **Status media**: pilihan **Semua status (tanpa arsip)**, **Aktif**, **Menunggu review**, **Ditolak**, **Nonaktif**, dan **Diarsipkan**. Media yang diarsipkan hanya tampil bila Anda memilih **Diarsipkan**.
- **Status Dewan Pers**: pilihan **Semua status Dewan Pers**, **Terverifikasi Dewan Pers**, dan **Belum terverifikasi**. Pilihan terakhir mencakup media yang belum mengajukan, sedang menunggu, ditolak, atau berstatus konflik.

## Pindah halaman

Tabel menampilkan 50 media per halaman. Di bawahnya tertulis, misalnya, "Menampilkan 50 dari 120 media". Klik **Berikutnya** atau **Sebelumnya** untuk berpindah halaman. **Sebelumnya** nonaktif di halaman pertama, dan **Berikutnya** nonaktif di halaman terakhir.

## Kolom tabel di komputer

Tabel muncul di layar selebar 768 piksel atau lebih. Urutan kolomnya: **Media**, **Verifikasi Kepemilikan**, **Kategori**, **Mode Integrasi**, **Harga**, **Kualitas**, **DA**, **Platform**, **Status**, **Order**, dan **Aksi**. Bila layar kurang lebar, tabel bisa digeser ke samping.

### Media

Kolom ini menampilkan favicon situs, lalu domain dengan huruf pertama kapital, misalnya "Arusberita.com". Klik domain untuk membuka situs di tab baru. Bila media tidak punya alamat situs, yang tampil adalah nama media. Nama media yang Anda isi di formulir tidak tampil di tabel ini.

Di sebelah domain ada ikon mata yang menunjukkan apakah media tampil di katalog klien. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

Di bawah domain ada tautan **Contoh artikel** (bila sudah diisi) atau **+ Tambah contoh artikel**. Cara mengisinya dibahas di [Menambah media dan identitas media](02-menambah-media-dan-identitas.md).

### Verifikasi Kepemilikan

Kolom ini berisi dua ikon bulat. Ikon pertama menunjukkan verifikasi domain. Ikon kedua menunjukkan pengajuan Dewan Pers.

| Ikon | Artinya |
|---|---|
| Perisai penuh | Domain sudah terverifikasi. |
| Perisai bergaris putus | Domain belum terverifikasi. |
| Centang | Dewan Pers terverifikasi. |
| Tanda seru | Pengajuan Dewan Pers berstatus Menunggu, Ditolak, atau Konflik. |
| Tanda silang abu-abu | Belum ada pengajuan Dewan Pers. |

Arahkan kursor ke ikon (atau ketuk di HP) untuk melihat ringkasan, misalnya "Domain: terverifikasi · Dewan Pers: belum diajukan". Klik ikon untuk membuka dialog verifikasi. Lihat [Verifikasi domain dan Dewan Pers](07-verifikasi-domain-dan-dewan-pers.md).

### Kategori

Kolom ini menampilkan kategori media, dipisah koma. Klik kategori untuk membuka kotak **Kelola Kategori**. Lihat [Mengedit media dan status](05-mengedit-media-dan-status.md).

### Mode Integrasi

Lencana **API** berarti artikel dikirim otomatis ke situs Anda. Lencana **Manual** berarti Anda menempel sendiri artikel dan mengirim alamat artikelnya. Ikon tanda seru di judul kolom menampilkan penjelasan yang sama.

Bila Secure Direct aktif di akun Anda, di bawah lencana ada kotak centang **Secure Direct**.

### Harga

Angka di kolom ini adalah penghasilan bersih Anda per artikel setelah potongan platform. Angka ini bukan harga yang Anda ajukan.

Klik angkanya untuk membuka **Rincian penghasilan** dengan tiga baris: **Harga diajukan**, **Potongan platform**, dan **Pendapatan Anda**. Aturan pembagian hasil dijelaskan di [Penghasilan](../09-penghasilan.md).

### Kualitas dan DA

Kolom **Kualitas** menampilkan skor kesehatan media dari 0 sampai 100. Bila skor belum pernah dihitung, tertulis **Belum dihitung**. Kolom **DA** menampilkan skor Domain Authority, atau tanda "—" bila belum diukur.

Sistem mengisi kedua kolom ini. Anda tidak bisa mengubahnya di halaman ini. Arti lengkapnya ada di [Transparansi Data dan detail media](09-transparansi-data-dan-detail-media.md).

### Platform

Kolom ini menunjukkan platform situs media.

- Pada media yang punya platform tujuan, yang tampil adalah platform itu: **WordPress**, **EmDash**, **Blogspot**, atau **Web Custom**. Platform tujuan dipilih di formulir saat mode **API** dipakai.
- Pada media tanpa platform tujuan (mode **Manual** di formulir), Tauthub membaca halaman depan situs Anda dan menentukan platformnya sendiri: **WordPress**, **Blogspot**, atau **Custom**. Tanda "—" berarti platform belum terdeteksi, misalnya karena alamat situs belum diisi atau halamannya tidak bisa dibaca. Deteksi diulang otomatis, dan dimulai lagi bila Anda mengubah alamat situs.

Anda tidak bisa mengubah hasil deteksi secara manual. Di tab **Transparansi Data** dan di sisi klien, **Web Custom** ditulis **Custom**.

Teks merah **Belum terhubung** di bawah nama platform muncul pada media yang punya platform tujuan tetapi situsnya belum tersambung. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

### Status

Lencana status menunjukkan posisi media dalam proses tinjauan: **Aktif**, **Menunggu review**, **Ditolak**, **Nonaktif**, **Draf**, atau **Diarsipkan**. Di bawahnya bisa ada teks tambahan:

- **Perubahan menunggu persetujuan**: Anda mengajukan perubahan pada media Aktif dan tim Tauthub belum memutuskannya. Arahkan kursor ke teks itu untuk melihat daftar perubahannya.
- **Alasan: …**: alasan penolakan pada media berstatus **Ditolak**.

Arti tiap status dan langkah Anda berikutnya ada di [Mengedit media dan status](05-mengedit-media-dan-status.md).

### Order

Kolom ini menampilkan jumlah artikel klien yang menunggu konfirmasi Anda, misalnya "3 menunggu". Bila angkanya lebih dari 0, angka itu menjadi tautan. Klik tautan untuk membuka menu **Publikasi**, tab **Menunggu**, dengan daftar yang sudah disaring untuk media itu. Bila tidak ada artikel yang menunggu, tertulis "0 menunggu" dengan warna abu-abu. Cara menyetujui atau menolak artikel dijelaskan di [Publikasi](../03-publikasi/README.md).

### Aksi

Klik ikon titik tiga di ujung baris untuk membuka menu aksi. Menu menutup sendiri bila Anda menggulir halaman atau mengubah ukuran jendela.

| Menu | Fungsi |
|---|---|
| **Ajukan Kembali** | Hanya untuk media **Ditolak**. Mengirim ulang media ke tinjauan. |
| **Edit** | Membuka formulir edit media. |
| **Test Koneksi** | Hanya muncul pada media dengan jenis integrasi API lama yang diatur tim Tauthub. Menguji koneksi dan menampilkan hasilnya di bagian atas halaman. |
| **Hapus** | Mengarsipkan media. Media tidak dihapus permanen. |

Pada media berstatus **Diarsipkan**, kolom ini menampilkan lencana **Terarsip** dan tombol **Pulihkan**. Cara kerja aksi ini ada di [Mengedit media dan status](05-mengedit-media-dan-status.md).

## Tampilan di HP

Di layar yang lebih sempit dari 768 piksel, setiap media tampil sebagai kartu.

- Bagian atas kartu memuat domain, ikon mata, status, jumlah **menunggu**, kategori, alasan penolakan (bila ada), tautan contoh artikel, dan tombol titik tiga.
- Bagian tengah memuat **Harga**, **Kualitas**, **DA**, dan **Platform**.
- Bagian bawah memuat **Verifikasi Kepemilikan**, **Mode Integrasi**, dan kotak centang **Secure Direct** bila fitur itu aktif.

Kotak **Kelola Kategori** tidak tersedia di kartu. Gunakan menu **Edit** untuk mengubah kategori dari HP. Ketuk ikon untuk melihat penjelasan yang di komputer muncul saat kursor diarahkan.

## Kondisi kosong dan pesan umum

| Tampilan | Artinya |
|---|---|
| Memuat media… | Daftar sedang diambil. |
| Belum ada media. Tambahkan media pertama Anda. | Akun Anda belum punya media. Klik **Tambah Media**. |
| Tidak ada media yang cocok dengan filter ini. | Pencarian atau status yang dipilih tidak menemukan media. Ubah atau kosongkan filter. |
| Gagal memuat daftar media. | Daftar tidak berhasil diambil. Muat ulang halaman, lalu hubungi [Dukungan](../12-dukungan.md) bila berulang. |

Pesan hijau di atas daftar menandakan aksi berhasil. Pesan merah menandakan aksi gagal beserta sebabnya.

## Pertanyaan yang sering diajukan

**Mengapa nama media yang saya isi tidak tampil di tabel?**
Tabel menampilkan domain situs. Nama media hanya tampil bila alamat situs kosong.

**Mengapa angka di kolom Harga lebih kecil dari harga yang saya isi?**
Kolom itu menampilkan penghasilan bersih Anda setelah potongan platform. Klik angkanya untuk melihat rinciannya.

**Mengapa media Aktif saya tidak muncul di katalog klien?**
Ada beberapa syarat selain status Aktif. Arahkan kursor ke ikon mata di baris media untuk melihat sebabnya. Lihat [Tampil di katalog klien](06-tampil-di-katalog-klien.md).

**Mengapa media yang saya arsipkan tidak ada di daftar?**
Pilihan **Semua status (tanpa arsip)** menyembunyikan media yang diarsipkan. Pilih **Diarsipkan** di filter status untuk melihatnya.

## Lihat juga

- [Menambah media dan identitas media](02-menambah-media-dan-identitas.md)
- [Mengedit media dan status](05-mengedit-media-dan-status.md)
- [Katalog Media (tampilan Klien)](../../klien/02-katalog-media/README.md)
