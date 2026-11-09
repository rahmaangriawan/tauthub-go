# VIP Dashboard

> VIP Dashboard adalah halaman keanggotaan khusus Mitra VVIP. Isinya saldo affiliate, statistik referral dan domain, serta riwayat penghasilan. Halaman ini juga menjelaskan apa yang berbeda di layar antara Mitra Reguler, VIP Reguler, dan VVIP.

**Lokasi:** menu **VIP Dashboard** (grup **RINGKASAN**)
**Untuk:** Mitra VVIP dengan keanggotaan aktif. Mitra Reguler dan VIP Reguler tidak memiliki menu ini.

## Gambaran singkat

Keanggotaan VIP diaktifkan dan dinonaktifkan oleh tim Tauthub. Dashboard tidak memiliki tombol untuk mengajukannya. Ada dua tingkat keanggotaan:

- **VIP Reguler**: Mitra mendapat fee untuk setiap domain referral yang disetujui. Besar fee bertingkat menurut jumlah domain.
- **VVIP**: Mitra mendapat fee per domain, komisi transaksi, halaman **VIP Dashboard**, dan akses ke Dashboard Klien.

Saat keanggotaan diaktifkan, lonceng menampilkan notifikasi ucapan selamat yang menyebut tingkat Anda, yaitu VVIP atau VIP Reguler. Isinya merinci ketentuan awal tingkat itu. Saat keanggotaan dinonaktifkan, lonceng menampilkan "Status VIP Mitra dinonaktifkan".

## Perbedaan tampilan antar tingkat

| Yang tampil di layar | Mitra Reguler | VIP Reguler | VVIP |
|---|---|---|---|
| Pil tier di bagian atas layar | Tidak ada | **VIP Reguler** | **Brand Ambassador** |
| Hasil klik pil tier | Tidak ada pil | Membuka **Referral Mitra** | Membuka **VIP Dashboard** |
| Menu **VIP Dashboard** di sidebar | Tidak ada | Tidak ada | Ada |
| **Dashboard Klien** di menu akun | Tidak ada | Tidak ada | Ada |
| Lencana tier di **Referral Mitra** | **Regular** | **VIP Reguler** | **Brand Ambassador** |
| Kartu **VIP Mitra** di **Referral Mitra** | Tidak ada | Ada, memuat ringkasan fee per domain | Ada, dengan tombol **Dashboard Klien →** |

Satu tingkat bisa punya dua nama di layar. VVIP tertulis **Brand Ambassador** di pil tier dan di **Referral Mitra**, tetapi **VVIP** di halaman ini dan di notifikasi. Dashboard Klien kadang disebut Client Workspace di teks layar.

> **Catatan:** Panduan ini hanya mencakup yang tampil di dashboard: halaman ini, fee dan komisi pada kartu saldo, serta akses Dashboard Klien untuk VVIP. Besar fee, persentase komisi, dan masa kontrak ditetapkan tim Tauthub dan dapat berbeda antarmitra. Lihat angka yang berlaku untuk Anda di kartu pada halaman ini.

## Kontrak dan masa berlaku

Tim Tauthub menetapkan masa kontrak keanggotaan. Pada kartu **CPA Balance**, lencana "Berakhir" disertai tanggal menunjukkan akhir kontrak Anda.

Setelah tanggal berakhir, tingkat Anda kembali menjadi Mitra Reguler secara otomatis saat Anda membuka dashboard. Dampaknya di layar:

- Menu **VIP Dashboard** hilang. Bila Anda membuka alamat halamannya langsung, Anda dikembalikan ke **Ringkasan**.
- **Dashboard Klien** hilang dari menu akun, dan akses ke Dashboard Klien ditutup.
- Pil tier berubah menjadi **Reguler**.
- Penghasilan yang sudah tercatat tetap ada di [Penghasilan](09-penghasilan.md).

Bila Anda sempat membuka halaman ini tepat saat kontrak berakhir, tampil kotak peringatan "Masa kontrak VIP telah berakhir" disertai lama kontrak dan tanggal berakhir, lalu keterangan "CPA Balance nonaktif. Hubungi Admin untuk perpanjangan." Variannya berbunyi "Status VIP tidak aktif saat ini." Untuk perpanjangan, buat tiket di [Dukungan](12-dukungan.md).

## Yang tampil di layar

### Bagian atas halaman

