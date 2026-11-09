# Detail publikasi

> Dialog **Detail publikasi** merangkum satu artikel klien: ringkasan isi, hasil pemeriksaan konten, penghasilan, URL terbit, catatan klien, dan riwayat pengiriman ke media Anda. Dari dialog ini Anda juga dapat memperbarui URL terbit dan memeriksa ulang verifikasinya.

**Lokasi:** menu **Publikasi** › klik baris (di HP: ketuk kartu) atau ikon **Detail**
**Untuk:** Mitra

## Gambaran singkat

Dialog terbuka di atas daftar. Bagian atasnya tetap terlihat, isinya dapat digulir, dan tombol keputusan di bagian bawah hanya muncul untuk artikel yang menunggu keputusan Anda.

Tautan di notifikasi (lonceng, email, atau WhatsApp) membuka halaman Publikasi dan langsung menampilkan dialog ini untuk artikel yang dimaksud. Jika publikasinya tidak ditemukan di akun Anda, dialog tidak terbuka dan muncul pesan galat berwarna merah, misalnya "Publication not found". Pastikan Anda masuk dengan akun Mitra yang benar.

Untuk menutup dialog, klik tombol **Tutup** (tanda silang) di pojok kanan atas, atau klik di luar dialog.

## Yang tampil di layar

Dialog memuat bagian-bagian berikut dari atas ke bawah. Bagian yang tidak relevan untuk artikel tersebut tidak ditampilkan.

### Pemberitahuan Secure Direct

Jika media artikel ini memakai Secure Direct, di bagian paling atas tampil kotak biru: artikel terbit otomatis tanpa persetujuan per artikel, dan Anda perlu menonaktifkan Secure Direct pada media itu bila ingin meninjau dulu. Penjelasannya ada di [Cara artikel terbit di situs Anda](07-cara-artikel-terbit.md).

### Ringkasan artikel

Kartu pertama memuat gambar sampul, judul, deskripsi singkat (meta deskripsi, atau kalimat awal artikel bila meta deskripsi kosong), nama klien, nama media, dan lencana status di sisi kanan.

> **Catatan:** Nama status di dialog sedikit berbeda dari nama di daftar. Misalnya daftar menulis **Menunggu persetujuan**, sedangkan dialog menulis **Menunggu**. Padanan lengkapnya ada di [Status publikasi dan penghasilan](09-status-dan-penghasilan.md).

### Pemeriksaan konten dan tombol Pratinjau Artikel

Kartu kedua menampilkan hasil pemeriksaan konten otomatis (misalnya **Lolos pemeriksaan konten**) dan tombol **Pratinjau Artikel**. Tombol ini tersedia untuk semua status. Penjelasan lengkapnya ada di [Pratinjau artikel dan pemeriksaan konten](05-pratinjau-artikel.md).

### Informasi publikasi

Kartu ini memuat dua kolom keterangan:

| Keterangan | Isi |
|---|---|
| **No. Order** | Nomor order publikasi. Klik ikon **Salin No. Order**; ikon berubah menjadi centang dan tooltipnya menjadi **Tersalin!**. |
| **Media** | Nama media |
| **Situs media** | Alamat situs media, berupa tautan yang membuka tab baru |
| **Status** | Status publikasi |
| **Klien** | Nama klien |
| **Dibuat** | Waktu klien pertama kali membuat pesanan |
| **Terbit** atau **Terjadwal** | **Terbit** menampilkan waktu artikel tayang (untuk artikel yang sudah terbit). **Terjadwal** menampilkan jadwal yang diminta klien (untuk artikel yang belum terbit). |
| **Integrasi** | Platform integrasi media, misalnya WordPress atau Manual |

Arahkan kursor ke ikon info di sebelah **Dibuat**, **Terbit**, atau **Terjadwal** untuk membaca penjelasan singkatnya.

### Pendapatan

Kartu hijau menampilkan hitungan penghasilan: **Harga pengajuan**, dikurangi **Potongan platform** (persentase dan nominalnya), sama dengan **Pendapatan Anda**. Arti ketiga angka itu sama dengan panel **Rincian penghasilan** di daftar. Pada publikasi yang belum terbit, ditolak, dibatalkan, atau gagal, angkanya hanya perkiraan.

