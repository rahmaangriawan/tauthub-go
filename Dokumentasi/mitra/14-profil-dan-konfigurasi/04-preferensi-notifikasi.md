# Preferensi notifikasi

> Tab **Notifikasi** di halaman **Konfigurasi** menyimpan dua email tambahan, nomor WhatsApp, dan izin WhatsApp Anda. Di tab yang sama ada kartu untuk mengaktifkan notifikasi di perangkat.

**Lokasi:** **Konfigurasi** › tab **Notifikasi**
**Untuk:** Mitra

## Gambaran singkat

Tab ini punya dua bagian:

- Kartu **Notifikasi** (dijelaskan di halaman ini), tempat Anda mengisi **Email Tambahan 1**, **Email Tambahan 2**, **Nomor WhatsApp**, dan izin WhatsApp.
- Kartu **Notifikasi perangkat**, yang dijelaskan di [Notifikasi perangkat](05-notifikasi-perangkat.md).

Notifikasi di lonceng dan halaman **Notifikasi** selalu ada dan tidak bergantung pada isian di sini. Nomor dan izin WhatsApp dipakai untuk kanal WhatsApp. Apa saja yang masuk lewat lonceng dijelaskan di [Notifikasi](../15-notifikasi.md), dan perbandingan kanalnya ada di [Saluran notifikasi](../../umum/04-saluran-notifikasi.md).

## Yang tampil di layar

Kartu **Notifikasi** berisi keterangan "Email tambahan & nomor WhatsApp penerima notifikasi dashboard Tauthub." dan satu formulir dengan empat isian:

| Isian | Fungsinya |
|---|---|
| **Email Tambahan 1** | Alamat email lain yang Anda simpan di profil, selain email login. |
| **Email Tambahan 2** | Alamat email kedua, juga selain email login. |
| **Nomor WhatsApp** | Nomor yang sama dengan kartu **Nomor WhatsApp** di tab **Profil**. |
| Kotak izin | "Klik setuju maka anda menyetujui bahwa nomor WA anda akan menjadi penerima notifikasi dari dashboard tauthub." |

Di bawahnya ada tombol **Simpan Pengaturan Notifikasi**.

Email login Anda tetap menjadi alamat utama akun dan tidak diubah dari sini. Email dari sistem, seperti pemberitahuan login dan pengaturan ulang kata sandi, dikirim ke email login Anda.

Bila media Anda sudah lebih dari satu dan nomor atau izin WhatsApp belum lengkap, kotak peringatan **Nomor WhatsApp wajib diisi** tampil di atas formulir. Isinya: "Media Anda sudah lebih dari satu, jadi nomor WhatsApp dan izin dihubungi wajib diisi. Menambah media baru diblokir sampai bagian ini lengkap." Aturan lengkapnya ada di [Nomor WhatsApp](02-nomor-whatsapp.md).

## Cara menggunakan

### Cara menyimpan email tambahan dan nomor WhatsApp

1. Klik **Konfigurasi** di sidebar, lalu klik tab **Notifikasi**.
2. Isi **Email Tambahan 1** dengan alamat email yang valid, misalnya `notif1@domain.com`.
3. Isi **Email Tambahan 2** dengan alamat email lain.
4. Isi **Nomor WhatsApp**. Bila nomor sudah pernah disimpan, kolom ini sudah terisi.
5. Centang kotak izin.
6. Klik **Simpan Pengaturan Notifikasi**. Selama proses, tombol berubah menjadi **Menyimpan…**.
7. Tunggu pesan hijau **Pengaturan notifikasi berhasil disimpan.**

> **Penting:** Tombol **Simpan Pengaturan Notifikasi** hanya aktif bila keempat isian sudah lengkap: kedua email tambahan terisi, nomor WhatsApp terisi, dan kotak izin dicentang. Bila Anda hanya ingin menyimpan nomor WhatsApp, gunakan kartu **Nomor WhatsApp** di tab **Profil**. Kartu itu hanya menuntut nomor dan izin.

### Cara mengganti email tambahan

1. Buka tab **Notifikasi**.
2. Hapus alamat lama di kolom **Email Tambahan 1** atau **Email Tambahan 2**, lalu ketik alamat baru.
3. Klik **Simpan Pengaturan Notifikasi**.

Halaman ini tidak punya cara untuk mengosongkan salah satu email, karena tombol simpan mengharuskan keduanya terisi.

## Aturan dan batasan

- **Format email:** harus berupa alamat email yang valid dan paling panjang 320 karakter. Huruf besar diubah menjadi huruf kecil saat disimpan.
- **Nomor WhatsApp:** paling panjang 30 karakter. Cara menulisnya dijelaskan di [Nomor WhatsApp](02-nomor-whatsapp.md).
- **Satu data, dua tempat:** nomor dan izin WhatsApp di tab ini sama dengan yang ada di kartu **Nomor WhatsApp** pada tab **Profil**. Menyimpan dari salah satu tidak menimpa kolom lain dengan nilai kosong.
- **Pengiriman email dan WhatsApp diatur tim Tauthub.** Tidak semua kejadian dikirim lewat email atau WhatsApp. Kejadian yang selalu tersedia adalah notifikasi di lonceng.
- **Tidak ada tombol hapus.** Halaman ini tidak bisa dipakai untuk mencabut izin WhatsApp atau mengosongkan nomor. Bila Anda perlu melakukannya, sampaikan lewat [Dukungan](../12-dukungan.md).
- **Hak akses:** setiap Mitra hanya mengelola preferensinya sendiri.

## Pesan galat yang umum

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Email tambahan 1 tidak valid | Alamat di **Email Tambahan 1** tidak berbentuk email yang benar. | Perbaiki penulisannya, misalnya nama@domain.com. |
| Email tambahan 2 tidak valid | Alamat di **Email Tambahan 2** tidak berbentuk email yang benar. | Perbaiki penulisannya. |
| Nomor WhatsApp terlalu panjang | Nomor lebih dari 30 karakter. | Hapus spasi atau karakter yang tidak perlu. |
| Permintaan gagal. | Koneksi atau server bermasalah. | Klik **Simpan Pengaturan Notifikasi** lagi. |

## Pertanyaan yang sering diajukan

**Mengapa tombol Simpan Pengaturan Notifikasi tidak bisa diklik?**
Salah satu dari empat isian belum lengkap. Pastikan kedua email tambahan dan nomor WhatsApp terisi serta kotak izin tercentang.

**Saya hanya ingin mengisi nomor WhatsApp.**
Pakai kartu **Nomor WhatsApp** di tab **Profil**. Kartu itu tidak menuntut email tambahan.

**Apakah saya tetap menerima notifikasi bila tidak mengisi apa pun di sini?**
Ya. Notifikasi di lonceng tetap masuk. Notifikasi perangkat diaktifkan terpisah lewat [Notifikasi perangkat](05-notifikasi-perangkat.md).

## Lihat juga

- [Konfigurasi](README.md)
- [Nomor WhatsApp](02-nomor-whatsapp.md)
- [Notifikasi perangkat](05-notifikasi-perangkat.md)
- [Notifikasi](../15-notifikasi.md)
- [Saluran notifikasi](../../umum/04-saluran-notifikasi.md)
