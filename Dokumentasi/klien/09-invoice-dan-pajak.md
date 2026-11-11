# Invoice dan Pajak

> Menu Invoice & Pajak menampilkan daftar invoice (surat tagihan) brand Anda, tempat menyimpan data penagihan, dan tombol unduh PDF. Faktur pajak atas Top Up Anda minta saat Top Up.

**Lokasi:** menu **KEUANGAN** › **Invoice & Pajak**
**Untuk:** Klien (semua anggota brand yang bisa melihat brand tersebut)

## Gambaran singkat

Ada dua asal invoice: invoice yang dibuat tim Tauthub untuk kampanye atau paket, dan invoice otomatis untuk Top Up yang Anda minta berfaktur (dibuat saat pembayaran berhasil, satu per pembayaran). Anda tidak membuat invoice sendiri. Di halaman ini Anda menyimpan data penagihan, melihat daftar, dan mengunduh PDF tiap invoice. Klaim PPh 23 belum ada di halaman ini.

## Yang tampil di layar

- **Judul dan jumlah.** Judul **Invoice & Pajak** dengan keterangan "N invoice tercatat untuk brand ini."
- **Filter status.** Kotak pilihan dengan isi **Semua status**, **Draf**, **Diterbitkan**, **Lunas**, dan **Dibatalkan**.
- **Data penagihan.** Kartu berisi nama perusahaan, nomor NPWP, alamat sesuai NPWP, dan email tagihan (opsional). Isi sekali, lalu form Top Up berfaktur terisi otomatis. Hanya anggota dengan peran writer ke atas yang melihatnya, dan hanya admin yang bisa menyimpan. NPWP harus 15 atau 16 digit (titik dan tanda hubung boleh dipakai); isian yang salah ditolak dengan pesan, tidak dibuang diam-diam.
- **Tabel invoice** dengan kolom:

| Kolom | Isinya |
|---|---|
| **NOMOR** | Nomor invoice |
| **KAMPANYE** | Nama kampanye yang terkait. Tanda strip bila invoice tidak terkait kampanye |
| **STATUS** | Status invoice |
| **JUMLAH** | Nominal invoice dalam rupiah |
| **NOMOR PO** | Nomor PO yang Anda isi saat Top Up berfaktur. Tanda strip bila tidak ada |
| **JATUH TEMPO** | Tanggal batas bayar. Tanda strip bila belum ada |
| **PDF** | Tombol unduh invoice dalam PDF: nomor, tanggal, status, data penagihan, PO, deskripsi, dan jumlah |

Bila belum ada invoice, layar menulis "Belum ada invoice." dengan keterangan "Invoice diterbitkan admin setelah kampanye atau pembelian paket." Bila filter tidak menemukan apa pun, layar menulis "Tidak ada invoice dengan status ini."

## Status dan artinya

| Status | Warna | Artinya |
|---|---|---|
| **Draf** | Abu-abu | Invoice belum diterbitkan |
| **Diterbitkan** | Kuning | Invoice sudah diterbitkan dan belum tercatat lunas |
| **Lunas** | Hijau | Invoice sudah tercatat lunas |
| **Dibatalkan** | Abu-abu | Invoice dibatalkan |

## Cara menggunakan

### Cara menyaring invoice berdasarkan status

1. Klik kotak pilihan status di kanan atas halaman.
2. Pilih salah satu status.
3. Baca tabel yang sudah berubah. Pilih **Semua status** untuk menampilkan semuanya lagi.

## Faktur pajak saat Top Up

Anda meminta faktur pajak atas Top Up dari jendela **Top Up Saldo**, bukan dari halaman ini.

1. Klik **Top Up Saldo**.
2. Centang **Saya butuh Faktur Pajak (PPN 11%)**. Keterangannya: tagihan akan dibuat atas nama PT Tauthub dengan tambahan PPN 11%.
3. Isi **Nama Perusahaan (Sesuai NPWP)**, **Nomor NPWP**, dan **Alamat NPWP**. Ketiganya wajib. Bila data penagihan sudah tersimpan, kolom terisi otomatis. **Nomor PO** boleh diisi.
4. Lanjutkan Top Up seperti biasa.

Data faktur diteruskan ke tim Finance, yang menerbitkan faktur pajak. Nomor NPWP harus 15 atau 16 digit, nama perusahaan paling banyak 200 huruf, alamat 500 huruf, dan nomor PO 60 karakter. Isian yang tidak memenuhi syarat ditolak dengan pesan, dan pembayaran tidak dibuat sampai diperbaiki. Setelah pembayaran berhasil, invoice otomatis muncul di daftar dengan data penagihan dan PO. PPN tidak menambah saldo. Langkah lengkap Top Up ada di [Saldo dan Top Up](08-saldo-dan-top-up.md).

## Klaim PPh 23

Di Beranda, kartu **Akses Cepat Keuangan** punya tautan **Klaim PPh 23**. Jendela Top Up juga punya catatan **Punya Bukti Potong PPh 23?**. Catatan itu menyebut bahwa klien perusahaan bisa mengklaim bukti potong PPh 23 menjadi tambahan saldo lewat halaman Invoice, setelah pembayaran berhasil.

Saat ini halaman **Invoice & Pajak** belum punya tombol, formulir, atau kolom unggah untuk klaim itu. Tautan **Klaim PPh 23** hanya membuka daftar invoice. Bila Anda butuh klaim PPh 23, tanyakan caranya kepada tim Tauthub.

## Aturan dan batasan

- Halaman ini hanya untuk melihat. Tim Tauthub yang membuat invoice.
- Halaman memuat sampai 100 invoice terbaru dan tidak punya nomor halaman.
- Semua anggota brand yang bisa melihat brand itu bisa membuka halaman ini.

## Pertanyaan yang sering diajukan

**Di mana saya mengunduh invoice?**
Halaman ini tidak punya tombol unduh. Tanyakan kepada tim Tauthub bila Anda butuh salinan.

**Mengapa daftar invoice saya kosong?**
Invoice baru muncul setelah tim Tauthub menerbitkannya untuk kampanye atau pembelian paket.

**Apakah faktur pajak muncul di daftar ini setelah saya memintanya saat Top Up?**
Halaman ini hanya menampilkan invoice yang diterbitkan tim Tauthub. Permintaan faktur pajak diteruskan ke tim Finance dan tidak punya tampilan sendiri di halaman ini.

## Lihat juga

- [Saldo dan Top Up](08-saldo-dan-top-up.md)
- [Beranda](01-beranda.md)
