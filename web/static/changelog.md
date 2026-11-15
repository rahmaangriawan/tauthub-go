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

## v0.6.5 — 2026-11-15

### Fitur
- **Perjalanan artikel**: di detail publikasi sekarang ada urutan kejadian tiap artikel beserta tanggal dan jamnya, mulai dari pesanan dibuat, menunggu media, sampai terbit dan cek link. Artikel yang sudah lebih dari 2 hari menunggu media diberi catatan khusus, dan kamu menerima kabar sekali sebelum batas 3 hari lewat.
- **Pakai untuk Artikel**: tombol di Taut Brief menjadikan brief yang sedang kamu buka sebagai brief aktif. Di Pengelola Naskah, brief itu langsung terpilih untuk gaya Yang Proper, jadi tidak perlu memilih ulang di tiap artikel.
- **Cari brief dan pindah brief**: daftar brief bisa dicari, dan halaman Instan Brief punya pilihan brief di bagian atas.
- **Ambil tautan dari brief**: di Pengelola Naskah, tombol Ambil dari brief mengisi baris tautan dari keyword dan URL target yang sudah ada di brief.
- **Ubah isian langsung di tab Entitas**: klik ikon pensil di tiap baris, ubah, lalu simpan, tanpa pindah ke Instan Brief.
- **Peringatan ringkasan belum diperbarui**: kalau kamu mengubah brief setelah ringkasannya dibuat, muncul peringatan dan tombol untuk memperbaruinya, supaya artikel memakai isi brief yang terbaru.
- **Buat artikel dari Target Pertanyaan AI**: tombol di tiap pertanyaan membawa topiknya ke Pengelola Naskah.
- **Cek kata terlarang**: kata atau frasa yang kamu larang di brief sekarang diperiksa langsung di Cek Validasi, dan artikel yang memuatnya diminta direvisi.
- **Generate Semua yang sungguhan**: tulis banyak artikel sekaligus. Dikerjakan di server, jadi halaman boleh ditutup, dan kamu mendapat notifikasi saat selesai. Kredit dipotong per artikel setelah berhasil.
- **Buat semua cover**: cover AI untuk semua artikel yang belum punya, dibuat dari judul artikel.
- **Pakai untuk media lain**: dari satu artikel, Tauthub menulis versi untuk media lain dengan topik dan fakta yang sama tetapi kalimat dan susunan berbeda, supaya tidak dianggap duplikat. Biayanya lebih murah dan tampil di Total Tagihan Akhir.
- **Ringkasan kekurangan di bilah bawah**: halaman Penyedia Konten kini menyebut langsung apa yang kurang (misalnya artikel atau cover), dan satu klik membawamu ke barisnya.
- **Aksi massal**: generate, buat cover, dan hapus untuk artikel yang dicentang.

### Perbaikan
- Tab Entitas sekarang menampilkan nama alternatif, kanal resmi, serta keyword dan URL target yang sudah kamu isi.
- Persentase kelengkapan brief kini disertai daftar isian yang masih kosong, dan formulir Instan Brief menandai kolom yang ikut dihitung untuk 100%.
- Kabar artikel gagal terbit ditulis dengan kalimat biasa, tanpa istilah teknis.
- Pesan saat peranmu belum cukup untuk suatu aksi kini jelas dan berbahasa Indonesia.

## v0.6.4 — 2026-11-12

### Fitur
- **Audit Publikasi**: dashboard kepatuhan artikel yang sudah terbit. Ringkasan sehat dan bermasalah, tren 12 bulan, kondisi tautan, dan media terbanyak. Bisa difilter per periode terbit dan diunduh sebagai PDF atau CSV.
- **Kalender editorial**: tampilan kalender bulanan untuk melihat sebaran tanggal terbit dan jadwal artikelmu. Klik satu tanggal untuk melihat penempatannya beserta tautan artikel yang sudah terbit.

## v0.6.3 — 2026-11-12

