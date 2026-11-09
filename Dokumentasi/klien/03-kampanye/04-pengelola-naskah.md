# Pengelola Naskah

> Pengelola Naskah adalah jendela untuk menulis, mengunggah, atau meminta AI menyusun naskah satu artikel. Di sini Anda juga mengatur judul, gambar utama, kategori, dan cuplikan singkat, lalu menyimpan artikel.

**Lokasi:** menu **Kampanye** › pilih kampanye › **Penyedia Konten** › tombol **Generate Konten**, **Tulis Sendiri**, atau **Edit Artikel** di baris artikel
**Untuk:** Klien dengan peran Writer atau lebih tinggi pada brand

## Gambaran singkat

Pengelola Naskah bekerja untuk satu artikel pada satu media. Ada tiga tab:

| Tab | Fungsi | Penjelasan |
|---|---|---|
| **Generate dengan AI** | Meminta The Writer menulis draf dari topik dan angle | [Menulis artikel dengan AI](23-menulis-artikel-dengan-ai.md) |
| **Unggah Dokumen** | Memasukkan naskah dari berkas DOCX, TXT, PDF, atau teks tempelan | [Mengunggah dokumen naskah](24-mengunggah-dokumen-naskah.md) |
| **Editor Post** | Menyunting naskah, mengatur gambar utama, kategori, dan cuplikan, lalu menyimpan | Halaman ini |

Tab yang tampil mengikuti cara Anda membuka jendela:

| Pintu masuk | Tab yang tampil |
|---|---|
| **Generate Konten** | **Generate dengan AI** dan **Editor Post** |
| **Tulis Sendiri** | **Unggah Dokumen** dan **Editor Post** |
| **Edit Artikel** pada artikel hasil AI | **Generate dengan AI** dan **Editor Post** |
| **Edit Artikel** pada artikel unggahan | **Unggah Dokumen** dan **Editor Post** |
| **Lihat Artikel** (artikel terkunci atau kampanye selesai) | Hanya isi artikel, tanpa tab |

Pada **Generate Konten**, tab **Editor Post** terkunci sampai draf pertama ada. Mengarahkan kursor ke tab itu menampilkan "Anda harus bikin artikel dengan AI dulu". Tab **Generate dengan AI** diberi tanda **Beta**; hasil AI perlu Anda kaji sebelum diterbitkan.

## Yang tampil di layar

### Header

Header memuat nama jendela, domain media (huruf pertama kapital), dan beberapa chip:

- **Media Mainstream** atau **Media Blogger**: jenis media tujuan.
- Chip aturan panjang, misalnya "300–700 kata", bila Mitra menetapkan rentang kata. Chip ini tidak tampil untuk media yang membebaskan panjang artikel.
- **N link**: jumlah tautan di isi artikel. Chip berubah merah bila melebihi batas media. Arahkan kursor untuk melihat kapasitas, misalnya "Kapasitas tautan media: 2/2".
- **N gambar**: hanya menghitung gambar utama (0 atau 1). Gambar di dalam isi tidak dihitung. Chip merah berarti gambar utama belum ada.

### Tab Editor Post

Kolom kiri berisi:

- **Judul Artikel** (maksimal 300 karakter).
- **Isi Artikel (Konten)** dengan penghitung kata di sisi kanan labelnya. Editor teks dijelaskan di [Editor teks naskah](25-editor-teks-naskah.md).
- Kotak **Gambar Otomatis Artikel** untuk menyisipkan gambar AI ke dalam isi. Lihat [Gambar utama dan Pustaka Gambar](26-gambar-utama-dan-pustaka-gambar.md).

Kolom kanan berisi:

- **Gambar Utama** (WAJIB). Anda bisa mengunggah gambar, memilih dari pustaka, membuat dengan AI, atau memakai thumbnail default Tauthub.
- **Kategori / Rubrik** (WAJIB). Pilihannya adalah daftar kategori yang ditentukan Mitra untuk media itu. Setelah draf AI jadi, rubrik terpilih otomatis dengan keterangan "Dipilih otomatis saat draf AI jadi — ubah bila perlu."
- **CUPLIKAN SINGKAT (META DESC)** (OPSIONAL): ringkasan satu sampai dua kalimat untuk hasil pencarian, maksimal 500 karakter. Draf AI mengisinya otomatis.

Di HP, kolom kanan pindah ke bawah kolom isi.

### Footer

Footer memuat tombol **Batal** (menjadi **Tutup** pada mode lihat saja), pesan status, dan tombol **Simpan Artikel**.

## Cara menggunakan

### Cara menyimpan artikel

1. Isi **Judul Artikel**.
2. Tulis atau tempel isi artikel, atau buat dengan AI.
3. Atur **Gambar Utama**.
4. Pilih **Kategori / Rubrik**.
5. Isi **CUPLIKAN SINGKAT (META DESC)** bila perlu.
6. Klik **Simpan Artikel**.

Tombol **Simpan Artikel** baru aktif bila judul, isi, gambar utama, dan kategori sudah ada, dan jumlah tautan tidak melebihi batas media. Pesan di footer menyebut yang kurang, misalnya "Lengkapi judul, kategori, dan thumbnail untuk menyimpan." atau "Kurangi tautan agar sesuai batas media. (3/2)".

