# Tagihan di Penyedia Konten

> Di bar bawah Penyedia Konten ada kotak TOTAL TAGIHAN AKHIR. Halaman ini menjelaskan isi angka itu, kapan saldo Anda berkurang, dan apa yang harus dilakukan bila saldo kurang.

**Lokasi:** menu **Kampanye** › pilih kampanye › **Penyedia Konten** › kotak **TOTAL TAGIHAN AKHIR** di bar bawah
**Untuk:** Klien dengan peran Writer atau lebih tinggi pada brand

## Gambaran singkat

Biaya sebuah kampanye terdiri dari tiga bagian:

- **Biaya media**: harga untuk memasang artikel di tiap media.
- **Biaya penulisan AI**: Anda bayar setiap kali The Writer (penulis otomatis dari Tauthub) menulis atau menulis ulang satu artikel. AI adalah program komputer yang bisa menulis sendiri.
- **Biaya gambar AI**: Anda bayar untuk setiap gambar yang dibuat oleh AI.

Bila Anda menulis atau mengunggah artikel sendiri, Anda tidak membayar biaya penulisan AI. Baris penulisan akan tetap kosong. Biaya media tetap berlaku untuk semua artikel. Semua angka tampil dalam rupiah. Tim Tauthub yang menentukan tarif penulisan dan gambar, dan tarif itu bisa berubah. Jadi lihat angka terbaru di layar.

## Yang tampil di layar

### Kotak TOTAL TAGIHAN AKHIR

Kotak ini ada di sisi kiri bar bawah. Angkanya adalah jumlah dari biaya media, penulisan AI, dan gambar AI. Untuk penulisan dan gambar, Tauthub menghitung jumlah proses yang sudah ditagih, lalu mengalikannya dengan tarif yang berlaku sekarang.

Saat total berubah, sebuah label kecil melayang di atas angka lalu memudar. Label hijau seperti "+…" berarti total naik, misalnya setelah artikel AI selesai ditulis. Label merah seperti "-…" berarti total turun, misalnya setelah Anda melepas media dari kampanye. Label tidak muncul saat halaman baru dibuka.

### Rincian biaya

Arahkan kursor ke kotak total untuk membuka jendela kecil **Rincian biaya**. Klik kotak itu supaya jendelanya tetap terbuka. Klik di luar jendela untuk menutupnya. Di HP, ketuk kotak untuk membuka dan menutup.

| Baris | Isi |
|---|---|
| **Biaya media** | Jumlah artikel dan total biaya pemasangannya |
| **Biaya penulisan AI** | Jumlah penulisan yang sudah ditagih, ditulis "N × tarif". Bila belum ada, tertulis "belum memakai AI" |
| **Biaya gambar AI** | Jumlah gambar yang sudah ditagih, ditulis "N × tarif" (termasuk cover dari **Buat semua cover**) |
| **Artikel untuk media lain** | Hanya muncul bila Anda sudah memakai **Pakai untuk media lain**. Jumlah artikel turunan yang sudah ditagih, ditulis "N × tarif". Tarifnya lebih murah daripada penulisan biasa |
| **TOTAL TAGIHAN AKHIR** | Jumlah dari baris-baris di atas |

Pada baris penulisan, keterangan seperti "(5 artikel + 2 tulis ulang)" berarti ada 5 penulisan yang ditagih. Dua di antaranya adalah tulis ulang. Angka pertama sudah termasuk tulis ulang.

## Kapan saldo dipotong

| Aksi | Saat saldo dipotong | Bila gagal |
|---|---|---|
| Menulis artikel baru dengan AI | Setelah artikel selesai ditulis | Tidak ditagih |
| Menulis ulang artikel dengan AI | Di awal proses | Tidak dikembalikan otomatis. Bila Anda mengulang upaya yang sama, saldo tidak dipotong lagi |
| Membuat gambar dengan AI | Setelah gambar jadi. Tauthub mengecek saldo lebih dulu | Tidak ditagih |
| Menerbitkan kampanye | Biaya media ditahan dari saldo tersedia saat Anda klik **Terbitkan** | Dikembalikan bila penerbitan dibatalkan atau ditolak |
| Artikel tayang di media | Biaya media yang ditahan dipotong untuk selamanya | Lihat [Ringkasan dan distribusi](06-ringkasan-distribusi.md) |

