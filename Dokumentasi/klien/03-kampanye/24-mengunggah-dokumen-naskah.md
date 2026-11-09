# Mengunggah dokumen naskah

> Tab Unggah Dokumen memasukkan naskah yang sudah Anda tulis ke Pengelola Naskah. Anda bisa memakai berkas DOCX, TXT, atau PDF, atau menempel teks. Layanan ini gratis dan AI tidak menulis ulang isi naskah Anda.

**Lokasi:** menu **Kampanye** › pilih kampanye › **Penyedia Konten** › tombol **Tulis Sendiri** (atau **Edit Artikel** pada artikel unggahan) › tab **Unggah Dokumen**
**Untuk:** Klien dengan peran Writer atau lebih tinggi pada brand

## Gambaran singkat

Pakai tab ini bila naskah Anda sudah ada di Word, Google Docs, atau berkas lain. AI adalah program komputer yang bisa menulis sendiri. Hasil unggahan masuk ke tab **Editor Post**. Di sana Anda memeriksa isi, mengatur gambar utama dan kategori, lalu menyimpan. Di catatan pembaruan lama, tombol **Tulis Sendiri** disebut "Unggah Artikel".

## Yang tampil di layar

Tab berjudul "Punya Draf Sendiri?" dan terbagi dua kolom:

- Kolom berkas: kotak bergaris putus-putus bertuliskan "Klik, Tarik, atau Tempel File ke sini" dan "Mendukung DOCX, TXT, PDF (Maks 10MB)".
- Kolom teks: kotak teks "Tempel teks artikel di sini (Ctrl/Cmd+V). Baris pertama yang pendek akan jadi judul." dengan tombol **Gunakan Teks Ini**.

Saat berkas sedang diproses, kotak berkas menampilkan "Memproses dokumen…". Untuk berkas yang dirapikan, tulisannya lalu berubah menjadi "Merapikan dokumen dengan AI...".

## Cara menggunakan

### Cara mengunggah berkas

1. Klik kotak berkas, tarik berkas ke sana, atau tempel berkas dengan Ctrl/Cmd+V.
2. Pilih berkas DOCX, TXT, atau PDF dengan ukuran paling besar 10 MB.
3. Tunggu sampai tab pindah ke **Editor Post**.
4. Periksa judul dan isi, lalu lengkapi **Gambar Utama** dan **Kategori / Rubrik**.
5. Klik **Simpan Artikel**.

Berkas Word lama (.doc) belum bisa dipakai. Simpan dulu sebagai .docx.

### Cara menempel teks

1. Salin naskah dari Word, Google Docs, atau sumber lain.
2. Tempel di kotak teks. Bila naskah punya lebih dari satu baris, baris pertama yang pendek (sampai 120 karakter) menjadi judul.
3. Klik **Gunakan Teks Ini**.
4. Periksa hasilnya di **Editor Post**.

Tempelan dari Google Docs atau Word web ikut membawa subjudul (heading), tabel, dan daftar. Layar menampilkan "Terdeteksi tempelan Google Docs. Heading, tabel, dan daftar ikut terjaga." atau "Format dari clipboard ikut terjaga (heading, tabel, daftar)." Tauthub membuang gaya huruf dan font bawaan dokumen. Huruf tebal, miring, garis bawah, coretan, dan tautan tetap ada. Teks biasa dirapikan: baris yang terpotong disatukan, spasi ganda dibuang, dan butir atau nomor menjadi daftar.

## Apa yang terjadi pada berkas

| Jenis berkas | Perlakuan |
|---|---|
| TXT | Dibaca apa adanya, lalu AI merapikannya |
| DOCX tanpa gambar | Teks dibaca per paragraf, lalu AI merapikannya |
| DOCX dengan gambar | Teks dan gambar dipindahkan ke editor apa adanya, tanpa dirapikan AI. Gambar PNG, JPG, GIF, dan WebP ikut. Format lain, seperti EMF atau WMF, dilewati |
| PDF | Teksnya dibaca, lalu AI merapikannya. PDF hasil pindai yang tidak berisi teks tidak bisa dibaca |

