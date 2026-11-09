# Membaca tabel dan metrik media

> Setiap baris di Katalog Media merangkum satu media: nama, kategori, trafik, DA, skor AI Trust, kecepatan terbit, dan harga. Halaman ini menjelaskan arti setiap kolom, lencana, dan angka agar Anda bisa membandingkan media dengan tepat.

**Lokasi:** menu **RUANG KERJA** › **Katalog Media**
**Untuk:** Klien

## Gambaran singkat

Tabel memuat sembilan kolom di layar lebar. Klik bagian mana pun pada sebuah baris untuk membuka [panel detail media](03-detail-media.md). Kotak centang, tombol bookmark, dan tautan **Sample** punya fungsi sendiri dan tidak membuka panel detail.

Arahkan kursor ke ikon info pada judul kolom **Trafik (Est)**, **DA**, **GEO**, dan **SLA** untuk membaca penjelasan singkat dari Tauthub.

## Yang tampil di layar

| Kolom | Isi | Bisa diurutkan |
|---|---|---|
| Kotak centang | Memilih media untuk kampanye | Tidak |
| **Nama Media** | Domain media, lencana verifikasi, dan keterangan tambahan | Ya |
| **Kategori** | Kategori media | Tidak |
| **Trafik (Est)** | Perkiraan kunjungan bulanan | Ya |
| **DA** | Domain Authority | Ya |
| **GEO** | Skor AI Trust | Ya |
| **SLA** | Rata-rata waktu terbit | Ya |
| **Harga** | Harga satu publikasi | Ya |
| **Aksi** | Tombol bookmark dan **Lihat Detail** | Tidak |

### Kolom Nama Media

Nama yang tampil adalah domain media dengan huruf pertama dibesarkan, misalnya "Contoh.id". Di sebelah kirinya ada ikon situs. Klik nama itu untuk membuka panel detail.

Di sebelah nama bisa muncul penanda berikut:

| Penanda | Artinya |
|---|---|
| Tanda centang biru | **Terverifikasi Tauthub**. Tauthub sudah memverifikasi kepemilikan domain media |
| Ikon perisai | **Terverifikasi Dewan Pers** |
| **Root** | Artikel tayang di halaman utama (root domain) |
| **Subdomain** | Artikel tayang di subdomain, bukan halaman utama domain. Audiensnya bisa berbeda |

Di bawah nama, bila datanya ada, tampil baris keterangan:

- **Sample**: tautan ke contoh artikel yang pernah diterbitkan media itu. Tautan terbuka di tab baru.
- **Kebijakan tautan**: lencana dengan ikon tautan, misalnya **2 Dofollow • Permanen**. Penjelasannya ada di bagian berikutnya.
- **Jumlah distribusi**: tulisan seperti **12 artikel**, atau **Belum ada distribusi**. Angkanya menghitung artikel yang sudah terbit lewat media ini di seluruh Tauthub, bukan hanya milik brand Anda.

#### Membaca lencana kebijakan tautan

Lencana kebijakan tautan memakai pola **jumlah jenis • masa**:

- **Jumlah** adalah batas tautan ke situs luar per artikel.
- **Jenis** adalah **Dofollow**, **Nofollow**, atau **Sponsored**. Secara umum, dofollow adalah tautan biasa yang bisa meneruskan nilai SEO, nofollow meminta mesin pencari tidak meneruskannya, dan sponsored menandai tautan berbayar.
- **Masa** adalah lama media menjamin tautan tetap ada: **Permanen**, atau jumlah bulan seperti **12 Bln SLA**.

Bentuk lain yang mungkin tampil:

| Lencana | Artinya |
|---|---|
| **Tanpa backlink** | Media tidak menerima tautan ke situs luar |
| **Backlink • Permanen** | Media menerima tautan, tetapi jumlah batasnya belum ditetapkan |

Arahkan kursor ke lencana untuk membaca kalimat lengkapnya, misalnya bahwa media menjamin tautan selama 12 bulan sejak artikel tayang. Masa jaminan ini juga menjadi acuan pemantauan oleh [Link Guardian](../06-link-guardian.md).

### Kolom Kategori

Kolom ini menampilkan kategori pertama media. Bila media punya lebih dari satu kategori, muncul tombol kecil seperti **+2**. Klik tombol itu untuk membuka daftar **Semua Kategori**, atau arahkan kursor untuk melihat semua nama kategori.

### Kolom Trafik (Est)

Kolom ini menunjukkan perkiraan kunjungan bulanan situs media. Contoh tampilannya: **± 2M / bulan**, **± 350 rb / bulan**, atau **± 850 / bulan**. Di bawah angka tertulis **Via SimilarWeb**.

Menurut keterangan pada ikon info, angka berasal dari Similarweb bila tersedia. Bila tidak, angka berasal dari estimasi internal media. Angka **0** berarti belum ada data trafik.

### Kolom DA

**DA** (Domain Authority) adalah skor 0 sampai 100 yang menggambarkan seberapa kuat dan berwibawa sebuah domain di mata mesin pencari. Makin tinggi skornya, makin besar peluang artikel ikut terangkat di Google.

Tanda **—** berarti domain belum diperiksa. Tauthub tidak memperkirakan DA dari data lain.

