# The Profiler (jalur lama)

> The Profiler adalah langkah pertama pada alur kampanye lama. Anda mengisi keterangan tentang bisnis, tujuan, target media, dan anggaran. Lalu AI (kecerdasan buatan) menyusun dokumen Executive PR Brief. Halaman ini hanya muncul untuk kampanye lama yang belum punya media.

**Lokasi:** menu **Kampanye** › kartu kampanye lama › **Buka Kampanye**  
**Untuk:** Klien dengan peran Writer atau lebih tinggi

> **Catatan:** Di layar tertulis "Jalur lama" disusul keterangan bahwa jalur ini hanya untuk kampanye yang dibuat sebelum alur Kilat/Katalog Media. Kampanye baru dimulai dari Katalog Media (lihat [Kampanye](README.md)). Jangan tertukar dengan tombol **Profiler** di halaman Manajemen Kampanye. Tombol itu membuka Taut Brief ([Profiler dan Taut Brief](../04-profiler-dan-taut-brief/README.md)).

## Gambaran singkat

The Profiler (di layar tertulis "Agent 1: The Profiler") mengumpulkan bahan kampanye dalam satu formulir yang terdiri dari empat bagian. Setelah Anda menjalankan analisis, AI menyusun **Executive PR Brief**. Ini ringkasan strategi humas yang berisi tujuan, audiens (orang yang ingin Anda jangkau), juru bicara, aturan merek, dan petunjuk menulis untuk tiap jalur media. Dokumen itu menjadi dasar untuk memilih media di [The Matchmaker](11-the-matchmaker.md).

## Kapan halaman ini terbuka

**Buka Kampanye** membawa Anda ke The Profiler bila kampanye berstatus **Draft**, **Profiling**, **Bahan**, atau **Anggaran**, belum punya media, dan bukan kampanye Kilat. Kampanye berstatus **Profiled** yang belum punya hasil analisis juga dibawa ke sini. Bila hasil analisis sudah ada, halaman langsung beralih ke ringkasan Executive PR Brief.

## Yang tampil di layar

Bagian atas menampilkan judul "Agent 1: The Profiler", keterangan "Susun Executive PR Brief dari konteks bisnis, tujuan kampanye, target media, dan anggaran Anda.", saldo Anda dalam rupiah, dan status simpan (**Tersimpan otomatis**, **Menyimpan...**, **Tersimpan**, atau **Gagal**). Isian disimpan sendiri sebagai draf sesaat setelah Anda berhenti mengetik.

| Bagian | Isian | Aturan |
|---|---|---|
| **01 Konteks Bisnis & Tujuan Kampanye** | **Auto-Fill Profil Bisnis** (kolom URL dan tombol **Tarik Data**), **Tujuan Utama Kampanye**, **Target Audiens Spesifik**, **Guardrails (Do's & Don'ts Merek)** | Tujuan wajib dipilih. Bila Anda memilih **Other / Lainnya** pada audiens, kolom **Target Audiens Lainnya** wajib diisi. |
| **02 Penanaman Backlink & SEO** | Kotak centang **Aktifkan Penanaman Backlink (Hyperlink)**, **URL Target Utama**, **Anchor Text Utama**, dan **Gunakan AI Smart Variations** | Boleh tidak diisi. Bila dinyalakan, URL dan anchor text wajib diisi. |
| **03 Target Media & Bahan Baku** | **Jalur Distribusi (Multi-Kanal)** beserta **Angle Artikel**, **Juru Bicara / Narasumber**, dan **Bahan Referensi Kampanye** | Pilih satu atau dua jalur. Tiap jalur wajib punya angle. Jalur Mainstream wajib punya narasumber lengkap. |
| **04 Kendali Anggaran** | **Mode Efisien (Kalkulator)** dengan **Nominal Anggaran (Rupiah)**, atau **Mode Optimal (Strategis)** | Pada Mode Efisien, nominal wajib diisi, harus dalam kisaran yang tertera, dan tidak boleh melebihi saldo. |

Penjelasan istilah di tabel: backlink adalah tautan dari artikel ke website Anda. Anchor text adalah kata yang bisa diklik untuk menuju tautan itu. SEO adalah usaha agar website Anda muncul di urutan atas hasil pencarian. Angle adalah sudut cerita artikel. Guardrails adalah batasan yang harus dipatuhi AI saat menulis tentang merek Anda.

