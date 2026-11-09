# Profil dan Akun

> Menu Profil & Akun berisi data diri akun Anda, pilihan pemberitahuan (WhatsApp dan notifikasi di perangkat), dan pengaturan keamanan (ganti password dan verifikasi dua langkah). Pengaturan di sini berlaku untuk akun Anda, bukan hanya untuk satu brand.

**Lokasi:** menu **PENGATURAN** › **Profil & Akun**, atau menu akun di bagian bawah menu samping › **Profil akun**
**Untuk:** Klien (semua anggota brand)

## Gambaran singkat

Halaman ini punya tiga tab: **Akun**, **Preferensi notifikasi**, dan **Keamanan**. Di bawah judul **Profil & Akun** tertulis "Kelola identitas akun, preferensi notifikasi, dan keamanan." Pesan berhasil (hijau) dan pesan galat (merah) tampil di atas deretan tab.

## Tab Akun

Tab ini menampilkan kartu data diri berisi huruf awal nama, nama tampilan, email, dan peran. Di bawahnya ada tiga baris: **Nama tampilan**, **Email**, dan **Peran**. Peran ditulis dengan huruf besar, misalnya CLIENT untuk akun klien. Anda tidak bisa mengubah email dan peran dari halaman ini.

### Cara mengubah nama tampilan

1. Klik **Ubah nama tampilan**.
2. Ketik nama baru, paling sedikit 2 dan paling banyak 100 huruf.
3. Klik **Simpan**. Tombol mati bila nama belum berubah atau kurang dari 2 huruf.

Klik **Batal** atau tekan **Esc** untuk membatalkan. Setelah tersimpan, muncul "Nama tampilan diperbarui." Nama baru tampil pada sapaan di Beranda. Tombol akun di menu samping memakai nama baru setelah Anda memuat ulang halaman. Bila gagal, muncul "Gagal memperbarui nama."

## Tab Preferensi notifikasi

### Nomor WhatsApp

Tab ini punya kolom **Nomor WhatsApp** (contoh isian 62812xxxxxxx), kotak **Setuju dihubungi via WhatsApp**, dan tombol **Simpan preferensi**.

1. Ketik nomor WhatsApp Anda dengan angka saja, diawali 62 atau 0.
2. Centang **Setuju dihubungi via WhatsApp**.
3. Klik **Simpan preferensi**. Muncul "Preferensi notifikasi tersimpan."

Tombol **Simpan preferensi** baru bisa diklik bila kotak persetujuan dicentang. Halaman ini tidak punya cara terpisah untuk mencabut persetujuan.

Nomor ini dipakai untuk kabar status publikasi (disetujui, ditolak, terbit, atau gagal) dan untuk kabar bila ada yang login baru ke akun Anda. Pesan hanya terkirim bila layanan WhatsApp Tauthub sedang aktif. Nomor yang berisi huruf atau tidak lengkap tidak dipakai, dan layar tidak menampilkan pesan galat untuk hal itu.

### Notifikasi perangkat

Kartu **Notifikasi perangkat** ada di bawah formulir WhatsApp. Kartu ini mengirim kabar status publikasi, revisi, dan pengumuman langsung ke browser (aplikasi untuk membuka internet) di komputer atau ponsel Anda.

#### Cara mengaktifkan notifikasi di perangkat ini

1. Klik **Aktifkan notifikasi di perangkat ini**.
2. Klik izinkan pada permintaan dari browser.
3. Tunggu pesan "Notifikasi perangkat aktif."
4. Klik **Kirim notifikasi uji** untuk mencoba. Pesan seperti "1/1 perangkat menerima notifikasi uji." menunjukkan hasilnya.

Setiap browser atau perangkat harus Anda aktifkan sendiri-sendiri. Untuk berhenti di perangkat ini, klik **Nonaktifkan perangkat ini**. Bila lebih dari satu perangkat terdaftar, layar menulis "N perangkat terdaftar untuk akun ini."

| Pesan di layar | Artinya | Yang perlu Anda lakukan |
|---|---|---|
| "Browser ini tidak mendukung notifikasi perangkat." | Browser tidak mendukung fitur ini | Pakai browser lain |
| "Di iPhone/iPad, notifikasi baru aktif setelah app dipasang…" | iPhone dan iPad hanya mengizinkan setelah situs dipasang di layar utama | Tekan **Bagikan** di Safari, pilih "Tambahkan ke Layar Utama", lalu buka dari ikon itu |
| "Notifikasi perangkat belum diaktifkan oleh admin platform." | Tim Tauthub belum mengaktifkan fitur ini | Anda tidak perlu berbuat apa pun |
| "Izin notifikasi diblokir di browser ini…" | Anda pernah menolak izin | Buka pengaturan situs lewat ikon gembok di kolom alamat, lalu izinkan notifikasi |

Penjelasan semua saluran notifikasi ada di [Saluran notifikasi](../umum/04-saluran-notifikasi.md).

