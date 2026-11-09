# Akses Crawler

> Akses Crawler memberi website media Anda kunci khusus (kredensial) supaya crawler Tauthub bisa memeriksa halaman publik tanpa diblokir firewall (WAF). Crawler adalah program Tauthub yang membuka situs Anda sendiri. Di sini Anda menyetujui dokumen akses, membuat, memutar, menonaktifkan, atau mencabut kredensial per media, serta menguji sambungannya.

**Lokasi:** menu **Akses Crawler** (grup **MEDIA & PUBLIKASI**)
**Untuk:** Mitra

## Gambaran singkat

Tauthub memeriksa website media secara otomatis (crawling) untuk menilai mutu konten dan kecepatan website. Hasilnya membuat media Anda tampil lengkap dan terpercaya di Katalog Media. Hasil itu juga menjadi dasar penerbitan artikel Klien secara otomatis.

Sebagian situs memakai firewall, misalnya Cloudflare, yaitu penjaga yang menolak pengunjung otomatis. Kalau crawler ditolak, pemeriksaan gagal. Kredensial di halaman ini mengatasinya. Crawler menandatangani setiap permintaannya, seperti membubuhkan cap rahasia. Sebuah Cloudflare Worker (program kecil yang berjalan di Cloudflare) di situs Anda mengenali tanda tangan itu, lalu membiarkan permintaan lewat. Tanpa tanda tangan, firewall Anda tetap berlaku untuk semua orang.

Anda tidak wajib membuat kredensial. Jalankan **Uji Koneksi** lebih dulu. Kalau hasilnya **Normal**, situs Anda tidak diblokir dan kredensial tidak diperlukan.

## Isi dokumen persetujuan

Dokumen persetujuan (versi 2) tampil di layar. Ringkasannya sebagai berikut.

- **Tujuan:** menilai mutu konten dan kecepatan website, serta mendukung penerbitan artikel Klien secara otomatis.
- **Yang diakses:** halaman publik website seperti pengunjung biasa, ditambah data teknis dasar seperti kecepatan, susunan halaman, dan ketersediaan konten.
- **Kapan:** jendela crawl harian pukul 00.00 sampai 04.00 WIB, saat pengunjung paling sepi. Di luar jam itu crawler tidak menjalankan pemeriksaan.
- **Yang tidak pernah diakses:** panel admin, akun, atau bagian yang butuh login; data pengunjung atau data pribadi; pengaturan keamanan situs. Cloudflare atau WAF tetap aktif untuk semua orang.
- **Keamanan kredensial:** tiap website mendapat kredensial berbeda yang hanya berlaku untuk website itu. Setiap permintaan crawler ditandatangani dan kedaluwarsa dalam hitungan menit. Anda bisa menonaktifkan akses kapan saja tanpa menghubungi siapa pun.
- **Persetujuan:** dengan membuat kredensial, Anda menyetujui semua poin di atas. Tauthub mencatat persetujuan bersama waktu dan versi dokumennya.

## Yang tampil di layar

- **Pilih media…:** daftar semua media Anda. Setelah Anda memilih satu media, tombol **Uji Koneksi** muncul di sebelahnya.
- **Kartu hasil uji:** tampil setelah **Uji Koneksi**. Isinya "Hasil uji: Berhasil" atau "Hasil uji: Gagal", penilaian (verdict), pesan, dan petunjuk berwarna kuning kalau ada.
- **Kartu kredensial baru:** tampil sekali setelah **Buat Kredensial** atau **Rotasi Secret**. Setelah **Buat Kredensial**, isinya **Site ID**, secret, langkah pemasangan di Cloudflare, dan kode Worker. Setelah **Rotasi Secret**, kartu hanya berisi **Site ID** dan secret baru. Site ID adalah nomor pengenal kunci Anda. Secret adalah kata sandi rahasia kunci itu.
- **Kartu Kredensial Crawler:** berisi keterangan **Jam crawl Tauthub: 00.00 – 04.00 WIB** setiap hari. Kalau media belum punya kredensial, kartu menampilkan kotak persetujuan dan tombol **Buat Kredensial**. Kalau sudah punya, kartu menampilkan data berikut dan tombol **Rotasi Secret**, **Nonaktifkan**, dan **Cabut**.

| Data kredensial | Arti |
|---|---|
| **Site ID** | Pengenal kredensial untuk media ini |
| **Status** | Keadaan kredensial: `active` (aktif), `disabled` (dinonaktifkan), atau `revoked` (dicabut) |
| **Domain** | Domain media (alamat situs) yang dicakup kredensial |
| **Fingerprint** | Kode pendek untuk membedakan versi secret. Ini bukan secret itu sendiri |
| **Dibuat**, **Rotasi terakhir** | Waktu kredensial dibuat dan terakhir diputar |
| **Crawl sukses terakhir** | Waktu crawl terakhir yang berhasil |
| **Gagal (kumulatif)** | Jumlah kegagalan crawl yang tercatat untuk kredensial ini |

