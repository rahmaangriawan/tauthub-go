# Notifikasi perangkat

> Kartu **Notifikasi perangkat** mengaktifkan pemberitahuan Tauthub langsung di layar desktop atau HP Anda, bahkan saat tab Tauthub tidak sedang dibuka. Pengaturannya berlaku per perangkat dan browser.

**Lokasi:** **Konfigurasi** › tab **Notifikasi** › kartu **Notifikasi perangkat**
**Untuk:** Mitra

## Gambaran singkat

Notifikasi perangkat adalah pemberitahuan yang muncul di layar perangkat Anda, seperti notifikasi aplikasi lain. Keterangan di kartu berbunyi: "Terima pemberitahuan status publikasi, revisi, dan pengumuman langsung di desktop atau ponsel Anda."

Setiap notifikasi baru yang masuk ke lonceng Anda juga dikirim ke perangkat yang sudah diaktifkan. Mengetuk notifikasi membuka Tauthub di halaman yang terkait, misalnya publikasi, tiket, atau pesan. Notifikasi yang sudah lebih dari 48 jam tidak lagi dikirim ke perangkat, tetapi tetap ada di lonceng.

Pengaturan ini berlaku untuk satu browser di satu perangkat. Untuk menerima notifikasi di laptop dan HP, aktifkan di keduanya.

## Yang tampil di layar

Kartu berjudul **Notifikasi perangkat** dengan ikon lonceng, satu paragraf keterangan, dan tombol yang berubah menurut keadaan perangkat Anda:

| Yang Anda lihat | Artinya |
|---|---|
| Tombol **Aktifkan notifikasi di perangkat ini** | Perangkat ini belum terdaftar. Klik untuk mengaktifkan. |
| Tombol **Nonaktifkan perangkat ini** dan **Kirim notifikasi uji** | Perangkat ini sudah terdaftar. |
| Teks **N perangkat terdaftar untuk akun ini.** | Ada lebih dari satu perangkat terdaftar. Teks ini hanya tampil saat perangkat ini sudah aktif. |
| Teks **Browser ini tidak mendukung notifikasi perangkat.** | Browser Anda tidak mendukung fitur ini. Coba browser lain. |
| Teks **Notifikasi perangkat belum diaktifkan oleh admin platform.** | Tim Tauthub belum menyalakan fitur ini, jadi tidak ada tombol yang bisa dipakai. |
| Teks merah tentang izin diblokir | Browser Anda menolak izin notifikasi untuk situs Tauthub. Lihat bagian pemecahan masalah di bawah. |
| Kotak petunjuk tentang iPhone atau iPad | Perangkat Apple yang belum memasang Tauthub ke Layar Utama. Lihat langkahnya di bawah. |

## Cara menggunakan

### Cara mengaktifkan notifikasi di perangkat

1. Klik **Konfigurasi** di sidebar, lalu klik tab **Notifikasi**.
2. Gulir ke kartu **Notifikasi perangkat**.
3. Klik **Aktifkan notifikasi di perangkat ini**. Browser menampilkan permintaan izin.
4. Pilih izinkan (allow) pada permintaan itu.
5. Tunggu pemberitahuan **Notifikasi perangkat aktif.** dan tombol berganti menjadi **Nonaktifkan perangkat ini**.

Selama proses, tombol menampilkan ikon berputar dan tidak bisa diklik.

### Cara mencoba notifikasi

1. Pastikan perangkat sudah aktif, yaitu tombol **Nonaktifkan perangkat ini** terlihat.
2. Klik **Kirim notifikasi uji**.
3. Lihat layar perangkat Anda. Notifikasi uji berjudul "Notifikasi perangkat aktif" dengan isi "Tauthub akan mengabari Anda di perangkat ini."
4. Periksa pemberitahuan di Tauthub. Teksnya berbentuk **N/M perangkat menerima notifikasi uji.**, misalnya "1/1".

Notifikasi uji dikirim ke semua perangkat yang terdaftar untuk akun Anda dan tidak muncul di lonceng.

### Cara menonaktifkan notifikasi di perangkat

1. Buka tab **Notifikasi** di **Konfigurasi** dari perangkat yang ingin dimatikan.
2. Klik **Nonaktifkan perangkat ini**.

