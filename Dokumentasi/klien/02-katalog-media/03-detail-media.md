# Panel detail media

> Panel detail menampilkan profil lengkap satu media: metrik, kinerja Mitra, jalur publikasi, aturan redaksi, dan biaya. Dari panel ini Anda bisa membagikan alamat situs, mengunduh ringkasan PDF, atau langsung memilih media untuk kampanye.

**Lokasi:** menu **RUANG KERJA** › **Katalog Media**, lalu klik nama media atau tombol **Lihat Detail**
**Untuk:** Klien

## Gambaran singkat

Panel detail berguna saat Anda ingin memastikan sebuah media cocok sebelum memesan. Isinya lebih lengkap daripada baris di tabel, dan semua angka yang disembunyikan di layar sempit tersedia di sini.

Anda bisa membuka panel dengan tiga cara:

- klik nama media atau bagian mana pun pada baris tabel;
- klik **Lihat Detail** (di HP, **Detail**);
- klik **Detail** pada kartu media di jendela [AI Matchmaker](08-ai-matchmaker.md).

Tutup panel dengan ikon silang, dengan mengklik area di luar panel, atau dengan menekan Esc. Di layar lebar, isi panel terbagi menjadi dua kolom. Di HP, isinya tersusun satu kolom.

## Yang tampil di layar

### Kepala panel

Kepala panel memuat ikon situs, nama media, dan penanda:

- **Root Domain** atau **Subdomain**: tempat artikel tayang.
- **VERIFIED**: media terverifikasi Tauthub.
- **DEWAN PERS**: media terverifikasi Dewan Pers.

Di bawah nama ada alamat situs yang bisa diklik (terbuka di tab baru), **Tipe** media (Mainstream atau Blogger), dan **Kategori**. Bila media punya beberapa kategori, tombol seperti **+2** membuka daftar lengkapnya.

Di kanan atas ada dua tombol:

- **Bagikan** menyalin alamat situs media ke papan klip. Tombol berubah sebentar menjadi tanda centang hijau dengan keterangan Disalin.
- **Ekspor Info** mengunduh ringkasan media sebagai PDF.

### Strip metrik

Satu baris di bawah kepala panel memuat lima angka:

| Metrik | Isi |
|---|---|
| **Estimasi Trafik** | Perkiraan kunjungan bulanan, misalnya **350 rb / bln** atau **± 350.000 / bln**. Tertulis **0** bila belum ada data |
| **Domain Auth** | Skor DA, atau **—** bila belum diperiksa |
| **AI Trust Score** | Skor AI Trust, atau **—** bila belum diperiksa |
| **Rating** | Rata-rata rating kecepatan terbit dari klien, misalnya **4,5/5**, atau **—** bila belum ada penilaian |
| **Distribusi** | Jumlah artikel yang sudah terbit lewat media ini di seluruh Tauthub, atau **Belum ada distribusi** |

Di HP, baris **Distribusi** pindah dari strip ini ke kartu **Parameter AI & Redaksi**.

Arti DA, AI Trust, dan trafik sama seperti di tabel. Lihat [Membaca tabel dan metrik media](02-membaca-tabel-dan-metrik-media.md). Penjelasan rating ada di [Rating media](09-rating-media.md).

### Jalur publikasi

Kartu **JALUR PUBLIKASI** menunjukkan bagaimana artikel Anda diterbitkan di media itu. Ada dua kemungkinan:

| Jalur | Artinya |
|---|---|
| **Tauthub Secure Direct™** | Artikel langsung terbit ke media setelah pesanan dikirim, tanpa persetujuan manual. Riwayat tayang terpantau di ringkasan kampanye |
| **Editorial Review (SLA Tracked)** | Redaksi media meninjau naskah secara manual. Antrean penerbitan, revisi, dan waktu respons dipantau di halaman ringkasan kampanye |

Pada jalur **Editorial Review**, Mitra punya batas 3×24 jam untuk menyetujui. Bila Mitra tidak merespons sampai batas itu, pesanan dibatalkan otomatis dan nilainya kembali ke saldo Anda. Status pesanan dijelaskan di [Status Publikasi](../05-status-publikasi.md).

### Indikator Kinerja Mitra

Kartu ini merangkum rekam jejak media. Di kanan atas kartu ada label **HEALTH**:

