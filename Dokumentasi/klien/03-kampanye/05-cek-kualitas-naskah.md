# Cek Kualitas Naskah

> Setiap artikel harus lolos pemeriksaan QC sebelum kampanye bisa diterbitkan. Halaman ini menjelaskan panel Analisis QC, popup Cek Validasi, arti status dan kartu pemeriksa, serta langkah agar artikel Anda lolos.

**Lokasi:** menu **Kampanye** › pilih kampanye › **Penyedia Konten** › tombol **Analisis QC** atau **Cek Validasi** di baris artikel
**Untuk:** Klien dengan peran Writer atau lebih tinggi pada brand

## Gambaran singkat

Ada dua alat pemeriksaan:

| Alat | Ikon | Fungsi |
|---|---|---|
| **Analisis QC** | Daftar bercentang | Menampilkan hasil pemindaian dan menjalankannya ulang. Hasilnya menentukan boleh tidaknya artikel diterbitkan |
| **Cek Validasi** | Perisai bercentang | Menampilkan ringkasan indikator artikel. Tidak menjalankan pemindaian |

Pemindaian QC berjalan otomatis untuk satu artikel setiap kali Anda mengklik **Simpan Artikel** di **Pengelola Naskah**. Pemindaian tidak memotong saldo.

Dua pemeriksa bertanda **Hard Gate** wajib lulus: **Illegal Content Guard** dan **Link Health Validator**. Pemeriksa ketiga, **Brand Voice Limits**, hanya memberi catatan.

## Yang tampil di layar

### Panel Analisis QC Artikel

Panel terbuka di sisi kanan dengan subjudul nama media. Isinya:

- **Konteks Artikel**: chip status artikel, judul, **Jumlah kata**, **Tautan aktif**, jenis tautan (rel), dan waktu **Terakhir diubah**.
- Blok progres saat pemindaian: "Memindai artikel…" dengan pewaktu, dan "x/3 pemeriksaan selesai untuk artikel ini". Panel memperbarui diri tiap 4 detik.
- **Pemeriksa Artikel**: tiga kartu, dengan keterangan "Illegal Content & Link Health adalah gerbang wajib untuk bisa distribusi."
- Footer: tombol **OKE** bila artikel **Siap Terbit**, atau **Scan Ulang Artikel Ini** bila belum.

Titik kecil di pojok tombol **Analisis QC** pada kartu artikel menandai hasil terakhir: hijau berarti lulus, merah berarti sudah dipindai tetapi belum lulus.

### Kartu pemeriksa

| Kartu | Hard Gate | Yang diperiksa |
|---|---|---|
| **Illegal Content Guard** | Ya | Apakah isi artikel memuat konten terlarang |
| **Link Health Validator** | Ya | Keamanan tautan dan jumlah tautan keluar dibanding batas media |
| **Brand Voice Limits** | Tidak | Panjang naskah dan nada promosi |

Setiap kartu menampilkan satu baris ringkasan dan satu chip status: **Memindai…**, **Lulus**, **Perlu Review** (pemeriksa ragu), **Gagal** (ada pelanggaran), atau **Pending** (belum ada hasil). Klik **Lihat detail** untuk membaca rincian.

| Kartu | Ringkasan yang mungkin tampil |
|---|---|
| **Illegal Content Guard** | "Risiko low"; "Perlu review (risiko medium)"; "Diblokir: …" diikuti nama kategori dalam bahasa Inggris, misalnya gambling |
| **Link Health Validator** | "Semua link aman"; "N URL tidak aman"; "N tautan keluar · maksimal M"; "Media tidak menerima tautan" |
| **Brand Voice Limits** | "Konsisten dengan suara brand"; "Penyimpangan kecil dari suara brand" |

