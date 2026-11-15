# Brief untuk AI Writer, biaya, dan batas pemakaian

> AI Writer memakai brief saat Anda memilih gaya Yang Proper di Pengelola Naskah. Halaman ini menjelaskan langkahnya, syarat ringkasan, cara kerja biaya secara umum, dan batas pemakaian di Taut Brief.

**Lokasi:** Pengelola Naskah › tab **Generate dengan AI** › gaya **Yang Proper** › **Memori Brand (Tauthub Copilot)**

**Untuk:** Klien yang menulis artikel di Pengelola Naskah.

## Gambaran singkat

Brief artinya catatan tertulis tentang brand Anda. AI Writer adalah AI yang menulis artikel untuk Anda. Brief tidak menempel otomatis pada kampanye, dan pilihan Profiler tidak tampil di jendela pembuatan kampanye di Katalog Media. AI Writer memakai brief lewat pilihan di Pengelola Naskah, untuk tiap artikel. Kalau Anda menekan **Pakai untuk Artikel** di Taut Brief, brief itu menjadi brief aktif dan terpilih otomatis di Pengelola Naskah.

Di Pengelola Naskah ada dua gaya artikel:

- **Sederhana Aja**, ditandai **Tanpa Perlu data perusahaan / Copilot**. Gaya ini tidak perlu brief.
- **Yang Proper**, ditandai **Terhubung Copilot & Paste Mentahan**. Gaya ini untuk rilis resmi brand, peluncuran produk, atau artikel yang butuh fakta tepat. AI Writer menulis dengan terkunci pada fakta, dan brief menjadi sumber faktanya. Pada gaya ini brand wajib punya brief.

Pada gaya **Yang Proper**, AI Writer hanya memakai fakta dari tiga sumber: kotak **Fakta / Data Produk yang Wajib Masuk**, **Ringkasan Entitas**, dan **Ringkasan File** dari brief yang Anda pilih. AI Writer tidak menambah fakta di luar sumber itu. Pemeriksaan kualitas naskah mencocokkan klaim dengan bahan yang sama.

## Cara menggunakan

### Cara memakai brief pada gaya Yang Proper

1. Buka Pengelola Naskah untuk artikel yang akan Anda tulis. Lihat [Pengelola Naskah](../03-kampanye/04-pengelola-naskah.md).
2. Buka tab **Generate dengan AI**.
3. Pada pertanyaan **Kamu mau buat artikel seperti apa?**, klik **Yang Proper**.
4. Di bagian **Memori Brand (Tauthub Copilot)**, buka kotak **Brief Copilot** lalu pilih brief. Anda bisa mencarinya lewat **Cari brief…**.
5. Tunggu sampai muncul baris **Brand** dengan nama brand, keterangan **Profil & Produk Terkunci**, dan **Data Copilot Aktif** yang menyebut kategori, nama, dan jumlah fakta tersimpan.
6. Klik **Tarik dari Copilot**. Kotak **Fakta / Data Produk yang Wajib Masuk** terisi dengan isian brief dalam bentuk baris "Label: nilai".
7. Kalau perlu, tambahkan atau ubah isinya. Anda juga boleh menempel teks mentah dari chat, brosur, atau spesifikasi produk.
8. Lengkapi bagian lain di tab itu, lalu buat draf. Bagian lain dijelaskan di Pengelola Naskah.

Kotak fakta menerima maksimal 20.000 karakter.

Tombol **Buat Draf dengan AI** mati selama brief belum dipilih. Tombol itu juga mati selama kotak fakta kosong dan brief belum menyimpan fakta. Arahkan kursor ke tombol untuk melihat alasannya.

### Cara membuat brief dari Pengelola Naskah

1. Kalau brand belum punya brief, bagian **Memori Brand (Tauthub Copilot)** menampilkan kotak **Memori Brand wajib diisi**.
2. Klik **Buat Brief Copilot**. Pintu masuk Taut Brief terbuka di tab browser baru, jadi isian Anda di Pengelola Naskah tidak hilang.
3. Susun brief. Lihat [Membuka Taut Brief dan memilih pintu masuk](01-membuka-taut-brief-dan-gerbang.md).
4. Kembali ke tab Pengelola Naskah, lalu klik **Cek ulang**. Brief baru muncul di kotak **Brief Copilot**.