| Label | Artinya |
|---|---|
| **SANGAT SEHAT** | Skor kesehatan 70 ke atas |
| **PERLU PERHATIAN** | Skor 40 sampai 69 |
| **KONDISI RENDAH** | Skor di bawah 40 |
| **BELUM DISKOR** | Skor belum dihitung |

Klik ikon info di samping judul kartu untuk melihat jumlah transaksi yang menjadi dasar penilaian dan waktu pembaruan terakhir.

Empat kotak di dalam kartu menunjukkan komponen kinerja:

| Kotak | Keterangan di layar | Cara membaca |
|---|---|---|
| **Fulfillment Rate** | Rasio order sukses diterbitkan | Persentase. Makin tinggi makin baik |
| **SLA Responsiveness** | Rata-rata waktu tayang | Jumlah jam. Makin kecil makin cepat. **—** bila belum ada riwayat |
| **Link Permanence** | Integritas retensi artikel | Persentase. Makin tinggi makin baik |
| **Server Uptime** | Stabilitas akses web (30 hari) | Persentase. Makin tinggi makin baik |

> **Catatan:** Media baru yang belum punya riwayat publikasi menampilkan angka 0% pada kotak-kotak ini dan **—** pada **SLA Responsiveness**. Itu berarti datanya belum ada. Cek **Distribusi** untuk memastikan apakah media sudah punya riwayat.

Di bawah empat kotak ada kartu **Content Velocity** dengan keterangan "Rata-rata artikel terbit / minggu". Kartu ini menunjukkan seberapa aktif situs menerbitkan artikel, dari pemeriksaan otomatis atas situs media:

- Angka **N artikel** menunjukkan artikel yang terdeteksi terbit dalam tujuh hari terakhir.
- Lencana **PRODUKTIF** berarti ada minimal satu artikel. Lencana **STAGNAN** berarti tidak ada.
- Baris **Data per** menyebut waktu pemeriksaan dan sumbernya, misalnya sitemap atau RSS/Atom.
- Bila datanya sudah lama, lencana disembunyikan dan baris data diberi catatan **(belum diperbarui)**, **(diblokir)**, atau **(tak ada sitemap/feed)**.
- Bila belum ada data, tampil keterangan seperti **Belum ada data crawl**, **Diblokir situs (Cloudflare/robots)**, atau **Tak ada sitemap/feed di situsnya**.

### Parameter AI & Redaksi

Kartu ini memuat aturan penulisan dan penerbitan yang ditetapkan Mitra:

| Baris | Artinya |
|---|---|
| **Garansi Tayang (SLA)** | Lama tautan di artikel dijamin tetap ada: **Permanen — tautan tidak pernah dilepas**, atau jumlah bulan seperti **12 Bulan · kebijakan media** |
| **Label Sponsor / Adv** | **Tidak Ada (Organik)** atau **Ada Label Sponsor / Adv**. Menunjukkan apakah artikel diberi label sponsor |
| **Batas Tautan Keluar** | Jumlah dan jenis tautan ke situs luar per artikel, misalnya **Maks. 2 Dofollow**, atau **Tidak Ada (Tanpa Link)** |
| **Panjang Artikel** | Misalnya **300–700 kata**, **800–1500 kata**, atau **Sesuai Permintaan Klien** dengan catatan kisaran kata |
| **Subjudul (H2/H3)** | Apakah artikel boleh memakai subjudul, misalnya **Boleh memakai subjudul** atau **Tanpa subjudul** |
| **Bahasa Artikel** | Bahasa yang diterima media, ditampilkan sebagai bendera. Arahkan kursor untuk melihat nama bahasanya |
| **Support Embed (Video/Maps)** | **Diizinkan** atau **Tidak Diizinkan** |
| **Platform** | **WordPress**, **EmDash**, **Blogspot**, **Custom**, atau **—** bila belum terdeteksi |
| **Contoh Artikel** | Tautan **Lihat contoh**. Baris ini hanya ada bila Mitra menyediakan contoh |
| **Distribusi** | Hanya tampil di HP. Jumlah artikel terbit, sama seperti di strip metrik |

Pada **Panjang Artikel** dengan nilai **Sesuai Permintaan Klien**, panel menambahkan catatan kisaran kata. Untuk media blogger kisarannya sekitar 500 sampai 5.000 kata. Untuk media lain, panjang mulai sekitar 300 kata.

