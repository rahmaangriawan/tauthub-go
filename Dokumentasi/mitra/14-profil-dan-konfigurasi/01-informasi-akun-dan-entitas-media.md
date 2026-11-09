# Informasi akun dan entitas media

> Tab **Profil** di halaman **Konfigurasi** menampilkan identitas akun Mitra Anda dan memungkinkan Anda mengubah nama penanggung jawab serta nama entitas media. Menyimpan profil untuk pertama kalinya juga mencatat persetujuan syarat integrasi.

**Lokasi:** **Konfigurasi** › tab **Profil**
**Untuk:** Mitra

## Gambaran singkat

Tab **Profil** terbuka lebih dulu setiap kali Anda masuk ke **Konfigurasi**. Isinya tiga kartu:

- **Informasi Akun & Entitas Media**, yang dijelaskan di halaman ini.
- **Nomor WhatsApp**, yang dijelaskan di [Nomor WhatsApp](02-nomor-whatsapp.md).
- **Sindikasi Konten (RSS)**, yang dijelaskan di bagian bawah halaman ini.

Satu kotak peringatan, **Syarat Integrasi Belum Disetujui**, muncul di antara kartu bila Anda belum menyetujui syarat integrasi publikasi.

## Yang tampil di layar

### Kepala identitas

Di atas semua tab ada kepala identitas:

- Lingkaran berisi inisial dari dua kata pertama nama mitra.
- Nama entitas media Anda sebagai judul.
- Satu baris keterangan: **Mitra Media**, status kemitraan, dan jumlah **Media Terdaftar**. Status tampil dengan istilah bawaan sistem. Akun yang aktif tertulis `active`.
- Tombol **Edit Profil** di sisi kanan. Tombol ini hanya ada di tab **Profil**, dan hilang selama formulir edit terbuka.

### Kartu Informasi Akun & Entitas Media

Subjudul kartu: "Data kontak dan identitas resmi penerbit media partner di jaringan Tauthub". Isinya lima baris:

| Baris | Isinya | Bisa diubah di sini? |
|---|---|---|
| **Nama Entitas Media** | Nama media atau perusahaan Anda sebagai Mitra. | Ya, lewat **Edit Profil** (di formulir bernama **Nama Mitra**). |
| **Nama Penanggung Jawab** | Nama orang yang memegang akun. Nama ini tersimpan di akun login Anda. | Ya, lewat **Edit Profil**. |
| **Email Login (Penerbit)** | Email yang Anda pakai untuk masuk, dengan tanda **Email akun terverifikasi**. | Tidak. |
| **Status Kemitraan** | Status akun Mitra Anda. | Tidak. |
| **Media Terdaftar** | Jumlah media yang terdaftar atas akun Anda. | Tidak. Kelola media di [Daftar media](../04-daftar-media/README.md). |

## Cara menggunakan

### Cara mengubah nama penanggung jawab dan nama entitas media

1. Klik **Konfigurasi** di sidebar. Tab **Profil** terbuka.
2. Klik **Edit Profil** di sisi kanan kepala identitas. Kartu berubah menjadi formulir.
3. Ubah **Nama Penanggung Jawab** bila perlu. Isian ini wajib dan minimal 2 karakter.
4. Ubah **Nama Mitra** bila perlu. Isian ini wajib dan menjadi **Nama Entitas Media** setelah disimpan.
5. Klik **Simpan Profil**. Selama proses, tombol berubah menjadi **Menyimpan…**.
6. Tunggu pesan hijau **Informasi profil media partner berhasil disimpan.** Formulir menutup dan kartu menampilkan data terbaru.

Klik **Batal** untuk menutup formulir tanpa menyimpan. Di bawah isian tertulis keterangan: "Nama Penanggung Jawab disimpan ke akun login Anda; nama entitas media disimpan ke profil mitra."

### Cara menyetujui syarat integrasi lewat profil

Bila kotak **Syarat Integrasi Belum Disetujui** tampil, tidak ada langkah tambahan selain menyimpan profil.

1. Buka tab **Profil** dan klik **Edit Profil**.
2. Klik **Simpan Profil**, dengan atau tanpa mengubah isian.
3. Setelah berhasil, persetujuan tercatat dan kotak peringatan hilang.

