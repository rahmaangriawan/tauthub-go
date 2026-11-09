# Tampil di katalog klien dan ikon mata

> Media Anda hanya terlihat di Katalog Media klien kalau beberapa syarat terpenuhi sekaligus. Ikon mata di Daftar Media menunjukkan keadaannya dan menjelaskan sebabnya kalau media tersembunyi. Ikon ini juga berfungsi sebagai tombol untuk menyembunyikan atau menampilkan media.

**Lokasi:** menu **Daftar Media**, ikon mata di kolom **Media** (di HP, di bagian atas kartu)
**Untuk:** Mitra

## Gambaran singkat

Klien memilih media dari **Katalog Media**. Katalog hanya berisi media yang benar-benar bisa dipesan. Setiap baris di Daftar Media punya satu ikon mata. Ikon itu menunjukkan apakah media ikut tampil di katalog. Apa yang dilihat klien ada di [Katalog Media (tampilan Klien)](../../klien/02-katalog-media/README.md).

## Syarat tampil di katalog

Semua syarat berikut harus terpenuhi sekaligus.

| Syarat | Keterangan | Yang bisa Anda lakukan |
|---|---|---|
| Status **Aktif** | Tim Tauthub sudah menyetujui media. | Tunggu keputusan, atau perbaiki media yang **Ditolak**. |
| Tidak ditangguhkan | Tim Tauthub bisa menangguhkan media. | Hubungi [Dukungan](../12-dukungan.md). |
| Harga lebih dari 0 | Media tanpa harga tidak tampil. | Isi harga lewat **Edit**. |
| Sambungan siap | Media tanpa platform tujuan (mode **Manual** di formulir) selalu siap. Media dengan platform tujuan perlu sambungan yang aktif. | Sambungkan situs di menu **Integrasi**. |
| Tidak Anda sembunyikan | Anda bisa menyembunyikan media sendiri. | Klik ikon mata untuk menampilkannya lagi. |
| Akun Mitra aktif | Akun Anda harus berstatus aktif. | Hubungi [Dukungan](../12-dukungan.md). |

Syarat sambungan untuk media **API** bergantung pada platformnya:

- **WordPress** dan **EmDash**: ada sambungan aktif di menu **Integrasi**, dan alamat situs yang tersambung sama dengan **Website / Domain** pada media. Awalan `www` diabaikan.
- **Blogspot** dan **Web Custom**: sambungannya berstatus terhubung di tab yang bersangkutan.

## Arti ikon mata

| Ikon | Artinya |
|---|---|
| Mata hijau | Media tampil di katalog klien. |
| Mata abu-abu dengan garis coret | Media tidak tampil. Arahkan kursor (atau ketuk di HP) untuk membaca sebabnya. |

Klik atau ketuk ikon untuk menjalankan aksi yang sesuai dengan keadaan media. Ikon tidak bisa diklik sebentar selama aksi berjalan.

## Aksi dan penjelasan tiap keadaan

| Keadaan | Teks penjelasan | Hasil klik ikon |
|---|---|---|
| Tampil | "Tampil di katalog klien" | Kotak **Sembunyikan dari katalog klien?** terbuka. |
| Anda yang menyembunyikan | "Disembunyikan dari katalog klien oleh Anda." | Media langsung tampil lagi. |
| Situs belum tersambung | "Disembunyikan dari katalog klien: situs belum tersambung ke" disertai nama platform | Halaman **Integrasi** terbuka pada tab platform yang sesuai, dengan media disorot. |
| Harga belum diisi | "Disembunyikan dari katalog klien: harga belum diisi." | Formulir **Edit** terbuka. |
| Menunggu review | "Disembunyikan dari katalog klien: media menunggu review admin, tampil otomatis setelah disetujui." | Pesan penjelasan muncul. |
| Ditolak | "Disembunyikan dari katalog klien: media ditolak admin. Perbaiki lewat Edit lalu ajukan ulang." | Pesan penjelasan muncul. |
| Diarsipkan | "Disembunyikan dari katalog klien: media diarsipkan." | Pesan penjelasan muncul. |
| Belum aktif (**Nonaktif** atau **Draf**) | "Disembunyikan dari katalog klien: media belum aktif." | Pesan penjelasan muncul. |
| Ditangguhkan | "Disembunyikan dari katalog klien: media sedang ditangguhkan admin." | Pesan penjelasan muncul. |
| Sebab lain | "Disembunyikan dari katalog klien." | Tidak ada aksi. Hubungi [Dukungan](../12-dukungan.md). |