Detail **Illegal Content Guard** memuat tingkat risiko (low, medium, atau high), keyakinan, kategori, dan sampai tiga alasan. Detail **Link Health Validator** memuat jumlah tautan keluar dibanding batas, serta daftar URL berlabel **OK**, **Tidak terjangkau**, **Perlu review**, atau **Tidak aman**. Detail **Brand Voice Limits** memuat tingkat (consistent, minor_drift, atau off_brand) dan catatan.

**Brand Voice Limits** selalu berstatus **Lulus**, sehingga ringkasannya bisa tetap berbunyi "Konsisten dengan suara brand" walau ada catatan. Baca catatan di **Lihat detail**.

### Status artikel

| Illegal Content Guard | Link Health Validator | Status artikel |
|---|---|---|
| **Lulus** | **Lulus** | **Siap Terbit** |
| **Perlu Review** | **Lulus** | **Perlu Revisi** |
| **Lulus** | **Gagal** karena melebihi batas tautan | **Perlu Revisi** |
| **Gagal** | Apa pun | **Diblokir** |
| Apa pun | **Gagal** karena ada URL tidak aman | **Diblokir** |

Sebelum tiga pemeriksa berjalan, sistem memeriksa moderasi konten. Bila isi ditolak, artikel langsung **Diblokir** dan kartu belum menampilkan hasil (chip **Pending**).

| Status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **Draft Tersedia** | Naskah ada, belum dipindai | Simpan artikel atau klik **Scan Ulang Artikel Ini** |
| **Siap Terbit** | Dua gerbang wajib lulus | Lanjut ke penerbitan |
| **Perlu Revisi** | Ada bagian yang perlu diperbaiki | Buka **Lihat detail**, perbaiki, lalu klik **Simpan Artikel** |
| **Diblokir** | Isi atau tautan melanggar | Lihat "Cara menangani artikel Diblokir" |

Pada kartu artikel di **Penyedia Konten**, status **Perlu Revisi** tampil sebagai chip **Menunggu Artikel**. Buka **Analisis QC** untuk membedakannya dari artikel yang belum dibuat.

## Cara menggunakan

### Cara membaca hasil QC

1. Klik **Analisis QC** pada baris artikel.
2. Lihat chip status di **Konteks Artikel**.
3. Klik **Lihat detail** pada kartu yang berstatus **Gagal** atau **Perlu Review**.
4. Perbaiki naskah sesuai rincian, lalu simpan lewat **Edit Artikel**.

### Cara menjalankan pemindaian ulang

1. Klik **Analisis QC** pada baris artikel.
2. Klik **Scan Ulang Artikel Ini**. Tombol berubah menjadi **Memindai…**.
3. Tunggu hasil muncul. Pemindaian bisa memakan waktu beberapa puluh detik sampai beberapa menit.

Menutup panel tidak menghentikan pemindaian. Bila pemindaian lama, muncul "Pemindaian masih berjalan di server. Hasilnya muncul otomatis di sini begitu selesai." Tombol nonaktif bila AI Gatekeeper belum diaktifkan: "AI Gatekeeper belum dikonfigurasi oleh admin. Pemindaian tidak akan berjalan sampai admin mengaktifkannya."

Untuk memindai banyak artikel sekaligus, pakai **Cek QC Sekarang** atau **Cek Semua QC** di popover **Belum siap terbit** ([Meninjau dan menerbitkan kampanye](22-meninjau-dan-menerbitkan-kampanye.md)). Bila setelah pemindaian dari panel semua artikel kampanye sudah memenuhi dua gerbang wajib, panel menutup sendiri dan alur penerbitan terbuka.

### Cara membuka Cek Validasi

1. Klik **Cek Validasi** (ikon perisai) pada baris artikel.
2. Baca enam kartu indikator, lalu klik **Tutup**.

Popup **Cek Validasi Artikel** hanya membaca naskah tersimpan dan tidak bisa dibuka untuk artikel terkunci.

