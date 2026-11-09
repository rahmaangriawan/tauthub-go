# Beberapa artikel untuk satu media

> Satu media di kampanye boleh memuat lebih dari satu artikel. Halaman ini menjelaskan cara mengatur jumlah artikel, membuat semuanya sekaligus lewat satu dialog, dan menghapus artikel tertentu.

**Lokasi:** menu **Kampanye** › pilih kampanye › **Penyedia Konten** › header kartu media
**Untuk:** Klien dengan peran Writer atau lebih tinggi pada brand

## Gambaran singkat

Secara bawaan, tiap media memuat satu artikel. Anda bisa menambah jumlahnya bila ingin beberapa artikel terbit di domain yang sama, misalnya dengan sudut bahasan berbeda. Setiap artikel adalah baris tersendiri ("Artikel 1", "Artikel 2", dan seterusnya) dengan naskah, cover, dan hasil QC masing-masing.

Biaya media dihitung per artikel. Empat artikel di satu media berarti empat kali harga media itu. Lihat [Tagihan di Penyedia Konten](20-tagihan-di-penyedia-konten.md).

## Yang tampil di layar

Header kartu media memuat tiga kontrol untuk mengelola artikel:

- **Buat N artikel**: tombol biru; muncul bila media punya lebih dari satu artikel dan masih ada yang belum punya naskah. Angka N adalah jumlah artikel yang belum dibuat.
- Pengatur jumlah dengan tombol **−** dan **+** serta angka di tengahnya.
- Menu **Aksi lain** (tiga titik) di tiap baris artikel, hanya untuk media dengan lebih dari satu artikel.

Baris status di header menyesuaikan diri, misalnya "3 artikel belum dibuat" atau "4 artikel · semua sudah ada naskahnya".

## Cara menggunakan

### Cara mengubah jumlah artikel

1. Klik **+** di header kartu media untuk menambah satu artikel kosong.
2. Klik **−** untuk mengurangi satu artikel.

Aturan pengaturnya:

- Jumlah minimal 1. Tombol **−** nonaktif saat jumlah 1.
- Jumlah paling banyak 200 artikel per media.
- Pengurangan hanya menghapus artikel yang masih kosong, dimulai dari nomor terbesar. Bila tidak ada cukup artikel kosong, muncul pesan "Artikel N sudah punya naskah. Hapus naskahnya dulu sebelum mengurangi jumlah artikel." Hapus artikel itu lewat menu **Aksi lain** terlebih dulu.
- Angka berubah menjadi "…" sementara perubahan diproses.

### Cara membuat semua artikel kosong sekaligus

1. Klik **Buat N artikel** di header kartu media. Dialog **Buat N artikel** terbuka dengan nama domain di bawah judulnya.
2. Bila kampanye sudah punya brief, baca keterangan hijau. Topik boleh dikosongkan karena artikel mengikuti brief.
3. Bila kampanye belum punya brief, isi blok **Konteks untuk semua artikel**: **Produk/jasa yang dipromosikan**, **Target pembaca (opsional)**, dan **Pesan utama yang ingin disampaikan**. Tanpa konteks, artikel cenderung generik dan mirip satu sama lain.
4. Klik **Isi topik dari konteks** untuk mengisi kolom topik semua artikel dalam satu klik. Hasilnya bisa Anda ubah. Tombol ini butuh minimal produk/jasa atau pesan utama.
5. Periksa tiap blok **Artikel N**. Isi kolom **Topik atau instruksi (opsional)** bila perlu. Pilih **Angle** untuk tiap artikel. Angle sudah diisi berbeda-beda secara berurutan supaya hasil tidak serupa.
6. Klik **Buat N artikel**.

Selama proses, tiap blok menampilkan "… membuat", lalu tanda centang dengan kata "selesai" bila berhasil, atau pesan galat bila gagal. Dua artikel dibuat berbarengan. Tombol tutup dan **Batal** nonaktif selama proses. Bila semua artikel berhasil, dialog menutup sendiri.

Tombol **Buat N artikel** nonaktif saat proses berjalan. Tombol ini juga nonaktif bila kampanye tanpa brief dan Anda belum mengisi konteks atau topik apa pun, dengan petunjuk "Isi konteks atau topik dulu supaya artikelnya tidak generik."

> **Penting:** Bila sebagian artikel gagal, dialog tetap terbuka dengan pesan galat di blok yang bermasalah. Klik **Batal** untuk menutupnya, lalu buka lagi lewat **Buat N artikel**. Dialog yang baru hanya memuat artikel yang masih kosong, sehingga artikel yang sudah jadi tidak ikut ditulis ulang dan ditagih lagi.

Setiap artikel dibuat dengan cara **Sederhana Aja** dan sudut yang Anda pilih. Dialog ini tidak menyediakan kolom tautan, gaya **Yang Proper**, maupun gambar utama. Untuk itu, buka artikelnya satu per satu lewat **Generate Konten** (lihat [Menulis artikel dengan AI](23-menulis-artikel-dengan-ai.md)). Biaya penulisan AI tampil di **Rincian biaya**.

Bila AI tidak menjawab saat **Isi topik dari konteks** diklik, topik disusun langsung dari konteks dan nama angle, dengan catatan "Topik disusun tanpa AI (model tidak menjawab). Silakan periksa dan rapikan."

### Cara menghapus satu artikel

1. Klik tombol tiga titik (**Aksi lain**) di baris artikel.
2. Klik **Hapus artikel N** (N adalah nomor artikelnya).
3. Bila artikel belum punya naskah, artikel langsung dihapus. Bila sudah punya naskah, dialog **Hapus Artikel N?** muncul. Klik **Ya, Hapus Artikel**.

Naskah yang dihapus tidak bisa dikembalikan. Nomor artikel yang tersisa biasanya dirapatkan kembali menjadi 1, 2, 3, dan seterusnya.

Aturan penghapusan:

- Artikel terakhir sebuah media tidak bisa dihapus lewat menu ini. Pesan: "Media ini hanya punya satu artikel. Pakai tombol hapus media kalau ingin melepasnya dari kampanye."
- Artikel yang sudah punya publikasi tidak bisa dihapus. Pesan: "Artikel ini sudah punya publikasi, jadi tidak bisa dihapus."
- Pada kampanye yang sudah selesai, atau untuk artikel yang sudah terkunci, perintah hapus tidak dijalankan.

### Cara melepas seluruh media

Klik ikon tong sampah di header kartu media. Semua artikel media itu ikut dilepas dari kampanye. Langkah lengkapnya ada di [Penyedia Konten](03-penyedia-konten.md).

## Pertanyaan yang sering diajukan

**Apakah artikel pada satu media bisa memakai sudut yang sama?**
Bisa, tetapi hasilnya cenderung mirip. Pilih angle berbeda untuk tiap artikel.

**Kenapa tombol Buat N artikel hilang?**
Tombol itu hanya tampil bila media punya lebih dari satu artikel dan masih ada yang belum punya naskah. Bila media hanya punya satu artikel, gunakan **Generate Konten** di baris artikelnya.

**Apakah tiap artikel diperiksa QC sendiri-sendiri?**
Ya. Hasil QC, cover, dan status dihitung per artikel. Lihat [Cek Kualitas Naskah](05-cek-kualitas-naskah.md).

## Lihat juga

- [Penyedia Konten](03-penyedia-konten.md)
- [Menulis artikel dengan AI](23-menulis-artikel-dengan-ai.md)
- [Tagihan di Penyedia Konten](20-tagihan-di-penyedia-konten.md)
