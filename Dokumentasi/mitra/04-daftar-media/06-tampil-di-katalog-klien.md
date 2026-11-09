# Tampil di katalog klien dan ikon mata

> Media Anda hanya tampil di Katalog Media klien bila memenuhi beberapa syarat sekaligus. Ikon mata di Daftar Media menunjukkan keadaannya, menjelaskan sebabnya bila tersembunyi, dan berfungsi sebagai saklar untuk menyembunyikan atau menampilkan media.

**Lokasi:** menu **Daftar Media**, ikon mata di kolom **Media** (di HP, di bagian atas kartu)
**Untuk:** Mitra

## Gambaran singkat

Klien memilih media dari **Katalog Media**. Katalog hanya berisi media yang benar-benar bisa dipesan. Setiap baris di Daftar Media punya satu ikon mata yang menunjukkan apakah media itu ikut tampil di katalog. Apa yang dilihat klien dibahas di [Katalog Media (tampilan Klien)](../../klien/02-katalog-media/README.md).

## Syarat tampil di katalog

Semua syarat berikut harus terpenuhi sekaligus.

| Syarat | Keterangan | Yang bisa Anda lakukan |
|---|---|---|
| Status **Aktif** | Media sudah disetujui tim Tauthub. | Tunggu keputusan, atau perbaiki media yang **Ditolak**. |
| Tidak ditangguhkan | Tim Tauthub dapat menangguhkan media. | Hubungi [Dukungan](../12-dukungan.md). |
| Harga lebih dari 0 | Media tanpa harga tidak tampil. | Isi harga lewat **Edit**. |
| Siap dari sisi koneksi | Media tanpa platform tujuan (mode **Manual** di formulir) selalu siap. Media dengan platform tujuan perlu koneksi aktif. | Sambungkan situs di menu **Integrasi**. |
| Tidak Anda sembunyikan | Anda bisa menyembunyikan media sendiri. | Klik ikon mata untuk menampilkannya lagi. |
| Akun Mitra aktif | Akun Anda harus berstatus aktif. | Hubungi [Dukungan](../12-dukungan.md). |

Syarat koneksi untuk media **API** bergantung pada platformnya:

- **WordPress** dan **EmDash**: ada koneksi aktif di menu **Integrasi**, dan alamat situs yang dihubungkan sama dengan **Website / Domain** pada media. Awalan `www` diabaikan.
- **Blogspot** dan **Web Custom**: koneksinya berstatus terhubung di tab yang bersangkutan.

## Arti ikon mata

| Ikon | Artinya |
|---|---|
| Mata hijau | Media tampil di katalog klien. |
| Mata abu-abu dengan garis coret | Media tidak tampil. Arahkan kursor (atau ketuk di HP) untuk membaca sebabnya. |

Klik atau ketuk ikon untuk menjalankan aksi yang sesuai dengan keadaan media. Ikon nonaktif sebentar selama aksi diproses.

## Aksi dan penjelasan tiap keadaan

| Keadaan | Teks penjelasan | Hasil klik ikon |
|---|---|---|
| Tampil | "Tampil di katalog klien" | Dialog **Sembunyikan dari katalog klien?** terbuka. |
| Anda yang menyembunyikan | "Disembunyikan dari katalog klien oleh Anda." | Media langsung tampil lagi. |
| Situs belum tersambung | "Disembunyikan dari katalog klien: situs belum tersambung ke" disertai nama platform | Halaman **Integrasi** terbuka pada tab platform yang sesuai, dengan media disorot. |
| Harga belum diisi | "Disembunyikan dari katalog klien: harga belum diisi." | Formulir **Edit** terbuka. |
| Menunggu review | "Disembunyikan dari katalog klien: media menunggu review admin, tampil otomatis setelah disetujui." | Pesan penjelasan muncul. |
| Ditolak | "Disembunyikan dari katalog klien: media ditolak admin. Perbaiki lewat Edit lalu ajukan ulang." | Pesan penjelasan muncul. |
| Diarsipkan | "Disembunyikan dari katalog klien: media diarsipkan." | Pesan penjelasan muncul. |
| Belum aktif (**Nonaktif** atau **Draf**) | "Disembunyikan dari katalog klien: media belum aktif." | Pesan penjelasan muncul. |
| Ditangguhkan | "Disembunyikan dari katalog klien: media sedang ditangguhkan admin." | Pesan penjelasan muncul. |
| Sebab lain | "Disembunyikan dari katalog klien." | Tidak ada aksi. Hubungi [Dukungan](../12-dukungan.md). |