Hanya perangkat yang sedang Anda pakai yang dinonaktifkan. Perangkat lain tetap aktif.

### Cara mengaktifkan notifikasi di iPhone atau iPad

Di iPhone dan iPad, notifikasi baru bisa aktif setelah Tauthub dipasang ke Layar Utama.

1. Buka Tauthub di Safari.
2. Tekan tombol Bagikan.
3. Pilih **Tambahkan ke Layar Utama**.
4. Buka Tauthub dari ikon yang baru muncul di Layar Utama.
5. Masuk ke **Konfigurasi** › tab **Notifikasi**, lalu ikuti langkah "Cara mengaktifkan notifikasi di perangkat".

### Cara memperbaiki izin yang diblokir

Bila kartu menampilkan teks merah tentang izin yang diblokir, browser telah menolak izin untuk Tauthub. Layar menyarankan: "Buka pengaturan situs (ikon gembok di address bar) lalu izinkan notifikasi."

1. Klik ikon gembok di sebelah alamat situs pada browser Anda.
2. Ubah pengaturan **Notifikasi** untuk Tauthub menjadi diizinkan.
3. Muat ulang halaman.
4. Klik **Aktifkan notifikasi di perangkat ini** lagi.

## Aturan dan batasan

- **Per perangkat:** tiap browser di tiap perangkat didaftarkan sendiri. Bila Anda memakai dua browser di satu laptop, aktifkan di keduanya.
- **Perangkat bersama:** bila dua akun memakai browser yang sama, perangkat itu berpindah ke akun yang terakhir mengaktifkannya.
- **Keluar dari akun tidak otomatis menonaktifkan perangkat.** Pada perangkat bersama, klik **Nonaktifkan perangkat ini** sebelum keluar.
- **Notifikasi sejenis dapat saling menggantikan.** Bila beberapa notifikasi dengan jenis yang sama datang berdekatan, layar perangkat dapat hanya menampilkan yang terbaru. Semuanya tetap tercatat di lonceng.
- **Notifikasi lama dilewati.** Notifikasi yang sudah lebih dari 48 jam tidak dikirim ke perangkat.
- **Tidak bergantung pada email dan WhatsApp.** Notifikasi perangkat berjalan terpisah dari [Preferensi notifikasi](04-preferensi-notifikasi.md).

## Pesan galat yang umum

| Pesan | Penyebab | Cara mengatasi |
|---|---|---|
| Izin notifikasi diblokir. Aktifkan kembali dari pengaturan situs di browser Anda. | Anda menolak permintaan izin, atau browser memblokirnya. | Ikuti "Cara memperbaiki izin yang diblokir". |
| Izin notifikasi diblokir di browser ini. Buka pengaturan situs (ikon gembok di address bar) lalu izinkan notifikasi. | Izin untuk Tauthub masih diblokir. | Ikuti "Cara memperbaiki izin yang diblokir". |
| Gagal memuat status notifikasi. | Status perangkat tidak terbaca. | Muat ulang halaman. |
| Gagal mengubah status notifikasi. | Pendaftaran atau pembatalan gagal. | Klik tombolnya lagi. |
| Gagal mengirim notifikasi uji. | Pengiriman uji gagal. | Coba lagi. Bila terus gagal, nonaktifkan lalu aktifkan kembali perangkat. |

## Pertanyaan yang sering diajukan

**Mengapa tombol aktifkan tidak ada?**
Perangkat Anda menampilkan salah satu keadaan khusus pada tabel di atas: browser tidak mendukung, fitur belum dinyalakan tim Tauthub, atau perangkat Apple yang perlu memasang Tauthub ke Layar Utama.

**Apakah saya perlu membuka Tauthub supaya notifikasi masuk?**
Tidak. Notifikasi muncul di perangkat yang sudah aktif walau tab Tauthub tertutup, selama browser masih berjalan.

**Apakah notifikasi perangkat memakai nomor WhatsApp atau email saya?**
Tidak. Notifikasi perangkat berjalan lewat browser Anda dan diatur terpisah dari kartu **Notifikasi** di atasnya.

## Lihat juga

- [Konfigurasi](README.md)
- [Preferensi notifikasi](04-preferensi-notifikasi.md)
- [Notifikasi](../15-notifikasi.md)
- [Saluran notifikasi](../../umum/04-saluran-notifikasi.md)
