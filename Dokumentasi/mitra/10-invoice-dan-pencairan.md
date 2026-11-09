# Invoice dan pencairan dana

> Menu Invoice adalah arsip penarikan uang Anda. Invoice adalah catatan tagihan atau pembayaran. Setiap pengajuan tercatat sebagai satu invoice, lengkap dengan status, tanggal selesai, dan bukti transfer. Halaman ini menjelaskan alurnya, dari mengajukan sampai uang Anda terima.

**Lokasi:** menu **Invoice** (grup **KEUANGAN**). Judul halaman di layar adalah **Invoice & Penarikan**.
**Untuk:** Mitra

## Gambaran singkat

Alur pencairan uang:

1. Anda mengajukan penarikan di **Penghasilan** › **Tarik Saldo**.
2. Tauthub otomatis membuat invoice berstatus **Menunggu review**.
3. Admin Tauthub memeriksa pengajuan Anda. Kalau disetujui, status menjadi **Disetujui**. Setelah itu status menjadi **Sedang Ditransfer** (tahap ini kadang terlewati), lalu **Selesai / Terbayar**.
4. Kalau ditolak, status menjadi **Ditolak / Dibatalkan** dan saldo Anda dikembalikan.

Langkah mengajukan ada di [Penghasilan](09-penghasilan.md). Halaman ini fokus pada invoice dan cara memantau statusnya.

## Yang tampil di layar

### Bagian atas halaman

Keterangan di bawah judul berbunyi "Daftar invoice penarikan dana, status pencairan, dan bukti transfer dari Tauthub", disusul batas minimum penarikan. Di kanan atas ada kotak pilihan **Semua status** untuk menyaring invoice.

### Tabel invoice

| Kolom | Isinya |
|---|---|
| **No. Invoice / ID** | Nomor invoice. Di bawahnya ada catatan pengajuan (30 karakter pertama), atau "Penarikan Mitra" kalau Anda tidak menulis catatan |
| **Tanggal Pengajuan** | Tanggal Anda mengajukan penarikan |
| **Nominal Penarikan** | Jumlah penarikan sebelum biaya transfer |
| **Status Pembayaran** | Status invoice. Alasan penolakan (kalau ditolak) dan nomor referensi transfer (kalau ada) tampil di bawah status |
| **Tanggal Selesai** | Tanggal transfer selesai, atau tanda strip bila belum selesai |
| **Aksi** | Tombol **Detail & Bukti** |

Kalau invoice lebih dari 20, di bawah tabel muncul keterangan seperti "Menampilkan 1–20 dari 35", tombol **Sebelumnya** dan **Berikutnya**, dan nomor halaman. Di HP, geser tabel ke samping untuk melihat semua kolom.

### Jendela Detail Invoice Penarikan

Klik **Detail & Bukti** untuk membukanya. Isinya:

- **STATUS PENCAIRAN**: jumlah penarikan, lencana status, dan satu kalimat yang menjelaskan status. Lencana adalah label kecil.
- **Nomor Invoice** dan **Tanggal Pengajuan**.
- **Status Transfer**: tanggal transfer, atau tulisan "Menunggu Transfer" dengan keterangan "Transfer akan diproses setelah disetujui."
- **Rekening Tujuan**: catatan pengajuan Anda kalau ada, atau tulisan "Rekening terdaftar", dan di bawahnya metode **Transfer Bank** atau **E-Wallet**. Nomor rekening lengkap tidak tampil di sini.
- **Nomor Referensi Transfer**: tampil kalau tim Tauthub mengisinya.
- **Penarikan Ditolak Admin**: tampil kalau pengajuan ditolak. Isinya alasan, atau "Tidak ada alasan tercatat.", dan keterangan "Saldo earnings telah dikembalikan dan dapat diajukan kembali."
- **Bukti Pembayaran / Transfer Bank**: gambar bukti transfer dengan tautan **Buka gambar ukuran penuh**. Kalau bukti belum ada dan invoice sudah selesai, tampil "Pembayaran Selesai". Kalau belum selesai, tampil "Belum tersedia".

> **Catatan:** Angka besar di jendela detail dan di tabel adalah jumlah penarikan sebelum biaya transfer. Mitra menanggung biaya transfer, dan biaya itu dipotong dari jumlah tersebut. Jadi uang yang masuk ke rekening sama dengan **Diterima (net)** yang Anda lihat saat mengajukan.

## Cara menggunakan

### Cara mengajukan penarikan

