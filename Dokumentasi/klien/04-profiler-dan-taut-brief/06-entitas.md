# Entitas

> Tab Entitas menampilkan data brand yang sudah tersimpan di brief, dalam lima kartu, beserta persentase kelengkapan isian wajib dan daftar isian yang belum terisi. Isian bisa diubah langsung lewat ikon pensil di tiap baris. Untuk mengisi dari nol atau mengubah arti isian, pakai Copilot Chat atau formulir Instan Brief.

**Lokasi:** menu **Kampanye** › **Profiler** › pilih pintu masuk › tab **Entitas**

**Untuk:** Klien. Semua peran dapat melihat tab ini.

## Gambaran singkat

Di tab ini, brand Anda dilihat sebagai satu entitas. Entitas artinya satu sosok yang jelas, dengan nama resmi, kategori, fakta, dan aturan yang harus sama di setiap artikel. Brief artinya catatan tertulis tentang brand Anda. Kartu pertama menyebut datanya sebagai data resmi yang dipakai sistem pencarian dan AI untuk mengenali brand.

Tab **Entitas** diperbarui otomatis saat Tubi mengisi data di **Copilot Chat**, dan setelah Anda menyimpan formulir di **Instan Brief**. Lencana tab menampilkan persentase kelengkapan: hijau pada 100%, kuning kalau di bawahnya.

## Yang tampil di layar

### Kartu kelengkapan

Kartu paling atas berisi bilah progres dan teks berbentuk "X / Y field wajib terisi · Z sumber dibaca". X adalah jumlah isian wajib yang sudah terisi. Y adalah total isian wajib. Z adalah jumlah sumber (dokumen dan tautan) yang tercatat untuk brief ini. Lencana persentase ada di sisi kanan. Kalau semua isian wajib sudah terisi tetapi masih ada isian opsional yang kosong, teks bertambah "· N field opsional masih kosong". Lalu muncul tautan **Lengkapi field opsional lewat Copilot →**.

### Lima kartu isi

| Kartu | Isi |
|---|---|
| **Identitas Inti Entitas (Core Entity)** | Baris **Nama resmi entitas**, **Kategori entitas**, **Deskripsi resmi**, dan **Teks "Tentang [brand]"**. |
| **Batasan & Panduan Narasi Media (Guardrails Advertorial)** | Kotak hijau **PILAR NARASI UTAMA (DIANJURKAN)** berisi klaim yang boleh dipakai. Kotak merah **LARANGAN KERAS (NEGATIVE CONSTRAINTS)** berisi aturan klaim serta kata dan frasa terlarang. |
| **Fakta Resmi & Sinyal Entitas (Entity Facts)** | Baris **Fakta resmi (tahun, jumlah, lokasi)**, **Keunggulan unik (USP)**, **Fakta penting yang wajib dipertahankan**, **Wilayah operasional**, dan **Website resmi (canonical domain)**. |
| **Target Pertanyaan AI (GEO Target Query)** | Daftar pertanyaan yang ingin Anda menangkan di jawaban mesin AI. Tiap pertanyaan punya lencana **Sinyal Siap** atau **Belum terverifikasi**. |
| **Sebutan, Kanal & Target Konten** | Tiga bagian: **Nama alternatif atau alias** (tampil sebagai label kecil), **Kanal resmi** (daftar tautan yang bisa diklik), dan **Keyword prioritas dan URL target** (satu baris per pasangan keyword dan tautannya). Teks miring **Belum diisi. Lengkapi lewat Instan Brief atau obrolan Copilot.** tampil kalau bagian itu masih kosong. |

### Asal tiap baris

| Yang tampil di tab Entitas | Berasal dari |
|---|---|
| **Nama resmi entitas** | **Nama resmi brand** |
| **Kategori entitas** | **Kategori brand** |
| **Deskripsi resmi** | **Deskripsi resmi** |
| **Teks "Tentang [brand]"** | **Boilerplate "Tentang brand"** |
| **PILAR NARASI UTAMA** | **Klaim yang boleh dipakai** |
| **LARANGAN KERAS** | **Aturan klaim dan batasan penulisan**, **Kata atau frasa yang harus dihindari**, dan isian sejenis dari gaya penulisan kalau ada |
| **Fakta resmi** | **Fakta resmi** |
| **Keunggulan unik (USP)**, **Fakta penting yang wajib dipertahankan**, **Wilayah operasional**, **Website resmi** | Isian tambahan yang diisi lewat Copilot Chat |
| **Target Pertanyaan AI** | **Target pertanyaan di mesin pencari AI** |

Nama kolom formulir ada di [Instan Brief: Profil Brand](04-instan-brief-profil-brand.md).

### Keadaan kosong

Kartu atau baris yang belum terisi menampilkan teks miring seperti **Belum diisi. Lengkapi lewat obrolan Copilot.**, **Belum ada. Lengkapi lewat obrolan Copilot.**, atau **Belum ada pertanyaan target. Lengkapi lewat obrolan Copilot.** Isilah lewat [Copilot Chat](03-copilot-chat.md) atau formulir **Instan Brief**.

## Cara menggunakan

### Cara memeriksa data brand

1. Buka tab **Entitas**.
2. Baca kartu kelengkapan di bagian atas untuk melihat berapa isian wajib yang sudah terisi.
3. Baca keempat kartu satu per satu. Periksa terutama angka, tahun, dan klaim.
4. Kalau ada yang salah atau kosong, perbaiki lewat **Copilot Chat** atau **Instan Brief**.

### Cara melengkapi isian opsional lewat Copilot