Bilah bawah menampilkan pesan yang menyebut isian mana yang masih kurang. Atau bilah itu menampilkan "Semua perubahan disimpan otomatis sebagai draft." di sebelah tombol **Jalankan Analisis Profiler**.

## Cara menggunakan

### Cara mengisi formulir

1. Pada **Auto-Fill Profil Bisnis**, isi alamat website brand lalu klik **Tarik Data**. AI membaca website (tertulis "rata-rata 30–45 dtk") dan mengisi sebagian isian di bawahnya. Profil bisnis (nama usaha, industri, nilai jual) hanya terisi lewat langkah ini dan tidak bisa diketik sendiri.
2. Periksa lagi hasil **Tarik Data**. Langkah ini bisa mengganti **Tujuan Utama Kampanye**, **Target Audiens Spesifik**, **Guardrails**, dan **Jalur Distribusi** dengan hasil bacaan AI.
3. Pilih satu dari empat **Tujuan Utama Kampanye**: **Brand Awareness & Edukasi**, **Dominasi SEO & Backlink**, **Konversi & Promosi Langsung**, atau **Manajemen Reputasi / Klarifikasi**.
4. Klik **Pilih audiens…** untuk memilih target audiens. Pakai kolom pencarian bila daftarnya panjang.
5. Tulis aturan merek di **Guardrails**, satu aturan per baris. Isi kolom ini dengan kata yang dilarang, pesaing yang tidak boleh disebut, atau klaim yang tidak boleh dibuat. Klik **Expand Guardrails** untuk memperluas kotaknya.
6. Nyalakan backlink bila kampanye membutuhkan tautan. Isi **URL Target Utama** dan **Anchor Text Utama**.
7. Centang jalur distribusi (paling banyak dua), lalu pilih **Angle Artikel** untuk tiap jalur.
8. Untuk jalur Mainstream, klik **+ Tambah Narasumber** lalu isi nama, jabatan, dan kutipan. Klik **Hapus** atau **Hapus semua** untuk membuang narasumber.
9. Pilih **Bahan Referensi Kampanye**. Pilih **Saya Punya Dokumen (Upload)** untuk mengunggah berkas. Atau pilih **Saya Tidak Punya Bahan** untuk menjawab pertanyaan singkat (satu kalimat) pada **Dynamic PR Template**.
10. Pilih mode anggaran di **Kendali Anggaran**.

Unggahan menerima berkas PDF, DOCX, dan TXT, masing-masing paling besar 10 MB. Setiap berkas menampilkan status **Siap dipakai AI**, **Memproses isi dokumen…**, **Menunggu proses**, atau **Gagal dibaca**. Klik ikon tempat sampah untuk menghapus berkas.

### Cara menjalankan analisis

1. Pastikan bilah bawah tidak menampilkan pesan peringatan. Tombol **Jalankan Analisis Profiler** nonaktif selama masih ada isian wajib yang kurang.
2. Klik **Jalankan Analisis Profiler**. Halaman "Profiler Sedang Bekerja" muncul dengan persen kemajuan dan keterangan langkah.
3. Tunggu sampai muncul tanda centang dan halaman berpindah ke ringkasan. Proses bisa berlangsung beberapa menit.

Bila halaman dimuat ulang saat analisis berjalan, halaman memeriksa hasilnya tiap beberapa detik. Bila lebih dari 10 menit tidak ada hasil, muncul "Koneksi terputus" dengan tombol **Kembali ke form**.

### Cara membaca Executive PR Brief dan melanjutkan

Halaman ringkasan berjudul "Executive PR Brief Selesai". Isinya:

- **Kompilasi Strategi Bisnis** (**Tujuan**, **Juru Bicara**, **Target Audiens**, **Guardrails**, dan **Gambaran**).
- **Instruksi Anatomi Penulisan (Terkunci)** untuk tiap jalur media dan angle.
- **Catatan Strategis & Verifikasi**. Pada butir **Perlu verifikasi**, klik **Tanggapi**, lalu isi jawaban dan klik **Tambahkan**. Atau klik **Abaiakan** untuk melewatinya.
- Panel samping **Ringkasan**, **Insight Utama**, dan **Detail Sumber**.