AI memperbaiki baris yang terputus. AI membuang isi yang berulang, header, footer, dan nomor halaman. AI juga menandai subjudul sebagai heading. AI diminta menjaga makna, fakta, angka, nama, dan kutipan, dan tidak menambah informasi. Perapian ini tidak memotong saldo. Periksa bagian akhir naskah di **Editor Post**, terutama untuk dokumen yang sangat panjang.

Tauthub mengambil judul dari baris pertama bila barisnya pendek (sampai 120 karakter) dan dokumen punya lebih dari satu baris. Bila tidak, Tauthub memakai nama berkas tanpa ekstensi (akhiran seperti .docx).

Bila perapian gagal, naskah tetap masuk dengan teks aslinya. Banner merah muncul: "AI gagal merapikan dokumen: … Teks asli tetap dipakai." Tutup banner dengan ikon silang.

Setelah naskah masuk, Tauthub tetap memeriksa tautan dan keamanan isinya lewat QC. QC adalah pemeriksaan kualitas naskah. Lihat [Cek Kualitas Naskah](05-cek-kualitas-naskah.md).

## Aturan dan batasan

- Format yang diterima: DOCX, TXT, dan PDF. Ukuran paling besar 10 MB.
- Naskah yang Anda unggah menjadi isi artikel. AI tidak menulis ulang isinya.
- Tab ini tidak punya kolom tautan. Tambahkan tautan di **Editor Post** lewat tombol **Sisipkan tautan**. Lihat [Editor teks naskah](25-editor-teks-naskah.md).

## Pesan yang mungkin muncul

| Pesan | Artinya | Langkah Anda |
|---|---|---|
| "Format tidak didukung. Gunakan DOCX, TXT, atau PDF." | Jenis berkas tidak diterima | Ubah format berkas |
| "Ukuran file melebihi 10 MB." | Berkas terlalu besar | Kecilkan berkas atau pecah naskah |
| "Isi file tidak cocok dengan tipe PDF." atau "…DOCX." | Isi berkas tidak sesuai ekstensinya | Simpan ulang dari aplikasi aslinya |
| "DOCX tidak dapat dibuka. Arsipnya rusak atau bukan berkas Office yang valid." | Berkas rusak | Simpan ulang sebagai DOCX |
| "Tidak ada teks yang bisa diekstrak dari DOCX ini." | Dokumen kosong atau hanya berisi gambar | Tempel teksnya |
| "PDF tidak berisi teks yang bisa diekstrak (kemungkinan hasil pemindaian/gambar)." | PDF hasil pindai | Tempel teksnya atau pakai berkas asli |
| "PDF tidak dapat dibaca. Berkas rusak atau terproteksi." | PDF diberi kata sandi atau rusak | Buka proteksinya, atau tempel teksnya |
| "Dokumen tersimpan, tetapi teksnya tidak bisa dibaca. Coba PDF lain atau tempel teksnya." | Teks gagal dibaca | Coba berkas lain atau tempel teks |
| "File gagal dibaca (HTTP …). Gunakan DOCX, PDF, atau TXT." | Unggahan gagal | Coba lagi |

## Pertanyaan yang sering diajukan

**Apakah unggah dokumen memotong saldo?**
Tidak. Unggah, tempel, dan perapian dokumen gratis.

**Bolehkah saya menyunting naskah setelah diunggah?**
Boleh. Anda bisa mengubah semua isian di **Editor Post** sebelum mengklik **Simpan Artikel**.

**Bagaimana bila hasil perapian kurang rapi?**
Sunting langsung di **Editor Post**, atau tempel ulang naskah lewat **Gunakan Teks Ini**.

## Lihat juga

- [Pengelola Naskah](04-pengelola-naskah.md)
- [Editor teks naskah](25-editor-teks-naskah.md)
- [Gambar utama dan Pustaka Gambar](26-gambar-utama-dan-pustaka-gambar.md)
- [Cek Kualitas Naskah](05-cek-kualitas-naskah.md)