### Fitur
- **Level loyalitas**: makin besar total belanjamu di Tauthub, makin tinggi tier kamu (Bronze, Silver, Gold, Platinum) dan makin besar diskonnya. Diskon otomatis terpotong saat kamu membuat kampanye, jadi harga partner tidak berubah dan potongan diambil dari margin kami. Status level, total belanja, dan sisa menuju tier berikutnya bisa dilihat di Profil, tab Akun.
- **Saran media sejenis saat checkout**: ketika kamu menyimpan kampanye, muncul rekomendasi media yang mirip dengan pilihanmu (kategori dan kelas yang sama) dan belum ada di keranjang, supaya lebih mudah menambah jangkauan sekali jalan.
- **Laporan bulanan lewat email**: ringkasan aktivitas brand tiap bulan, berisi artikel yang terbit, total belanja, saldo saat ini, dan media terbaik, dikirim otomatis ke email admin brand di awal bulan. Bisa dimatikan kapan saja di Preferensi notifikasi.

## v0.6.2 — 2026-11-11

### Fitur
- **Ulangi kampanye dalam satu klik**: kampanye yang sudah pernah jalan bisa disalin jadi draf baru dengan media yang sama dan pengaturan yang sama. Media yang sudah tidak bisa dipesan dilewati dan disebutkan. Tidak ada saldo yang terpotong sampai kamu menjalankannya.
- **Pengingat saldo menipis**: kalau saldo brand kamu mulai lebih rendah dari kebiasaan pemesananmu, kamu diberi tahu lewat notifikasi dan push, paling sering sebulan sekali.
- **Analisis Kinerja**: ringkasan hasil tiap kampanye, berisi artikel yang sudah terbit, media yang dipakai, kondisi tautan, dan status garansi. Bisa dilihat per kampanye atau untuk semua kampanye brand.
- **Profil penagihan**: simpan nama perusahaan, NPWP, dan alamat penagihan sekali saja. Top up berfaktur otomatis memakai data itu, termasuk nomor PO kalau kamu punya.
- **Invoice otomatis berPDF**: setelah top up berfaktur lunas, invoice terbit sendiri dan bisa diunduh sebagai PDF di halaman Invoice & Pajak.
- **Peran Finance untuk tim keuangan**: anggota berperan Finance bisa melihat saldo, melakukan top up, dan mengelola invoice, tanpa bisa membuat atau mendistribusikan kampanye.
- **Bonus top up dan kode promo**: paket tertentu memberi bonus saldo, dan kode promo bisa dipakai sekali per brand. Bonus tercatat sebagai baris tersendiri di riwayat saldo.
- **Bukti tayang dan garansi tautan**: Link Guardian mencatat bukti tayang dan memberi garansi tautan secara otomatis. Berita acara publik memuat ringkasannya.
- **Pusat Penjual**: halaman harian untuk mitra berisi tugas yang perlu dikerjakan, pesanan yang menunggu dengan sisa waktu 72 jam, ringkasan penghasilan 30 hari, dan minat klien terhadap media Anda.
- **Data Search Console per media**: hubungkan Google Search Console untuk tiap media, dan data trafik diperbarui setiap minggu.
- **Peringatan Pusat Penjual**: pesanan yang hampir kedaluwarsa dan penurunan trafik dikirim lewat notifikasi dan push. Pesanan yang hampir kedaluwarsa juga dikirim lewat WhatsApp.

### Perbaikan
- **Saldo tidak lagi hangus**: saldo yang belum terpakai tetap ada dan tidak akan dihanguskan, berapa lama pun.
- **Angka saldo dan penghasilan tanpa "Rp"**: di seluruh tampilan klien dan mitra, nominal kini ditulis sebagai angka saja.

## v0.6.1 — 2026-11-09

### Fitur
- **Halaman Dokumentasi untuk klien dan mitra**: panduan lengkap dari mendaftar sampai artikel terbit, ditulis dengan bahasa yang mudah dipahami. Bisa dibuka tanpa login di app.tauthub.com/dokumentasi, lengkap dengan pencarian dan pertanyaan yang sering diajukan. Menunya juga ada di sidebar dashboard klien dan mitra, di bawah grup Sistem.
- **Gambar buatan AI kini boleh memuat satu teks pendek**: cover dan gambar isi bisa memuat satu kata kunci atau judul singkat (maksimal 7 kata), supaya gambar tidak hambar dan tidak penuh tulisan yang salah eja.

