# Invoice dan pencairan dana

> Menu Invoice adalah arsip penarikan dana Anda. Setiap pengajuan tercatat sebagai invoice lengkap dengan status, tanggal selesai, dan bukti transfer. Halaman ini menjelaskan alur dari pengajuan sampai dana diterima.

**Lokasi:** menu **Invoice** (grup **KEUANGAN**). Judul halaman di layar adalah **Invoice & Penarikan**.
**Untuk:** Mitra

## Gambaran singkat

Alur pencairan dana:

1. Anda mengajukan penarikan di **Penghasilan** › **Tarik Saldo**.
2. Tauthub membuat invoice otomatis berstatus **Menunggu review**.
3. Admin Tauthub memeriksa pengajuan. Bila disetujui, status menjadi **Disetujui**. Setelah itu status menjadi **Sedang Ditransfer** (tahap ini kadang terlewati), lalu **Selesai / Terbayar**.
4. Bila ditolak, status menjadi **Ditolak / Dibatalkan** dan saldo Anda dikembalikan.

Langkah pengajuan ada di [Penghasilan](09-penghasilan.md). Halaman ini fokus pada invoice dan pemantauan statusnya.

## Yang tampil di layar

### Bagian atas halaman

Keterangan di bawah judul berbunyi "Daftar invoice penarikan dana, status pencairan, dan bukti transfer dari Tauthub", disusul batas minimum penarikan. Di kanan atas ada kotak pilihan **Semua status** untuk menyaring invoice.

### Tabel invoice

| Kolom | Isinya |
|---|---|
| **No. Invoice / ID** | Nomor invoice. Di bawahnya tertulis catatan pengajuan (30 karakter pertama), atau "Penarikan Mitra" bila Anda tidak menulis catatan |
| **Tanggal Pengajuan** | Tanggal Anda mengajukan penarikan |
| **Nominal Penarikan** | Nominal penarikan sebelum biaya transfer |
| **Status Pembayaran** | Status invoice. Alasan penolakan (bila ditolak) dan nomor referensi transfer (bila ada) tampil di bawah status |
| **Tanggal Selesai** | Tanggal transfer selesai, atau tanda "—" bila belum selesai |
| **Aksi** | Tombol **Detail & Bukti** |

Bila invoice lebih dari 20, di bawah tabel tampil keterangan seperti "Menampilkan 1–20 dari 35", tombol **Sebelumnya** dan **Berikutnya**, dan nomor halaman. Di HP, geser tabel ke samping untuk melihat semua kolom.

### Jendela Detail Invoice Penarikan

Klik **Detail & Bukti** untuk membukanya. Isinya:

- **STATUS PENCAIRAN**: nominal penarikan, lencana status, dan satu kalimat penjelasan status.
- **Nomor Invoice** dan **Tanggal Pengajuan**.
- **Status Transfer**: tanggal transfer, atau tulisan "Menunggu Transfer" dengan keterangan "Transfer akan diproses setelah disetujui."
- **Rekening Tujuan**: catatan pengajuan Anda bila ada, atau tulisan "Rekening terdaftar", dan di bawahnya metode **Transfer Bank** atau **E-Wallet**. Nomor rekening lengkap tidak ditampilkan di sini.
- **Nomor Referensi Transfer**: tampil bila tim Tauthub mengisinya.
- **Penarikan Ditolak Admin**: tampil bila ditolak, berisi alasan atau "Tidak ada alasan tercatat." dan keterangan "Saldo earnings telah dikembalikan dan dapat diajukan kembali."
- **Bukti Pembayaran / Transfer Bank**: gambar bukti transfer dengan tautan **Buka gambar ukuran penuh**. Bila bukti belum ada dan invoice sudah selesai, tampil "Pembayaran Selesai". Bila belum selesai, tampil "Belum tersedia".

> **Catatan:** Angka besar di jendela detail dan di tabel adalah nominal penarikan sebelum biaya transfer. Biaya transfer ditanggung Mitra dan dipotong dari nominal itu, jadi dana yang masuk ke rekening sama dengan **Diterima (net)** yang Anda lihat saat mengajukan.

## Cara menggunakan

### Cara mengajukan penarikan

