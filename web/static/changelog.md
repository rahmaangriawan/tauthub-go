<!--
Changelog Taut Pintar — sumber kebenaran entri "Apa yang Baru".

Cara pakai: tambahkan blok baru di PALING ATAS (terbaru dulu), lalu buka
/admin/changelog dan klik "Impor entri baru". Entri baru otomatis dikirim ke
lonceng notifikasi + web push klien & mitra.

Format yang dibaca parser:
  ## v<versi> — <YYYY-MM-DD>   pembuka satu versi
  ### <Kategori>               fitur | perbaikan | performa | lainnya
  - satu poin perubahan        (boleh beberapa baris)

Catatan:
- Satu versi boleh punya beberapa kategori; setiap kombinasi versi + kategori
  menjadi SATU entri, dengan kunci impor "versi|kategori" (impor ulang tidak
  menggandakan entri, tapi menimpa isinya dengan yang ada di file ini).
- Baris di luar pola di atas (termasuk komentar ini) diabaikan.
- Jangan mengubah entri lama yang sudah terkirim kecuali perlu koreksi — entri
  yang sudah ada ikut diperbarui saat impor, tapi tidak memicu notifikasi ulang.
-->

## v0.5.1 — 2026-10-10

### Fitur
- Balik dari Penyedia Konten ke Katalog Media kini membawa pilihanmu: media yang sudah kamu pilih tetap tercentang, jadi kamu cukup menambah media baru lalu klik "Tambahkan ke Kampanye" untuk kembali ke Penyedia Konten. Sebelumnya pilihan itu hilang dan kamu harus memilih ulang, dan sistem malah membuat kampanye baru.

### Perbaikan
- Di jendela pengelola naskah kini ada penghitung jumlah kata di sisi kanan label "Isi Artikel (Konten)". Panjang naskah bisa langsung dipantau tanpa menghitung manual, dan angkanya ikut berubah saat naskah diedit.

## v0.5.0 — 2026-10-09

### Fitur
- Profiler: halaman baru untuk menyimpan "buku panduan brand" Anda — nama dan bidang usaha, gaya bahasa, kata yang dilarang, pedoman konten, instruksi khusus, sampai dokumen, gambar, dan tautan referensi. Isi sekali, lalu tiap artikel AI bisa memakai acuan itu. Satu akun boleh punya banyak Profiler, dan bisa diganti nama, disalin, atau dihapus.
- Pilih jenis artikel saat membuat naskah dengan AI: Editorial, Advertorial, atau Opini. Masing-masing punya gaya penulisan sendiri. Tersedia untuk media mainstream; untuk media blog gayanya sudah menyesuaikan otomatis.
- Minta gambar lewat kolom topik, AI membuat gambarnya dan memasukkannya ke dalam artikel. Perkiraan jumlah gambar dan biayanya ditampilkan lebih dulu sebelum diproses, jadi saldo tidak terpakai tanpa Anda sadari. Gambar utama tetap Anda atur sendiri.
- Unggah dokumen Word atau PDF kini ikut membawa gambarnya, bukan hanya teksnya. Gambar dari dokumen otomatis disimpan sehingga ikut terbit di media tujuan.
- Profiler bisa dipilih saat menyimpan kampanye dari katalog media, dan pilihannya tersimpan rapi bersama kampanye tersebut.

### Perbaikan
- Di jendela pengelola naskah kini terlihat jenis media tujuannya: Media Mainstream atau Media Blogger.
- Katalog media lebih nyaman dibuka dari HP: kolom yang kurang penting disembunyikan otomatis, jadi yang tampil tinggal nama media, harga, dan tombolnya.
- Nama kategori media kini berwarna berbeda-beda supaya lebih cepat dikenali.
- Ada animasi halus sebagai tanda pilihan tercatat: saat mencentang media, saat menandai media simpanan, dan saat menambah media ke kampanye. Semua animasi otomatis mati bila perangkat Anda diatur mengurangi gerakan.
- Di pengaturan Agent, tab Blogger dan Media Mainstream kini terpisah rapi — masing-masing punya kolom prompt sendiri sehingga tidak saling tertukar.
- Toolbar katalog media diringkas jadi satu tombol corong di samping kolom pencarian. Diklik, muncul panel dari kanan berisi kategori, filter lanjutan, impor daftar, dan media simpanan — jadi toolbar tidak lagi berjejal.
- Header kolom tabel tetap menempel di atas saat daftar digulir, termasuk dari HP.
- Daftar kategori menyesuaikan posisi otomatis: tampil ke atas bila barisnya ada di dekat dasar layar, jadi tidak terpotong.
- Saat semua pilihan media dibatalkan, penanda tahapan hilang dengan animasi halus (bukan hilang mendadak).
- Ikon saldo kredit dibuat hijau supaya lebih mudah dikenali.
- Toolbar katalog jadi lebih rapi: kategori kini berupa pilihan tombol (yang tampil 5 dulu, sisanya lewat "Lihat semua"), filter lanjutan langsung terlihat tanpa perlu diklik, dan tombol Impor List serta Bookmark berdampingan.
- Tampilan katalog di HP jauh lebih ringkas: tiap media hanya menampilkan nama, harga kecil di bawahnya, dan tombol Detail. Informasi lain (kategori, trafik, DA, AI Trust, AI Cited, waktu rilis, dan tombol simpan) muncul di panel kanan saat tombol Detail diklik.
- Badge Dewan Pers dan tanda verifikasi kini berdekatan dengan nama media, dan kolom pencarian tetap sebaris dengan tombol alat katalog.

