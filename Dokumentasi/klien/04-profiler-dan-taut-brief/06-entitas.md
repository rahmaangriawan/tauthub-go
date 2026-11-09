# Entitas

> Tab Entitas menampilkan data brand yang sudah tersimpan di brief, dikelompokkan dalam empat kartu, beserta persentase kelengkapan isian wajib. Tab ini hanya untuk membaca. Untuk mengisi atau mengubah data, gunakan Copilot Chat atau formulir Instan Brief.

**Lokasi:** menu **Kampanye** › **Profiler** › pilih gerbang › tab **Entitas**

**Untuk:** Klien. Semua peran dapat melihat tab ini.

## Gambaran singkat

Di tab ini brand Anda dilihat sebagai satu "entitas": sosok dengan nama resmi, kategori, fakta, dan aturan yang harus sama di setiap artikel. Kartu pertama menyebut datanya sebagai data resmi yang dipakai sistem pencarian dan AI untuk mengenali brand.

Tab **Entitas** diperbarui otomatis saat Tubi mengisi data di **Copilot Chat**, dan setelah Anda menyimpan formulir di **Instan Brief**. Lencana tab menampilkan persentase kelengkapan: hijau pada 100%, kuning di bawahnya.

## Yang tampil di layar

### Kartu kelengkapan

Kartu di bagian paling atas berisi bilah progres dan teks berbentuk "X / Y field wajib terisi · Z sumber dibaca". X adalah jumlah isian wajib yang sudah terisi, Y adalah total isian wajib, dan Z adalah jumlah sumber (dokumen dan tautan) yang tercatat untuk brief ini. Lencana persentase ada di sisi kanan. Jika semua isian wajib sudah terisi tetapi isian opsional masih ada yang kosong, teks bertambah "· N field opsional masih kosong" dan muncul tautan **Lengkapi field opsional lewat Copilot →**.

### Empat kartu isi

| Kartu | Isi |
|---|---|
| **Identitas Inti Entitas (Core Entity)** | Baris **Nama resmi entitas**, **Kategori entitas**, **Deskripsi resmi**, dan **Teks "Tentang [brand]"**. |
| **Batasan & Panduan Narasi Media (Guardrails Advertorial)** | Kotak hijau **PILAR NARASI UTAMA (DIANJURKAN)** berisi klaim yang boleh dipakai. Kotak merah **LARANGAN KERAS (NEGATIVE CONSTRAINTS)** berisi aturan klaim serta kata dan frasa terlarang. |
| **Fakta Resmi & Sinyal Entitas (Entity Facts)** | Baris **Fakta resmi (tahun, jumlah, lokasi)**, **Keunggulan unik (USP)**, **Fakta penting yang wajib dipertahankan**, **Wilayah operasional**, dan **Website resmi (canonical domain)**. |
| **Target Pertanyaan AI (GEO Target Query)** | Daftar pertanyaan yang ingin Anda menangkan di jawaban mesin AI. Tiap pertanyaan punya lencana **Sinyal Siap** atau **Belum terverifikasi**. |

### Asal tiap baris

| Yang tampil di tab Entitas | Berasal dari |
|---|---|
| **Nama resmi entitas** | **Nama resmi brand** |
| **Kategori entitas** | **Kategori brand** |
| **Deskripsi resmi** | **Deskripsi resmi** |
| **Teks "Tentang [brand]"** | **Boilerplate "Tentang brand"** |
| **PILAR NARASI UTAMA** | **Klaim yang boleh dipakai** |
| **LARANGAN KERAS** | **Aturan klaim dan batasan penulisan**, **Kata atau frasa yang harus dihindari**, dan isian sejenis dari gaya penulisan bila ada |
| **Fakta resmi** | **Fakta resmi** |
| **Keunggulan unik (USP)**, **Fakta penting yang wajib dipertahankan**, **Wilayah operasional**, **Website resmi** | Isian tambahan yang diisi lewat Copilot Chat |
| **Target Pertanyaan AI** | **Target pertanyaan di mesin pencari AI** |

Nama kolom formulir ada di [Instan Brief: Profil Brand](04-instan-brief-profil-brand.md).

### Keadaan kosong