### Kolom GEO

Judul kolomnya **GEO**, tetapi isinya adalah **AI Trust Score**, yaitu angka 0 sampai 100. Skor ini menunjukkan seberapa siap situs media dibaca dan dipercaya mesin AI, dilihat dari kecepatan halaman dan kelengkapan data situsnya. Tanda **—** berarti kecepatannya belum bisa diukur.

Warna lencana bergeser dari merah ke hijau mengikuti skornya. Batas warnanya adalah 40, 60, dan 80.

> **Catatan:** Keterangan pada ikon info kolom ini menyebut centang biru untuk media yang dikutip jawaban AI Google. Tabel belum menampilkan centang itu. Untuk melihat media yang dikutip AI, pakai filter **Dikutip AI Overview** di [Menjelajah dan menyaring media](01-menjelajah-dan-menyaring-media.md). Informasi yang sama juga ada di file hasil **Ekspor Proposal**.

### Kolom SLA

**SLA** di tabel berarti SLA waktu terbit, yaitu rata-rata waktu nyata yang dibutuhkan media dari artikel dikirim sampai tayang. Contoh tampilannya: **± <1 jam**, **± 6 jam**, atau **± 4,2 hari**. Arahkan kursor ke angka untuk membaca kalimat lengkapnya.

Tanda **—** berarti belum ada riwayat penerbitan yang cukup untuk menghitung SLA.

Beberapa hal tentang angka ini:

- Angka dihitung dari publikasi yang tayang dalam 7 hari sejak dikirim. Publikasi yang tayang lebih lambat tidak dihitung.
- Untuk media yang terhubung lewat integrasi otomatis, angka yang tampil dibatasi paling lama 3 hari. Media yang menerbitkan manual tidak dibatasi.
- **SLA** di sini berbeda dari **Garansi Tayang (SLA)** di panel detail. Yang kedua adalah lama jaminan tautan, bukan kecepatan terbit.

### Kolom Harga

Kolom ini menampilkan harga satu publikasi, yaitu satu artikel di media itu, misalnya **Rp500.000**. Di HP, harga pindah ke bawah nama media.

Media yang belum memiliki harga tidak tampil di katalog. Harga di kolom ini dijumlahkan menjadi total biaya di bar bawah dan di jendela **Simpan ke Kampanye**.

### Kolom Aksi

- **Ikon bookmark** menyimpan media ke daftar favorit. Arahkan kursor untuk membaca **Simpan ke Bookmark** atau **Hapus dari Bookmark**.
- **Lihat Detail** membuka panel detail. Di HP tombol ini bernama **Detail**.

## Kolom yang disembunyikan di layar sempit

| Lebar layar | Kolom yang disembunyikan | Cara melihat datanya |
|---|---|---|
| Sekitar 1.280 piksel ke atas | Tidak ada | Semua kolom tampil |
| Sekitar 769 sampai 1.279 piksel | **DA**, **GEO**, **SLA** | Buka panel detail. Tabel juga bisa digeser ke samping |
| Sekitar 768 piksel ke bawah (HP) | **DA**, **GEO**, **SLA**, **Kategori**, **Trafik (Est)**, dan kolom **Harga** | Harga tampil di bawah nama media. Data lain ada di panel detail |

Di HP, baris hanya menampilkan kotak centang, nama media beserta harga dan penandanya, serta tombol **Aksi**.

## Cara membandingkan beberapa media

1. Urutkan daftar berdasarkan metrik yang paling penting bagi Anda, misalnya **DA** atau **Harga**.
2. Persempit daftar dengan **Filter**, misalnya dengan **Minimum DA** atau **SLA Waktu Terbit**.
3. Klik **Lihat Detail** pada beberapa media teratas untuk membaca kinerja Mitra dan aturan redaksinya.
4. Centang media yang cocok.

> **Tips:** Perhatikan jumlah distribusi dan kebijakan tautan, bukan hanya DA dan harga. Media dengan DA tinggi tetap bisa membatasi jumlah tautan atau menjamin tautan hanya beberapa bulan.

## Pertanyaan yang sering diajukan

**Apa arti tanda — pada sebuah kolom?** Datanya belum ada. Pada **DA** dan **GEO**, domain belum diperiksa. Pada **SLA**, media belum punya riwayat penerbitan yang cukup.

**Mengapa trafik media tertulis 0?** Belum ada data trafik, baik dari Similarweb maupun dari estimasi media. Angka 0 bukan berarti situs tanpa pengunjung.

**Apa bedanya DA dan AI Trust?** DA menilai kekuatan domain di mata mesin pencari. AI Trust menilai kesiapan situs dibaca dan dipercaya mesin AI. Keduanya diperiksa terpisah.

**Mengapa DA, GEO, dan SLA tidak tampil di layar saya?** Ketiganya disembunyikan di layar yang lebarnya di bawah sekitar 1.280 piksel. Angkanya tetap tersedia di panel detail.

## Lihat juga

- [Panel detail media](03-detail-media.md)
- [Menjelajah dan menyaring media](01-menjelajah-dan-menyaring-media.md)
- [Rating media](09-rating-media.md)
- [Link Guardian](../06-link-guardian.md)
