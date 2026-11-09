# Ketentuan redaksi dan tautan media

> Bagian Ketentuan redaksi dan Backlink & SEO pada formulir media menentukan aturan naskah dan tautan yang berlaku di media Anda. Klien melihat aturan ini sebelum memesan, dan sistem memakainya saat menyusun serta memeriksa artikel.

**Lokasi:** menu **Daftar Media** › **Tambah Media** atau **Edit**, bagian **Ketentuan redaksi** dan **Backlink & SEO**
**Untuk:** Mitra

## Gambaran singkat

Dua bagian ini mengatur apa yang boleh dan tidak boleh ada di artikel untuk media Anda. Klien melihat ringkasannya di detail media pada katalog. Taut Pintar juga memakainya saat mencocokkan media dengan brief klien, misalnya soal jumlah backlink, masa berlaku tautan, panjang artikel, dan topik terlarang.

Bagian lain pada formulir dibahas di [Menambah media dan identitas media](02-menambah-media-dan-identitas.md) dan [Integrasi, harga, dan Secure Direct](04-integrasi-harga-dan-secure-direct.md).

## Bagian Ketentuan redaksi

Teks di bawah judul bagian: "Aturan naskah yang diterima media ini."

### Jumlah kata

Pilih panjang artikel yang diterima media Anda.

| Pilihan | Artinya |
|---|---|
| **Sesuai permintaan klien (bebas)** | Pilihan awal. Panjang artikel mengikuti permintaan klien. |
| **300–700 kata** | Artikel berkisar 300 sampai 700 kata. |
| **800–1500 kata** | Artikel berkisar 800 sampai 1500 kata. |
| **Custom…** | Anda menentukan rentang sendiri. |

Bila memilih **Custom…**, dua kolom angka muncul dengan isian awal 500 dan 1000. Rentang yang diterima adalah 50 sampai 50.000 kata. Angka maksimal tidak boleh lebih kecil dari angka minimal. Bila aturan ini dilanggar, layar menampilkan "Rentang jumlah kata harus antara 50 dan 50.000 kata." atau "Jumlah kata maksimal tidak boleh lebih kecil dari minimal."

Pilihan ini dipakai sebagai aturan panjang saat naskah disusun otomatis, dan tampil ke klien sebagai **Panjang Artikel**.

### Support embed (Video/Maps)

Geser tombol untuk memilih **Diizinkan** atau **Tidak diizinkan**. Pilihan awalnya **Tidak diizinkan**. Ikon tanda seru di sebelah judul menjelaskan: "Bila diizinkan, artikel berita bisa menyematkan video/peta interaktif."

### Topik terlarang

Daftar topik yang tidak boleh dibahas di media Anda.

1. Ketik topik di kolom, misalnya `Politik`. Kolom ini berisi contoh "Contoh: Politik, SARA, Judi…".
2. Klik **＋ Tambah** atau tekan Enter.
3. Klik tanda × pada topik untuk menghapusnya.

Maksimal 20 topik. Bila lebih, penyimpanan ditolak dengan pesan "Invalid topik_terlarang".

Penyusunan naskah otomatis menghindari topik yang Anda daftar. Klien juga melihat daftar ini di kotak **Pembatasan Gatekeeper** pada detail media, bersama pembatasan umum Tauthub.

### Label sponsor pada artikel

Pilih salah satu.

- **Tidak ada (organik)**: artikel tayang seperti artikel biasa tanpa tanda sponsor. Klien melihat "Tidak Ada (Organik)".
- **Ada label sponsor**: artikel tayang dengan tanda sponsor atau advertorial yang terlihat pembaca. Klien melihat "Ada Label Sponsor / Adv".

Pilihan ini tidak mengubah atribut `rel` pada tautan. Atribut itu diatur terpisah di bagian **Backlink & SEO**.

### Subjudul (H2/H3) dalam artikel

Pilih izin subjudul pada artikel.

| Pilihan | Artinya |
|---|---|
| **Ikut bawaan tipe media** | Media blog boleh memakai subjudul, media berita tidak. |
| **Boleh memakai subjudul** | Artikel boleh memuat subjudul, termasuk di media berita. |
| **Tanpa subjudul** | Artikel tidak memuat subjudul, termasuk di media blog. |

Klien melihat hasilnya sebagai **Subjudul (H2/H3)**, misalnya "Boleh (bawaan media blog)" atau "Tanpa subjudul (bawaan media berita)".

> **Catatan:** Saat menambah media baru, pilihan subjudul belum tersimpan. Atur lewat **Edit** setelah media dibuat. Pada media yang sudah punya pilihan **Boleh** atau **Tanpa**, memilih kembali **Ikut bawaan tipe media** belum membatalkan pilihan sebelumnya.

