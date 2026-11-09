# Tagihan di Penyedia Konten

> Bar bawah Penyedia Konten menampilkan TOTAL TAGIHAN AKHIR kampanye beserta rinciannya. Halaman ini menjelaskan isi angka itu, kapan saldo Anda dipotong, dan apa yang perlu dilakukan bila saldo kurang.

**Lokasi:** menu **Kampanye** › pilih kampanye › **Penyedia Konten** › kotak **TOTAL TAGIHAN AKHIR** di bar bawah
**Untuk:** Klien dengan peran Writer atau lebih tinggi pada brand

## Gambaran singkat

Biaya sebuah kampanye terdiri dari tiga komponen:

- **Biaya media**: harga penempatan artikel di tiap media.
- **Biaya penulisan AI**: dikenakan setiap kali The Writer menulis atau menulis ulang sebuah artikel.
- **Biaya gambar AI**: dikenakan untuk setiap gambar yang dibuat dengan AI.

Artikel yang Anda tulis atau unggah sendiri tidak dikenai biaya penulisan AI, sehingga baris penulisan tetap kosong bila Anda tidak memakai AI. Biaya media tetap berlaku untuk semua artikel. Nilai tampil dalam rupiah. Besar tarif penulisan dan gambar ditetapkan oleh tim Tauthub dan dapat berubah, jadi lihat angka terbaru di layar.

## Yang tampil di layar

### Kotak TOTAL TAGIHAN AKHIR

Kotak ini berada di sisi kiri bar bawah. Angka di dalamnya adalah jumlah biaya media, penulisan AI, dan gambar AI. Biaya penulisan dan gambar dihitung dari jumlah proses yang sudah ditagih, dikalikan tarif yang berlaku sekarang.

Saat total berubah, sebuah chip kecil melayang di atas angka lalu memudar. Chip hijau seperti "+Rp…" menandakan kenaikan, misalnya setelah artikel AI selesai ditulis. Chip merah seperti "-Rp…" menandakan penurunan, misalnya setelah media dilepas dari kampanye. Chip tidak muncul saat halaman pertama kali dimuat.

### Rincian biaya

Arahkan kursor ke kotak total untuk membuka popover **Rincian biaya**. Klik kotak itu untuk menahan popover tetap terbuka, lalu klik di luar popover untuk menutupnya. Di HP, ketuk kotak untuk membuka dan menutup.

| Baris | Isi |
|---|---|
| **Biaya media** | Jumlah artikel dan total biaya penempatannya |
| **Biaya penulisan AI** | Jumlah penulisan yang sudah ditagih, ditulis "N × tarif". Bila belum ada, tertulis "belum memakai AI" |
| **Biaya gambar AI** | Jumlah gambar yang sudah ditagih, ditulis "N × tarif" |
| **TOTAL TAGIHAN AKHIR** | Jumlah ketiga baris di atas |

Pada baris penulisan, keterangan seperti "(5 artikel + 2 tulis ulang)" berarti ada 5 penulisan yang ditagih, dan 2 di antaranya adalah tulis ulang. Angka pertama sudah mencakup tulis ulang.

## Kapan saldo dipotong

| Aksi | Saat saldo dipotong | Bila gagal |
|---|---|---|
| Menulis artikel baru dengan AI | Setelah artikel selesai ditulis | Tidak ditagih |
| Menulis ulang artikel dengan AI | Di awal proses | Tidak dikembalikan otomatis; mengulang upaya yang sama tidak dipotong lagi |
| Membuat gambar dengan AI | Setelah gambar jadi; saldo dicek lebih dulu | Tidak ditagih |
| Menerbitkan kampanye | Biaya media ditahan dari saldo tersedia saat Anda klik **Terbitkan** | Dikembalikan bila publikasi dibatalkan atau ditolak |
| Artikel tayang di media | Biaya media yang ditahan dipotong permanen | Lihat [Ringkasan dan distribusi](06-ringkasan-distribusi.md) |

Aksi berikut tidak memotong saldo:

- **Tulis Sendiri**, termasuk unggah dokumen dan tempel teks (perapian dokumen oleh AI juga gratis)
- **Racik Instruksi dari Anchor** dan **Isi topik dari konteks**
- **Analisis QC**, **Cek Validasi**, dan penulisan ulang otomatis setelah QC
- unggah gambar sendiri ke cover maupun **Pustaka Gambar AI**

> **Penting:** Penulisan ulang dengan AI menimpa naskah sebelumnya dan versi lama tidak bisa dikembalikan. Pastikan Anda ingin menulis ulang sebelum mengklik tombol buat draf.

## Cara memeriksa tagihan sebelum menerbitkan

1. Buka **Rincian biaya** di bar bawah.
2. Lihat baris **Biaya media**. Angka ini ditahan dari saldo saat kampanye diterbitkan.
3. Buka menu **Saldo** dan bandingkan dengan saldo tersedia. Lihat juga [Saldo dan top up](../08-saldo-dan-top-up.md).
4. Bila saldo kurang, lakukan **Top Up** lebih dulu.

Media dengan beberapa artikel dihitung per artikel. Empat artikel di satu media berarti empat kali harga media itu.

## Saldo tidak cukup

Pesan berikut dapat muncul pada tahap berbeda:

| Pesan | Tahap | Langkah Anda |
|---|---|---|
| "Saldo kredit tidak cukup untuk generate (butuh …, tersedia …)" | Menulis atau menulis ulang artikel | Top up saldo, lalu ulangi |
| "Saldo kredit tidak cukup untuk generate gambar…" | Membuat gambar AI | Top up saldo, lalu ulangi |
| "Saldo tidak mencukupi untuk membuat gambar. Silakan top up terlebih dahulu." | Dialog konfirmasi gambar; tombol konfirmasi nonaktif | Top up saldo, lalu buka dialog lagi |
| **Saldo Tidak Cukup**: "Distribusi butuh Rp…, saldo tersedia Rp…" | Menerbitkan kampanye | Klik **Top Up Saldo**, lalu buka **Ringkasan** untuk memeriksa status publikasi |

Angka dalam dua pesan pertama memakai satuan kredit, bukan rupiah. Anda tidak perlu menghitungnya: pesan itu berarti saldo belum cukup. Saldo di halaman **Saldo** tampil dalam rupiah, dan biaya yang ditahan untuk publikasi ditandai "ditahan".

## Pertanyaan yang sering diajukan

**Kenapa angka biaya penulisan tidak berubah saat saya menulis sendiri?**
Artikel yang Anda tulis atau unggah sendiri tidak dihitung sebagai penulisan AI.

**Kenapa total bertambah setelah saya membuat gambar otomatis?**
Setiap gambar AI ditagih saat dibuat, lalu angka total diperbarui tanpa memuat ulang halaman.

**Bisakah saya menutup Pengelola Naskah saat AI sedang menulis?**
Tidak. Tombol tutup dan **Batal** hanya menampilkan peringatan sampai draf selesai. Proses berjalan sampai selesai.

## Lihat juga

- [Penyedia Konten](03-penyedia-konten.md)
- [Menulis artikel dengan AI](23-menulis-artikel-dengan-ai.md)
- [Gambar utama dan Pustaka Gambar](26-gambar-utama-dan-pustaka-gambar.md)
- [Meninjau dan menerbitkan kampanye](22-meninjau-dan-menerbitkan-kampanye.md)
- [Saldo dan top up](../08-saldo-dan-top-up.md)