## v0.6.0 — 2026-11-05

### Fitur
- **Daftar akun lebih aman lewat kode OTP email**: kode verifikasi dikirim sebelum akun dibuat, email sementara (temp-mail) ditolak, dan kamu mendapat notifikasi setiap ada login baru ke akunmu.
- **Halaman Integrasi mitra didesain ulang** dengan tab WordPress, Web Custom, dan Blogspot yang lebih rapi. Tes koneksi WordPress kini memberi petunjuk perbaikan yang jelas, dan ada catatan bahwa akun WordPress berperan Penulis tidak bisa membuat kategori baru (gunakan peran Editor).
- **Tab Integrasi menyesuaikan platform media Anda**: tab WordPress, EmDash, Web Custom, dan Blogspot yang belum dipakai media mana pun tampil abu-abu dan tidak bisa diklik, jadi mitra langsung tahu kanal mana yang relevan.
- **Ikon mata di Daftar Media jadi saklar**: mitra bisa menyembunyikan atau menampilkan media dari katalog klien. Untuk media yang belum tersambung, klik ikon mata membawa langsung ke halaman Integrasi dengan media itu disorot.
- **Kolom Platform di Daftar Media**: WordPress, Blogspot, atau Custom terdeteksi otomatis dari website media.
- **Form Tambah/Edit media dikelompokkan jadi 5 bagian** agar lebih mudah diisi, dengan pilihan jenis tautan Dofollow, Nofollow, atau Sponsored.
- **Pencarian dan halaman di Daftar Media mitra** kini bekerja untuk semua media, bukan hanya yang tampil di halaman pertama.
- **Matchmaker lebih paham brief**: wajib memilih jenis media, membaca konteks anggaran dari kalimat brief, dan anggaran tanpa jumlah media dianggap paket beberapa media.
- **Redesain halaman klien dan mitra**: Saldo, Publikasi, Kampanye, dan Link Guardian di sisi klien; Penghasilan, Link Guardian, dan Daftar Media di sisi mitra; serta halaman Profil & Akun klien. Judul dan deskripsi halaman kini seragam.
- **WA artikel terbit** kini menyebut nama media ("sudah TERBIT di nama media") dan sapaannya memakai nama penerima.
- **Tab EmDash di halaman Integrasi mitra**: media yang memakai EmDash kini bisa disambungkan dengan token akses pribadi. Artikel dikirim lengkap dengan judul, deskripsi meta, isi, cover, dan gambar. Gambar dipindahkan ke media library situs Anda, bukan sekadar tautan ke Tauthub.
- **Mitra diberi tahu** bila akun WordPress yang terhubung tidak bisa membuat kategori, lengkap dengan cara mengatasinya.
- **Persetujuan media dibatasi 3×24 jam**: bila media tidak merespons, publikasi otomatis dibatalkan dan saldo kembali ke klien.
- **Putaran baru kampanye di Taut Pintar** beserta perbaikan alur kampanye klien.
- **Pustaka gambar**: unggah banyak gambar sekaligus, lalu pilih beberapa untuk disisipkan ke artikel.
- **Email dari Tauthub kini seragam**: satu kerangka dengan logo untuk semua email transaksional.
- **Template WhatsApp dipisah per penerima** (mitra, klien, admin), dan WA kini juga dikirim untuk artikel terbit, pencairan, dan referral.
- **Katalog media lebih lengkap**: jenis media dan root domain/subdomain, penanda verifikasi, dropdown kategori yang didesain ulang, serta trafik kosong ditulis 0 bukan strip.
- **Content Velocity** media kini terisi lebih banyak, termasuk untuk situs yang memblokir pemeriksa kami, lewat jalur cadangan (WordPress API, feed, sitemap, Google News).
- **Redesain kartu Link Guardian dan Mitra**, palet warna kategori, dan sidebar yang bisa dilipat.
- **Panduan Penghasilan mitra**: arahan arsip penarikan, tombol ke halaman invoice, nomor invoice penarikan 6 karakter, dan tarif transfer dijelaskan sebagai keterangan.
- **Dua pintu masuk Brief**: Chat Brief (ngobrol dengan AI) dan Instan Brief (isi template atau unggah dokumen). Form Profil Brand juga bisa disimpan sekaligus tanpa AI.
- **Matchmaker membedakan anggaran total dan anggaran per media**, dan rekomendasi alternatif tidak lagi melewati batas anggaran.
- **Katalog media publik** dapat dilihat di data.tauthub.com tanpa login.
- **Tiket Dukungan untuk mitra**, lengkap dengan badge jumlah tiket yang belum dibaca.
- **Taut Pintar**: satu domain boleh memuat beberapa artikel, semua artikel satu media bisa dibuat dalam satu dialog, konteks kampanye cukup diisi sekali dengan usulan topik, dan artikel bisa dihapus satu per satu. Ringkasan Distribusi kini berupa kartu dengan 10 media per halaman.
- **Tampilan HP diperbarui**: kartu media dan aksi artikel di Taut Pintar, daftar media terpilih di katalog (dengan favicon), serta obrolan yang kini satu kartu dengan laci info media.
- **Masuk langsung ke brand terakhir** yang kamu buka, tanpa halaman pemilih brand.
- **Pengelola Naskah**: tombol Analisis QC sendiri, penghitung link dan gambar di kepala naskah, Generate, Tulis Sendiri, dan QC berjalan per artikel, serta menu klik kanan pada tautan di editor.
- **Beri Rating media** langsung dari halaman klien.
- **Untuk mitra**: pratinjau artikel terjadwal, jadwal terbit di daftar publikasi, penjelasan status Antrean dan Secure Direct, sampul artikel, favicon media di baris penghasilan, serta pengajuan Dewan Pers yang bisa dicabut.
- **Izin subjudul per media**: mitra menentukan apakah artikelnya boleh memakai subjudul.
- **Pendaftaran langsung masuk dashboard**, dengan form dua kolom di layar lebar.

