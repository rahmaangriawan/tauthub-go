# Meninjau dan menerbitkan kampanye

> Tombol Lanjut Tinjau & Terbit mengecek apakah semua artikel sudah siap. Lalu tombol itu membuka panel Terbitkan Kampanye untuk memilih waktu terbit tiap media. Halaman ini menjelaskan syarat, langkah, dan pesan galat di tahap itu.

**Lokasi:** menu **Kampanye** › pilih kampanye › **Penyedia Konten** › tombol **Lanjut Tinjau & Terbit** di bar bawah (di HP bertuliskan **Lanjut**)
**Untuk:** Klien dengan peran Writer atau lebih tinggi pada brand

## Gambaran singkat

Menerbitkan berarti mengirim semua artikel yang belum terkirim ke media tujuannya. Sebelum itu, Tauthub memastikan tiap artikel punya naskah, hasil QC yang lulus, cover, dan kategori bila media mewajibkannya. QC adalah pemeriksaan kualitas naskah. Cover adalah gambar sampul artikel. Bila ada yang kurang, tombol membuka daftar syarat yang belum dipenuhi, bukan panel penerbitan.

Setelah penerbitan berhasil, Anda dibawa ke halaman **Ringkasan**. Kampanye lalu hanya bisa dilihat, tidak bisa diubah. Lihat [Ringkasan dan distribusi](06-ringkasan-distribusi.md).

## Yang tampil di layar

### Jendela Belum siap terbit

Jendela kecil ini muncul di atas tombol bila ada syarat yang belum dipenuhi. Judulnya **Belum siap terbit**, dengan keterangan "Selesaikan prasyarat berikut dulu." Tiap syarat yang kurang menjadi satu kelompok, lengkap dengan jumlah artikel dan nama media:

| Kelompok | Penyebab | Tindakan |
|---|---|---|
| **Artikel belum dibuat** | Ada artikel tanpa naskah | Buat lewat **Generate Konten** atau **Tulis Sendiri** |
| **QC belum dicek** | Artikel belum pernah diperiksa | Klik **Cek QC Sekarang** pada kelompok itu |
| **QC belum lulus** | Sudah diperiksa, tetapi belum **Siap Terbit** | Perbaiki naskah, atau klik **Cek QC Sekarang** |
| **Cover belum ada** | Artikel belum punya gambar utama | Klik **Pakai cover default**, atau tambahkan cover lewat **Edit Artikel** |
| **Kategori belum dipilih** | Media mewajibkan kategori dan artikel belum punya | Buka **Edit Artikel** dan pilih **Kategori / Rubrik** |

Di dasar jendela ada tombol **Cek Semua QC (N)** bila masih ada artikel yang perlu diperiksa. Bila tidak ada, tombolnya **Tutup**. Saat pemeriksaan berjalan, jendela menampilkan bilah kemajuan dan tulisan "Memeriksa N artikel…". Bila semua syarat terpenuhi setelah pemeriksaan, jendela menutup sendiri dan panel penerbitan langsung terbuka.

Pesan "QC belum bisa diverifikasi. Muat ulang halaman lalu coba lagi." muncul bila jendela kosong tetapi status QC belum terbaca. Muat ulang halaman.

### Panel Terbitkan Kampanye

Panel ini terbuka di sisi kanan, dengan subjudul "Pilih terbit langsung atau jadwalkan, lalu kirim ke media terpilih." Bagian atasnya berisi **Waktu Terbit per Media**: satu baris untuk tiap artikel yang belum terkunci. Tiap baris menampilkan nama media, tanda **Langsung** atau **Terjadwal**, empat tombol cepat, dan tombol tanggal.

| Tombol | Hasil |
|---|---|
| **Sekarang** | Artikel langsung dikirim |
| **+1 jam**, **+24 jam**, **+3 hari** | Jadwal dihitung dari saat Anda mengklik |
| **Pilih tanggal terbit…** | Membuka pemilih tanggal untuk jadwal bebas |
| Tanda silang di tombol tanggal | Membatalkan jadwal. Artikel kembali **Langsung** |

Tombol cepat yang sedang dipakai berwarna penuh. Jadwal yang Anda pilih tampil dalam WIB (waktu Indonesia bagian barat).

Di bagian bawah panel ada:

- **Pemeriksaan Cover**: tulisan "Semua artikel sudah punya cover." atau daftar artikel yang belum punya cover, dengan tombol **Pakai thumbnail default Tauthub**.
- Ringkasan: **Media terpilih** ("N Media · M Artikel"), **Waktu terbit** ("X langsung · Y terjadwal"), dan **Total tagihan**.
- Tombol **Batal** dan tombol penerbitan. Tombol penerbitan bertuliskan **Terbitkan Sekarang** bila tidak ada jadwal. Bila ada paling sedikit satu jadwal, tulisannya **Terbitkan & Jadwalkan**. Tombol tidak bisa diklik selama cover belum lengkap atau proses masih berjalan.

Panel menyebut bahwa media yang dijadwalkan terbit belakangan masih bisa diedit dan diubah jadwalnya dari halaman **Ringkasan**. Kenyataannya, artikel yang sudah masuk jadwal terkunci. Anda tidak bisa mengubahnya dari dashboard. Untuk meminta perubahan, hubungi Mitra (pemilik media) lewat **Hubungi Media**.

## Cara menggunakan

### Cara menerbitkan kampanye