Pada media bermode **API** yang tersembunyi karena status atau penangguhan (bukan karena harga atau pilihan Anda sendiri), klik ikon menampilkan pesan penjelasan lalu membuka halaman **Integrasi**. Dengan begitu Anda bisa menyiapkan atau memeriksa koneksinya lebih dulu.

Bila beberapa sebab berlaku bersamaan, ikon menampilkan yang pertama dalam urutan ini: status belum aktif, ditangguhkan, harga belum diisi, belum tersambung, lalu disembunyikan oleh Anda.

## Cara menyembunyikan media

1. Klik ikon mata hijau pada baris media.
2. Baca dialog **Sembunyikan dari katalog klien?**. Isinya: media tidak akan tampil dan tidak bisa dipesan klien baru, sedangkan order yang sudah berjalan tidak terpengaruh.
3. Klik **Sembunyikan**.

Pesan hijau muncul: nama media disembunyikan dari katalog klien dan order yang sudah berjalan tidak terpengaruh. Penyembunyian berlaku langsung tanpa tinjauan tim Tauthub.

Klien yang masih membuka tautan lama dan mencoba memesan melihat pesan: "Media ini sedang disembunyikan mitranya dari katalog, jadi belum bisa dipesan. Silakan pilih media lain."

## Cara menampilkan media lagi

1. Klik ikon mata abu-abu pada media yang Anda sembunyikan. Teks penjelasannya "Disembunyikan dari katalog klien oleh Anda."
2. Pesan hijau menyatakan media tampil lagi di katalog klien.

Bila ada syarat lain yang belum terpenuhi, ikon tetap abu-abu dan teks penjelasannya berganti ke sebab yang baru.

## Media yang belum tersambung

Media dengan platform tujuan yang sudah **Aktif** dan berharga, tetapi situsnya belum tersambung, tidak tampil di katalog. Dua tanda muncul untuk Anda:

- Kotak kuning di atas tabel menyebut jumlahnya: "media disembunyikan dari katalog klien · situsnya belum tersambung, jadi klien belum bisa memesan. Klik ikon mata di barisnya untuk menyambungkan."
- Tauthub mengirim satu notifikasi untuk tiap media dengan judul "Media disembunyikan dari katalog" disertai nama domain. Isinya menjelaskan bahwa situs belum tersambung dan media otomatis tampil lagi begitu tersambung.

Cara menyambungkan:

1. Klik ikon mata abu-abu pada media itu. Halaman **Integrasi** terbuka dengan media disorot.
2. Bila **Syarat & Ketentuan** belum disetujui, tab itu terbuka lebih dulu. Setujui, lalu buka tab platform Anda.
3. Ikuti langkah koneksi di [Integrasi](../13-integrasi/README.md).

Setelah koneksi aktif, media tampil kembali tanpa perlu menyimpan apa pun di Daftar Media.

## Pertanyaan yang sering diajukan

**Media saya berstatus Aktif, mengapa klien belum bisa memesannya?**
Periksa ikon mata. Teks penjelasannya menyebut syarat yang belum terpenuhi, misalnya situs belum tersambung atau harga belum diisi.

**Apakah menyembunyikan media membatalkan pesanan yang sedang berjalan?**
Tidak. Order yang sudah berjalan tidak terpengaruh. Hanya klien baru yang tidak bisa memesan.

**Bisakah saya menyembunyikan media sementara tanpa mengarsipkannya?**
Bisa. Gunakan ikon mata. Arsip dipakai bila media tidak akan dipakai lagi, dan media yang sudah diarsipkan perlu disetujui ulang saat dipulihkan.

**Mengapa media yang saya buat dengan mode Manual tidak pernah menampilkan Belum terhubung?**
Media itu tidak punya platform tujuan, sehingga tidak membutuhkan koneksi dan selalu dianggap siap dari sisi koneksi. Media hasil impor XLSX bisa berbeda. Lihat [Impor XLSX](08-impor-xlsx.md).

## Lihat juga

- [Integrasi, harga, dan Secure Direct](04-integrasi-harga-dan-secure-direct.md)
- [Mengedit media dan status](05-mengedit-media-dan-status.md)
- [Integrasi](../13-integrasi/README.md)
