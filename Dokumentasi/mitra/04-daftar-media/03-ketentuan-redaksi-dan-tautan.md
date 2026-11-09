# Aturan naskah dan tautan media

> Bagian Ketentuan redaksi dan Backlink & SEO di formulir media menentukan aturan naskah dan tautan untuk media Anda. Klien melihat aturan ini sebelum memesan. Sistem memakainya saat menyusun dan memeriksa artikel.

**Lokasi:** menu **Daftar Media** › **Tambah Media** atau **Edit**, bagian **Ketentuan redaksi** dan **Backlink & SEO**
**Untuk:** Mitra

## Gambaran singkat

Dua bagian ini mengatur apa yang boleh dan tidak boleh ada di artikel untuk media Anda. Klien melihat ringkasannya di detail media pada katalog. Taut Pintar (fitur pencocok media) juga memakainya saat mencocokkan media dengan brief klien. Brief adalah catatan permintaan klien tentang artikelnya. Yang dicocokkan misalnya jumlah backlink, lama tautan berlaku, panjang artikel, dan topik terlarang. Backlink adalah tautan di artikel yang menuju situs klien.

Bagian lain di formulir dibahas di [Menambah media dan identitas media](02-menambah-media-dan-identitas.md) dan [Integrasi, harga, dan Secure Direct](04-integrasi-harga-dan-secure-direct.md).

## Bagian Ketentuan redaksi

Teks di bawah judul bagian berbunyi: "Aturan naskah yang diterima media ini."

### Jumlah kata

Pilih panjang artikel yang diterima media Anda.

| Pilihan | Artinya |
|---|---|
| **Sesuai permintaan klien (bebas)** | Pilihan awal. Panjang artikel mengikuti permintaan klien. |
| **300–700 kata** | Artikel berkisar 300 sampai 700 kata. |
| **800–1500 kata** | Artikel berkisar 800 sampai 1500 kata. |
| **Custom…** | Anda menentukan rentang sendiri. |

Kalau Anda memilih **Custom…**, dua kotak angka muncul dengan isian awal 500 dan 1000. Rentang yang diterima adalah 50 sampai 50.000 kata. Angka terbesar tidak boleh lebih kecil dari angka terkecil. Kalau Anda melanggar aturan ini, layar menampilkan "Rentang jumlah kata harus antara 50 dan 50.000 kata." atau "Jumlah kata maksimal tidak boleh lebih kecil dari minimal."

Pilihan ini menjadi aturan panjang saat naskah disusun otomatis. Klien melihatnya sebagai **Panjang Artikel**.

### Support embed (Video/Maps)

Geser tombol untuk memilih **Diizinkan** atau **Tidak diizinkan**. Pilihan awalnya **Tidak diizinkan**. Ikon tanda seru di sebelah judul menjelaskan: "Bila diizinkan, artikel berita bisa menyematkan video/peta interaktif."

### Topik terlarang

Ini daftar topik yang tidak boleh dibahas di media Anda.

1. Ketik topik di kotak, misalnya `Politik`. Kotak ini berisi contoh "Contoh: Politik, SARA, Judi…".
2. Klik **＋ Tambah** atau tekan Enter.
3. Klik tanda × pada topik untuk menghapusnya.

Anda boleh mengisi paling banyak 20 topik. Kalau lebih, penyimpanan ditolak dengan pesan "Invalid topik_terlarang".

Penyusunan naskah otomatis menghindari topik yang Anda daftarkan. Klien juga melihat daftar ini di kotak **Pembatasan Gatekeeper** pada detail media, bersama pembatasan umum Tauthub.

### Label sponsor pada artikel

Pilih salah satu.

- **Tidak ada (organik)**: artikel tayang seperti artikel biasa, tanpa tanda sponsor. Klien melihat "Tidak Ada (Organik)".
- **Ada label sponsor**: artikel tayang dengan tanda sponsor atau iklan advertorial yang terlihat pembaca. Klien melihat "Ada Label Sponsor / Adv".

Pilihan ini tidak mengubah atribut `rel` pada tautan. Atribut `rel` adalah tanda tersembunyi di tautan yang memberi tahu mesin pencari jenis tautan itu. Anda mengaturnya terpisah di bagian **Backlink & SEO**.

### Subjudul (H2/H3) dalam artikel

Pilih izin subjudul di artikel.

| Pilihan | Artinya |
|---|---|
| **Ikut bawaan tipe media** | Media blog boleh memakai subjudul, media berita tidak. |
| **Boleh memakai subjudul** | Artikel boleh berisi subjudul, termasuk di media berita. |
| **Tanpa subjudul** | Artikel tidak berisi subjudul, termasuk di media blog. |

Klien melihat hasilnya sebagai **Subjudul (H2/H3)**, misalnya "Boleh (bawaan media blog)" atau "Tanpa subjudul (bawaan media berita)".