Judul **VIP Dashboard** dengan keterangan tentang ringkasan affiliate, dompet, dan domain referral Anda. Di kanan atas ada lencana bergambar mahkota bertuliskan **VVIP**. Selama data dimuat, lencana bertuliskan "Memuat..." dan halaman menampilkan "Memuat VIP Dashboard…". Bila dompet gagal dimuat, tampil kotak merah "Gagal memuat dompet VIP. Coba lagi nanti."

### Saldo

| Kartu | Isinya | Keterangan di bawah angka |
|---|---|---|
| **CPA Balance** | Jumlah seluruh fee CPA yang tercatat untuk Anda. CPA adalah fee untuk setiap domain referral yang disetujui. Kartu ini hanya tampil saat keanggotaan aktif. | Nilai fee per domain, ditulis "per domain" |
| **Komisi Transaksi** | Jumlah seluruh komisi transaksi yang tercatat. Komisi dihitung dari nilai pesanan klien pada artikel yang terbit di media milik mitra yang Anda ajak. | Persentase komisi, ditulis "% dari transaksi" |
| **Total** | Akumulasi CPA dari domain yang disetujui pada daftar di bawah | "Akumulasi CPA", jumlah domain disetujui, dan nilai fee per domain |

Setiap kartu memiliki grafik mini. Angka pada kartu menghitung seluruh penghasilan yang pernah tercatat, bukan saldo yang bisa ditarik. Saldo yang bisa ditarik ada di [Penghasilan](09-penghasilan.md).

### Performance

| Baris | Artinya |
|---|---|
| **Total Klik** | Jumlah kunjungan ke tautan referral Anda |
| **Total Referral** | Jumlah akun yang mendaftar lewat tautan Anda |
| **Komisi Referral** | Total komisi referral yang tercatat |
| **Komisi Transaksi** | Sama dengan kartu **Komisi Transaksi** |

### Statistik Affiliate

| Baris | Artinya |
|---|---|
| **Total Klik** | Jumlah kunjungan ke tautan referral Anda |
| **Domain Terdaftar** | Jumlah domain referral |
| **Approved** | Domain yang disetujui |
| **Pending** | Domain yang menunggu persetujuan |
| **Ditolak** | Domain yang ditolak |
| **Estimasi CPA** | Perkiraan fee: jumlah domain **Approved** dikalikan nilai fee per domain |

### Riwayat Penghasilan Affiliate

Bagian ini memuat daftar domain referral. Di kanan atas ada dua lencana: jumlah "domain sukses" (yang disetujui) dan jumlah seluruh domain. Bila ada domain disetujui, tampil baris ringkasan total CPA. Daftar memuat hingga 50 domain terbaru.

| Kolom | Isinya |
|---|---|
| **Domain** | Nama domain dan nama media |
| **Owner** | Email pemilik domain |
| **Status** | Status domain; alasan penolakan tampil di bawahnya |
| **CPA** | Nominal fee bila sudah dicatat, atau tanda "—" |
| **Tanggal** | Tanggal domain terdaftar |

Baris domain yang disetujui berlatar hijau. Bila daftar kosong, tampil "Belum ada domain referral. Bagikan link referral Anda untuk memulai."

### Riwayat Earning

Bagian ini memuat hingga 200 entri penghasilan terbaru, dengan lencana jumlah "transaksi".

| Kolom | Isinya |
|---|---|
| **Jenis** | **CPA**, **Komisi Transaksi**, **Referral**, atau **Publikasi** |
| **Jumlah** | Nominal entri |
| **Status** | **Menunggu**, **Selesai**, atau **Dibatalkan** |
| **Tanggal** | Tanggal entri tercatat |

## Cara menggunakan

### Cara membuka VIP Dashboard

1. Klik **VIP Dashboard** di sidebar, atau klik pil **Brand Ambassador** di bagian atas layar.
2. Tunggu sampai tulisan "Memuat VIP Dashboard…" berganti dengan kartu saldo.

### Cara melihat riwayat earning

1. Gulir ke bagian **Riwayat Earning**.
2. Klik pilihan **Tampilkan** (tulisannya misalnya "10 per hal"), lalu pilih 5, 10, 20, atau 50 baris per halaman.
3. Klik **Berikutnya** atau **Sebelumnya** untuk berpindah halaman. Keterangan di bawah tabel menunjukkan posisi Anda, misalnya "Halaman 1 / 3".

### Cara pindah ke Dashboard Klien