**Garansi Tayang (SLA)** berbeda dari kolom **SLA** di tabel. Garansi tayang adalah lama jaminan tautan, sedangkan kolom **SLA** adalah kecepatan terbit.

### Pembatasan Gatekeeper

Kartu ini mendaftar topik yang tidak boleh ada di naskah untuk media tersebut. Isinya empat topik umum, yaitu **SARA**, **Judi / Kasino**, **Dewasa (Adult)**, dan **Kripto / Forex**, ditambah topik yang dilarang khusus oleh Mitra.

Menurut keterangan di kartu, naskah yang mengandung topik tersebut akan diblokir otomatis oleh AI Validator sebelum saldo Anda terpotong. Baca daftar ini sebelum menulis atau mengunggah naskah. Pemeriksaan naskah dibahas di [Cek kualitas naskah](../03-kampanye/05-cek-kualitas-naskah.md).

### Biaya dan tombol lanjut

Bagian bawah panel menampilkan **BIAYA PUBLIKASI**, yaitu harga satu artikel di media itu, misalnya **Rp500.000**. Di sebelahnya ada tombol **Lanjut ke Dapur Produksi**.

## Cara menggunakan

### Cara membagikan alamat media

1. Buka panel detail media.
2. Klik **Bagikan**.
3. Tempel alamat situs ke pesan atau dokumen Anda.

### Cara mengunduh ringkasan media

1. Buka panel detail media.
2. Klik **Ekspor Info**. Tombol berubah menjadi **Menyiapkan…**.
3. Tunggu sampai file PDF terunduh. Namanya berpola **Info-Media_Tauthub_** diikuti nama media.

PDF berukuran A4 satu halaman. Isinya mencakup identitas media, metrik, indikator kinerja, parameter redaksi, topik terlarang, dan biaya publikasi. Bila gagal, tombol menampilkan **Gagal, coba lagi** selama beberapa detik. Klik **Ekspor Info** sekali lagi.

### Cara memilih media dari panel detail

1. Buka panel detail media.
2. Klik **Lanjut ke Dapur Produksi**.

Hasilnya bergantung pada tempat Anda membuka panel:

| Dibuka dari | Yang terjadi setelah klik |
|---|---|
| Tabel katalog | Media masuk ke pilihan dan jendela **Simpan ke Kampanye** terbuka dengan nama terisi "Kampanye" diikuti domain media. Lihat [Membuat kampanye dari katalog](05-membuat-kampanye-dari-katalog.md) |
| Jendela AI Matchmaker | Media masuk ke pilihan, lalu panel detail dan jendela Matchmaker tertutup. Jendela kampanye tidak terbuka |
| Mode tambah media ke kampanye | Media hanya dicentang. Simpan lewat **Tambahkan ke Kampanye**. Lihat [Menambah media ke kampanye yang ada](06-menambah-media-ke-kampanye-yang-ada.md) |

## Aturan dan batasan

- **BIAYA PUBLIKASI** hanya harga media. Biaya lain, misalnya penulisan artikel dengan AI atau gambar, dihitung di tahap berikutnya dan dibahas di [Kampanye](../03-kampanye/README.md).
- Angka kinerja Mitra dihitung otomatis dari riwayat publikasi dan diperbarui berkala. Satu angka tidak menjamin hasil pesanan berikutnya.

## Pertanyaan yang sering diajukan

**Mengapa semua kotak kinerja bernilai 0% pada satu media?** Media itu belum punya riwayat publikasi. Cek baris **Distribusi**. Bila tertulis **Belum ada distribusi**, belum ada artikel yang terbit lewat media itu, sehingga datanya memang belum ada.

**Apa bedanya Secure Direct dan Editorial Review?** Pada Secure Direct, artikel langsung terbit tanpa persetujuan manual. Pada Editorial Review, Mitra menyetujui dulu, dalam batas 3×24 jam.

**Apakah tombol Bagikan membagikan halaman Tauthub?** Tidak. Tombol ini menyalin alamat situs media itu sendiri.

## Lihat juga

- [Membaca tabel dan metrik media](02-membaca-tabel-dan-metrik-media.md)
- [Memilih media dan bar seleksi](04-memilih-media-dan-bar-seleksi.md)
- [Rating media](09-rating-media.md)
- [Status Publikasi](../05-status-publikasi.md)