## Tab Keamanan

### Cara mengganti password

1. Buka tab **Keamanan**.
2. Isi **Password saat ini**. Sesaat setelah Anda berhenti mengetik, halaman menulis "Password saat ini cocok." atau "Password saat ini tidak cocok dengan yang terdaftar."
3. Isi **Password baru (min. 10)**. Password baru minimal 10 huruf, dan harus berisi huruf besar, huruf kecil, angka, dan tanda khusus. Pesan merah tampil selama aturan itu belum terpenuhi.
4. Klik ikon mata di dalam kolom untuk menampilkan atau menyembunyikan isinya.
5. Klik **Simpan password**. Tombol mati sampai password baru memenuhi aturan.

Setelah berhasil, muncul "Password diubah. Sesi lain dicabut." Perangkat lain yang sedang masuk akan keluar, dan Anda harus masuk lagi di sana. Perangkat ini tetap masuk.

Bila password saat ini salah, muncul pesan "Current password is incorrect" dalam bahasa Inggris. Bila Anda terlalu sering mencoba, muncul "Terlalu banyak percobaan. Tunggu satu menit lalu coba lagi."

### Cara mengaktifkan verifikasi dua langkah

Kartu **Two-Factor Authentication** (2FA) menambah satu kode lagi saat Anda masuk. Kode itu datang dari aplikasi autentikator, yaitu aplikasi di ponsel yang membuat kode angka.

1. Klik **Mulai setup 2FA**.
2. Pindai kode QR dengan aplikasi autentikator seperti Google Authenticator atau Authy. Anda juga bisa menyalin secret (kunci rahasia) yang tertulis di bawah kode QR.
3. Ketik kode 6 angka dari aplikasi di kolom yang ada.
4. Klik **Aktifkan**.
5. Simpan kode pemulihan yang muncul. Ada sepuluh kode, dan semuanya hanya tampil sekali.

Bila kode salah, muncul "Kode verifikasi salah." Setelah lima percobaan dalam satu menit, muncul "Terlalu banyak percobaan. Tunggu satu menit." Cara memakai kode saat masuk ada di [Daftar dan masuk](../umum/02-daftar-dan-masuk.md).

> **Penting:** Halaman ini tidak menampilkan apakah 2FA sudah aktif. Bila 2FA akun Anda sudah aktif lalu Anda klik **Mulai setup 2FA**, sistem membuat kunci baru dan mematikan 2FA lama sampai Anda memasukkan kode dari aplikasi. Selesaikan langkah 2 sampai 4, atau 2FA tetap mati. Halaman ini juga tidak punya tombol untuk mematikan 2FA.

## Brand yang sedang dibuka

Dashboard Klien membuka satu brand dalam satu waktu. Setelah Anda masuk, sistem membawa Anda ke brand pertama Anda. Bila Anda membuka halaman awal Dashboard Klien tanpa menyebut brand, sistem membawa Anda ke brand yang terakhir Anda buka di browser itu. Bila belum ada, sistem membawa Anda ke brand pertama. Tidak ada halaman atau menu untuk memilih brand lain, dan menu akun tidak memuat daftar brand.

Bila akun Anda belum punya brand, muncul halaman **Brand VIP belum dibuat** dengan label **VIP Partner**, keterangan "Anda dapat membuat brand sendiri untuk mulai menggunakan Dashboard Klien.", dan tombol **Buat Brand VIP**. Tombol ini hanya berhasil untuk akun Mitra dengan VIP aktif. Bila gagal, muncul "Gagal membuat brand. Pastikan akun VIP sudah aktif." Lihat [Akun, peran, dan pindah dashboard](../umum/03-akun-peran-dan-pindah-dashboard.md).

## Aturan dan batasan

- Nama tampilan 2 sampai 100 huruf. Email dan peran tidak bisa diubah dari halaman ini.
- Password baru minimal 10 huruf, dengan huruf besar, huruf kecil, angka, dan tanda khusus.
- Semua anggota brand bisa membuka halaman ini, dan perubahan berlaku untuk akun Anda.

## Pertanyaan yang sering diajukan

**Mengapa tombol Simpan preferensi tidak bisa diklik?**
Kotak **Setuju dihubungi via WhatsApp** belum Anda centang.

**Saya tidak menerima pesan WhatsApp. Apa penyebabnya?**
Periksa tiga hal. Nomor harus berisi angka saja. Kotak persetujuan harus dicentang dan sudah tersimpan. Layanan WhatsApp Tauthub harus sedang aktif.

**Bagaimana mematikan 2FA?**
Halaman ini tidak punya tombolnya. Hubungi tim Tauthub.

**Saya lupa password.**
Ikuti langkah di [Daftar dan masuk](../umum/02-daftar-dan-masuk.md).

## Lihat juga

- [Saluran notifikasi](../umum/04-saluran-notifikasi.md)
- [Notifikasi](12-notifikasi.md)
- [Beranda](01-beranda.md)