Setelah tersimpan, jendela menutup dan kartu artikel menampilkan **Memindai QC…**. Pemeriksaan QC berjalan otomatis untuk artikel itu. Lihat [Cek Kualitas Naskah](05-cek-kualitas-naskah.md).

### Cara menulis ulang artikel hasil AI

1. Klik **Edit Artikel** pada artikel itu.
2. Buka tab **Generate dengan AI**.
3. Ubah angle, tautan, atau topik bila perlu.
4. Klik **Buat Draf dengan AI**, pilih opsi, lalu klik **Buat Draf**.

Draf baru langsung menimpa naskah sebelumnya, bahkan sebelum Anda mengklik **Simpan Artikel**. Versi lama tidak bisa dikembalikan, dan suntingan yang belum disimpan ikut hilang. Penulisan ulang ditagih sesuai tarif; lihat [Tagihan di Penyedia Konten](20-tagihan-di-penyedia-konten.md).

### Cara mengganti naskah dengan dokumen lain

1. Klik **Edit Artikel** pada artikel unggahan.
2. Buka tab **Unggah Dokumen** dan unggah berkas baru.
3. Periksa hasilnya di tab **Editor Post**, lalu klik **Simpan Artikel**.

### Cara menutup jendela

Klik **Batal** atau ikon silang. Saat AI sedang menulis atau dokumen sedang diproses, jendela tidak bisa ditutup dan hanya muncul peringatan "Draf sedang disusun — tunggu sampai prosesnya selesai sebelum menutup." Saat artikel sedang disimpan, tombol tutup juga tertahan.

## Aturan per jenis media

Jenis media menentukan gaya tulisan AI dan beberapa batas editor.

| Hal | Mainstream | Blogger |
|---|---|---|
| Gaya AI | Berita atau feature objektif, paragraf pendek mengalir, tanpa bahasa sales berlebihan | Gaya santai mengalir dengan subjudul dan daftar, tetap akurat |
| Subjudul dan daftar poin | Pada umumnya tanpa | Pada umumnya dengan |
| Kutipan resmi (gaya **Yang Proper**) | Wajib diisi | Blok kutipan tidak tampil |

Aturan berikut ditetapkan Mitra per media dan berlaku untuk kedua jenis:

- Panjang artikel: rentang kata tertentu atau bebas. Bila bebas, AI memakai pedoman bawaan: Mainstream minimal 300 kata, Blogger 500 sampai 5.000 kata.
- Tautan: jumlah maksimal, dan jenisnya (Dofollow, Nofollow, atau Sponsored). Media yang tidak menerima tautan menampilkan "Media ini tidak mengizinkan tautan dalam artikel." di tab **Generate dengan AI**.
- Subjudul: Mitra boleh mengizinkan atau melarang subjudul untuk medianya, dan pengaturan itu menggantikan kebiasaan di tabel.
- Peta dan video: bila media tidak mengizinkan, tombolnya terkunci dengan keterangan "Media ini tidak mengizinkan embed."
- Kategori, bahasa artikel, dan topik terlarang: AI mengikuti daftar kategori, bahasa utama, dan topik yang dilarang untuk media itu.

## Pesan yang mungkin muncul

| Pesan | Artinya | Langkah Anda |
|---|---|---|
| "Artikel sudah masuk jadwal terbit… artikel terkunci" | Artikel sudah dikirim atau terjadwal | Hubungi admin bila perlu revisi |
| "Kampanye sudah selesai — mode lihat saja. Hubungi admin kalau perlu revisi." | Kampanye berstatus **Terjadwal** atau **Terkirim** | Buka **Ringkasan** |
| "Gagal menyimpan artikel." | Penyimpanan gagal | Periksa koneksi, lalu klik **Simpan Artikel** lagi |
| "Kategori harus dipilih dari kategori media yang ditentukan mitra." | Kategori tidak ada di daftar media | Pilih ulang dari daftar |
| "Artikel belum tersedia." | Naskah yang ingin dibuka tidak ditemukan | Muat ulang halaman, lalu buat naskahnya |

## Pertanyaan yang sering diajukan

**Apakah isian tab Generate dengan AI hilang bila saya menutup jendela?**
Tidak. Isian tersimpan sementara di browser per artikel selama sesi browser yang sama, lalu terisi lagi saat jendela dibuka.

**Kenapa tombol Simpan Artikel nonaktif?**
Salah satu dari judul, isi, gambar utama, atau kategori belum ada, atau tautan melebihi batas. Baca pesan di footer.

**Bagaimana bila daftar Kategori / Rubrik kosong?**
Daftar itu diatur Mitra pemilik media. Tombol **Simpan Artikel** tetap nonaktif sampai kategori dipilih, jadi hubungi tim Tauthub bila daftar kosong.

## Lihat juga

- [Menulis artikel dengan AI](23-menulis-artikel-dengan-ai.md)
- [Mengunggah dokumen naskah](24-mengunggah-dokumen-naskah.md)
- [Editor teks naskah](25-editor-teks-naskah.md)
- [Gambar utama dan Pustaka Gambar](26-gambar-utama-dan-pustaka-gambar.md)
- [Cek Kualitas Naskah](05-cek-kualitas-naskah.md)
- [Penyedia Konten](03-penyedia-konten.md)