> **Catatan:** Saat Anda menambah media baru, pilihan subjudul belum tersimpan. Atur lewat **Edit** setelah media dibuat. Pada media yang sudah punya pilihan **Boleh** atau **Tanpa**, memilih lagi **Ikut bawaan tipe media** belum membatalkan pilihan sebelumnya.

## Bagian Backlink & SEO

Teks di bawah judul bagian berbunyi: "Ketentuan tautan keluar yang dicari tim SEO klien." SEO adalah usaha agar situs mudah ditemukan di mesin pencari seperti Google.

### Jenis tautan

Pilih satu dari tiga jenis. Pilihan awalnya **Dofollow**.

| Pilihan | Artinya |
|---|---|
| **Dofollow** | Tautan biasa ke situs klien. Mesin pencari ikut menghitungnya untuk SEO. |
| **Nofollow** | Anda tetap boleh memasang tautan, tetapi sistem memberinya tanda nofollow otomatis. Mesin pencari tidak menghitungnya untuk SEO. Artikel tidak ditolak. |
| **rel=sponsored** | Tautan diberi atribut `rel="sponsored"`. Menurut Google, ini tanda tautan berbayar. |

Jenis tautan terpisah dari label sponsor di artikel. Media boleh organik, tetapi tetap memakai `rel=sponsored`.

### Jumlah backlink yang diperbolehkan dalam artikel

Pilih batas jumlah backlink dalam satu artikel. Teks di layar berbunyi: "Default 1; pilih 0 bila artikel tidak boleh memuat backlink."

Daftar pilihan menampilkan **0 — Tanpa backlink**, **1 Link (default)**, **2 Link**, dan **3 Link**.

> **Catatan:** Saat ini sistem hanya menerima **1 Link** dan **2 Link**. Kalau Anda memilih 0 atau 3 lalu menyimpan, layar menampilkan kesalahan "Invalid max_links" dan media tidak tersimpan. Pilih 1 atau 2.

Kalau jumlah backlink 0 dan Anda memilih jenis tautan, sistem otomatis mengubah jumlahnya menjadi 1.

### Masa berlaku tautan (backlink)

Pilih berapa lama tautan di artikel harus tetap tayang.

- **Permanen**: pilihan awal. Teks di layar berbunyi: "Tautan tetap tayang terus, tidak pernah dilepas dan tidak ada masa kedaluwarsanya."
- **Jangka Waktu**: setelah Anda memilih tab ini, muncul pilihan **12 Bulan**, **24 Bulan**, atau **36 Bulan**. Nilai lama di luar daftar, misalnya 6 bulan, tetap tampil kalau media Anda sudah memakainya. Tujuannya supaya nilainya tidak berubah diam-diam.

Layar mengingatkan bahwa klien akan mengutamakan situs yang memberi garansi tautan tayang permanen. Klien melihat pilihan ini sebagai **Garansi tayang**.

Masa berlaku ini juga menjadi dasar masa pemantauan tautan dan masa garansi klaim di Link Guardian. Selama masa itu, klien bisa mengajukan klaim kalau tautan hilang. Pilihan **Permanen** berarti garansi tidak pernah kedaluwarsa. Klien hanya bisa mengajukan klaim untuk media yang domainnya sudah terverifikasi. Untuk media yang belum terverifikasi, klien melihat tulisan **Tidak Digaransi**. Lihat [Link Guardian](../05-link-guardian.md) dan [Verifikasi domain dan Dewan Pers](07-verifikasi-domain-dan-dewan-pers.md).

## Perubahan setelah media Aktif

Jumlah kata, embed, dan subjudul langsung berlaku setelah Anda menyimpan. Topik terlarang, label sponsor, jenis tautan, jumlah backlink, dan masa berlaku tautan harus diajukan dulu ke tim Tauthub kalau media Anda berstatus **Aktif**. Selama menunggu, aturan lama tetap berlaku. Penjelasan lengkap ada di [Mengedit media dan status](05-mengedit-media-dan-status.md).

## Pertanyaan yang sering diajukan

**Apa bedanya label sponsor dan jenis tautan?**
Label sponsor adalah tanda yang terlihat pembaca di artikel. Jenis tautan adalah tanda di tautan untuk mesin pencari. Anda mengaturnya terpisah.

**Bisakah media saya menerima artikel tanpa tautan keluar?**
Pilihan **0 — Tanpa backlink** ada di daftar, tetapi saat ini belum bisa disimpan. Lihat catatan di bagian jumlah backlink.

**Mengapa naskah media berita saya tidak memakai subjudul?**
Bawaan media berita adalah tanpa subjudul. Kalau media Anda menerima subjudul, pilih **Boleh memakai subjudul** lewat **Edit**.

## Lihat juga

- [Menambah media dan identitas media](02-menambah-media-dan-identitas.md)
- [Integrasi, harga, dan Secure Direct](04-integrasi-harga-dan-secure-direct.md)
- [Katalog Media (tampilan Klien)](../../klien/02-katalog-media/README.md)
