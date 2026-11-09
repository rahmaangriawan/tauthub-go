# Profil dan Akun

> Menu Profil & Akun berisi identitas akun Anda, preferensi notifikasi (WhatsApp dan notifikasi perangkat), serta pengaturan keamanan (ganti password dan verifikasi dua langkah). Pengaturan di sini berlaku untuk akun Anda, bukan hanya untuk satu brand.

**Lokasi:** menu **PENGATURAN** › **Profil & Akun**, atau menu akun di bagian bawah menu samping › **Profil akun**
**Untuk:** Klien (semua anggota brand)

## Gambaran singkat

Halaman ini punya tiga tab: **Akun**, **Preferensi notifikasi**, dan **Keamanan**. Di bawah judul **Profil & Akun** tertulis "Kelola identitas akun, preferensi notifikasi, dan keamanan." Pesan keberhasilan (hijau) dan pesan galat (merah) tampil di atas deretan tab.

## Tab Akun

Tab ini menampilkan kartu identitas berisi inisial, nama tampilan, email, dan peran. Di bawahnya ada tiga baris: **Nama tampilan**, **Email**, dan **Peran**. Peran tampil dengan huruf besar, misalnya CLIENT untuk akun klien. Email dan peran tidak bisa diubah dari halaman ini.

### Cara mengubah nama tampilan

1. Klik **Ubah nama tampilan**.
2. Ketik nama baru, minimal 2 dan maksimal 100 karakter.
3. Klik **Simpan**. Tombol nonaktif jika nama belum berubah atau kurang dari 2 karakter.

Klik **Batal** atau tekan **Esc** untuk membatalkan. Setelah tersimpan muncul "Nama tampilan diperbarui." Nama baru tampil pada sapaan di Beranda. Tombol akun di menu samping memakai nama baru setelah halaman dimuat ulang. Jika gagal, muncul "Gagal memperbarui nama."

## Tab Preferensi notifikasi

### Nomor WhatsApp

Tab ini memuat kolom **Nomor WhatsApp** (contoh isian 62812xxxxxxx), kotak **Setuju dihubungi via WhatsApp**, dan tombol **Simpan preferensi**.

1. Ketik nomor WhatsApp Anda dalam angka, diawali 62 atau 0.
2. Centang **Setuju dihubungi via WhatsApp**.
3. Klik **Simpan preferensi**. Muncul "Preferensi notifikasi tersimpan."

Tombol **Simpan preferensi** hanya aktif jika kotak persetujuan dicentang. Halaman ini tidak punya cara terpisah untuk mencabut persetujuan.

Nomor dipakai untuk pemberitahuan status publikasi (disetujui, ditolak, terbit, atau gagal) dan untuk pemberitahuan login baru ke akun Anda. Pesan hanya terkirim jika layanan WhatsApp Tauthub sedang aktif. Nomor yang memuat huruf atau tidak lengkap tidak dipakai, dan sistem tidak menampilkan pesan galat untuk hal itu.

### Notifikasi perangkat

Kartu **Notifikasi perangkat** berada di bawah form WhatsApp. Kartu ini mengirim pemberitahuan status publikasi, revisi, dan pengumuman langsung ke browser di desktop atau ponsel Anda.

#### Cara mengaktifkan notifikasi di perangkat ini

1. Klik **Aktifkan notifikasi di perangkat ini**.
2. Klik izinkan pada permintaan dari browser.
3. Tunggu pesan "Notifikasi perangkat aktif."
4. Klik **Kirim notifikasi uji** untuk mencoba. Pesan seperti "1/1 perangkat menerima notifikasi uji." menunjukkan hasilnya.

Setiap browser atau perangkat harus diaktifkan sendiri. Untuk berhenti di perangkat ini, klik **Nonaktifkan perangkat ini**. Jika lebih dari satu perangkat terdaftar, tampil "N perangkat terdaftar untuk akun ini."

| Pesan di layar | Artinya | Yang perlu Anda lakukan |
|---|---|---|
| "Browser ini tidak mendukung notifikasi perangkat." | Browser tidak mendukung fitur ini | Pakai browser lain |
| "Di iPhone/iPad, notifikasi baru aktif setelah app dipasang…" | iPhone dan iPad hanya mengizinkan setelah situs dipasang di layar utama | Tekan **Bagikan** di Safari, pilih "Tambahkan ke Layar Utama", lalu buka dari ikon itu |
| "Notifikasi perangkat belum diaktifkan oleh admin platform." | Fitur belum diaktifkan oleh tim Tauthub | Tidak ada tindakan dari Anda |
| "Izin notifikasi diblokir di browser ini…" | Anda menolak izin sebelumnya | Buka pengaturan situs lewat ikon gembok di address bar, lalu izinkan notifikasi |