Klik **Lanjut ke Matchmaker** untuk memilih media ([The Matchmaker](11-the-matchmaker.md)). Klik **Edit ulang isian profiler** untuk kembali ke formulir. Hasil analisis yang lama dihapus. Jadi Anda perlu menjalankan analisis lagi.

## Aturan dan batasan

- Anda butuh peran Writer atau lebih tinggi untuk menyimpan draf dan menjalankan analisis.
- Jawaban pada butir **Perlu verifikasi** paling banyak 2.000 huruf.
- Saldo harus cukup. Bila nominal anggaran Mode Efisien melebihi saldo, bilah bawah menampilkan "Saldo tidak mencukupi. Top Up dulu sebelum menjalankan analisis." dan tombol analisis nonaktif.
- Pada kampanye yang sudah selesai, ringkasan hanya bisa dibaca. Bila Anda memilih **Edit ulang isian profiler**, muncul penolakan: "Kampanye sudah selesai" disusul "hasil analisis tidak bisa diubah lagi."
- Di layar lebar, ringkasan tersusun dua kolom. Di HP semuanya tersusun ke bawah.

## Biaya

Menjalankan analisis memotong saldo Anda. Jumlah potongan tidak ditampilkan di halaman ini. Cara melihat dan mengisi saldo ada di [Saldo dan Top Up](../08-saldo-dan-top-up.md). Membaca website lewat **Tarik Data** tidak memotong saldo.

## Pesan galat yang umum

| Pesan | Penyebab | Yang bisa Anda lakukan |
|---|---|---|
| **Media Mainstream membutuhkan minimal 1 narasumber lengkap (nama, jabatan, kutipan).** | Jalur Mainstream dipilih tanpa narasumber lengkap. | Isi nama, jabatan, dan kutipan narasumber. |
| **Pilih angle untuk channel …** | Ada jalur terpilih tanpa angle. | Pilih angle pada jalur itu. |
| **Upload minimal satu dokumen bahan referensi.** | Pilihan "Saya Punya Dokumen" menyala tanpa berkas. | Unggah berkas atau ganti ke "Saya Tidak Punya Bahan". |
| **Anggaran minimum belum terpenuhi. Isi minimal …** | Nominal Mode Efisien di bawah batas. | Naikkan nominal sesuai kisaran yang tertera pada label kolom. |
| **… melebihi 10 MB.** atau **Tipe file tidak diizinkan. Izinkan: PDF, DOCX, TXT.** | Berkas terlalu besar atau jenisnya tidak didukung. | Pakai berkas PDF, DOCX, atau TXT di bawah 10 MB. |
| **Website belum dapat dibaca otomatis (…). Anda tetap dapat melanjutkan dengan mengisi informasi secara manual.** | Website menolak akses, halaman tidak ditemukan, atau isinya kosong. | Coba alamat halaman utama, atau lanjutkan dengan mengisi isian lain. |
| **Saldo tidak mencukupi untuk menjalankan kampanye ini. Silakan Top Up Saldo terlebih dahulu.** | Saldo tidak cukup untuk analisis. | Isi saldo, lalu jalankan analisis lagi. |
| **Analisis AI mengalami kendala sementara. Draft Anda tetap tersimpan.** | AI gagal menghasilkan dokumen yang sah. | Klik **Kembali ke form**, lalu jalankan analisis lagi. |

## Pertanyaan yang sering diajukan

**Apakah The Profiler sama dengan tombol Profiler di halaman Manajemen Kampanye?**
Tidak. Tombol **Profiler** membuka Taut Brief. The Profiler di halaman ini hanya untuk kampanye lama.

**Mengapa saya tidak bisa mengetik nama usaha dan industri?**
Kedua data itu terisi dari hasil **Tarik Data**. Formulir ini tidak punya kolom ketik untuk keduanya.

**Apakah isian saya hilang bila halaman tertutup?**
Tidak. Isian disimpan sendiri sebagai draf. Buka kembali kampanye lewat **Buka Kampanye** untuk melanjutkan.

## Lihat juga

- [Kampanye: alur dan tahap](README.md)
- [The Matchmaker (jalur lama)](11-the-matchmaker.md)
- [Profiler dan Taut Brief](../04-profiler-dan-taut-brief/README.md)
- [Saldo dan Top Up](../08-saldo-dan-top-up.md)