1. Buka **Penghasilan**.
2. Klik **Tarik Saldo**.
3. Ikuti langkah di [Penghasilan](09-penghasilan.md), bagian "Cara menarik saldo".
4. Klik **Buka Invoice** pada kotak hijau "Permintaan penarikan terkirim" untuk melihat invoice Anda.

### Cara memantau status penarikan

1. Buka **Invoice**.
2. Cari invoice Anda di tabel. Invoice terbaru ada di bagian atas.
3. Baca kolom **Status Pembayaran**.
4. Klik **Detail & Bukti** untuk melihat penjelasan status.

### Cara menyaring invoice berdasarkan status

1. Klik kotak pilihan **Semua status** di kanan atas.
2. Pilih satu status: **Menunggu review**, **Disetujui**, **Sedang Ditransfer**, **Selesai / Terbayar**, atau **Ditolak / Dibatalkan**.
3. Pilih **Semua status** untuk menampilkan semua invoice lagi.

### Cara melihat bukti transfer

1. Klik **Detail & Bukti** pada invoice yang berstatus **Selesai / Terbayar**.
2. Gulir ke bagian **Bukti Pembayaran / Transfer Bank**.
3. Klik **Buka gambar ukuran penuh** untuk melihat gambar lebih besar.
4. Klik **Tutup** atau tekan Esc untuk menutup jendela.

## Status dan artinya

| Status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **Menunggu review** | Pengajuan sedang diperiksa oleh admin Tauthub. **Saldo siap ditarik** Anda sudah berkurang | Tunggu |
| **Disetujui** | Pengajuan disetujui admin dan menunggu proses transfer | Tunggu |
| **Sedang Ditransfer** | Dana sedang dalam proses transfer ke rekening Anda | Tunggu |
| **Selesai / Terbayar** | Dana sudah ditransfer. **Tanggal Selesai** terisi, dan bukti atau nomor referensi tampil bila tersedia | Periksa rekening. Bila dana belum terlihat, hubungi [Dukungan](12-dukungan.md) |
| **Ditolak / Dibatalkan** | Pengajuan ditolak atau dibatalkan admin. Alasan tampil di tabel dan di jendela detail. Saldo dikembalikan | Baca alasannya, perbaiki bila perlu, lalu ajukan lagi dari **Penghasilan** |

## Aturan dan batasan

### Syarat penarikan

- Rekening bank atau e-wallet sudah diisi di **Konfigurasi**, tab **Withdraw**.
- **Saldo siap ditarik** mencapai batas minimum penarikan. Batas ini ditetapkan Tauthub dan tertera di halaman ini, di **Penghasilan**, dan di jendela **Tarik Saldo**.
- Untuk **Tarik sebagian**, nominal harus mencapai batas minimum dan tidak melebihi saldo.
- Metode penerimaan mengikuti pengaturan di **Konfigurasi**. Jendela penarikan tidak bisa mengubahnya.

### Biaya transfer

Biaya transfer ditanggung Mitra. Untuk rekening bank, biayanya berupa nominal tetap. Untuk e-wallet, biayanya berupa persentase dari nominal penarikan. Angkanya ditetapkan Tauthub dan tertera di kotak biaya di **Penghasilan**, di jendela **Tarik Saldo**, dan di **Konfigurasi**, tab **Withdraw**. Jendela **Tarik Saldo** menghitung **Bruto**, **Biaya transfer**, dan **Diterima (net)** sebelum Anda mengajukan.

### Nomor invoice

Nomor invoice dibuat otomatis, terdiri dari 6 huruf besar dan angka, misalnya **K7M3QX**. Karakter yang mudah tertukar (0, O, 1, I, L) tidak dipakai. Invoice lama atau yang dibuat tim Tauthub dapat memakai format yang lebih panjang. Bila nomor tidak tersedia, tampil kode berawalan "INV-".

### Kapan dana diterima

Dashboard tidak menuliskan batas waktu pencairan. Pembayaran diproses tim Tauthub setelah review Admin. Dana dianggap sudah dikirim bila status invoice **Selesai / Terbayar** dan **Tanggal Selesai** terisi. Pantau status di halaman ini atau lewat notifikasi.

### Rekening tujuan dan pembatalan

