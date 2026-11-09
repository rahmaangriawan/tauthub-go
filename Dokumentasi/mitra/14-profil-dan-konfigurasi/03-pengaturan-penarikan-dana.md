# Pengaturan penarikan dana

> Tab **Withdraw** menyimpan rekening bank atau e-wallet yang menjadi tujuan pencairan penghasilan Anda. Tanpa data ini, Anda tidak bisa mengajukan penarikan dana.

**Lokasi:** **Konfigurasi** › tab **Withdraw**
**Untuk:** Mitra

## Gambaran singkat

Tab **Withdraw** hanya menyimpan tujuan transfer. Penarikan dana sendiri diajukan dari menu **Penghasilan**, dan riwayatnya ada di menu **Invoice**. Lihat [Penghasilan](../09-penghasilan.md) dan [Invoice dan pencairan](../10-invoice-dan-pencairan.md).

Anda memilih satu metode penerimaan dana: **Transfer Rekening Bank** atau **E-Wallet Digital**. Hanya satu tujuan yang tersimpan pada satu waktu. Tim Tauthub membaca data rekening dari profil Anda saat memproses pencairan, jadi data di tab ini harus selalu mutakhir.

Bila rekening atau e-wallet belum diisi, halaman **Penghasilan** menampilkan kotak kuning **Data rekening atau e-wallet belum diisi.** dengan tautan **Buka Profil**. Tautan itu membuka halaman **Konfigurasi** di tab **Profil**. Klik tab **Withdraw** setelahnya.

## Yang tampil di layar

Di tab **Withdraw** ada satu kartu bernama **Pengaturan Rekening & E-Wallet Penarikan Dana**, dengan keterangan: "Tentukan rekening atau e-wallet yang akan digunakan untuk menerima pencairan pendapatan dari publikasi." Isinya, dari atas ke bawah:

- **Metode Penerimaan Dana**: dua tombol pilihan, **Transfer Rekening Bank** (keterangan: BCA, Mandiri, BRI, BNI, BSI, dll) dan **E-Wallet Digital** (keterangan: GoPay, OVO, DANA, ShopeePay). Pilihan yang aktif ditandai ceklis.
- Kotak **Tentang Penarikan Dana**, berisi biaya transfer, petunjuk bahwa penarikan diajukan lewat menu **Penghasilan**, dan saldo minimal penarikan. Angka-angkanya ditentukan tim Tauthub dan ditampilkan langsung di kotak ini, jadi selalu ikuti angka yang tampil di layar. Bila angka belum terbaca, tertulis tanda hubung (-).
- Isian rekening atau e-wallet, yang berbeda menurut metode (lihat bagian cara mengisi).
- Tombol **Simpan Pengaturan**.

Kotak **Tentang Penarikan Dana** menyebut bahwa biaya transfer ditanggung Mitra: biaya tetap per penarikan untuk bank, atau persentase dari nominal untuk e-wallet.

## Cara menggunakan

### Cara mengisi rekening bank

1. Klik **Konfigurasi** di sidebar, lalu klik tab **Withdraw**.
2. Pilih **Transfer Rekening Bank** pada **Metode Penerimaan Dana**.
3. Pilih bank di kolom **Nama Bank**. Daftarnya bisa dicari lewat kotak **Cari bank…**. Pilihannya: BCA (Bank Central Asia), Bank Mandiri, BRI (Bank Rakyat Indonesia), BNI (Bank Negara Indonesia), BSI (Bank Syariah Indonesia), CIMB Niaga, Permata Bank, Bank Danamon, Bank Jago, Bank Tabungan Negara (BTN), dan **Bank Lainnya**.
4. Bila bank Anda tidak ada di daftar, pilih **Bank Lainnya**, lalu tulis nama bank di kolom **Nama Bank Lainnya**.
5. Isi **Nomor Rekening Bank** dengan angka saja. Karakter selain angka langsung terhapus saat Anda mengetik.
6. Isi **Nama Pemilik Rekening** dengan nama yang sama persis dengan buku tabungan.
7. Klik **Simpan Pengaturan**. Selama proses, tombol berubah menjadi **Menyimpan…**.
8. Tunggu pesan hijau **Pengaturan rekening penarikan dana (withdraw) berhasil disimpan**.

### Cara mengisi e-wallet

