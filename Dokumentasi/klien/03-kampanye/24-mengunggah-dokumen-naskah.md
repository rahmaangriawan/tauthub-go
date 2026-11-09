# Mengunggah dokumen naskah

> Tab Unggah Dokumen memasukkan naskah yang sudah Anda tulis ke Pengelola Naskah, dari berkas DOCX, TXT, atau PDF, atau dari teks yang ditempel. Layanan ini gratis dan isi naskah tidak ditulis ulang oleh AI.

**Lokasi:** menu **Kampanye** › pilih kampanye › **Penyedia Konten** › tombol **Tulis Sendiri** (atau **Edit Artikel** pada artikel unggahan) › tab **Unggah Dokumen**
**Untuk:** Klien dengan peran Writer atau lebih tinggi pada brand

## Gambaran singkat

Gunakan tab ini bila naskah sudah ada di Word, Google Docs, atau berkas lain. Hasil unggahan masuk ke tab **Editor Post**, tempat Anda memeriksa isi, mengatur gambar utama dan kategori, lalu menyimpan. Pembaruan lama menyebut tombol **Tulis Sendiri** sebagai "Unggah Artikel".

## Yang tampil di layar

Tab berjudul "Punya Draf Sendiri?" dan terbagi dua kolom:

- Kolom berkas: kotak putus-putus bertuliskan "Klik, Tarik, atau Tempel File ke sini" dan "Mendukung DOCX, TXT, PDF (Maks 10MB)".
- Kolom teks: kotak teks "Tempel teks artikel di sini (Ctrl/Cmd+V). Baris pertama yang pendek akan jadi judul." dengan tombol **Gunakan Teks Ini**.

Saat berkas diproses, kotak berkas menampilkan "Memproses dokumen…", lalu "Merapikan dokumen dengan AI..." untuk berkas yang dirapikan.

## Cara menggunakan

### Cara mengunggah berkas

1. Klik kotak berkas, tarik berkas ke sana, atau tempel berkas dengan Ctrl/Cmd+V.
2. Pilih berkas DOCX, TXT, atau PDF dengan ukuran maksimal 10 MB.
3. Tunggu sampai tab berpindah ke **Editor Post**.
4. Periksa judul dan isi, lalu lengkapi **Gambar Utama** dan **Kategori / Rubrik**.
5. Klik **Simpan Artikel**.

Berkas Word lama (.doc) belum didukung. Simpan sebagai .docx lebih dulu.

### Cara menempel teks

1. Salin naskah dari Word, Google Docs, atau sumber lain.
2. Tempel di kotak teks. Baris pertama yang pendek (sampai 120 karakter) menjadi judul bila naskah punya lebih dari satu baris.
3. Klik **Gunakan Teks Ini**.
4. Periksa hasilnya di **Editor Post**.

Tempelan dari Google Docs atau Word web membawa heading, tabel, dan daftar. Layar menampilkan "Terdeteksi tempelan Google Docs — heading, tabel, dan daftar ikut terjaga." atau "Format dari clipboard ikut terjaga (heading, tabel, daftar)." Gaya huruf dan font bawaan dokumen dibuang, sedangkan tebal, miring, garis bawah, coret, dan tautan dipertahankan. Teks biasa dirapikan: baris yang terpotong disatukan, spasi ganda dibuang, dan butir atau nomor menjadi daftar.

## Apa yang terjadi pada berkas

| Jenis berkas | Perlakuan |
|---|---|
| TXT | Dibaca apa adanya, lalu dirapikan AI |
| DOCX tanpa gambar | Teks dibaca per paragraf, lalu dirapikan AI |
| DOCX dengan gambar | Teks dan gambar dipindahkan ke editor apa adanya, tanpa perapian AI. Gambar PNG, JPG, GIF, dan WebP ikut. Format lain, seperti EMF atau WMF, dilewati |
| PDF | Teksnya dibaca, lalu dirapikan AI. PDF hasil pindai tanpa teks tidak bisa dibaca |

