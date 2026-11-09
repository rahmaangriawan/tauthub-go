# Nomor WhatsApp

> Kartu **Nomor WhatsApp** menyimpan nomor yang dipakai tim Tauthub untuk mengonfirmasi pesanan Klien, beserta izin Anda untuk dihubungi. Mulai media kedua, nomor ini wajib diisi sebelum Anda bisa menambah media.

**Lokasi:** **Konfigurasi** › tab **Profil** › kartu **Nomor WhatsApp**. Nomor yang sama juga muncul di tab **Notifikasi**.
**Untuk:** Mitra

## Gambaran singkat

Kartu ini menyimpan dua hal: nomor WhatsApp Anda dan izin untuk dihubungi lewat nomor itu. Keduanya dipakai untuk:

- **Konfirmasi pesanan.** Keterangan di layar berbunyi, "Nomor untuk konfirmasi pesanan klien." Dengan nomor yang aktif, tim Tauthub bisa mengonfirmasi pesanan lebih cepat.
- **Notifikasi lewat WhatsApp.** Sebagian kejadian, seperti artikel terbit, media disetujui, atau pencairan dibayar, dapat dikirim ke nomor ini. Daftar kejadiannya ada di [Notifikasi](../15-notifikasi.md).
- **Pemberitahuan login.** Setiap kali akun Anda masuk dari sesi baru, pemberitahuan login dikirim ke email login Anda. Pesan WhatsApp ikut dikirim bila nomor dan izin Anda sudah tersimpan.

Pengiriman WhatsApp bergantung pada pengaturan tim Tauthub, jadi tidak semua kejadian dijamin sampai lewat kanal ini. Penjelasan umum kanal notifikasi ada di [Saluran notifikasi](../../umum/04-saluran-notifikasi.md).

## Yang tampil di layar

Kartu **Nomor WhatsApp** punya keterangan: "Nomor untuk konfirmasi pesanan klien. Menambah media baru diblokir sampai nomor dan izin dihubungi diisi." Isinya:

- Kolom **Nomor WhatsApp**.
- Kotak centang izin: "Saya setuju nomor ini disimpan dan dihubungi lewat WhatsApp untuk konfirmasi pesanan klien."
- Tombol **Simpan nomor WhatsApp**.

## Cara menggunakan

### Cara mengisi nomor WhatsApp

1. Klik **Konfigurasi** di sidebar. Tab **Profil** terbuka.
2. Cari kartu **Nomor WhatsApp**.
3. Ketik nomor Anda di kolom **Nomor WhatsApp**. Gunakan angka saja, dengan format seperti contoh di kolom (`6281234567890`: kode negara 62 lalu nomor tanpa angka 0 di depan). Nomor yang diawali 0 juga diterima dan diubah otomatis ke format dengan kode negara saat pesan dikirim.
4. Centang kotak izin di bawah kolom.
5. Klik **Simpan nomor WhatsApp**. Selama proses, tombol berubah menjadi **Menyimpan…**.
6. Tunggu pesan hijau **Nomor WhatsApp berhasil disimpan.**

Tombol **Simpan nomor WhatsApp** nonaktif sampai kolom nomor terisi dan kotak izin dicentang.

### Cara mengganti nomor WhatsApp

1. Buka kartu **Nomor WhatsApp**.
2. Hapus nomor lama dan ketik nomor baru. Pastikan kotak izin tetap tercentang.
3. Klik **Simpan nomor WhatsApp**.

Nomor baru langsung menggantikan nomor lama.

### Cara mengisi nomor ketika media Anda sudah lebih dari satu

Bila media Anda sudah lebih dari satu dan nomor belum lengkap, layar memberi tahu Anda di tiga tempat:

- Di halaman [Ringkasan](../01-ringkasan.md), kotak peringatan **Nomor WhatsApp wajib diisi** muncul dengan tombol **Isi nomor WhatsApp sekarang**. Tombol itu membuka halaman **Konfigurasi**.
- Di tab **Notifikasi**, kotak peringatan yang sama muncul di atas formulir.
- Di jendela **Daftar Media**, pesan galat muncul saat Anda mencoba menambah media, dengan tautan **Isi nomor WhatsApp sekarang**.

Langkah pemulihannya sama dengan "Cara mengisi nomor WhatsApp" di atas. Setelah nomor dan izin tersimpan, ulangi penambahan media.

## Aturan dan batasan

- **Media pertama bebas.** Anda boleh menambah media pertama tanpa nomor WhatsApp. Mulai media kedua, nomor dan izin wajib lengkap. Aturan ini juga berlaku untuk impor massal yang membuat jumlah media Anda lebih dari satu.
- **Media yang sudah ada tidak terganggu.** Aturan ini hanya menahan penambahan media baru.
- **Dua syarat sekaligus.** Nomor harus terisi dan izin harus dicentang. Salah satunya saja tidak cukup.
- **Panjang nomor:** maksimal 30 karakter.
- **Tidak ada verifikasi kode.** Tauthub tidak mengirim kode ke nomor Anda untuk memastikan kepemilikan. Pastikan nomor aktif dan bisa menerima WhatsApp.
- **Tidak ada tombol hapus.** Karena tombol simpan hanya aktif bila nomor terisi dan izin dicentang, halaman ini tidak bisa dipakai untuk mengosongkan nomor atau mencabut izin. Bila Anda perlu melakukannya, sampaikan lewat [Dukungan](../12-dukungan.md).
- **Satu data, dua tempat.** Kartu ini dan tab **Notifikasi** memakai data yang sama. Mengubah nomor di salah satunya langsung tercermin di yang lain.

> **Catatan:** Pesan galat server dan sebagian pesan di layar menyebut "Profil → tab Notifikasi" sebagai tempat mengisi nomor. Anda tidak harus ke sana. Kartu di tab **Profil** ini cukup dan lebih mudah, sebab tombol simpannya tidak menuntut kedua email tambahan terisi.

## Pesan galat yang umum

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Menambah media kedua dan seterusnya wajib mengisi nomor WhatsApp dan mencentang izin dihubungi. | Anda menambah media kedua atau berikutnya, tetapi nomor atau izin belum lengkap. | Isi nomor, centang izin, simpan, lalu tambahkan media lagi. |
| Nomor WhatsApp terlalu panjang | Nomor lebih dari 30 karakter. | Hapus spasi atau karakter yang tidak perlu. |
| Permintaan gagal. | Koneksi atau server bermasalah. | Klik **Simpan nomor WhatsApp** lagi. |

## Pertanyaan yang sering diajukan

**Apakah saya wajib mengisi nomor WhatsApp?**
Tidak untuk media pertama. Untuk media kedua dan seterusnya, ya.

**Mengapa notifikasi WhatsApp belum masuk padahal nomor sudah tersimpan?**
Kanal WhatsApp bergantung pada pengaturan tim Tauthub, dan tidak semua kejadian dikirim lewat WhatsApp. Notifikasi di lonceng dan perangkat tetap berjalan. Lihat [Notifikasi](../15-notifikasi.md).

**Format nomor apa yang sebaiknya saya tulis?**
Tulis nomor lengkap dengan kode negara, seperti contoh di layar yang memakai kode 62. Spasi, tanda hubung, titik, dan tanda kurung tidak masalah, tetapi huruf akan membuat nomor tidak bisa dipakai untuk mengirim pesan.

## Lihat juga

- [Konfigurasi](README.md)
- [Preferensi notifikasi](04-preferensi-notifikasi.md)
- [Daftar media](../04-daftar-media/README.md)
- [Ringkasan](../01-ringkasan.md)
- [Saluran notifikasi](../../umum/04-saluran-notifikasi.md)