### Perbaikan
- **Notifikasi WhatsApp yang sebelumnya tertahan di antrean** kini terkirim lagi, termasuk notifikasi login.
- **Mode Manual** tidak lagi tersimpan sebagai WordPress.
- **Gagal menambah media kini ada alasannya**, misalnya nomor WA wajib diisi, bukan lagi sekadar "Gagal menyimpan perubahan".
- **Perubahan kecil oleh mitra** (yang tidak sensitif) tidak lagi menyembunyikan media dari katalog.
- **Tampilan Daftar Media mitra** dirapikan: tabel tidak berantakan di layar sedang, label kategori selebar isinya, dan kolom "Menunggu" diganti "Order".
- **Kartu koneksi WordPress** tidak lagi tampil dobel di desktop maupun HP.
- **Tombol AI Matchmaker di katalog** tidak lagi tertutup kolom pencarian di lebar layar sekitar 1050px.
- **Harga klien di pratinjau media** kini mengikuti markup 0%.
- **Kredit ditagih sebelum artikel diproses dan dikembalikan bila gagal**, jadi saldo tidak terpotong untuk proses yang tidak selesai.
- **Media tanpa harga ditolak** di semua pintu, sehingga tidak ada lagi pesanan dengan harga nol.
- **Notifikasi artikel terbit tidak ganda lagi**: hasil verifikasi URL digabung ke satu notifikasi.
- **Penghasilan mitra sebelum disetujui** kini sudah dipotong platform 25%, sama seperti setelah disetujui.
- **Sidebar**: scrollbar menempel di tepi, submenu tidak terpotong, dan label grup tidak lagi terjepit.
- **Hasil tulisan AI lebih tertib**: satu artikel tidak lagi membahas dua brand, isi artikel tidak dibuka dengan judul, struktur mengikuti angle yang dipilih, dan tautan klien dipastikan benar-benar tertanam di paragraf yang relevan.
- **Pencarian media** tidak lagi mencocokkan nama pemilik, dan media tanpa data trafik menampilkan strip.
- **Pengingat saldo** hanya dikirim ke akun klien, bukan mitra.
- **Dokumen yang sama bisa diunggah ulang** tanpa dianggap gagal dibaca.
- **Celah akses data** pada hasil profiler dan unduhan file sudah ditutup.