Perapian AI memperbaiki baris yang terputus, membuang duplikasi, header, footer, dan nomor halaman, serta menandai subjudul sebagai heading. AI diminta mempertahankan makna, fakta, angka, nama, dan kutipan, serta tidak menambah informasi. Perapian tidak memotong saldo. Periksa bagian akhir naskah di **Editor Post**, terutama untuk dokumen yang sangat panjang.

Judul diambil dari baris pertama bila pendek (sampai 120 karakter) dan dokumen punya lebih dari satu baris. Bila tidak, nama berkas tanpa ekstensi yang dipakai.

Bila perapian gagal, naskah tetap masuk dengan teks aslinya, dan banner merah muncul: "AI gagal merapikan dokumen: … Teks asli tetap dipakai." Tutup banner dengan ikon silang.

Setelah masuk, Tauthub tetap memeriksa tautan dan keamanan isi naskah lewat QC. Lihat [Cek Kualitas Naskah](05-cek-kualitas-naskah.md).

## Aturan dan batasan

- Format yang diterima: DOCX, TXT, dan PDF. Ukuran maksimal 10 MB.
- Naskah yang diunggah menjadi isi artikel. AI tidak menulis ulang isinya.
- Tab ini tidak menyediakan kolom tautan. Tambahkan tautan di **Editor Post** lewat tombol **Sisipkan tautan**. Lihat [Editor teks naskah](25-editor-teks-naskah.md).

## Pesan yang mungkin muncul

| Pesan | Artinya | Langkah Anda |
|---|---|---|
| "Format tidak didukung. Gunakan DOCX, TXT, atau PDF." | Jenis berkas tidak diterima | Ubah format berkas |
| "Ukuran file melebihi 10 MB." | Berkas terlalu besar | Kecilkan berkas atau pecah naskah |
| "Isi file tidak cocok dengan tipe PDF." atau "…DOCX." | Isi berkas tidak sesuai ekstensinya | Simpan ulang dari aplikasi aslinya |
| "DOCX tidak dapat dibuka — arsipnya rusak atau bukan berkas Office yang valid." | Berkas rusak | Simpan ulang sebagai DOCX |
| "Tidak ada teks yang bisa diekstrak dari DOCX ini." | Dokumen kosong atau hanya gambar | Tempel teksnya |
| "PDF tidak berisi teks yang bisa diekstrak (kemungkinan hasil pemindaian/gambar)." | PDF hasil pindai | Tempel teksnya atau pakai berkas asli |
| "PDF tidak dapat dibaca — berkas rusak atau terproteksi." | PDF diberi kata sandi atau rusak | Buka proteksinya, atau tempel teksnya |
| "Dokumen tersimpan, tetapi teksnya tidak bisa dibaca. Coba PDF lain atau tempel teksnya." | Teks gagal dibaca | Coba berkas lain atau tempel teks |
| "File gagal dibaca (HTTP …). Gunakan DOCX, PDF, atau TXT." | Unggahan gagal | Coba lagi |

## Pertanyaan yang sering diajukan

**Apakah unggah dokumen memotong saldo?**
Tidak. Unggah, tempel, dan perapian dokumen gratis.

**Bolehkah saya menyunting naskah setelah diunggah?**
Boleh. Semua isian bisa diubah di **Editor Post** sebelum Anda mengklik **Simpan Artikel**.

**Bagaimana bila hasil perapian kurang rapi?**
Sunting langsung di **Editor Post**, atau tempel ulang naskah lewat **Gunakan Teks Ini**.

## Lihat juga

- [Pengelola Naskah](04-pengelola-naskah.md)
- [Editor teks naskah](25-editor-teks-naskah.md)
- [Gambar utama dan Pustaka Gambar](26-gambar-utama-dan-pustaka-gambar.md)
- [Cek Kualitas Naskah](05-cek-kualitas-naskah.md)