## v0.4.0 — 2026-10-07

### Fitur
- Katalog media: kolom DA (Domain Authority) dan AI Trust kini menampilkan nilai hasil pemeriksaan yang sebenarnya, dan kolom AI Cited menandai media yang situsnya disebut sebagai salah satu sumber jawaban AI Google.
- Katalog media: tautan Top Up pada bar bawah langsung membuka jendela isi saldo di halaman itu, tanpa berpindah halaman, dan saldo di bar ikut diperbarui begitu jendelanya ditutup.
- Admin: pemeriksaan nilai media bisa dijalankan untuk semua media sekaligus di latar belakang ("Cek DA semua", "Cek AI & SEO Semua"), media yang sudah punya skor otomatis dilewati, dan tersedia tombol "Batalkan antrean" bila ingin dihentikan di tengah jalan.
- Admin: media yang baru diajukan mitra langsung diperiksa saat itu juga, jadi nilai DA dan AI Trust-nya sudah tersedia saat admin meninjau pengajuan.

### Perbaikan
- Skor AI Trust tidak lagi memakai angka cadangan: nilainya hanya diisi bila kecepatan situs benar-benar terukur, dan dibiarkan kosong bila pengukuran tidak memungkinkan. Sebelumnya puluhan media menampilkan angka yang seolah hasil pemeriksaan padahal bukan.
- Kolom DA dan AI Trust di halaman admin tidak lagi tampil kosong padahal datanya tersimpan.
- Status AI Cited tidak lagi langsung dianggap "tidak dikutip" ketika data jawaban AI tidak tersedia; statusnya ditulis belum bisa dinilai supaya tidak menyesatkan.
- Situs mitra yang menolak akses dari server kami, misalnya karena aturan firewall atau sertifikat TLS bermasalah, tidak lagi dilaporkan sebagai situs tidak terjangkau, dan artikel yang tayang di sana tidak ikut ditandai perlu ditinjau.
- Pemeriksaan otomatis tidak lagi menumpuk: pekerjaan yang tertinggal karena server dimulai ulang dibersihkan sendiri, dan percobaan yang gagal tidak lagi diulang tanpa batas.
- Tombol pemeriksaan per media tidak lagi berakhir dengan pesan "Server terlalu lama merespons".
- Header kolom tabel media di katalog tetap terlihat saat daftar media digulir.

## v0.3.0 — 2026-10-07

### Fitur
- Katalog media: bar seleksi kini menampilkan saldo kredit, lengkap dengan peringatan "kurang kredit" dan tautan Top Up.
- Katalog media: pilihan media tersimpan antar sesi — tidak hilang lagi saat halaman di-refresh atau berpindah halaman.
- Katalog media: modal konfirmasi menyebut total biaya, saldo, dan langkah setelahnya sebelum kampanye dibuat.
- Katalog media: peringatan bila media yang dipilih sudah dipesan di kampanye aktif lain.
- Katalog media: pencarian pilihan media dari CSV hasil "Ekspor Proposal" tetap bisa dilanjutkan (nama kampanye opsional, dibuat otomatis bila dikosongkan).
- Taut Pintar: kampanye yang sudah selesai (terkirim/terjadwal/terdistribusi) kini mode lihat saja — Matchmaker, Writer, Kilat, dan Ringkasan Profiler tetap bisa dibaca, tapi tidak bisa diubah.
- The Writer: tombol "Generate Konten" membuka Pengelola Naskah di tab "Generate dengan AI", jadi topik, cover, dan kategori diisi dulu sebelum artikel ditulis.
- Halaman "Apa yang Baru" ini sendiri: catatan rilis bisa dibaca dari dashboard klien & mitra.

### Perbaikan
- Advance kampanye yang sudah melewati tahapnya tidak lagi berakhir dengan error 422 — aplikasi mengarahkan ke Ringkasan.
- Kolom pengaju di halaman Pending admin media tidak lagi menampilkan "—": nama mitra diambil langsung dari data media, dan bisa diklik untuk memeriksa mitranya.
- Request yang melewati batas 12 detik kini memberi pesan jelas ("Server terlalu lama merespons"), dan error AbortError yang mengotori konsol tidak lagi muncul.
- Setelah rilis baru, aplikasi mengenali versi terbaru dan memuat ulang sendiri, sehingga navigasi tidak lagi mentok "Failed to fetch dynamically imported module".

### Performa
- Daftar media dan daftar mitra memakai cache singkat (stale-while-revalidate): halaman terasa instan saat dibuka ulang, dan cache dibersihkan otomatis setiap kali ada aksi simpan.