1. Buka **Penghasilan**.
2. Klik **Tarik Saldo**.
3. Ikuti langkah di [Penghasilan](09-penghasilan.md), bagian "Cara menarik saldo".
4. Klik **Buka Invoice** di kotak hijau "Permintaan penarikan terkirim" untuk melihat invoice Anda.

### Cara memantau status penarikan

1. Buka **Invoice**.
2. Cari invoice Anda di tabel. Invoice terbaru ada di bagian atas.
3. Baca kolom **Status Pembayaran**.
4. Klik **Detail & Bukti** untuk membaca penjelasan status.

### Cara menyaring invoice berdasarkan status

1. Klik kotak pilihan **Semua status** di kanan atas.
2. Pilih satu status: **Menunggu review**, **Disetujui**, **Sedang Ditransfer**, **Selesai / Terbayar**, atau **Ditolak / Dibatalkan**.
3. Pilih **Semua status** untuk menampilkan semua invoice lagi.

### Cara melihat bukti transfer

1. Klik **Detail & Bukti** pada invoice yang berstatus **Selesai / Terbayar**.
2. Gulir ke bagian **Bukti Pembayaran / Transfer Bank**.
3. Klik **Buka gambar ukuran penuh** untuk melihat gambar yang lebih besar.
4. Klik **Tutup** atau tekan Esc untuk menutup jendela.

## Status dan artinya

| Status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **Menunggu review** | Admin Tauthub sedang memeriksa pengajuan Anda. **Saldo siap ditarik** Anda sudah berkurang | Tunggu |
| **Disetujui** | Admin sudah menyetujui pengajuan, dan transfer menunggu diproses | Tunggu |
| **Sedang Ditransfer** | Uang sedang dalam proses transfer ke rekening Anda | Tunggu |
| **Selesai / Terbayar** | Uang sudah ditransfer. **Tanggal Selesai** terisi, dan bukti atau nomor referensi tampil kalau ada | Periksa rekening. Kalau uang belum terlihat, hubungi [Dukungan](12-dukungan.md) |
| **Ditolak / Dibatalkan** | Admin menolak atau membatalkan pengajuan. Alasan tampil di tabel dan di jendela detail. Saldo dikembalikan | Baca alasannya, perbaiki bila perlu, lalu ajukan lagi dari **Penghasilan** |

## Aturan dan batasan

### Syarat penarikan

- Rekening bank atau e-wallet sudah Anda isi di **Konfigurasi**, tab **Withdraw**. E-wallet adalah dompet uang di aplikasi HP.
- **Saldo siap ditarik** sudah mencapai batas minimum penarikan. Tauthub menetapkan batas ini. Angkanya tertera di halaman ini, di **Penghasilan**, dan di jendela **Tarik Saldo**.
- Untuk **Tarik sebagian**, nominal harus mencapai batas minimum dan tidak boleh melebihi saldo.
- Metode penerimaan mengikuti pengaturan di **Konfigurasi**. Jendela penarikan tidak bisa mengubahnya.

### Biaya transfer

Mitra menanggung biaya transfer. Untuk rekening bank, biayanya jumlah rupiah yang tetap. Untuk e-wallet, biayanya persentase dari nominal penarikan. Tauthub menetapkan angkanya. Angkanya tertera di kotak biaya di **Penghasilan**, di jendela **Tarik Saldo**, dan di **Konfigurasi**, tab **Withdraw**. Sebelum Anda mengajukan, jendela **Tarik Saldo** sudah menghitung **Bruto**, **Biaya transfer**, dan **Diterima (net)**.

### Nomor invoice

Nomor invoice dibuat otomatis. Isinya 6 huruf besar dan angka, misalnya **K7M3QX**. Karakter yang mudah tertukar (0, O, 1, I, L) tidak dipakai. Invoice lama, atau invoice yang dibuat tim Tauthub, bisa memakai nomor yang lebih panjang. Kalau nomor tidak ada, yang tampil adalah kode berawalan "INV-".

### Kapan uang diterima

Dashboard tidak menuliskan batas waktu pencairan. Tim Tauthub memproses pembayaran setelah Admin memeriksanya. Uang dianggap sudah dikirim bila status invoice **Selesai / Terbayar** dan **Tanggal Selesai** terisi. Pantau status di halaman ini atau lewat notifikasi.

### Rekening tujuan dan pembatalan