Ada media bermode **API** yang tersembunyi karena status atau penangguhan (bukan karena harga atau pilihan Anda sendiri). Kalau Anda mengklik ikon pada media seperti itu, pesan penjelasan muncul, lalu halaman **Integrasi** terbuka. Dengan begitu Anda bisa menyiapkan atau memeriksa sambungannya lebih dulu.

Kalau beberapa sebab berlaku bersamaan, ikon menampilkan yang pertama dalam urutan ini: status belum aktif, ditangguhkan, harga belum diisi, belum tersambung, lalu disembunyikan oleh Anda.

## Cara menyembunyikan media

1. Klik ikon mata hijau pada baris media.
2. Baca kotak **Sembunyikan dari katalog klien?**. Isinya: media tidak akan tampil dan tidak bisa dipesan klien baru, sedangkan order yang sudah berjalan tidak terpengaruh.
3. Klik **Sembunyikan**.

Pesan hijau muncul: nama media disembunyikan dari katalog klien dan order yang sudah berjalan tidak terpengaruh. Penyembunyian langsung berlaku tanpa pemeriksaan tim Tauthub.

Klien yang masih membuka tautan lama dan mencoba memesan melihat pesan: "Media ini sedang disembunyikan mitranya dari katalog, jadi belum bisa dipesan. Silakan pilih media lain."

## Cara menampilkan media lagi

1. Klik ikon mata abu-abu pada media yang Anda sembunyikan. Teks penjelasannya "Disembunyikan dari katalog klien oleh Anda."
2. Pesan hijau menyatakan media tampil lagi di katalog klien.

Kalau masih ada syarat lain yang belum terpenuhi, ikon tetap abu-abu dan teks penjelasannya berganti ke sebab yang baru.

## Media yang belum tersambung

Ada media dengan platform tujuan yang sudah **Aktif** dan berharga, tetapi situsnya belum tersambung. Media seperti itu tidak tampil di katalog. Dua tanda muncul untuk Anda:

- Kotak kuning di atas tabel menyebut jumlahnya: "media disembunyikan dari katalog klien · situsnya belum tersambung, jadi klien belum bisa memesan. Klik ikon mata di barisnya untuk menyambungkan."
- Tauthub mengirim satu notifikasi untuk tiap media, dengan judul "Media disembunyikan dari katalog" disertai nama domain. Isinya menjelaskan bahwa situs belum tersambung dan media otomatis tampil lagi begitu tersambung.

Cara menyambungkan:

1. Klik ikon mata abu-abu pada media itu. Halaman **Integrasi** terbuka dengan media disorot.
2. Kalau **Syarat & Ketentuan** belum Anda setujui, tab itu terbuka lebih dulu. Setujui, lalu buka tab platform Anda.
3. Ikuti langkah sambungan di [Integrasi](../13-integrasi/README.md).

Setelah sambungan aktif, media tampil kembali. Anda tidak perlu menyimpan apa pun di Daftar Media.

## Pertanyaan yang sering diajukan

**Media saya berstatus Aktif, mengapa klien belum bisa memesannya?**
Periksa ikon mata. Teks penjelasannya menyebut syarat yang belum terpenuhi, misalnya situs belum tersambung atau harga belum diisi.

**Apakah menyembunyikan media membatalkan pesanan yang sedang berjalan?**
Tidak. Order yang sudah berjalan tidak terpengaruh. Hanya klien baru yang tidak bisa memesan.

**Bisakah saya menyembunyikan media sementara tanpa mengarsipkannya?**
Bisa. Pakai ikon mata. Arsip dipakai kalau media tidak akan dipakai lagi. Media yang sudah diarsipkan harus disetujui ulang saat dipulihkan.

**Mengapa media yang saya buat dengan mode Manual tidak pernah menampilkan Belum terhubung?**
Media itu tidak punya platform tujuan, jadi tidak butuh sambungan dan selalu dianggap siap. Media hasil impor XLSX bisa berbeda. Lihat [Impor XLSX](08-impor-xlsx.md).

## Lihat juga

- [Integrasi, harga, dan Secure Direct](04-integrasi-harga-dan-secure-direct.md)
- [Mengedit media dan status](05-mengedit-media-dan-status.md)
- [Integrasi](../13-integrasi/README.md)