1. Klik **Konfigurasi** di sidebar, lalu klik tab **Withdraw**.
2. Pilih **E-Wallet Digital** pada **Metode Penerimaan Dana**.
3. Pilih penyedia di kolom **Penyedia E-Wallet**. Daftarnya bisa dicari lewat kotak **Cari e-wallet…**. Pilihannya: GoPay, OVO, DANA, ShopeePay, dan LinkAja.
4. Isi **Nomor Akun E-Wallet (Nomor HP)** dengan angka saja.
5. Isi **Nama Terdaftar Akun E-Wallet** sesuai nama pada akun e-wallet yang sudah terverifikasi KTP.
6. Klik **Simpan Pengaturan**.
7. Tunggu pesan hijau **Pengaturan rekening penarikan dana (withdraw) berhasil disimpan**.

### Cara mengganti rekening atau e-wallet

1. Buka tab **Withdraw**. Data yang tersimpan sudah terisi di formulir.
2. Pilih metode dan ubah isian yang perlu diganti.
3. Klik **Simpan Pengaturan**.

Data baru langsung menggantikan data lama. Bila Anda berpindah metode, isian untuk metode yang tidak dipilih tidak dipakai.

> **Penting:** Karena data rekening dibaca dari profil saat pencairan diproses, hindari mengubahnya selama ada penarikan berstatus **Menunggu review** atau **Sedang Ditransfer**. Cek statusnya di menu **Invoice**.

## Aturan dan batasan

- **Nomor rekening atau nomor HP:** angka saja, tanpa spasi atau tanda hubung. Isian ini wajib.
- **Nama pemilik:** wajib. Gunakan nama yang sama dengan buku tabungan (untuk bank) atau dengan akun e-wallet yang terverifikasi KTP.
- **Bank Lainnya:** nama bank wajib ditulis sendiri.
- **Biaya transfer dan saldo minimal:** ditentukan tim Tauthub dan tampil di kotak **Tentang Penarikan Dana**. Biaya dihitung saat Anda mengajukan penarikan.
- **Satu tujuan aktif:** metode, penyedia, nomor, dan nama yang tersimpan dipakai untuk semua penarikan berikutnya.
- **Hak akses:** Anda hanya bisa mengubah data rekening akun Anda sendiri.

## Pesan galat yang umum

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Harap isi nomor rekening bank. | Kolom nomor rekening kosong. | Isi nomor rekening. |
| Harap isi nomor HP e-wallet. | Kolom nomor e-wallet kosong. | Isi nomor HP yang terdaftar di e-wallet. |
| Nomor rekening hanya boleh berisi angka. | Nomor memuat karakter selain angka. | Hapus huruf, spasi, atau tanda hubung. |
| Harap isi nama pemilik rekening / nama terdaftar akun. | Kolom nama kosong. | Isi nama sesuai buku tabungan atau akun e-wallet. |
| Tuliskan nama bank Anda pada kolom nama bank. | **Bank Lainnya** dipilih, tetapi nama bank belum ditulis. | Isi kolom **Nama Bank Lainnya**. |
| Gagal menyimpan pengaturan withdraw. | Koneksi atau server bermasalah. | Klik **Simpan Pengaturan** lagi. |
| Lengkapi dulu nomor rekening atau e-wallet pencairan di Profil → Pengaturan Withdraw / Rekening. | Anda mengajukan penarikan di **Penghasilan** sebelum rekening tersimpan. | Isi dan simpan data di tab **Withdraw**, lalu ajukan penarikan lagi. |

## Pertanyaan yang sering diajukan

**Berapa biaya transfer dan saldo minimal penarikan?**
Angkanya diatur tim Tauthub dan tampil di kotak **Tentang Penarikan Dana** pada tab ini serta di halaman **Penghasilan**. Dokumen ini tidak mencantumkan angka karena bisa berubah.

**Bisakah saya menyimpan lebih dari satu rekening?**
Tidak. Tab ini menyimpan satu tujuan. Ubah data bila Anda ingin pindah rekening atau e-wallet.

**Di mana saya mengajukan penarikan?**
Di menu **Penghasilan**. Tab **Withdraw** hanya menyimpan tujuan transfer.

**Mengapa nama bank saya tampil sebagai Bank Lainnya?**
Bank yang tidak ada di daftar ditampilkan sebagai **Bank Lainnya**, dengan nama yang Anda tulis di kolom **Nama Bank Lainnya**.

## Lihat juga

- [Konfigurasi](README.md)
- [Penghasilan](../09-penghasilan.md)
- [Invoice dan pencairan](../10-invoice-dan-pencairan.md)
- [Dukungan](../12-dukungan.md)