Aksi berikut tidak memotong saldo:

- **Tulis Sendiri**, termasuk unggah dokumen dan tempel teks. Bila AI hanya merapikan dokumen Anda, itu juga gratis.
- **Racik Instruksi dari Anchor** dan **Isi topik dari konteks**
- **Analisis QC**, **Cek Validasi**, dan penulisan ulang otomatis setelah QC
- Unggah gambar sendiri, baik untuk cover maupun **Pustaka Gambar AI**

> **Penting:** Penulisan ulang dengan AI menimpa naskah sebelumnya. Naskah lama tidak bisa dikembalikan. Pastikan Anda memang ingin menulis ulang sebelum klik tombol buat draf.

## Cara memeriksa tagihan sebelum menerbitkan

1. Buka **Rincian biaya** di bar bawah.
2. Lihat baris **Biaya media**. Angka ini akan ditahan dari saldo saat Anda menerbitkan kampanye.
3. Buka menu **Saldo** dan bandingkan dengan saldo tersedia. Lihat juga [Saldo dan top up](../08-saldo-dan-top-up.md).
4. Bila saldo kurang, lakukan **Top Up** dulu.

Tauthub menghitung biaya media per artikel. Empat artikel di satu media berarti empat kali harga media itu.

## Saldo tidak cukup

Pesan berikut bisa muncul di tahap yang berbeda:

| Pesan | Tahap | Langkah Anda |
|---|---|---|
| "Saldo kredit tidak cukup untuk generate (butuh …, tersedia …)" | Menulis atau menulis ulang artikel | Top up saldo, lalu ulangi |
| "Saldo kredit tidak cukup untuk generate gambar…" | Membuat gambar AI | Top up saldo, lalu ulangi |
| "Saldo tidak mencukupi untuk membuat gambar. Silakan top up terlebih dahulu." | Kotak konfirmasi gambar. Tombol konfirmasi tidak bisa diklik | Top up saldo, lalu buka kotak konfirmasi lagi |
| **Saldo Tidak Cukup**: "Distribusi butuh …, saldo tersedia …" | Menerbitkan kampanye | Klik **Top Up Saldo**, lalu buka **Ringkasan** untuk melihat status penerbitan |

Dua pesan pertama memakai satuan kredit, bukan rupiah. Anda tidak perlu menghitungnya. Pesan itu hanya berarti saldo belum cukup. Saldo di halaman **Saldo** tampil dalam rupiah. Biaya yang ditahan untuk penerbitan diberi tanda "ditahan".

## Pertanyaan yang sering diajukan

**Kenapa angka biaya penulisan tidak berubah saat saya menulis sendiri?**
Artikel yang Anda tulis atau unggah sendiri tidak dihitung sebagai penulisan AI.

**Kenapa total bertambah setelah saya membuat gambar otomatis?**
Setiap gambar AI ditagih saat jadi. Angka total lalu berubah sendiri tanpa Anda membuka ulang halaman.

**Bisakah saya menutup Pengelola Naskah saat AI sedang menulis?**
Tidak. Tombol tutup dan **Batal** hanya menampilkan peringatan sampai draf selesai. AI terus bekerja sampai selesai.

## Lihat juga

- [Penyedia Konten](03-penyedia-konten.md)
- [Menulis artikel dengan AI](23-menulis-artikel-dengan-ai.md)
- [Gambar utama dan Pustaka Gambar](26-gambar-utama-dan-pustaka-gambar.md)
- [Meninjau dan menerbitkan kampanye](22-meninjau-dan-menerbitkan-kampanye.md)
- [Saldo dan top up](../08-saldo-dan-top-up.md)