Penjelasan seluruh saluran notifikasi ada di [Saluran notifikasi](../umum/04-saluran-notifikasi.md).

## Tab Keamanan

### Cara mengganti password

1. Buka tab **Keamanan**.
2. Isi **Password saat ini**. Sesaat setelah Anda berhenti mengetik, halaman menampilkan "Password saat ini cocok." atau "Password saat ini tidak cocok dengan yang terdaftar."
3. Isi **Password baru (min. 10)**. Password baru harus minimal 10 karakter dengan kombinasi huruf besar, huruf kecil, angka, dan karakter khusus. Pesan merah tampil selama aturan itu belum terpenuhi.
4. Klik ikon mata di dalam kolom untuk menampilkan atau menyembunyikan isinya.
5. Klik **Simpan password**. Tombol nonaktif sampai password baru memenuhi aturan.

Setelah berhasil muncul "Password diubah. Sesi lain dicabut." Perangkat lain yang sedang masuk akan keluar dan harus masuk ulang. Sesi di perangkat ini tetap berjalan.

Jika password saat ini salah, muncul pesan "Current password is incorrect" dalam bahasa Inggris. Jika Anda terlalu sering mencoba, muncul "Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi."

### Cara mengaktifkan verifikasi dua langkah

Kartu **Two-Factor Authentication** menambahkan kode dari aplikasi autentikator saat Anda masuk.

1. Klik **Mulai setup 2FA**.
2. Pindai kode QR dengan aplikasi autentikator seperti Google Authenticator atau Authy. Anda juga dapat menyalin secret yang tertulis di bawah kode QR.
3. Ketik kode 6 digit dari aplikasi pada kolom yang tersedia.
4. Klik **Aktifkan**.
5. Simpan kode pemulihan yang muncul. Ada sepuluh kode, dan semuanya hanya tampil sekali.

Jika kode salah, muncul "Kode verifikasi salah." Setelah lima percobaan dalam satu menit, muncul "Terlalu banyak percobaan. Tunggu satu menit." Cara memakai kode saat masuk dibahas di [Daftar dan masuk](../umum/02-daftar-dan-masuk.md).

> **Penting:** Halaman ini tidak menampilkan apakah 2FA sudah aktif. Jika 2FA akun Anda sudah aktif lalu Anda klik **Mulai setup 2FA**, sistem membuat kunci baru dan menonaktifkan 2FA lama sampai Anda memasukkan kode dari aplikasi. Selesaikan langkah 2 sampai 4, atau 2FA tetap nonaktif. Halaman ini juga tidak punya tombol untuk menonaktifkan 2FA.

## Brand yang sedang dibuka

Dashboard Klien membuka satu brand pada satu waktu. Setelah masuk, sistem membawa Anda ke brand pertama Anda. Jika Anda membuka halaman awal Dashboard Klien tanpa menyebut brand, sistem membawa Anda ke brand yang terakhir Anda buka di browser itu, atau ke brand pertama jika belum ada. Tidak ada halaman atau menu untuk memilih brand lain, dan menu akun tidak memuat daftar brand.

Jika akun Anda belum punya brand, muncul halaman **Brand VIP belum dibuat** dengan lencana **VIP Partner**, keterangan "Anda dapat membuat brand sendiri untuk mulai menggunakan Dashboard Klien.", dan tombol **Buat Brand VIP**. Tombol ini hanya berhasil untuk akun Mitra dengan VIP aktif. Jika gagal, muncul "Gagal membuat brand. Pastikan akun VIP sudah aktif." Lihat [Akun, peran, dan pindah dashboard](../umum/03-akun-peran-dan-pindah-dashboard.md).

## Aturan dan batasan

- Nama tampilan 2 sampai 100 karakter. Email dan peran tidak dapat diubah dari halaman ini.
- Password baru minimal 10 karakter dengan huruf besar, huruf kecil, angka, dan karakter khusus.
- Semua anggota brand dapat membuka halaman ini, dan perubahan berlaku untuk akun Anda.

## Pertanyaan yang sering diajukan

**Mengapa tombol Simpan preferensi tidak bisa diklik?**
Kotak **Setuju dihubungi via WhatsApp** belum dicentang.

**Saya tidak menerima pesan WhatsApp. Apa penyebabnya?**
Periksa bahwa nomor berisi angka saja, kotak persetujuan tercentang dan tersimpan, serta layanan WhatsApp Tauthub sedang aktif.

**Bagaimana menonaktifkan 2FA?**
Halaman ini tidak menyediakan tombolnya. Hubungi tim Tauthub.

**Saya lupa password.**
Ikuti langkah pada [Daftar dan masuk](../umum/02-daftar-dan-masuk.md).

## Lihat juga

- [Saluran notifikasi](../umum/04-saluran-notifikasi.md)
- [Notifikasi](12-notifikasi.md)
- [Beranda](01-beranda.md)