## Bagian Backlink & SEO

Teks di bawah judul bagian: "Ketentuan tautan keluar yang dicari tim SEO klien."

### Jenis tautan

Pilih satu dari tiga jenis. Pilihan awalnya **Dofollow**.

| Pilihan | Artinya |
|---|---|
| **Dofollow** | Tautan biasa ke situs klien. Mesin pencari ikut menghitungnya untuk SEO. |
| **Nofollow** | Tautan tetap boleh dipasang, tetapi sistem memberinya tanda nofollow otomatis. Mesin pencari tidak menghitungnya untuk SEO. Artikel tidak ditolak. |
| **rel=sponsored** | Tautan diberi atribut `rel="sponsored"`, yaitu tanda tautan berbayar menurut Google. |

Jenis tautan terpisah dari label sponsor di artikel. Media boleh organik tetapi tetap memakai `rel=sponsored`.

### Jumlah backlink yang diperbolehkan dalam artikel

Pilih batas jumlah backlink dalam satu artikel. Teks di layar: "Default 1; pilih 0 bila artikel tidak boleh memuat backlink."

Daftar pilihan menampilkan **0 — Tanpa backlink**, **1 Link (default)**, **2 Link**, dan **3 Link**.

> **Catatan:** Saat ini sistem hanya menerima **1 Link** dan **2 Link**. Bila Anda memilih 0 atau 3 lalu menyimpan, layar menampilkan galat "Invalid max_links" dan media tidak tersimpan. Pilih 1 atau 2.

Bila jumlah backlink 0 dan Anda memilih jenis tautan, sistem otomatis mengubah jumlahnya menjadi 1.

### Masa berlaku tautan (backlink)

Pilih berapa lama tautan di artikel harus tetap tayang.

- **Permanen**: pilihan awal. Teks di layar: "Tautan tetap tayang terus — tidak pernah dilepas dan tidak ada masa kedaluwarsanya."
- **Jangka Waktu**: pilih **12 Bulan**, **24 Bulan**, atau **36 Bulan** yang muncul setelah Anda memilih tab ini. Nilai lama di luar daftar, misalnya 6 bulan, tetap tampil bila media Anda sudah memakainya supaya tidak berubah diam-diam.

Layar mengingatkan bahwa klien akan memprioritaskan situs yang memberi garansi tautan tayang permanen. Klien melihat pilihan ini sebagai **Garansi tayang**.

Masa berlaku ini juga menjadi dasar masa pemantauan tautan dan masa garansi klaim di Link Guardian: selama masa itu klien dapat mengajukan klaim bila tautan hilang. Pilihan **Permanen** berarti garansi tidak pernah kedaluwarsa. Klaim hanya bisa diajukan untuk media yang domainnya sudah terverifikasi. Untuk media yang belum terverifikasi, klien melihat tulisan **Tidak Digaransi**. Lihat [Link Guardian](../05-link-guardian.md) dan [Verifikasi domain dan Dewan Pers](07-verifikasi-domain-dan-dewan-pers.md).

## Perubahan setelah media Aktif

Jumlah kata, embed, dan subjudul berlaku langsung setelah disimpan. Topik terlarang, label sponsor, jenis tautan, jumlah backlink, dan masa berlaku tautan diajukan ke tim Tauthub lebih dulu bila media Anda berstatus **Aktif**. Selama menunggu, aturan lama tetap berlaku. Penjelasan lengkap ada di [Mengedit media dan status](05-mengedit-media-dan-status.md).

## Pertanyaan yang sering diajukan

**Apa bedanya label sponsor dan jenis tautan?**
Label sponsor adalah tanda yang terlihat pembaca di artikel. Jenis tautan adalah atribut tautan untuk mesin pencari. Keduanya diatur terpisah.

**Bisakah media saya menerima artikel tanpa tautan keluar?**
Pilihan **0 — Tanpa backlink** ada di daftar, tetapi saat ini belum dapat disimpan. Lihat catatan di bagian jumlah backlink.

**Mengapa naskah media berita saya tidak memakai subjudul?**
Bawaan media berita adalah tanpa subjudul. Pilih **Boleh memakai subjudul** lewat **Edit** bila media Anda menerima subjudul.

## Lihat juga

- [Menambah media dan identitas media](02-menambah-media-dan-identitas.md)
- [Integrasi, harga, dan Secure Direct](04-integrasi-harga-dan-secure-direct.md)
- [Katalog Media (tampilan Klien)](../../klien/02-katalog-media/README.md)