Kartu atau baris yang belum terisi menampilkan teks miring seperti **Belum diisi. Lengkapi lewat obrolan Copilot.**, **Belum ada. Lengkapi lewat obrolan Copilot.**, atau **Belum ada pertanyaan target. Lengkapi lewat obrolan Copilot.** Isi lewat [Copilot Chat](03-copilot-chat.md) atau formulir **Instan Brief**.

## Cara menggunakan

### Cara memeriksa data brand

1. Buka tab **Entitas**.
2. Baca kartu kelengkapan di bagian atas untuk melihat berapa isian wajib yang sudah terisi.
3. Telusuri empat kartu. Periksa terutama angka, tahun, dan klaim.
4. Jika ada yang salah atau kosong, perbaiki lewat **Copilot Chat** atau **Instan Brief**.

### Cara melengkapi isian opsional lewat Copilot

1. Pastikan persentase sudah 100% dan teks **N field opsional masih kosong** tampil.
2. Klik **Lengkapi field opsional lewat Copilot →**.
3. Tauthub berpindah ke tab **Copilot Chat** dan mengirim pesan atas nama Anda. Pesan itu meminta Tubi menawarkan isian untuk isian opsional yang masih kosong, paling banyak sepuluh, dimulai dari Keunggulan (USP) dan Fakta penting. Pesan itu tampil di percakapan.
4. Jawab tawaran Tubi dengan memakai, mengubah, atau melewatinya.

## Persentase dan artinya

| Tampilan | Artinya |
|---|---|
| Lencana hijau 100% | Semua isian wajib sudah terisi. |
| Lencana kuning di bawah 100% | Masih ada isian wajib yang kosong. |
| **N field opsional masih kosong** | Ada isian opsional, misalnya keunggulan, fakta penting, gaya penulisan, atau pedoman konten, yang belum terisi. Isian ini tidak dihitung dalam persentase. |
| **Sinyal Siap** | Pertanyaan target sudah tersimpan sebagai isian yang terisi atau terkonfirmasi. |
| **Belum terverifikasi** | Pertanyaan tampil, tetapi isiannya belum berstatus terisi atau terkonfirmasi. |

## Aturan dan batasan

- Tab ini tidak punya kolom atau tombol untuk mengedit.
- Angka 100% hanya berarti isian wajib sudah terisi. Isian opsional tidak dihitung, jadi Keunggulan atau Fakta penting bisa saja masih kosong.
- Daftar isian wajib ditentukan tim Tauthub dan bisa berubah.
- Tab ini tidak menampilkan semua isian. **Nama alternatif atau alias**, **Kanal resmi**, **Keyword prioritas dan URL target**, serta isian gaya penulisan dan pedoman konten tidak tampil di sini. Isinya tersimpan dan ikut menjadi bahan [Ringkasan brief](08-ringkasan-brief.md).
- Teks yang ditulis satu per baris tampil sebagai butir terpisah di **PILAR NARASI UTAMA**, **LARANGAN KERAS**, dan daftar pertanyaan. Teks yang dipisah titik koma tampil dalam satu butir. Tulis satu item per baris bila ingin butir terpisah.
- Anda tidak perlu menekan **Simpan Draf** di bagian atas halaman agar data tampil di tab ini.

## Pertanyaan yang sering diajukan

**Mengapa kartu saya kosong padahal saya sudah mengunggah dokumen?**
Mengunggah dokumen tidak mengisi tab ini. Minta Tubi membaca dokumen di **Copilot Chat**, atau isi formulir **Instan Brief**.

**Persentase sudah 100%, tetapi Keunggulan unik masih kosong. Apakah ada yang salah?**
Tidak. Keunggulan unik termasuk isian opsional. Klik **Lengkapi field opsional lewat Copilot →** bila ingin mengisinya.

**Bisakah saya mengedit langsung di tab Entitas?**
Tidak. Ubah data lewat **Copilot Chat** atau **Instan Brief**.

**Apa arti lencana Sinyal Siap?**
Pertanyaan itu sudah tersimpan sebagai isian yang terisi atau terkonfirmasi. Lencana ini tidak menjamin brand Anda muncul dalam jawaban mesin AI.

## Lihat juga

- [Copilot Chat](03-copilot-chat.md)
- [Instan Brief: Profil Brand](04-instan-brief-profil-brand.md)
- [Ringkasan brief](08-ringkasan-brief.md)