Kalau brief yang Anda pilih belum menyimpan fakta, tombol **Buka Taut Brief** tampil untuk membuka brief itu langsung.

### Cara mengisi tautan dari brief

Di bagian tautan, tombol **Ambil dari brief** mengisi baris tautan dari **Keyword prioritas dan URL target** milik brief yang dipilih (satu pasangan keyword dan URL per baris). Keyword menjadi teks tautan. Baris yang sudah ada tidak ditimpa dan pasangan yang sama tidak dimasukkan dua kali. Jumlahnya dibatasi oleh batas tautan media. Kalau brief belum punya pasangan keyword dan URL, muncul pemberitahuan.

### Pemeriksaan kata terlarang

Isian **Kata atau frasa yang harus dihindari** milik brief dicocokkan dengan teks artikel saat **Cek Validasi**. Pencocokan harfiah, tanpa membedakan huruf besar dan kecil, dan kata tunggal hanya cocok sebagai kata utuh. Kalau ada yang ditemukan, hasil cek menyebut kata itu, status artikel menjadi **Revisi**, dan Anda perlu menghapus atau mengganti katanya lalu menjalankan **Cek Validasi** lagi. Pemeriksaan ini hanya berlaku untuk artikel yang memakai brief (gaya **Yang Proper**). Aturan klaim terlarang tidak dicocokkan harfiah karena berbentuk kalimat, jadi tetap dinilai AI.

## Syarat ringkasan

Pada gaya **Yang Proper**, **Ringkasan Entitas** milik brief harus berstatus **Siap**.

- Kalau belum siap, Tauthub langsung memintanya dibuat saat Anda memilih brief. Lalu Tauthub menampilkan pesan agar Anda menunggu sebentar sebelum membuat draf lagi.
- **Ringkasan File** membantu kalau brief punya dokumen, tautan, atau post Timeline. Kalau belum siap, penulisan tetap berjalan tanpa ringkasan itu, dan Tauthub memintanya dibuat di latar belakang.
- Ringkasan yang berstatus **Perlu diperbarui** tetap dipakai, tetapi isi brief yang terbaru belum ikut. Di bawah pilihan brief muncul peringatan **Brief diubah setelah ringkasannya dibuat** dengan tombol **Perbarui ringkasan**. Setelah menekan tombolnya, tunggu sebentar lalu buat draf lagi.
- Tautan **Buka Ringkasan Brief** di pesan peringatan membuka tab **Ringkasan** milik brief itu di tab baru.

Detail ringkasan ada di [Ringkasan brief](08-ringkasan-brief.md).

> **Catatan:** Kampanye yang dulu dibuat dengan memilih Profiler tetap membawa pilihan itu. Untuk kampanye seperti itu, AI Writer ikut memakai isi brief tersebut. Kalau brief sudah dihapus, AI Writer memakai salinan yang tersimpan saat kampanye dibuat. Pemeriksaan kualitas juga mengenali tautan ke domain di **Kanal resmi** dan **Website** brief sebagai "Tautan milik brand/kampanye sendiri". Domain adalah nama alamat situs, misalnya contoh.co.id.

## Biaya

Taut Brief tidak punya tarif sendiri. Mengobrol dengan Tubi, mengisi formulir, mengunggah berkas, menulis Timeline, dan membuat ringkasan tidak memotong Saldo. Layarnya juga tidak menampilkan angka biaya.

Biaya muncul di langkah lain, yaitu saat artikel ditulis dan gambar AI dibuat di kampanye. Tarifnya tampil di layar Pengelola Naskah dan mengikuti pengaturan yang berlaku saat proses dijalankan. Penjelasan Saldo ada di [Saldo dan Top Up](../08-saldo-dan-top-up.md).

## Batas pemakaian