### URL Terbit

Kartu ini hanya muncul untuk artikel yang berstatus **Terbit** atau sudah punya URL. Isinya dijelaskan di bagian **URL terbit dan verifikasi** di bawah.

### Catatan klien, penolakan, dan pembatalan

- **Catatan klien** menampilkan catatan yang disertakan klien pada pesanan, bila ada.
- Kotak **Ditolak:** menampilkan alasan penolakan untuk publikasi yang Anda tolak.
- Kotak **Dibatalkan otomatis:** atau **Dibatalkan klien:** menampilkan alasan pembatalan untuk publikasi **Dibatalkan**. Label **Dibatalkan klien:** dipakai untuk semua pembatalan selain pembatalan otomatis.

### Permintaan Revisi Klien

Bagian ini hanya muncul pada status **Perlu Revisi**. Penjelasannya ada di [Revisi dari klien](04-revisi-dari-klien.md).

### Riwayat

Bagian **Riwayat** menampilkan "Riwayat proses pengiriman dan publikasi artikel". Setiap percobaan pengiriman tampil sebagai satu baris berisi status, keterangan, dan waktunya. Baris terakhir selalu **Pesanan dibuat**.

| Status di Riwayat | Artinya |
|---|---|
| **Terbit** | Artikel berhasil dipublikasikan di media Anda |
| **Mencoba ulang** | Percobaan gagal dan sistem akan mencoba lagi |
| **Gagal** | Percobaan terakhir gagal, dan pesan penyebabnya tertulis dengan huruf merah |

Pada media **Manual**, riwayat berisi satu baris **Terbit** setelah Anda mengirim URL. Pada artikel yang belum pernah dikirim, hanya baris **Pesanan dibuat** yang tampil. Daftar pesan galat yang sering muncul ada di [Cara artikel terbit di situs Anda](07-cara-artikel-terbit.md).

### Tombol di bagian bawah

Tombol keputusan muncul hanya pada dua status.

| Status | Tombol |
|---|---|
| **Menunggu persetujuan** | **Tolak**, serta **Setujui** (media **Manual**) atau **Setuju & Terbitkan** (media **API**) |
| **Perlu Revisi** | **Tolak Revisi** (hilang setelah 3 penolakan) dan **Terima Revisi** |

Berbeda dengan ikon di daftar, tombol **Setujui** atau **Setuju & Terbitkan** di dialog langsung bekerja tanpa jendela konfirmasi tambahan. Tombol nonaktif sementara tindakan lain sedang diproses.

## URL terbit dan verifikasi

Kartu **URL Terbit** berisi "Link artikel yang sudah terpublish di media". Bagian-bagiannya:

- **Kolom URL**: hanya bisa dibaca sampai Anda menekan **Perbarui URL**. Ikon **Salin URL** di dalam kolom menyalin URL; setelah disalin, tooltipnya menjadi **Tersalin!**.
- **Buka URL**: membuka artikel di tab baru. Tombol ini muncul bila artikel sudah punya URL.
- **Perbarui URL**: membuka kolom URL untuk diubah. Setelah itu muncul tombol **Simpan URL** (nonaktif bila kolom kosong) dan **Batal**.
- **Lencana hasil**: **URL terverifikasi** atau **URL bermasalah**, masing-masing disertai skor dari 0 sampai 100. Lencana tidak tampil bila Tauthub belum bisa memeriksa URL, misalnya karena situs Anda tidak dapat dijangkau saat itu.
- **Sentiment**: nada pemberitaan artikel (**Positif**, **Netral**, atau **Negatif**) beserta tingkat keyakinan analisisnya.
- **Message**: skor dari 0 sampai 100 yang menunjukkan seberapa banyak pesan utama dari klien tersampaikan di artikel. Lencana ini hanya muncul bila klien mengisi pesan utama dan analisisnya sudah selesai.
- **Kotak Verifikasi URL** dengan tombol **Cek Ulang**.

### Yang diperiksa Tauthub

Tauthub membuka URL artikel lalu memeriksa lima hal:

1. URL dapat diakses.
2. Judul artikel muncul di halaman.
3. Nama brand klien muncul di halaman.
4. Isi artikel sudah tayang (teks lebih dari 200 karakter).
5. Domain URL sesuai dengan domain media Anda.

Skor 60 atau lebih dinyatakan **URL terverifikasi**. Skor di bawahnya dinyatakan **URL bermasalah**. URL yang menghasilkan halaman tidak ditemukan (kode 404 atau 410) langsung mendapat skor 0.

> **Catatan:** Teks di kotak **Verifikasi URL** selalu sama, apa pun hasilnya. Lihat lencana di atasnya dan pesan petunjuk di bawah kolom URL untuk mengetahui hasil yang sebenarnya.

### Pesan petunjuk verifikasi

| Pesan | Artinya | Yang dapat Anda lakukan |
|---|---|---|
| "URL artikel terbit belum terisi — lengkapi URL di kolom di bawah lalu klik 'Perbarui URL' agar Tauthub bisa memverifikasi artikel." | Artikel belum punya URL untuk diperiksa | Isi URL lewat **Perbarui URL** |
| "HTTP 404 — artikel tidak ditemukan di URL" | Halaman di URL itu tidak ada | Periksa apakah URL benar dan artikel belum dihapus |
| "HTTP … — URL tidak dapat diverifikasi (robots/access?)" | Situs Anda tidak memberikan halaman artikel kepada pemeriksa, misalnya karena akses dibatasi atau ada gangguan sementara | Pastikan artikel dapat dibuka tanpa login, lalu klik **Cek Ulang** |

### Cara memperbarui URL terbit

1. Buka dialog **Detail publikasi** artikel yang Anda maksud.
2. Pada kartu **URL Terbit**, klik **Perbarui URL**.
3. Ketik atau tempel URL yang benar, diawali `http://` atau `https://`.
4. Klik **Simpan URL**.
5. Baca pesan hijau "URL tersimpan & verifikasi sedang dijalankan ulang."

Verifikasi berjalan di belakang layar dan dialog tidak menyegarkan hasilnya sendiri. Tutup dan buka lagi dialog beberapa saat kemudian untuk melihat lencana terbaru.

Aturan URL: wajib diawali `http://` atau `https://`, tidak boleh kosong, dan panjangnya maksimal 1.000 karakter. Jika URL tidak memenuhi syarat, muncul pesan "URL artikel tidak boleh kosong." atau "URL harus diawali http:// atau https://". Penyimpanan URL hanya diterima untuk artikel berstatus **Terbit** atau **Disetujui** (**Menunggu URL**). Pada status lain, penyimpanan ditolak.

### Cara memeriksa ulang verifikasi

1. Buka dialog **Detail publikasi** artikel yang sudah terbit.
2. Pada kotak **Verifikasi URL**, klik **Cek Ulang**. Tombol berubah menjadi **Memeriksa…**.
3. Tunggu beberapa saat, lalu buka lagi dialog untuk melihat lencana terbaru.

> **Catatan:** **Cek Ulang** juga menyimpan URL yang saat ini tampil di kolom sebagai URL terbit. Jika URL itu berasal dari hasil pengiriman otomatis, URL tersebut menjadi URL terbit yang tercatat.

## Pertanyaan yang sering diajukan

**Kenapa kartu URL Terbit tidak muncul?**
Kartu itu hanya tampil untuk artikel yang berstatus **Terbit** atau sudah punya URL. Pada media **Manual**, URL baru ada setelah Anda mengirimnya.

**Kenapa lencana verifikasi tidak muncul?**
Lencana disembunyikan bila Tauthub belum bisa memeriksa URL. Klik **Cek Ulang** setelah memastikan artikel dapat dibuka oleh umum.

**Apa arti "Pesanan dibuat" di paling bawah Riwayat?**
Itu waktu klien membuat pesanan. Baris ini selalu berada di akhir daftar, di bawah percobaan pengiriman.

## Lihat juga

- [Membaca daftar Publikasi](01-daftar-publikasi.md)
- [Pratinjau artikel dan pemeriksaan konten](05-pratinjau-artikel.md)
- [Cara artikel terbit di situs Anda](07-cara-artikel-terbit.md)
- [Status publikasi dan penghasilan](09-status-dan-penghasilan.md)