- Rekening tujuan diambil dari **Konfigurasi**. Hindari mengubahnya selama ada pengajuan yang belum selesai. Bila Anda harus mengubahnya, beri tahu [Dukungan](12-dukungan.md).
- Pengajuan yang sudah terkirim tidak bisa dibatalkan dari dashboard.
- Anda tidak bisa membuka kembali pengajuan yang ditolak. Buat pengajuan baru dari **Penghasilan**.

## Notifikasi pencairan

- **Lonceng.** Tauthub menampilkan notifikasi saat pengajuan diterima ("Penarikan dana diterima"), sedang diproses ("Withdrawal sedang diproses"), dibayar ("Withdrawal dibayarkan"), atau ditolak ("Withdrawal ditolak", lengkap dengan alasan dan keterangan bahwa saldo dikembalikan). Klik notifikasi untuk membuka halaman **Invoice**.
- **Email.** Tauthub dapat mengirim email ke alamat akun Anda untuk keempat peristiwa itu, selama saluran email untuk peristiwa tersebut aktif. Email hanya dikirim untuk notifikasi yang belum Anda baca di lonceng.
- **WhatsApp.** Tauthub mengirim WhatsApp saat pencairan sedang diproses, sudah dibayar, atau ditolak. Isinya kurang lebih "Halo, pencairan [jumlah] sedang diproses", "... sudah DIBAYAR", atau "... ditolak" disertai alasan, lalu tautan ke halaman **Invoice**. WhatsApp tidak dikirim saat Anda baru mengajukan.

WhatsApp terkirim bila nomor WhatsApp dan izin dihubungi sudah Anda isi di **Konfigurasi**, dan layanan WhatsApp Tauthub aktif. Bila syarat itu belum terpenuhi, pesan dilewati tanpa pemberitahuan, tetapi notifikasi di lonceng tetap muncul. Lihat [Konfigurasi](14-profil-dan-konfigurasi/README.md) dan [Notifikasi](15-notifikasi.md).

## Pesan galat dan kondisi kosong

| Yang Anda lihat | Artinya | Langkah Anda |
|---|---|---|
| "Belum ada invoice penarikan dana. Ajukan penarikan dari menu Penghasilan saat saldo mencapai minimal ..." | Anda belum pernah mengajukan penarikan | Ajukan dari [Penghasilan](09-penghasilan.md) saat saldo cukup |
| "Gagal memuat daftar penarikan." | Daftar invoice gagal dimuat | Muat ulang halaman. Bila berulang, buat tiket di [Dukungan](12-dukungan.md) |
| "Memuat penarikan…" | Data sedang dimuat | Tunggu beberapa detik |
| "Belum tersedia" di bagian bukti | Admin belum menyelesaikan transfer | Tunggu sampai status **Selesai / Terbayar** |
| "Tidak ada alasan tercatat." | Penolakan tidak disertai alasan tertulis | Hubungi [Dukungan](12-dukungan.md) untuk penjelasan |

## Pertanyaan yang sering diajukan

**Berapa lama dana sampai ke rekening?**
Dashboard tidak menuliskan batas waktu. Pantau kolom **Status Pembayaran**. Bila status sudah **Selesai / Terbayar** tetapi dana belum terlihat, hubungi [Dukungan](12-dukungan.md) dengan menyertakan nomor invoice dan nomor referensi transfer.

**Mengapa dana yang masuk lebih kecil dari nominal di invoice?**
Biaya transfer ditanggung Mitra dan dipotong dari nominal penarikan. Dana yang masuk sama dengan **Diterima (net)** pada saat pengajuan.

**Mengapa saldo saya berkurang padahal invoice belum selesai?**
**Saldo siap ditarik** langsung berkurang begitu pengajuan terkirim. Bila pengajuan ditolak, saldo dikembalikan.

**Saya salah mengisi nominal. Bisakah pengajuan dibatalkan?**
Tidak ada tombol pembatalan di dashboard. Buat tiket di [Dukungan](12-dukungan.md) dan sebutkan nomor invoice.

**Di mana saya melihat biaya transfer?**
Lihat kotak biaya di [Penghasilan](09-penghasilan.md), atau rincian **Biaya transfer** di jendela **Tarik Saldo**.

## Lihat juga

- [Penghasilan](09-penghasilan.md)
- [Konfigurasi](14-profil-dan-konfigurasi/README.md)
- [Notifikasi](15-notifikasi.md)
- [Dukungan](12-dukungan.md)
- [Referral Mitra](08-referral-mitra.md)