| Hal | Batas |
|---|---|
| Pesan di **Copilot Chat** | Maksimal 4.000 karakter per pesan. Ada batas jumlah pesan per menit yang ditentukan Tauthub. |
| Tautan di satu pesan obrolan | Paling banyak dua tautan dibaca. |
| Ukuran berkas | Maksimal 10 MB per berkas. MB adalah satuan ukuran berkas. |
| Lampiran lewat **Copilot Chat** | Maksimal 5 berkas dan 25 MB per brief. |
| Unggahan harian | Ada batas jumlah berkas, total ukuran per hari, dan total unggahan aktif per akun. Tauthub yang menentukan angkanya. |
| Nama brief dan nama template | Maksimal 160 karakter. |
| Deskripsi template | Maksimal 300 karakter. |
| Post **Timeline** | Maksimal 2.000 karakter, 10 tagar, dan satu gambar. Post terhapus setelah 3 bulan. |
| Draf Timeline | Paling banyak 20 per brief, tersimpan di browser. |
| Kotak **Fakta / Data Produk yang Wajib Masuk** | Maksimal 20.000 karakter. |
| Bahan **Ringkasan File** | Dua belas post Timeline, lima dokumen, dan lima tautan terbaru. |

Angka di tabel adalah batas yang tampil di layar atau berlaku tetap. Angka yang disebut ditentukan Tauthub bisa berubah, jadi tidak ditulis di sini.

## Pesan masalah yang umum

| Pesan | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **Memori Brand wajib diisi — pilih atau buat Brief Copilot dulu sebelum membuat artikel.** | Gaya **Yang Proper** dipilih tanpa brief. | Pilih brief di kotak **Brief Copilot**, atau buat dulu. |
| **Isi fakta produk atau pilih Brief Copilot dulu — mode Proper ditulis terkunci ke fakta.** | Tidak ada bahan fakta. | Isi kotak fakta atau pilih brief. |
| **Brief ini belum menyimpan fakta — lengkapi dulu percakapannya di halaman Taut Brief.** | Brief yang dipilih masih kosong. | Buka brief lewat **Buka Taut Brief**, lalu isi lewat **Copilot Chat** atau **Instan Brief**. |
| **Ringkasan brief sedang disiapkan otomatis. Tunggu sebentar, lalu buat draf lagi.** | **Ringkasan Entitas** belum **Siap**. | Tunggu beberapa saat, lalu buat draf lagi. |
| **Ringkasan Entitas brief sedang disiapkan otomatis. Coba lagi beberapa detik lagi.** | Ringkasan sedang dibuat. | Tunggu, lalu coba lagi. |
| **Gagal memuat data Brief Copilot.** | Daftar atau isi brief belum bisa dimuat. | Klik **Cek ulang**, atau muat ulang halaman. |
| **Brief tidak ditemukan untuk brand ini.** | Brief yang dipilih sudah dihapus. | Pilih brief lain. |
| **Fakta terkunci maksimal 20.000 karakter.** | Isi kotak fakta terlalu panjang. | Persingkat isinya. |

## Pertanyaan yang sering diajukan

**Apakah saya harus punya brief untuk memakai AI Writer?**
Tidak. Gaya **Sederhana Aja** tidak memerlukannya. Brief wajib hanya pada gaya **Yang Proper**.

**Apakah brief harus 100% lengkap?**
Tidak ada syarat persentase. Yang diperlukan: brief dipilih, ada fakta (dari brief atau dari kotak fakta), dan **Ringkasan Entitas** berstatus **Siap**.

**Apakah mengubah brief mengubah artikel yang sudah ditulis?**
Tidak. Artikel yang sudah dibuat tidak ditulis ulang. Perubahan brief dipakai pada draf berikutnya. Kotak fakta memakai isian terbaru saat Anda menekan **Tarik dari Copilot**. Ringkasan baru ikut berubah setelah diperbarui.

**Apakah memilih brief menambah biaya?**
Taut Brief tidak punya tarif sendiri. Biaya penulisan mengikuti tarif di Pengelola Naskah dan [Saldo dan Top Up](../08-saldo-dan-top-up.md).

## Lihat juga

- [Ringkasan brief](08-ringkasan-brief.md)
- [Copilot Chat](03-copilot-chat.md)
- [Pengelola Naskah](../03-kampanye/04-pengelola-naskah.md)
- [Kampanye](../03-kampanye/README.md)
- [Saldo dan Top Up](../08-saldo-dan-top-up.md)