### Performa
- **Verdict QC lebih cepat**: hasilnya tampil lebih dulu, sementara penulisan ulang menyusul di latar belakang.

## v0.5.7 — 2026-10-25

### Fitur
- **Tab Timeline di Taut Brief**: tempat mencatat kabar terbaru brand — promo, kunjungan, peluncuran, atau momen lain — lengkap dengan gambar, tautan, dan tagar. Kabar yang kamu tulis di sini ikut dibaca AI Writer, jadi artikel yang dihasilkan terasa baru dan tidak sama dengan kompetitor.
- **Jendela "Utas baru"** untuk menulis kabar: tinggal klik kolom "Apa yang baru?", lalu tulis, lampirkan gambar, atau tempel tautan. Kursor langsung siap menulis saat jendela terbuka.
- **Pratinjau tautan otomatis**: tempel alamat situs dan muncul kartu berisi gambar, judul, serta ringkasan halaman tersebut — tanpa perlu mengunggah gambar sendiri.
- **Gambar bisa ditempel atau diseret** langsung ke jendela tulis. Pratinjaunya langsung tampil, dan proses unggah berjalan di belakang layar dengan penanda "Mengunggah…" lalu "Siap dikirim", jadi kamu bisa lanjut menulis tanpa menunggu.
- **Tagar otomatis berwarna**: cukup ketik #promo atau #peluncuran di dalam tulisan; setelah terbit, tagar tampil berwarna biru.
- **Draf**: kalau belum selesai, tutup jendela tulis dan pilih "Simpan ke draf". Draf bisa dibuka lagi kapan saja lewat ikon dokumen di kanan atas, lalu dilanjutkan atau dihapus. Kalau menutup jendela saat tulisan sudah ada isinya, sistem menawarkan simpan ke draf lebih dulu agar tulisan tidak hilang.
- **Tampilan Timeline seperti beranda media sosial**: kabar terbaru tampil berurutan dalam satu kartu, dengan menu titik tiga untuk menyalin tautan atau menghapus postingan. Postingan otomatis kedaluwarsa setelah 3 bulan.
- **Tab terakhir yang kamu buka diingat**: setelah halaman dimuat ulang, kamu tetap berada di tab yang sama — Copilot Chat, Entitas, Media, Timeline, atau Ringkasan — dan tiap brief menyimpan tab-nya sendiri.

## v0.5.6 — 2026-10-20

### Fitur
- **Obrolan langsung dengan mitra per artikel**: tombol "Hubungi" di Ringkasan Distribusi membuka ruang obrolan khusus untuk artikel itu — satu halaman tiga kolom berisi daftar percakapan, ruang pesan, dan info media/artikel. Sesi berlaku 3×24 jam sejak pesan pertama; setelah berakhir riwayat tetap bisa dibaca. Sebelum masuk, ada konfirmasi ketentuan ruang obrolan dengan ceklis persetujuan.
- **Menu "Pesan" di portal klien dan mitra**, lengkap dengan pencarian percakapan, tab Semua/Aktif/Selesai, penanda pesan belum dibaca, dan badge merah berisi jumlah pesan baru di sidebar.
- **Badge "Publikasi" untuk mitra**: begitu ada publikasi baru untuk medianya, menu Publikasi menampilkan angka merah; membuka halaman Publikasi menandai semuanya sudah dibaca dan angkanya hilang.
- **Kartu "Pesan" di dashboard mitra** menampilkan jumlah chat klien yang belum dibaca dan langsung menuju ruang obrolan.
- **Penyaring anti-bypass di obrolan**: nomor telepon, email, dan ajakan transaksi di luar platform disensor otomatis jadi `[KONTAK DISEMBUNYIKAN OLEH SISTEM]` — termasuk yang disamarkan seperti "0812,3456,7890", "w4ts4pp", "w h a t s a p p", atau "nama [at] domain [dot] com". Link artikel/draft (termasuk domain tanpa https) tetap utuh. Link messenger (wa.me, t.me) ikut disensor.
- **Jejak audit akses admin**: pesan asli hanya bisa dilihat super admin, dan setiap admin membuka log obrolan tercatat di `admin_chat_audit_logs` (IP, user agent, waktu) dengan penggabungan 15 menit agar log tidak menumpuk.
- **Admin punya menu "Log Obrolan"** untuk meninjau riwayat percakapan klien ↔ mitra (hanya baca, disertai jejak audit). Admin tidak lagi menerima notifikasi chat; notifikasi hanya dikirim ke peserta percakapan.

