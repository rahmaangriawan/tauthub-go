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

## v0.5.5 — 2026-10-16

### Fitur
- **Riwayat Saldo kini per kampanye**: setiap kampanye tampil dengan total terpakai dan jumlah transaksinya, klik untuk membuka rinciannya — mana yang biaya tulis artikel, mana yang penagihan publikasi. Top up dan penyesuaian saldo dikumpulkan di grup "Lainnya". Warna konsisten: merah dengan tanda − untuk saldo berkurang, hijau dengan tanda + untuk saldo bertambah, dan abu untuk dana yang ditahan (belum terpakai).
- **Ganti Cover punya dua pilihan**: unggah gambar sendiri (gratis), atau **Generate Gambar AI** — berbayar, dan sebelum diproses selalu ditampilkan biaya per gambar plus saldo kamu, dengan konfirmasi sekali klik. Kalau saldo kurang, tombolnya mati dan ada anjuran top up.
- **Tagihan top up yang belum dibayar kini terlihat dan dihitung**: banner menampilkan jumlah tagihan yang menunggu beserta totalnya dan tombol "Lanjutkan pembayaran", dan tagihan itu ikut tampil di riwayat. Nilai tagihan belum dihitung ke Saldo tersedia sampai pembayarannya benar-benar selesai.
- **Masa berlaku sesi top up 24 jam**: tagihan yang tidak dibayar dalam 24 jam otomatis hilang dari notifikasi dan riwayat — tidak menumpuk lagi.
- **Halaman Ringkasan distribusi dirapikan**: tabel media kini memakai logo asli situs media plus domainnya, tipe, DA, jumlah tautan, jadwal terbit, dan status tuntas-bayar sekali lirik; dua kartu ringkas (Biaya Distribusi Media dan Status Kampanye) — kartu status berubah kuning saat proses dan hijau begitu semua artikel terbit; tombol "Kembali ke The Writer" pindah rapi ke dalam banner.

### Perbaikan
- **Permintaan jumlah kata di kolom topik/konteks kini benar-benar dipatuhi**. Tulis "800 kata" dan hasilnya akan di sekitar 720–880 kata; tulis "minimal 2.500 kata" dan artikel tidak akan berhenti di bawah itu. Kalau draf pertama meleset dari rentang, AI otomatis menulis ulang sekali dengan koreksi panjang. Sebelumnya minta 800 kata bisa keluar 1.000 kata, dan minta 2.500 kata bisa berhenti di 1.873.
- **Tautan utama artikel tidak lagi bisa tertanam dua kali**: sistem menyisakan satu tautan dengan anchor persis seperti yang kamu isi, walau model sempat menulisnya berulang. Link yang sudah ada juga lebih jarang salah dianggap hilang (aturan href dirapikan), termasuk saat URL punya tanda `&` atau garis miring di akhir.
- **Gambar cover di editor artikel kembali termuat dengan benar** (berlaku juga untuk thumbnail cover di Status Publikasi dan Link Guardian), dan cover di editor kini rasio **16:9** persis seperti tampilannya saat terbit.
- **Ukuran gambar ditulis lebih ramah**: "3,4MB", bukan lagi "3422KB".
- **Favicon media di dashboard** kini logo asli tiap situs, dimuat lebih ringan (satu ukuran untuk semua tempat plus koneksi awal ke sumber gambar, jadi muncul lebih cepat).
- Admin: pemeriksaan SimilarWeb butuh DUA kredensial dari provider (publisher DataBee di RapidAPI) — key RapidAPI dan DataBee Token. Keduanya kini diatur di kartu SimilarWeb panel Konfigurasi dan dikirim benar (X-RapidAPI-Key + Authorization Bearer), dengan validasi yang menyebut mana yang belum diisi. Tombol "Tes koneksi provider" menampilkan jawaban provider apa adanya, jadi ketahuan tepat: kunci salah, token kurang, langganan belum aktif, atau kuota habis.

### Performa
- **Gambar hasil AI otomatis dikompres saat disimpan**: dikonversi ke WebP dengan batas maksimal 150 KB per gambar. Sebelumnya satu gambar bisa 1–1,5 MB sehingga satu artikel beratnya bisa mencapai 3,4 MB; sekarang artikel bisa lebih dari sepuluh kali lipat lebih ringan, dan gambar tetap tajam di ukuran tampilnya.

