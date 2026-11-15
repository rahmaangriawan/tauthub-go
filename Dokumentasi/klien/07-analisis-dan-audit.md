# Analisis Kinerja dan Audit Publikasi

> **Analisis Kinerja** merangkum hasil tiap kampanye dari data yang tercatat di Tauthub. **Audit Publikasi** merekap kepatuhan tiap artikel yang sudah terbit.

**Lokasi:** menu **Analisis Kinerja** dan menu **Audit Publikasi** (grup **ANALITIK & PELAPORAN**)

**Untuk:** Klien

## Analisis Kinerja

Bagian atas halaman menampilkan empat angka untuk seluruh brand: **Artikel terbit**, **Media**, **Tautan bermasalah**, dan **Garansi menunggu pilihan** (klaim garansi yang menunggu Anda memilih artikel pengganti atau kredit kembali). Di bawahnya ada daftar kampanye, yang terbaru terbit ada di atas. Tiap baris menunjukkan jumlah artikel, jumlah media, dan kondisi tautan (**Sehat** atau jumlah yang **bermasalah**).

Klik satu kampanye untuk melihat tiap artikelnya:

- judul artikel, media yang memuatnya, tanggal terbit, dan tautan ke artikelnya;
- kondisi tautan menurut [Link Guardian](06-link-guardian.md): sehat, artikel tidak ditemukan lagi, tautan di dalam artikel dicabut, ditandai noindex, atau atribut tautan berubah;
- masa garansi tayang, atau status klaim garansi bila sudah dipakai;
- kapan terakhir diperiksa.

Dari rincian kampanye ada tautan ke Link Guardian dan ke halaman Kampanye untuk berita acara yang bisa dibagikan.

> **Catatan:** Tauthub tidak memantau klik atau perilaku pembaca. Yang ditampilkan hanya fakta yang tercatat di Tauthub: artikel yang terbit, media, dan hasil pemeriksaan tautan.


## Audit Publikasi

Halaman ini merekap kepatuhan **artikel yang sudah terbit** di seluruh kampanye brand Anda.

**Ringkasan** di bagian atas menampilkan satu angka besar untuk **Terbit**, dan batang proporsi berisi **Sehat**, **Bermasalah**, **Tak terjangkau**, dan **Belum dicek**. Di sebelah kanan ada tiga angka: **Terbit**, **Hilang**, dan **Klaim terbuka**. Angka Hilang menampilkan persentasenya dari artikel terbit. Angka Klaim terbuka menandai klaim garansi yang sedang diproses.

**Grafik** yang tersedia:

- **Tren 12 bulan**: kolom bertumpuk per bulan terbit, dari Januari di kiri sampai Desember di kanan, untuk tahun berjalan. Biru untuk sehat, merah untuk bermasalah, abu-abu untuk lainnya. Bulan tanpa artikel tetap tampil dengan labelnya.
- **Media terbanyak**: delapan media teratas dengan jumlah artikel. Bagian merah menunjukkan artikel yang bermasalah.

**Periode terbit** di atas halaman membatasi ringkasan, grafik media, dan daftar. Pilih bulan dari daftar yang muncul, atau pilih **Semua periode** untuk kembali ke seluruh data. Tren 12 bulan tidak ikut terpengaruh.

**Unduh** tersedia dalam dua format:

- **CSV**: daftar artikel yang sedang tampil, untuk diolah di spreadsheet.
- **PDF**: laporan berisi judul, ringkasan angka, tren 12 bulan, kondisi tautan, media terbanyak, dan daftar artikel. Setiap halaman punya footer dengan nomor halaman.

Nama file mengikuti nama klien dan periode, misalnya `Audit_Publikasi_Nama_Klien_2026-10.pdf`.

**Arti status:**

- **Sehat**: artikel bisa dibuka, tautan di dalamnya masih ada, dan tidak ada noindex.
- **Bermasalah**: ada masalah pada tautan atau artikel. Lihat jenis masalahnya di baris itu.
- **Hilang**: artikel tidak ditemukan lagi (404 atau 410).
- **Tak terjangkau**: pemeriksaan gagal sementara. Artikel akan diperiksa ulang pada jadwal berikutnya.
- **Belum dicek**: artikel baru terbit dan belum diperiksa.

Pemeriksaan dilakukan otomatis oleh [Link Guardian](06-link-guardian.md). Tombol **Muat ulang** mengambil data terbaru.

## Yang bisa dipakai sekarang

- Hasil pemeriksaan artikel yang sudah terbit tampil di [Status Publikasi](05-status-publikasi.md). Isinya nada berita, pemeriksaan alamat, dan seberapa banyak pesan Anda sampai.
- Kondisi alamat terbit (tayang, 404, atau tidak terjangkau) tampil di [Link Guardian](06-link-guardian.md).
- Kemajuan penerbitan tiap kampanye tampil di [Ringkasan Distribusi](03-kampanye/06-ringkasan-distribusi.md).

## Lihat juga

- [Status Publikasi](05-status-publikasi.md)
- [Link Guardian](06-link-guardian.md)
- [Alur kampanye](03-kampanye/README.md)