> **Penting:** Kotak itu berbunyi, "Dengan menyimpan profil ini, Anda menyatakan menyetujui syarat integrasi publikasi Tauthub." Artinya, menyimpan profil untuk pertama kalinya otomatis menyetujui syarat tersebut. Syarat yang sama juga bisa disetujui langsung di menu **Integrasi**, yang dijelaskan di [Integrasi](../13-integrasi/README.md). Setelah disetujui, kotak tidak muncul lagi.

## Sindikasi Konten (RSS)

Kartu **Sindikasi Konten (RSS)** menyatakan bahwa pengaktifan hanya dapat dilakukan admin Tauthub dan bahwa fitur ini masih dalam pengembangan. Anda tidak bisa mengaktifkannya sendiri, dan halaman ini tidak menyediakan pengaturan RSS lain selain mematikannya.

- Status **Aktif** atau **Nonaktif** tidak tampil saat halaman baru dibuka. Status baru muncul di kanan judul kartu setelah Anda menyimpan profil.
- Bila statusnya **Aktif**, tombol **Matikan RSS** tersedia. Klik tombol itu, lalu setujui pertanyaan "Matikan sindikasi konten (RSS)? Pengaktifan hanya bisa dilakukan admin." Setelah berhasil, muncul pemberitahuan **Sindikasi RSS dimatikan.**
- Setelah dimatikan, Anda tidak bisa menyalakannya kembali sendiri. Hubungi tim Tauthub lewat [Dukungan](../12-dukungan.md) bila perlu mengaktifkannya lagi.
- Bila gagal, muncul **Gagal mematikan RSS. Coba lagi nanti.**

## Aturan dan batasan

- **Nama Penanggung Jawab:** wajib, minimal 2 karakter, dan maksimal 120 karakter.
- **Nama Mitra:** wajib diisi.
- **Dua penyimpanan sekaligus:** satu klik **Simpan Profil** menyimpan nama penanggung jawab ke akun login dan nama mitra ke profil mitra. Bila penyimpanan pertama berhasil tetapi yang kedua gagal, muncul pesan galat. Periksa kartu setelah memuat ulang halaman.
- **Hak akses:** setiap Mitra hanya melihat dan mengubah data akunnya sendiri.
- **Email login:** hanya ditampilkan, tidak bisa diubah di halaman ini.

## Pesan galat yang umum

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Nama penanggung jawab minimal 2 karakter. | Isian **Nama Penanggung Jawab** kurang dari 2 karakter. | Tulis nama yang lebih panjang. |
| Nama mitra wajib diisi | Isian **Nama Mitra** kosong. | Isi nama entitas media. |
| Gagal memperbarui profil. | Koneksi atau server bermasalah. | Klik **Simpan Profil** lagi. Bila terus gagal, buat tiket di [Dukungan](../12-dukungan.md). |
| display_name must be 2-120 characters | Nama penanggung jawab lebih dari 120 karakter. Pesan ini tampil dalam bahasa Inggris. | Persingkat nama menjadi maksimal 120 karakter. |
| Gagal memuat konfigurasi. | Data profil tidak terbaca. | Muat ulang halaman. |

## Pertanyaan yang sering diajukan

**Mengapa status kemitraan tertulis "active"?**
Layar menampilkan status dengan istilah bawaan sistem. Tulisan itu menandakan akun Mitra Anda aktif.

**Apakah mengubah nama entitas media mengubah nama media di Daftar Media?**
Tidak. Nama media dikelola per media di [Daftar media](../04-daftar-media/README.md). Kartu ini hanya mengubah nama entitas Mitra.

**Mengapa tombol Edit Profil tidak terlihat?**
Tombol itu hanya muncul di tab **Profil** dan hilang selama formulir edit terbuka. Pindah ke tab **Profil** atau klik **Batal** pada formulir.

## Lihat juga

- [Konfigurasi](README.md)
- [Nomor WhatsApp](02-nomor-whatsapp.md)
- [Integrasi](../13-integrasi/README.md)
- [Daftar media](../04-daftar-media/README.md)
- [Akun, peran, dan pindah dashboard](../../umum/03-akun-peran-dan-pindah-dashboard.md)