## v0.5.4 — 2026-10-15

### Perbaikan
- Nilai DA dan AI Trust kini tampil apa adanya di halaman Ringkasan, sama dengan yang terlihat di katalog media. Sebelumnya ada kesalahan pembacaan tipe data di server yang membuat media ber-DA nol atau AI Trust kosong padahal datanya tersimpan, termasuk di panel detail media. Sekalian memperbaiki kolom serupa lain, seperti PA dan Spam Score, di seluruh halaman.
- Kolom "DA / SS" di tabel Ringkasan disederhanakan jadi "DA": indikator SS tidak dipakai lagi.
- Tautan pada artikel kini dilabeli dengan status rel yang berlaku: 1 link dofollow bila tanpa atribut rel, bukan lagi kalimat "tanpa rel" yang membingungkan karena dofollow memang berarti tanpa rel.

## v0.5.3 — 2026-10-15

### Fitur
- Di kolom Topik & Konteks kini ada tombol Tips di samping tombol Contoh, yang membuka panel cara menulis topik yang baik: tulis detail produk dan datanya, sebutkan angka, harga, tanggal, dan narasumber dari sumber resmi, serta tentukan target pembaca, panjang artikel, dan jumlah gambar. Intinya: artikel yang kaya data lahir dari topik yang kaya data, dan AI tidak mengarang fakta di luar bahan.
- Tombol Tips ini tampil untuk semua angle di media mainstream, dan juga di media blogger yang memilih angle Review Produk, Cerita Pengalaman/Studi Kasus, Unboxing, atau Liputan Acara/Event Komunitas. Media blogger dengan angle lain tetap tanpa tombolnya.
- Panel tips muncul sebagai popover melayang di bawah tombolnya tanpa mendorong isi jendela, dan otomatis membuka ke atas ketika tombolnya berada terlalu dekat dasar layar supaya tidak terpotong.

## v0.5.2 — 2026-10-15

### Fitur
- Menulis permintaan rel di luar cakupan media (misalnya meminta rel="sponsored" di media biasa) kini dijawab jujur di tempat: muncul pemberitahuan di bawah kolom topik bahwa rel itu tidak diperbolehkan dan tautan akan ditulis dofollow. Sebelumnya permintaan seperti itu diam-diam diabaikan oleh AI saat menulis.
- Rubrik kini terisi otomatis dari daftar rubrik media begitu draf AI selesai dibuat, lengkap keterangan kecil bahwa itu pilihan otomatis; tinggal diganti bila dirasa kurang pas. Dulu kolom rubrik wajib diisi manual dan sering jadi penunda terakhir sebelum menyimpan.
- Saat AI sedang menyusun draf, tombol tutup dan Batal tidak lagi menutup jendela pengelola naskah; diklik paksa pun yang muncul hanya peringatan untuk menunggu proses selesai. Dulu menutup di tengah proses membuat kesan penulisan gagal, padahal artikel tetap tersimpan dan biayanya tetap tercatat.
- Angka TOTAL TAGIHAN AKHIR kini berubah begitu ada aksi yang mengubah biaya (buat draf AI, tulis ulang, buat gambar, lepas media) tanpa perlu memuat ulang halaman. Perubahan nominalnya tampil sebagai chip kecil mengapung: + hijau berarti biaya naik, - merah berarti turun.
- Halaman Penyedia Konten kini dibuka dengan judul "Isi Dapur Produksi", dan setiap kartu media menandai jenisnya: Mainstream atau Blogger, jadi cakupan media terbaca sekali lirik.
- Di Taut Brief, kirimkan saja tautan di ruang obrolan: AI membacanya sejak masa berpikir dan memakai isinya sebagai konteks jawaban. Dulu tautan hanya diperlakukan teks biasa.