1. Pastikan persentase sudah 100% dan teks **N field opsional masih kosong** tampil.
2. Klik **Lengkapi field opsional lewat Copilot →**.
3. Tauthub pindah ke tab **Copilot Chat** dan mengirim pesan atas nama Anda. Pesan itu meminta Tubi, asisten AI, menawarkan isian untuk isian opsional yang masih kosong, paling banyak sepuluh, dimulai dari Keunggulan (USP) dan Fakta penting. Pesan itu tampil di obrolan.
4. Tanggapi tawaran Tubi: pakai, ubah, atau lewati.

## Persentase dan artinya

| Tampilan | Artinya |
|---|---|
| Lencana hijau 100% | Semua isian wajib sudah terisi. |
| Lencana kuning di bawah 100% | Masih ada isian wajib yang kosong. |
| **Belum terisi:** diikuti nama isian | Muncul di bawah bilah progres selama persentase belum 100%. Isinya nama isian wajib yang masih kosong, supaya Anda tahu persis apa yang menahan angkanya. |
| **N field opsional masih kosong** | Ada isian opsional yang belum terisi, misalnya keunggulan, fakta penting, gaya penulisan, atau pedoman konten. Isian ini tidak dihitung dalam persentase. |
| **Sinyal Siap** | Pertanyaan target sudah tersimpan sebagai isian yang terisi atau terkonfirmasi. |
| **Belum terverifikasi** | Pertanyaan tampil, tetapi isiannya belum berstatus terisi atau terkonfirmasi. |

## Aturan dan batasan

- Ikon pensil muncul di baris teks (nama resmi, deskripsi, boilerplate, fakta, USP, wilayah, website, alias, kanal resmi, keyword dan URL target) serta di daftar **PILAR NARASI UTAMA** dan **Target Pertanyaan AI**. Klik pensil, ubah teksnya, lalu **Simpan**. Hanya satu isian yang bisa diubah pada satu waktu. Kategori, serta aturan klaim dan kata terlarang di kotak **LARANGAN KERAS**, diubah lewat Instan Brief atau Copilot Chat.
- Isian tidak bisa dikosongkan dari sini. Kalau teksnya dikosongkan lalu disimpan, muncul pesan penolakan.
- Peran Writer atau lebih tinggi diperlukan untuk menyimpan. Peran di bawahnya mendapat pesan bahwa perannya belum bisa mengubah bagian ini.
- Setelah isian diubah, **Ringkasan Entitas** menjadi usang. Muncul banner **Ringkasan belum diperbarui** dengan tombol **Perbarui ringkasan**. Artikel gaya **Yang Proper** memakai ringkasan, jadi tekan tombol itu sebelum menulis artikel.
- Angka 100% hanya berarti isian wajib sudah terisi. Isian opsional tidak dihitung, jadi Keunggulan atau Fakta penting bisa saja masih kosong.
- Tim Tauthub yang menentukan daftar isian wajib, dan daftar itu bisa berubah.
- Tab ini tidak menampilkan semua isian. **Nama alternatif atau alias**, **Kanal resmi**, **Keyword prioritas dan URL target**, serta isian gaya penulisan dan pedoman konten tidak tampil di sini. Isinya tersimpan dan ikut menjadi bahan [Ringkasan brief](08-ringkasan-brief.md).
- Teks yang ditulis satu per baris tampil sebagai butir terpisah di **PILAR NARASI UTAMA**, **LARANGAN KERAS**, dan daftar pertanyaan. Teks yang dipisah titik koma tampil dalam satu butir. Tulis satu item per baris kalau Anda ingin butir terpisah.
- Anda tidak perlu menekan **Simpan Draf** di bagian atas halaman agar data tampil di tab ini.

## Pertanyaan yang sering diajukan

**Mengapa kartu saya kosong padahal saya sudah mengunggah dokumen?**
Mengunggah dokumen tidak mengisi tab ini. Minta Tubi membaca dokumen di **Copilot Chat**, atau isi formulir **Instan Brief**.

**Persentase sudah 100%, tetapi Keunggulan unik masih kosong. Apakah ada yang salah?**
Tidak. Keunggulan unik termasuk isian opsional. Klik **Lengkapi field opsional lewat Copilot →** kalau Anda ingin mengisinya.

**Bisakah saya mengedit langsung di tab Entitas?**
Bisa, lewat ikon pensil di tiap baris. Setelah menyimpan, perbarui ringkasannya lewat banner di bagian atas supaya perubahan ikut ke artikel.

**Apa fungsi tombol Buat artikel di Target Pertanyaan AI?**
Tombol itu menyimpan pertanyaan sebagai topik artikel, menjadikan brief ini brief aktif, lalu membuka daftar **Kampanye**. Di sana ada petunjuk dengan topiknya. Buka satu kampanye, lalu Pengelola Naskah: topik terisi sendiri, gaya **Yang Proper** terpilih, dan brief aktif sudah terpilih di **Memori Brand**. Topik dipakai sekali dan hilang setelah 24 jam. Tombol **Batalkan topik** di banner menghapusnya.

**Apa arti lencana Sinyal Siap?**
Pertanyaan itu sudah tersimpan sebagai isian yang terisi atau terkonfirmasi. Lencana ini tidak menjamin brand Anda muncul dalam jawaban mesin AI.

## Lihat juga

- [Copilot Chat](03-copilot-chat.md)
- [Instan Brief: Profil Brand](04-instan-brief-profil-brand.md)
- [Ringkasan brief](08-ringkasan-brief.md)