### Perbaikan
- **Penghasilan mitra**: potongan platform kini **25%** — mitra menerima 75% dari harga pengajuan, sesuai aturan yang juga tertulis di halaman Media. Sebelumnya perhitungan memakai 5%. Seluruh publikasi lama ikut dikoreksi, bukan hanya earning baru.
- **Kartu media di Dapur Produksi** hanya melaporkan jenis tautan yang benar-benar ada: artikel dengan 1 link dofollow tidak lagi menampilkan "0 Nofollow"; artikel tanpa link tetap ditandai "0 Tautan".
- **Popup Cek Validasi** dirapikan menjadi kartu indikator dua kolom dengan chip status berwarna.
- **Peringatan queue gagal tidak lagi membanjiri lonceng admin**: kegagalan job kini tampil sebagai satu ringkasan di halaman Operations beserta tab **Queue gagal** yang mengelompokkan kegagalan per jenis job (jumlah + error terakhir 24 jam).
- **Admin tidak lagi menerima notifikasi aktivitas brand** (artikel terbit, publikasi disetujui, perubahan skor PR, dan sejenisnya) — notifikasi itu kembali khusus untuk anggota brand/klien. Admin tetap bisa memantau lewat menu Log Obrolan dan halaman admin terkait.

### Tampilan
- Tabel Media Terpilih di Ringkasan Distribusi dirapikan: header bersih, baris lebih lega, logo media, serta tombol Lihat dan Hubungi berbingkai.
- **Pengelola Naskah**: lebih lebar, header menampilkan domain media (huruf depan kapital) dengan favicon, dan hanya tab yang relevan yang muncul sesuai pintu masuk — dari Generate Konten tampil AI + Editor, dari Unggah Artikel tampil Unggah + Editor.
- Tombol aksi kartu media kini berupa ikon dengan tooltip di atas saat hover, bukan melebar.

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
- **Katalog media klien kembali lengkap**: ada cacat teknis yang membuat media yang sudah punya data trafik SimilarWeb terlempar diam-diam dari katalog dan dari Estimasi Media — makin banyak data trafiknya, makin kecil daftar yang terlihat. Sekarang semua media aktif tampil apa adanya (97 media aktif di admin = 97 di katalog klien).
- Admin: "Antrekan Cek Trafik Semua" kini membuat antrean latar belakang, bukan memanggil penyedia langsung dalam satu klik — dulu selalu berakhir "Server terlalu lama merespons" karena puluhan media diperiksa berurutan dalam satu request. Scheduler yang menguras antreannya otomatis, dan media aktif yang belum punya data trafik otomatis masuk antrean tanpa perlu diklik. Media yang datanya sudah ada dibiarkan tersimpan dan hanya diperiksa ulang setelah 6 bulan (sebelumnya 2 bulan). Indikator antrean (menunggu/berjalan/gagal + sebab) tampil di toolbar media.

### Performa
- **Gambar hasil AI otomatis dikompres saat disimpan**: dikonversi ke WebP dengan batas maksimal 150 KB per gambar. Sebelumnya satu gambar bisa 1–1,5 MB sehingga satu artikel beratnya bisa mencapai 3,4 MB; sekarang artikel bisa lebih dari sepuluh kali lipat lebih ringan, dan gambar tetap tajam di ukuran tampilnya.

### Admin
- Menu "Prompt Studio" diganti nama menjadi **Prompt Angle** dan alamatnya menjadi `/admin/prompt-angle` — isi fungsinya sama, sekadar penamaan; tautan lama otomatis dianggap hilang, jadi kalau ada yang menyimpan bookmark lama cukup buka menu baru.

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
