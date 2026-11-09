# Meninjau dan menerbitkan kampanye

> Tombol Lanjut Tinjau & Terbit memeriksa apakah semua artikel siap, lalu membuka panel Terbitkan Kampanye untuk memilih waktu terbit tiap media. Halaman ini menjelaskan syarat, langkah, dan pesan galat pada tahap itu.

**Lokasi:** menu **Kampanye** › pilih kampanye › **Penyedia Konten** › tombol **Lanjut Tinjau & Terbit** di bar bawah (di HP bertuliskan **Lanjut**)
**Untuk:** Klien dengan peran Writer atau lebih tinggi pada brand

## Gambaran singkat

Menerbitkan berarti mengirim semua artikel yang belum terkirim ke media tujuannya. Sebelum itu Tauthub memastikan tiap artikel punya naskah, hasil QC yang lulus, cover, dan kategori bila media mewajibkannya. Bila ada yang kurang, tombol membuka daftar prasyarat, bukan panel penerbitan.

Setelah penerbitan berhasil, Anda dibawa ke halaman **Ringkasan** dan kampanye berubah menjadi mode lihat saja. Lihat [Ringkasan dan distribusi](06-ringkasan-distribusi.md).

## Yang tampil di layar

### Popover Belum siap terbit

Popover ini muncul di atas tombol bila ada prasyarat yang belum terpenuhi. Judulnya **Belum siap terbit** dengan keterangan "Selesaikan prasyarat berikut dulu." Setiap prasyarat yang kurang menjadi satu kelompok dengan angka jumlah artikel dan nama media:

| Kelompok | Penyebab | Tindakan |
|---|---|---|
| **Artikel belum dibuat** | Ada artikel tanpa naskah | Buat lewat **Generate Konten** atau **Tulis Sendiri** |
| **QC belum dicek** | Artikel belum pernah dipindai | Klik **Cek QC Sekarang** pada kelompok itu |
| **QC belum lulus** | Sudah dipindai, tetapi belum **Siap Terbit** | Perbaiki naskah, atau klik **Cek QC Sekarang** |
| **Cover belum ada** | Artikel belum punya gambar utama | Klik **Pakai cover default**, atau tambahkan cover lewat **Edit Artikel** |
| **Kategori belum dipilih** | Media mewajibkan kategori dan artikel belum punya | Buka **Edit Artikel** dan pilih **Kategori / Rubrik** |

Di dasar popover ada tombol **Cek Semua QC (N)** bila masih ada artikel yang perlu dipindai. Bila tidak ada, tombolnya **Tutup**. Selama pemindaian, popover menampilkan bilah progres dan teks "Memeriksa N artikel…". Bila semua prasyarat terpenuhi setelah pemindaian, popover menutup sendiri dan panel penerbitan langsung terbuka.

Pesan "QC belum bisa diverifikasi. Muat ulang halaman lalu coba lagi." tampil bila popover kosong tetapi status QC belum terbaca. Muat ulang halaman.

### Panel Terbitkan Kampanye

Panel ini terbuka di sisi kanan, dengan subjudul "Pilih terbit langsung atau jadwalkan, lalu kirim ke media terpilih." Bagian atas berisi **Waktu Terbit per Media**: satu baris untuk tiap artikel yang belum terkunci. Tiap baris menampilkan nama media, penanda **Langsung** atau **Terjadwal**, empat tombol cepat, dan tombol tanggal.

| Tombol | Hasil |
|---|---|
| **Sekarang** | Artikel dikirim langsung |
| **+1 jam**, **+24 jam**, **+3 hari** | Jadwal dihitung dari saat Anda mengklik |
| **Pilih tanggal terbit…** | Membuka pemilih tanggal untuk jadwal bebas |
| Tanda silang di tombol tanggal | Membatalkan jadwal; artikel kembali **Langsung** |

Tombol cepat yang sedang berlaku diberi warna penuh. Jadwal yang dipilih tampil dalam WIB.

Di bagian bawah panel ada:

- **Pemeriksaan Cover**: "Semua artikel sudah punya cover." atau daftar artikel yang belum punya cover, dengan tombol **Pakai thumbnail default Tauthub**.
- Ringkasan: **Media terpilih** ("N Media · M Artikel"), **Waktu terbit** ("X langsung · Y terjadwal"), dan **Total tagihan**.
- Tombol **Batal** dan tombol penerbitan. Tombol penerbitan bertuliskan **Terbitkan Sekarang** bila tidak ada jadwal, atau **Terbitkan & Jadwalkan** bila ada minimal satu jadwal. Tombol nonaktif selama cover belum lengkap atau proses berjalan.

Panel menyebut bahwa media yang dijadwalkan terbit belakangan masih bisa diedit dan diubah jadwalnya dari halaman **Ringkasan**. Pada praktiknya, artikel yang sudah masuk jadwal terkunci dan tidak bisa diubah dari dashboard. Untuk meminta perubahan, hubungi Mitra lewat **Hubungi Media**.

## Cara menggunakan

### Cara menerbitkan kampanye