1. Pastikan semua chip artikel berstatus **Siap Terbit**.
2. Klik **Lanjut Tinjau & Terbit**. Sebentar saja tombol berubah menjadi "Memeriksa cover…".
3. Bila jendela **Belum siap terbit** muncul, selesaikan syaratnya satu per satu, atau klik **Cek Semua QC**.
4. Di panel **Terbitkan Kampanye**, atur waktu terbit tiap media. Biarkan **Sekarang** bila ingin terbit langsung.
5. Periksa **Pemeriksaan Cover**. Klik **Pakai thumbnail default Tauthub** bila ada artikel tanpa cover.
6. Periksa ringkasan **Total tagihan**.
7. Klik **Terbitkan Sekarang** atau **Terbitkan & Jadwalkan**.
8. Tunggu sampai tulisan "Menerbitkan…" di tombol hilang. Anda akan masuk ke **Ringkasan**.

### Cara menjadwalkan satu media

1. Di baris media, klik **Pilih tanggal terbit…**.
2. Pilih tanggal di kalender, lalu pilih **Jam** dan **Menit** (kelipatan lima menit).
3. Klik **Terapkan**.

Pemilih tanggal memakai jam perangkat Anda. Jadwal yang tampil di panel memakai WIB. Tanggal yang sudah lewat tidak bisa dipilih. Tombol **Sekarang** di pemilih mengisi tanggal dan jam dengan waktu perangkat saat ini. Geser ke waktu yang lebih lambat sebelum klik **Terapkan**. Waktu yang kurang dari satu menit dari sekarang dianggap **Sekarang**. Bila pemilih menolak pilihan Anda, pesannya "Pilih tanggal terbit dulu." atau "Waktu terbit harus di masa depan."

## Aturan dan batasan

- Semua artikel yang belum terkirim harus berstatus **Siap Terbit**. Tidak ada penerbitan sebagian.
- Cover wajib ada. Bila Anda tidak punya gambar, Tauthub menyediakan thumbnail default.
- Biaya media ditahan dari saldo saat Anda klik tombol penerbitan. Biaya itu dipotong untuk selamanya saat artikel tayang. Lihat [Tagihan di Penyedia Konten](20-tagihan-di-penyedia-konten.md).
- Artikel yang sudah dikirim terkunci. Anda boleh melihatnya, tetapi tidak bisa mengubah, menyimpan, atau menulis ulang.
- Tauthub mengecek ulang jumlah tautan di tiap artikel saat penerbitan, bukan hanya saat QC.

## Pesan yang mungkin muncul

| Pesan | Artinya | Langkah Anda |
|---|---|---|
| "Selesaikan cover dulu…" | Masih ada artikel tanpa cover | Klik **Pakai thumbnail default Tauthub** atau tambahkan cover |
| "Ada jadwal yang sudah lewat. Pilih waktu di masa depan." | Satu jadwal sudah terlewat | Pilih ulang waktu |
| "Masih ada artikel yang belum lolos QC." | Ada artikel yang belum memenuhi dua syarat wajib QC | Buka **Analisis QC** pada artikel itu |
| "Belum bisa distribusi: … Jalankan audit ulang (atau perbaiki artikel) sampai semua media berstatus ready." | Ada artikel yang belum **Siap Terbit** | Periksa ulang atau perbaiki naskah |
| "Distribusi diblokir. Tautan melebihi batas media…" | Jumlah tautan melebihi batas media | Hapus kelebihan tautan di naskah, lalu jalankan QC lagi |
| **Saldo Tidak Cukup** | Saldo kurang untuk membayar biaya media | Klik **Top Up Saldo**, lalu buka **Ringkasan** untuk melihat status penerbitan |
| "Penerbitan gagal. Periksa panel Analisis QC lalu coba lagi." | Penerbitan ditolak | Klik **Buka Panel QC**, perbaiki artikel, lalu coba lagi |

> **Tips:** Sebelum menekan tombol penerbitan, bandingkan baris **Biaya media** di **Rincian biaya** dengan saldo Anda. Lihat [Saldo dan top up](../08-saldo-dan-top-up.md).

## Pertanyaan yang sering diajukan

**Kenapa tombol Lanjut Tinjau & Terbit tidak membuka panel?**
Ada syarat yang belum dipenuhi. Jendela **Belum siap terbit** menyebutkan apa yang kurang.

**Apakah Terbitkan Sekarang berarti artikel langsung tayang?**
Artikel langsung dikirim ke media. Waktu tayang dan persetujuan Mitra mengikuti proses media itu. Pantau di [Ringkasan dan distribusi](06-ringkasan-distribusi.md) dan [Status Publikasi](../05-status-publikasi.md).

**Bisakah saya mengubah jadwal setelah menerbitkan?**
Tidak lewat dashboard. Setelah dikirim, artikel dan jadwalnya terkunci, meski tulisan di panel penerbitan bilang perubahan masih bisa dilakukan dari halaman **Ringkasan**. Untuk meminta perubahan, hubungi Mitra lewat **Hubungi Media** selama sesi obrolan 3×24 jam masih berlaku. Lihat [Ringkasan dan distribusi](06-ringkasan-distribusi.md).

## Lihat juga

- [Penyedia Konten](03-penyedia-konten.md)
- [Cek Kualitas Naskah](05-cek-kualitas-naskah.md)
- [Tagihan di Penyedia Konten](20-tagihan-di-penyedia-konten.md)
- [Ringkasan dan distribusi](06-ringkasan-distribusi.md)
- [Status Publikasi](../05-status-publikasi.md)