| Kartu | Nilai yang tampil |
|---|---|
| **Status artikel** | **Tersedia** (draf), **Siap Terbit**, **Diblokir**, atau **Menunggu Artikel** |
| **Jumlah kata** | Misalnya "850 kata" |
| **Cover artikel** | **Ada** atau **Tidak ada** |
| **Gambar di artikel** | Jumlah gambar di dalam isi |
| **Tautan aktif** | Misalnya "1 Dofollow · 1 Nofollow" |
| **Kesesuaian media tujuan** | Jenis media, **Mainstream** atau **Blogger**; bukan hasil analisis |

Bila data artikel belum selesai dimuat, indikator tertulis **Tidak ada** dengan catatan bahwa artikel belum dibuat. Tutup popup lalu buka lagi.

### Cara agar artikel lolos

1. **Illegal Content Guard** menilai konteks, bukan sekadar kata kunci. Membahas, mendidik, atau memberitakan topik sensitif berisiko rendah. Mempromosikan, memfasilitasi, mengagungkan, merekrut, atau menyasar pihak tertentu berisiko tinggi. Kategori yang diperiksa antara lain judi, penipuan, konten seksual, keselamatan anak, kekerasan, pemerasan, ujaran kebencian, ekstremisme, dan eksploitasi manusia. Hasil **Perlu Review** berarti ada kalimat yang meragukan: baca alasannya di **Lihat detail**, lalu tulis ulang bagian itu.
2. **Link Health Validator**: hapus tautan ke judi, pornografi, atau malware. Jaga jumlah tautan keluar sesuai batas media, atau hapus semua tautan bila media tidak menerima tautan. Tautan ke situs media itu sendiri tidak dihitung. Tautan ke situs brand atau kampanye Anda tidak dinilai keamanannya tetapi tetap dihitung terhadap batas. Tautan yang tidak terjangkau hanya dicatat dan tidak menggagalkan pemeriksaan.
3. **Brand Voice Limits** tidak wajib, tetapi disarankan: bila media menetapkan rentang kata, penuhi batas bawahnya (kurang dari 70 persen batas bawah ditandai berat), dan kurangi istilah promosi seperti diskon, promo, gratis, murah, beli sekarang, atau hubungi kami. Satu istilah memberi catatan ringan, lebih dari 8 istilah memberi catatan berat.

Setelah memperbaiki, klik **Simpan Artikel**. QC berjalan otomatis. Bila **Perlu Review** muncul karena analisis gagal ("Analisis gagal — konten perlu review manual."), klik **Scan Ulang Artikel Ini**.

### Cara menangani artikel Diblokir

Pemindaian ulang dan QC otomatis tidak memeriksa artikel berstatus **Diblokir**, dan menyunting naskahnya tidak mengubah status itu. Panel menyarankan memperbaiki isi atau membuat ulang artikel, tetapi hanya pembuatan ulang yang mengembalikan status.

- Untuk artikel hasil AI: buka **Edit Artikel**, tab **Generate dengan AI**, lalu buat draf baru. Status kembali **Draft Tersedia** dan penulisan ulang ditagih. Lihat [Menulis artikel dengan AI](23-menulis-artikel-dengan-ai.md).
- Untuk artikel unggahan: hapus artikel itu bila media punya lebih dari satu artikel, atau lepas media dari kampanye lalu pilih lagi dari **Katalog Media**. Lihat [Beberapa artikel untuk satu media](21-beberapa-artikel-untuk-satu-media.md).

## Penulisan ulang otomatis

Setelah hasil QC tampil, sistem juga menilai mutu isi artikel di balik layar: kejelasan struktur, kepadatan fakta, kredibilitas, dan kesesuaian dengan pertanyaan pembaca. Penilaian ini tidak tampil sebagai kartu. Bila artikel dinilai belum memenuhi standar, sistem menulis ulang artikel di latar belakang.