1. Pastikan semua chip artikel berstatus **Siap Terbit**.
2. Klik **Lanjut Tinjau & Terbit**. Tombol berubah menjadi "Memeriksa cover…" sebentar.
3. Bila popover **Belum siap terbit** muncul, selesaikan prasyarat satu per satu, atau klik **Cek Semua QC**.
4. Di panel **Terbitkan Kampanye**, atur waktu terbit tiap media. Biarkan **Sekarang** untuk terbit langsung.
5. Periksa **Pemeriksaan Cover**. Klik **Pakai thumbnail default Tauthub** bila ada artikel tanpa cover.
6. Periksa ringkasan **Total tagihan**.
7. Klik **Terbitkan Sekarang** atau **Terbitkan & Jadwalkan**.
8. Tunggu tombol selesai menampilkan "Menerbitkan…". Anda masuk ke **Ringkasan**.

### Cara menjadwalkan satu media

1. Di baris media, klik **Pilih tanggal terbit…**.
2. Pilih tanggal di kalender, lalu pilih **Jam** dan **Menit** (kelipatan lima menit).
3. Klik **Terapkan**.

Pemilih tanggal memakai jam perangkat Anda, sedangkan jadwal yang tampil di panel memakai WIB. Tanggal yang sudah lewat tidak bisa dipilih. Tombol **Sekarang** di pemilih mengisi tanggal dan jam dengan waktu perangkat saat ini; geser ke waktu yang lebih lambat sebelum klik **Terapkan**. Waktu yang kurang dari satu menit dari sekarang diperlakukan sebagai **Sekarang**. Bila pemilih menolak pilihan Anda, pesannya "Pilih tanggal terbit dulu." atau "Waktu terbit harus di masa depan."

## Aturan dan batasan

- Semua artikel yang belum terkirim harus berstatus **Siap Terbit**. Tidak ada penerbitan sebagian.
- Cover wajib ada. Tauthub menyediakan thumbnail default bila Anda tidak punya gambar.
- Biaya media ditahan dari saldo saat Anda mengklik tombol penerbitan, dan dipotong permanen saat artikel tayang. Lihat [Tagihan di Penyedia Konten](20-tagihan-di-penyedia-konten.md).
- Artikel yang sudah dikirim terkunci: boleh dilihat, tidak bisa diubah, disimpan, atau ditulis ulang.
- Jumlah tautan di tiap artikel dicek ulang saat penerbitan, tidak hanya saat QC.

## Pesan yang mungkin muncul

| Pesan | Artinya | Langkah Anda |
|---|---|---|
| "Selesaikan cover dulu…" | Masih ada artikel tanpa cover | Klik **Pakai thumbnail default Tauthub** atau tambahkan cover |
| "Ada jadwal yang sudah lewat — pilih waktu di masa depan." | Satu jadwal sudah terlewat | Pilih ulang waktu |
| "Masih ada artikel yang belum lolos QC." | Ada artikel yang belum memenuhi dua syarat wajib QC | Buka **Analisis QC** pada artikel itu |
| "Belum bisa distribusi: … Jalankan audit ulang (atau perbaiki artikel) sampai semua media berstatus ready." | Ada artikel yang belum **Siap Terbit** | Scan ulang atau perbaiki naskah |
| "Distribusi diblokir — tautan melebihi batas media…" | Jumlah tautan melebihi batas media | Hapus kelebihan tautan di naskah, lalu jalankan QC ulang |
| **Saldo Tidak Cukup** | Saldo tidak mencukupi biaya media | Klik **Top Up Saldo**, lalu buka **Ringkasan** untuk memeriksa status publikasi |
| "Penerbitan gagal. Periksa panel Analisis QC lalu coba lagi." | Penerbitan ditolak | Klik **Buka Panel QC**, perbaiki artikel, lalu coba lagi |

> **Tips:** Bandingkan baris **Biaya media** pada **Rincian biaya** dengan saldo Anda sebelum menekan tombol penerbitan. Lihat [Saldo dan top up](../08-saldo-dan-top-up.md).

## Pertanyaan yang sering diajukan

**Kenapa tombol Lanjut Tinjau & Terbit tidak membuka panel?**
Ada prasyarat yang belum terpenuhi. Popover **Belum siap terbit** menyebutkan apa yang kurang.

**Apakah Terbitkan Sekarang berarti artikel langsung tayang?**
Artikel langsung dikirim ke media. Waktu tayang dan persetujuan Mitra mengikuti proses media itu. Pantau di [Ringkasan dan distribusi](06-ringkasan-distribusi.md) dan [Status Publikasi](../05-status-publikasi.md).

**Bisakah saya mengubah jadwal setelah menerbitkan?**
Tidak lewat dashboard. Setelah dikirim, artikel dan jadwalnya terkunci, walau teks di panel penerbitan menyebut perubahan masih bisa dilakukan dari halaman **Ringkasan**. Untuk meminta perubahan, hubungi Mitra lewat **Hubungi Media** selama sesi obrolan 3×24 jam berlaku. Lihat [Ringkasan dan distribusi](06-ringkasan-distribusi.md).

## Lihat juga

- [Penyedia Konten](03-penyedia-konten.md)
- [Cek Kualitas Naskah](05-cek-kualitas-naskah.md)
- [Tagihan di Penyedia Konten](20-tagihan-di-penyedia-konten.md)
- [Ringkasan dan distribusi](06-ringkasan-distribusi.md)
- [Status Publikasi](../05-status-publikasi.md)