- **Dokumen persetujuan (versi 2):** selalu tampil di bawah kartu, jadi Anda bisa membacanya kapan saja.
- **Diagnosis Terakhir:** sampai 10 hasil pemeriksaan terakhir. Tiap hasil berisi penilaian, kode HTTP (angka jawaban dari situs), dan waktu. Daftar ini bisa kosong pada media yang belum pernah punya kredensial, walau Anda sudah menjalankan **Uji Koneksi**. Hasil ujinya tetap tampil di kartu hasil uji.

## Cara menggunakan

### Cara memeriksa apakah website Anda diblokir

1. Klik menu **Akses Crawler**.
2. Pilih media pada **Pilih media…**.
3. Klik **Uji Koneksi**.
4. Baca kartu hasil uji. Artinya dijelaskan di bagian **Hasil uji dan artinya** di bawah.

### Cara membuat kredensial

1. Pilih media pada **Pilih media…**. Media harus punya alamat website. Kalau belum, isi dulu di [Daftar Media](04-daftar-media/README.md).
2. Baca **Dokumen persetujuan (versi 2)** di kartu **Kredensial Crawler**.
3. Centang kotak persetujuan: "Saya menyetujui dokumen akses crawler: Tauthub hanya mengakses halaman publik website pada jam crawl 00.00 – 04.00 WIB, tidak pernah panel admin/akun, dan dapat dinonaktifkan kapan saja."
4. Klik **Buat Kredensial**. Tombol ini tidak bisa diklik sampai kotak persetujuan dicentang.
5. Salin **Site ID** dan secret dengan tombol salin di sampingnya. Layar menampilkan pesan "Secret hanya ditampilkan sekali. Salin ke Worker Secret Cloudflare."
6. Simpan keduanya di tempat aman sebelum Anda klik **Tutup**. Setelah kartu ditutup, Anda tidak bisa melihat secret lagi.

### Cara memasang kredensial di Cloudflare

Langkah ini muncul di kartu kredensial baru dengan judul "Langkah 2: Pasang di Cloudflare (sekali saja)".

1. Buka dashboard Cloudflare, pilih **Workers & Pages**, lalu klik **Create Worker**.
2. Hapus isi editor, lalu tempel kode Worker. Klik **Lihat kode Cloudflare Worker** dan **Salin Worker** untuk mengambilnya.
3. Buka **Settings** › **Variables**, lalu tambah variabel `TAUTHUB_SITE_ID` berisi Site ID Anda.
4. Tambah variabel `TAUTHUB_SECRET` berisi secret Anda, dan pilih jenis **Secret**.
5. Klik **Deploy**.
6. Buka **Settings** › **Domains & Routes**, klik **Add Route**, lalu isi pola berupa domain media Anda diikuti `/*`.
7. Kembali ke Tauthub dan jalankan **Uji Koneksi** untuk memastikan Worker menerima kredensial.

Kode Worker yang disediakan hanya untuk Cloudflare. Kalau situs Anda memakai firewall lain, hubungi [Dukungan](12-dukungan.md).

### Cara memutar secret

Memutar secret artinya mengganti secret lama dengan yang baru.

1. Pilih media yang sudah punya kredensial.
2. Klik **Rotasi Secret**.
3. Salin secret baru yang tampil di kartu. Secret ini juga hanya muncul sekali.
4. Ganti nilai `TAUTHUB_SECRET` di Worker Anda dengan secret baru.
5. Jalankan **Uji Koneksi**.

Setelah rotasi, Tauthub memakai secret baru. Kode Worker mendukung variabel `TAUTHUB_SECRET_PREVIOUS` (boleh diisi atau tidak) kalau Anda ingin menerima secret lama untuk sementara waktu.

### Cara menonaktifkan atau mencabut akses

1. Pilih media yang sudah punya kredensial.
2. Klik **Nonaktifkan** untuk menghentikan akses sementara, atau **Cabut** untuk mencabutnya. Kedua tombol langsung bekerja, tanpa konfirmasi.
3. Kalau Anda tidak lagi memakai Worker dan route di Cloudflare, hapus keduanya. Tauthub tidak mengubah pengaturan Cloudflare Anda.

> **Penting:** setelah **Nonaktifkan** atau **Cabut**, kartu tetap menampilkan kredensial lama dengan status `disabled` atau `revoked`. Tombol **Buat Kredensial** tidak muncul lagi untuk media itu. Kalau Anda perlu mengaktifkannya kembali, hubungi [Dukungan](12-dukungan.md). Tombol **Nonaktifkan** hanya tampil saat status `active`.

## Hasil uji dan artinya

**Uji Koneksi** membuka halaman utama website Anda seperti peramban. Kalau berhasil, ujian selesai. Kalau terblokir dan media sudah punya kredensial, Tauthub mencoba sekali lagi dengan permintaan bertanda tangan untuk memastikan Worker Anda menerimanya.