1. Klik area akun di bagian bawah sidebar.
2. Klik **Dashboard Klien**.
3. Tunggu Tauthub membuka Dashboard Klien. Pada pemakaian pertama, Tauthub menyiapkan satu brand atas nama mitra Anda secara otomatis.

Anda juga bisa menekan tombol **Dashboard Klien →** pada kartu **VIP Mitra** di [Referral Mitra](08-referral-mitra.md). Untuk memahami isi Dashboard Klien, baca [panduan Dashboard Klien](../klien/README.md). Cara berpindah dashboard dijelaskan di [Akun, peran, dan pindah dashboard](../umum/03-akun-peran-dan-pindah-dashboard.md).

## Status dan artinya

**Status domain referral**

| Status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **Menunggu** | Domain sudah terdaftar tetapi belum disetujui. Domain disetujui setelah pemilik media membuktikan kepemilikan domain, atau setelah ditinjau tim Tauthub | Tunggu. Tidak ada tindakan di sisi Anda |
| **Disetujui** | Domain disetujui. Fee CPA dicatat satu kali untuk domain ini, dan Anda menerima notifikasi "CPA ... dikreditkan" | Tidak ada |
| **Ditolak** | Tim Tauthub menolak domain. Alasannya tampil dengan huruf merah di bawah status | Tidak ada fee untuk domain ini. Hubungi [Dukungan](12-dukungan.md) bila perlu penjelasan |

**Status entri di Riwayat Earning**

| Status | Artinya | Langkah Anda berikutnya |
|---|---|---|
| **Menunggu** | Entri baru, masih dalam masa tunggu 2 hari (48 jam) | Tunggu sampai masa tunggu selesai |
| **Selesai** | Masa tunggu berakhir. Nominal sudah menjadi bagian dari saldo di [Penghasilan](09-penghasilan.md), atau sudah diajukan untuk ditarik | Ajukan penarikan di **Penghasilan** bila saldo masih tersedia |
| **Dibatalkan** | Entri dibatalkan dan tidak dihitung | Hubungi [Dukungan](12-dukungan.md) bila Anda tidak mengetahui sebabnya |

## Aturan dan batasan

- Menu dan halaman **VIP Dashboard** hanya untuk VVIP. Mitra Reguler dan VIP Reguler yang membuka alamat halamannya dikembalikan ke **Ringkasan**.
- Kartu **CPA Balance** hanya tampil saat keanggotaan aktif.
- Fee CPA dicatat satu kali untuk setiap domain referral.
- Komisi dan fee masuk ke saldo penghasilan dengan masa tunggu 2 hari (48 jam), sama seperti penghasilan dari artikel.
- Dashboard Klien hanya terbuka untuk VVIP yang kontraknya belum berakhir.
- Fee, persentase komisi, dan masa kontrak ditetapkan tim Tauthub.

## Pertanyaan yang sering diajukan

**Saya VIP Reguler. Mengapa tidak ada menu VIP Dashboard?**
Menu ini hanya untuk VVIP. Mitra VIP Reguler memantau komisinya di [Referral Mitra](08-referral-mitra.md). Klik pil **VIP Reguler** di bagian atas layar untuk membukanya.

**Mengapa kartu CPA Balance tidak tampil?**
Kartu ini hanya tampil saat keanggotaan VIP aktif. Cek apakah kontrak Anda sudah berakhir.

**Mengapa Komisi Transaksi bernilai Rp0?**
Komisi tercatat bila ada artikel yang terbit di media milik mitra yang mendaftar lewat tautan Anda. Bila Anda yakin ada transaksi tetapi angkanya tetap Rp0, buat tiket di [Dukungan](12-dukungan.md) dan sertakan tanggal artikel.

**Mengapa daftar domain referral kosong?**
Belum ada domain yang tercatat dari mitra yang mendaftar lewat tautan Anda. Bagikan tautan Anda dari [Referral Mitra](08-referral-mitra.md).

**Bagaimana memperpanjang keanggotaan?**
Hubungi tim Tauthub lewat [Dukungan](12-dukungan.md).

## Lihat juga

- [Ringkasan](01-ringkasan.md)
- [Referral Mitra](08-referral-mitra.md)
- [Penghasilan](09-penghasilan.md)
- [Dukungan](12-dukungan.md)
- [Akun, peran, dan pindah dashboard](../umum/03-akun-peran-dan-pindah-dashboard.md)
- [Dashboard Klien](../klien/README.md)