- Rekening tujuan diambil dari **Konfigurasi**. Sebaiknya jangan ubah rekening selama ada pengajuan yang belum selesai. Kalau terpaksa mengubahnya, beri tahu [Dukungan](12-dukungan.md).
- Pengajuan yang sudah terkirim tidak bisa Anda batalkan dari dashboard.
- Pengajuan yang ditolak tidak bisa dibuka lagi. Buat pengajuan baru dari **Penghasilan**.

## Notifikasi pencairan

- **Lonceng.** Tauthub menampilkan notifikasi saat pengajuan diterima ("Penarikan dana diterima"), sedang diproses ("Withdrawal sedang diproses"), dibayar ("Withdrawal dibayarkan"), atau ditolak ("Withdrawal ditolak", lengkap dengan alasan dan keterangan bahwa saldo dikembalikan). Klik notifikasi untuk membuka halaman **Invoice**.
- **Email.** Tauthub bisa mengirim email ke alamat akun Anda untuk keempat kejadian itu, selama saluran email untuk kejadian tersebut aktif. Email hanya dikirim untuk notifikasi yang belum Anda baca di lonceng.
- **WhatsApp.** Tauthub mengirim WhatsApp saat pencairan sedang diproses, sudah dibayar, atau ditolak. Isinya kurang lebih "Halo, pencairan [jumlah] sedang diproses", "... sudah DIBAYAR", atau "... ditolak" disertai alasan, lalu tautan ke halaman **Invoice**. WhatsApp tidak dikirim saat Anda baru mengajukan.

WhatsApp terkirim bila dua hal ini beres. Pertama, Anda sudah mengisi nomor WhatsApp dan izin dihubungi di **Konfigurasi**. Kedua, layanan WhatsApp Tauthub sedang aktif. Kalau syarat itu belum terpenuhi, pesan dilewati tanpa kabar. Notifikasi di lonceng tetap muncul. Lihat [Konfigurasi](14-profil-dan-konfigurasi/README.md) dan [Notifikasi](15-notifikasi.md).

## Pesan galat dan kondisi kosong

| Yang Anda lihat | Artinya | Langkah Anda |
|---|---|---|
| "Belum ada invoice penarikan dana. Ajukan penarikan dari menu Penghasilan saat saldo mencapai minimal ..." | Anda belum pernah mengajukan penarikan | Ajukan dari [Penghasilan](09-penghasilan.md) saat saldo cukup |
| "Gagal memuat daftar penarikan." | Daftar invoice tidak berhasil dimuat | Muat ulang halaman. Kalau terulang, buat tiket di [Dukungan](12-dukungan.md) |
| "Memuat penarikan…" | Data sedang dimuat | Tunggu beberapa detik |
| "Belum tersedia" di bagian bukti | Admin belum menyelesaikan transfer | Tunggu sampai status **Selesai / Terbayar** |
| "Tidak ada alasan tercatat." | Penolakan tidak disertai alasan tertulis | Hubungi [Dukungan](12-dukungan.md) untuk meminta penjelasan |

## Pertanyaan yang sering diajukan

**Berapa lama uang sampai ke rekening?**
Dashboard tidak menuliskan batas waktu. Pantau kolom **Status Pembayaran**. Kalau status sudah **Selesai / Terbayar** tetapi uang belum terlihat, hubungi [Dukungan](12-dukungan.md) dan sertakan nomor invoice serta nomor referensi transfer.

**Mengapa uang yang masuk lebih kecil dari nominal di invoice?**
Mitra menanggung biaya transfer, dan biaya itu dipotong dari nominal penarikan. Uang yang masuk sama dengan **Diterima (net)** saat Anda mengajukan.

**Mengapa saldo saya berkurang padahal invoice belum selesai?**
**Saldo siap ditarik** langsung berkurang begitu pengajuan terkirim. Kalau pengajuan ditolak, saldo dikembalikan.

**Saya salah mengisi nominal. Bisakah pengajuan dibatalkan?**
Dashboard tidak punya tombol pembatalan. Buat tiket di [Dukungan](12-dukungan.md) dan sebutkan nomor invoice.

**Di mana saya melihat biaya transfer?**
Lihat kotak biaya di [Penghasilan](09-penghasilan.md), atau rincian **Biaya transfer** di jendela **Tarik Saldo**.

## Lihat juga

- [Penghasilan](09-penghasilan.md)
- [Konfigurasi](14-profil-dan-konfigurasi/README.md)
- [Notifikasi](15-notifikasi.md)
- [Dukungan](12-dukungan.md)
- [Referral Mitra](08-referral-mitra.md)