- Penulisan ulang menyusul beberapa saat setelah hasil QC. Setelah QC otomatis dari **Simpan Artikel**, halaman memuat ulang data sekitar 20, 45, dan 90 detik kemudian, sehingga judul dan jumlah kata pada kartu bisa berubah sendiri. Setelah **Scan Ulang Artikel Ini** dari panel, muat ulang halaman untuk melihat perubahannya.
- Hasilnya tersimpan sebagai versi baru naskah. Buka **Edit Artikel** untuk meninjaunya.
- Penulisan ulang gratis dan tidak mengubah status artikel. Dua gerbang wajib tidak memindai ulang hasilnya sampai Anda menyimpan lagi.
- Bila Anda menyimpan suntingan saat penulisan ulang berjalan, hasil mesin dibuang dan suntingan Anda yang dipakai.
- Sistem tidak menulis ulang berturut-turut: bila pemindaian sebelumnya sudah menulis ulang artikel itu, pemindaian berikutnya tidak menulis ulang lagi.
- Aturan tautan media tetap berlaku pada hasilnya.

Tim Tauthub dapat mematikan penilaian ini. Klaim yang tidak ada di bahan, seperti nama, angka, atau studi kasus karangan, biasanya menjadi alasan penolakan. Sertakan fakta di topik untuk menghindarinya.

## Dampak ke penerbitan

Pesan di bar bawah berbunyi "QC sudah lulus" atau "QC belum lulus. Klik Lanjut Tinjau & Terbit untuk melihat apa yang kurang." Tombol **Lanjut Tinjau & Terbit** hanya membuka panel penerbitan bila dua gerbang wajib lulus untuk semua artikel yang belum terkirim. Bila belum, popover **Belum siap terbit** menampilkan kelompok **QC belum dicek** dan **QC belum lulus**.

Saat penerbitan dikirim, sistem memeriksa lagi. Penolakan berbunyi "Masih ada artikel yang belum lolos QC." atau, untuk tautan berlebih, "Distribusi diblokir — tautan melebihi batas media…". Hapus kelebihan tautan, lalu simpan agar QC berjalan lagi. Langkah lengkap ada di [Meninjau dan menerbitkan kampanye](22-meninjau-dan-menerbitkan-kampanye.md).

## Aturan dan batasan

- QC dihitung per artikel. Media dengan beberapa artikel punya hasil terpisah.
- **Analisis QC** tetap bisa dibuka pada kampanye yang sudah selesai untuk melihat hasil.
- Panel untuk artikel **Siap Terbit** menyebut perubahan manual tidak memicu pemindaian baru. Dalam praktik, menyimpan lewat **Pengelola Naskah** tetap menjalankan QC otomatis.

## Pesan yang mungkin muncul

| Pesan | Artinya | Langkah Anda |
|---|---|---|
| "Gagal memuat hasil pemeriksaan QC." | Hasil tidak terbaca | Tutup dan buka panel lagi |
| "Gagal menjalankan pemindaian QC." | Pemindaian gagal | Coba lagi |
| "Pemindaian belum mengirim hasil. Coba jalankan lagi." | Hasil tidak muncul dalam batas waktu | Klik **Scan Ulang Artikel Ini** |
| "Belum ada hasil pemindaian." | Kartu belum punya hasil | Klik **Scan Ulang Artikel Ini** |

## Pertanyaan yang sering diajukan

**Kenapa artikel yang tadi Siap Terbit menjadi Perlu Revisi setelah saya menyimpan?**
Setiap simpan menjalankan QC lagi untuk versi terbaru. Perubahan Anda mungkin menambah tautan atau memuat kalimat yang meragukan.

**Apakah QC memotong saldo?**
Tidak. Pemindaian, pemindaian ulang, dan penulisan ulang otomatis gratis.

## Lihat juga

- [Penyedia Konten](03-penyedia-konten.md)
- [Pengelola Naskah](04-pengelola-naskah.md)
- [Editor teks naskah](25-editor-teks-naskah.md)
- [Meninjau dan menerbitkan kampanye](22-meninjau-dan-menerbitkan-kampanye.md)
- [Ringkasan dan distribusi](06-ringkasan-distribusi.md)