### Perbaikan
- Biaya AI di bar tagihan kini hanya menghitung artikel dan gambar yang benar-benar dibuat AI, dengan tarif berlaku (tiap generate 1x tarif). Sebelumnya media yang belum mengerjakan apa pun tetap menampilkan "Biaya penulisan AI", dan gambar milik artikel unggahan sendiri ikut dihitung, padahal tidak ada AI yang dipakai.
- Setiap generate ulang kini terhitung di rincian biaya betul sesuai penagihannya (muncul sebagai "tulis ulang"). Dulu tulis ulang yang berulang-ulang tidak terlihat sama sekali di bar, padahal kredennya terpakai.
- Membuat ulang artikel kini ikut memperbarui Cuplikan Singkat (meta deskripsi) mengikuti artikel baru. Dulu ringkasan artikel lama tetap tertinggal di kolomnya, dan begitu disimpan malah menimpa meta deskripsi artikel yang baru.
- Menghapus media dari kampanye kini bersih: media yang pernah dilepas sebelumnya tidak lagi muncul kembali setelah penghapusan berikutnya, dan kartunya hilang seketika begitu dikonfirmasi, tanpa menunggu data dimuat ulang.

## v0.5.1 — 2026-10-10

### Fitur
- Balik dari Penyedia Konten ke Katalog Media kini membawa pilihanmu: media yang sudah kamu pilih tetap tercentang, jadi kamu cukup menambah media baru lalu klik "Tambahkan ke Kampanye" untuk kembali ke Penyedia Konten. Sebelumnya pilihan itu hilang dan kamu harus memilih ulang, dan sistem malah membuat kampanye baru.
- Artikel bergaya Editorial kini ditulis sebagai berita peristiwa: dibingkai sebagai peluncuran resmi, pembaruan sistem, atau perluasan ekosistem (bukan lagi teks profil "X adalah…"), lengkap dengan narasumber bernama dan jabatan C-level serta data efisiensi yang wajar — seolah siap terbit di media.
- Artikel Editorial kini selalu punya penutup yang konklusif — memadukan refleksi nilai dan ajakan halus — tidak berhenti menggantung di tengah pembahasan, dan tidak memakai frasa klise seperti "Kesimpulannya" atau "Akhir kata". Kalau harga/paket belum disebutkan, artikel menjelaskannya secara wajar tanpa teks cadangan.
- Artikel bergaya Opini kini menutup dengan identitas penulis yang disusun AI secara wajar saat kamu belum mengisi nama penulis — nama personal dan kepakaran Indonesia yang kredibel dan selaras topik, tanpa penanda [Nama Penulis] atau teks cadangan.
- Artikel bergaya Editorial kini benar-benar bebas tautan (standar redaksi berita): kolom Tautan & Anchor disembunyikan dan diganti pemberitahuan — kalau ingin memakai tautan, pilih jenis artikel Advertorial. Backlink kampanye juga tidak ditanam di naskah Editorial.
- Naskah Editorial kini mengikuti standar kurasi redaksi nasional tier-1: lead langsung ke peristiwa dengan dateline, kutipan narasumber yang bervariasi dan berbobot, tanpa gaya panduan beli, lalu ditutup blok "Tentang Perusahaan" dan "Narahubung Media" — semuanya tanpa teks cadangan.

### Perbaikan
- Jumlah kata yang kamu tulis eksplisit di kolom topik (mis. "minimal 2500 kata") kini benar-benar dipatuhi AI. Sebelumnya AI mengikuti batas panjang bawaan media dan sering berhenti jauh di bawah permintaan — misalnya diminta 2500 kata tapi hasilnya hanya sekitar 1.800 kata.
- Kampanye yang dibuat dari Katalog Media tidak lagi dipaksa mode manual — tombol "Generate Konten" (artikel dengan AI) kini muncul seperti seharusnya. Sebelumnya media hasil pilihannya hanya bisa diunggah, tidak bisa dibuat AI.
- Tombol "Generate Konten" kini juga tampil di kampanye mode manual, termasuk kampanye lama yang sudah telanjur ber-mode manual — sebelumnya di mode itu hanya ada "Unggah Artikel", padahal artikel dengan AI tetap bisa dibuat dari dalam jendela pengelola naskah.
- Kolom topik yang menulis "tanpa gambar", "jangan pakai gambar", atau "tidak perlu ilustrasi" tidak lagi salah dianggap permintaan gambar, jadi tidak muncul estimasi biaya gambar dan tidak ada tagihan gambar yang tidak diminta.
- Artikel bergaya Editorial tidak lagi memuat teks cadangan seperti [Placeholder: ...] di badan berita. Bila bahan menyediakan data resmi perusahaan, artikel boleh ditutup boilerplate singkat dan kontak media resmi yang bersih; kalau tidak ada, bagian itu dilewati tanpa penanda.
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