| Pesan | Artinya | Langkah Anda berikutnya |
|---|---|---|
| "Website dapat diakses crawler tanpa kendala (tidak terblokir)." | Situs terbuka normal. | Tidak perlu kredensial. |
| "Website terblokir dan belum ada kredensial crawler. Klik "Buat Kredensial" lalu ikuti langkah berikutnya." | Situs menolak crawler dan media belum punya kredensial. | Buat kredensial dan pasang di Cloudflare. |
| "Kredensial diterima worker Anda. Crawler bisa melewati proteksi. (Tanpa signature tetap diblokir, sesuai desain.)" | Worker mengenali tanda tangan crawler. Penilaian ditulis `worker_active`. | Tidak perlu tindakan. |
| "Kredensial belum diterima worker. Periksa variabel Worker." | Worker menjawab 401. Petunjuk kuning menyebut variabel yang perlu dicek. | Periksa `TAUTHUB_SITE_ID` dan `TAUTHUB_SECRET`, dan pastikan Worker sudah di-route ke domain ini. |
| "Kredensial terkirim tapi masih terblokir. Pastikan Worker sudah di-deploy dan route-nya mencakup domain ini." | Situs masih menolak permintaan bertanda tangan. | Deploy Worker dan periksa pola route. |
| "URL website tidak valid." | Alamat website media tidak terbaca. | Perbaiki alamat website di Daftar Media. |

Penilaian (verdict) yang bisa tampil:

| Penilaian | Artinya |
|---|---|
| **Normal** | Halaman menjawab sukses (kode HTTP 2xx). |
| **Diblokir Cloudflare** | Cloudflare menahan permintaan dengan tantangan atau penahanan lain. |
| **Diblokir WAF** | Firewall menjawab 403 dan jejak Cloudflare terlihat. |
| **Diblokir origin** | Server situs menolak, tidak menjawab, atau menjawab kode lain yang gagal. |
| **Rate limited** | Situs menjawab 429, artinya terlalu banyak permintaan. |
| **Signature ditolak** | Situs tetap menolak permintaan bertanda tangan. |

## Aturan dan batasan

- Satu media punya satu kredensial aktif. Site ID dan secret berbeda untuk tiap media, dan hanya berlaku untuk domain media itu.
- Secret hanya tampil sekali, yaitu saat kredensial dibuat atau diputar. Kalau hilang, putar secret dengan **Rotasi Secret**.
- Jam crawl di dokumen persetujuan adalah 00.00 sampai 04.00 WIB setiap hari.
- Tauthub mencatat persetujuan bersama versi dokumen. Kalau teks dokumen berubah di masa depan, versinya naik.
- Kalau pemeriksaan domain otomatis menemukan situs Anda diblokir firewall, atau gagal diakses tiga kali berturut-turut, Anda menerima notifikasi berjudul "Situs … tidak bisa diakses crawler Tauthub". Notifikasi ini memberi dua jalan keluar. Pertama, izinkan crawler di firewall atau Cloudflare Anda. Kedua, verifikasi kepemilikan domain lewat meta tag atau catatan TXT DNS di [Daftar Media](04-daftar-media/README.md). Tauthub mengirim notifikasi ini paling sering sekali per media setiap 30 hari.
- Kalau pesan kesalahan menyebut konfigurasi server, masalahnya ada di sisi Tauthub. Hubungi [Dukungan](12-dukungan.md).

## Pertanyaan yang sering diajukan

**Apakah saya wajib membuat kredensial?**
Tidak. Kalau **Uji Koneksi** menunjukkan **Normal**, crawler sudah bisa membuka situs Anda.

**Apakah crawler masuk ke area login atau data pengunjung?**
Tidak. Dokumen persetujuan menyatakan crawler hanya mengakses halaman publik, sama seperti pengunjung biasa.

**Apa bedanya Nonaktifkan dan Cabut?**
Keduanya menghentikan akses. **Cabut** juga mencatat waktu pencabutan. Dari halaman ini, keduanya tidak bisa dibatalkan.

**Mengapa tombol Buat Kredensial tidak bisa diklik?**
Kotak persetujuan belum dicentang, atau ada proses lain yang masih berjalan. Centang kotaknya, lalu tunggu proses selesai.

**Mengapa muncul pesan bahwa media belum punya alamat website?**
Media yang Anda pilih belum punya alamat website, jadi kredensial belum bisa dibuat. Isi alamatnya di Daftar Media, lalu coba lagi.

## Lihat juga

- [Link Guardian](05-link-guardian.md): kesehatan domain dan artikel media Anda.
- [Daftar Media](04-daftar-media/README.md): alamat website dan verifikasi domain.
- [SEO & Performa](07-seo-dan-performa.md): analisis kecepatan website.
- [Dukungan](12-dukungan.md)
