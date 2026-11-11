# Catatan penyusun dokumentasi

> Catatan internal untuk tim Tauthub yang akan memindahkan dokumentasi ini ke halaman Elementor. Berkas ini tidak untuk dipublikasikan. Isinya konvensi penulisan, cakupan, dan daftar temuan dari pembacaan kode selama menulis.

Disusun: 8 November 2026. Sumber kebenaran: kode di repo (frontend SvelteKit dan backend Go) pada tanggal itu, bukan catatan rilis. Entri catatan rilis terbaru saat disusun adalah v0.6.0 (5 November 2026).

## Konvensi untuk membangun halaman

- **Satu berkas = satu halaman.** Folder = satu bagian. Berkas `README.md` di dalam folder = halaman induk bagian itu.
- **Judul halaman** = baris `#` di baris pertama. **Deskripsi halaman (meta/ringkasan)** = kalimat dalam kotak kutipan (`>`) tepat setelah judul.
- **Baris Lokasi dan Untuk** (tebal, di bawah ringkasan) cocok dijadikan keterangan kecil di bawah judul.
- **Daftar "Isi bagian ini"** di `README.md` folder = menu anak. **"Lihat juga"** = tautan terkait di akhir halaman.
- **Bagian "Pertanyaan yang sering diajukan"** berisi pasangan **Pertanyaan?** lalu jawaban. Cocok dijadikan akordeon.
- **Kotak** `> **Catatan:**`, `> **Penting:**`, dan `> **Tips:**` cocok dijadikan kotak peringatan atau informasi.
- **Tautan antar-halaman** memakai jalur relatif ke berkas `.md`. Ganti dengan alamat halaman Elementor saat membangun. Semua tautan relatif sudah diperiksa valid pada tanggal penyusunan.
- **Urutan menu** mengikuti awalan angka pada nama berkas dan folder. Awalan `00` (Mulai cepat) tampil pertama.
- Tabel dibatasi lima kolom. Tidak ada gambar, HTML, atau front matter. Tangkapan layar belum ada. Halaman yang paling layak diberi gambar: Beranda Klien, Katalog Media, Penyedia Konten, Publikasi Mitra, dan Integrasi.

## Keputusan penulisan

- **Angka dari konfigurasi tidak ditulis** (harga paket Top Up, kurs, tarif per fitur, biaya transfer, batas minimum penarikan). Dokumen menunjukkan di mana angka itu tampil di layar.
- **Aturan tetap ditulis** dengan angkanya, misalnya batas persetujuan Mitra 3×24 jam, sesi obrolan 3×24 jam, masa tunggu penghasilan 2 hari, dan potongan platform "saat ini 25%". Bila aturan ini berubah di kode, dokumen perlu diperbarui.
- **Pesan di layar dikutip persis**, termasuk yang berbahasa Inggris atau berisi istilah teknis, supaya pembaca bisa mencocokkannya dengan yang mereka lihat.
- **Fitur yang tidak punya jalur dari menu, tombol, atau tautan tidak ditulis.** Fitur "segera hadir" ditulis singkat.
- **Beberapa halaman menjelaskan keanehan tampilan apa adanya** (lihat bagian B di bawah). Bila keanehan itu diperbaiki di aplikasi, sederhanakan halaman terkait.

## Sengaja tidak didokumentasikan

| Yang dilewati | Alasan |
|---|---|
| Halaman klien Member Brand, Usage, Knowledge Base, Overview Distribusi, dan "Buat Konten Baru" (alur intake lama) | Ada di kode tetapi tidak ada tautan dari menu atau tombol mana pun |
| Pilihan **Manual** dan **Kampanye Kilat** di dialog Kampanye Baru | Dialog itu sedang dialihkan ke Katalog Media. Kampanye Kilat yang sudah ada tetap dijelaskan |
| Chip "Generasi AI…" dan "Dalam Antrean…", serta Generate Semua di Penyedia Konten | Kode tidak terpakai |
| Jenis artikel Editorial, Advertorial, dan Opini | Hanya ada di backend, tidak ada pilihannya di layar |
| Ekspor info media berformat PDF untuk Mitra | Hanya tersedia di sisi Klien |
| Kode mati verifikasi domain dan Dewan Pers di Daftar Media Mitra | Tidak terpakai |
| Layar admin (tinjauan media, penangguhan, pengaturan harga dan tarif) | Di luar cakupan. Dokumen hanya menjelaskan dampaknya bagi Klien dan Mitra |
| **Analisis Kinerja** dan **Audit Publikasi** | Belum dirilis. Ditulis singkat di `klien/07-analisis-dan-audit.md` |

## Temuan untuk pemilik produk

Temuan ini muncul saat penulis membaca kode. Lokasi kode disertakan. Belum ada yang diubah di kode.

### A. Kemungkinan celah atau kesalahan uang

1. **Mitra bisa mengaktifkan sendiri media yang masih menunggu review.** Handler update media menerima `status: "active"` dari Mitra. Hanya status `rejected` yang diblokir sebelum itu, dan `requiresReview` hanya berlaku bila media sudah aktif. Akibatnya media `pending_review` dapat langsung menjadi `active` lewat permintaan API tanpa tinjauan admin (`api/internal/domains/partner/media_detail.go:138,173-174,325-341,371-374`).
2. **Secure Direct pada media bermode Manual** langsung menandai artikel terbit tanpa pengiriman apa pun (`api/internal/domains/brandflow/job_publication_execution.go:249`, `campaign_core.go:483`). Dokumen menyarankan Secure Direct hanya untuk media API.
3. **Penagihan penulisan AI** memakai kunci tanpa nomor artikel, sehingga artikel kedua dan seterusnya di media yang sama mungkin tidak ditagih. Tagihan pertama baru dibuat setelah artikel tersimpan, dan tulis ulang tidak mengembalikan saldo otomatis (`brandflow/taut_writer.go:506-507,1122-1123`).
4. **Gambar placeholder tetap ditagih**, padahal komentar kode menyatakan tidak (`brandflow/article_images.go:341-357`, `ai/image.go:7`).
5. **Terbitkan Kampanye membuat publikasi sebelum reservasi saldo.** Bila saldo kurang (HTTP 402), reservasi dilepas tetapi publikasi berstatus `scheduled` tertinggal (`brandflow/taut_extras.go:1331-1390`).
6. **Editor Artikel** dari Status Publikasi tidak mengunci artikel yang sudah dijadwalkan, dan menyimpan mengubah status `approved` menjadi `draft` (`content/documents.go:196-307`).
7. **Total tagihan di Penyedia Konten** ikut menghitung artikel dari putaran lama (`routes/klien/[brandId]/taut-pintar/[id]/writer/+page.svelte:1494`).
8. **Impor XLSX Mitra:** harga 0 lolos (bertentangan dengan aturan media tanpa harga ditolak), platform bawaan WordPress walau mode draf, dan kategori dibatasi 15 padahal formulir 20 (`api/internal/domains/partner/import.go:508,525,551`).
9. **Menu Daftar Media Mitra:** dropdown jumlah backlink menawarkan 0 dan 3, tetapi server menolaknya (`routes/mitra/[partnerId]/media/+page.svelte:2964-2967`, `partner/media.go:420`, `media_detail.go:229`).
10. **Notifikasi Link Guardian tidak pernah terkirim.** "Artikel Anda tidak bisa diakses" (Klien) dan "Artikel perlu dipulihkan" (Mitra) hanya dikirim bila `failures >= LinkFailureThreshold && !wasProblem`. Kegagalan kedua selalu didahului status masalah, jadi syarat itu mustahil terpenuhi (`brandflow/job_link_check.go:192-196`). Dokumen tidak menjanjikan notifikasi ini.
11. **Coba lagi dan Kembalikan ke Persetujuan tidak menahan saldo lagi** setelah kegagalan permanen. Tagihan terlewat, sementara Mitra tetap dibayar (`brandflow/publications.go:577-607,666-711`, `support.go:366-401`, `job_publication_execution.go:299-306`).
12. **Potensi open redirect.** Tautan pelacakan `/t/{kode}` mengalihkan 301 ke URL bebas pilihan Writer dengan pemeriksaan hanya `http`/`https` (`brandflow/publications.go:1366`, `platform/public_endpoints.go:538-569`).
13. **Komisi referral, CPA, dan komisi transaksi mungkin tidak pernah tercatat.** Penyisipan di `brandflow/job_publication_execution.go:2680,2727` dan `platform/misc_finance.go:2615` tampaknya bertabrakan dengan kunci asing, kunci unik, dan tipe `char(36)` di `api/migrations/schema-full.sql:1739,4843,7443`. Tidak ada `INSERT` ke `referred_domains` di kode Go, sehingga daftar domain dan CPA VIP bisa kosong. Dokumen Referral Mitra dan VIP Dashboard menjelaskan fitur sesuai layar.
14. **Mode Manual dan penghasilan.** Mengirim URL artikel pada media Manual tidak mencatat penghasilan. Satu-satunya pencatat ada di jalur API (`partner/publications.go:878-935`, `job_publication_execution.go:2473`). Dokumen Publikasi menyarankan Mitra Manual menghubungi Dukungan bila nominal belum muncul.
15. **Penarikan dan pembayaran:** tarik sebagian mengunci entri utuh dan selisihnya hilang bila dibayar (`partner/finance.go:297-321`). Admin yang mengubah invoice dari gagal ke menunggu tidak mengunci ulang penghasilan, sehingga ada risiko tarik ganda (`platform/misc_finance.go:1745,1802`). CPA tetap dicatat setelah kontrak VIP habis, padahal layar menyebut nonaktif (`misc_finance.go:2687`).
16. **Privasi:** email pendaftar dan pemilik domain tampil kepada perujuk (`partner/finance.go:530,611`).
17. **Taut Brief:** galat pembuatan ringkasan membocorkan nama agen dan pengaturan internal ke Klien (`brief/summary/generate.go:75-84`). Simpan Draf bisa menimpa isian lain karena UI hanya mengirim kunci yang berubah sementara PUT mengganti seluruh kolom JSON (`profiler/profiler.go:416-428`). Simpan sebagai template dan daftar template kemungkinan gagal bila URL memakai kode pendek brand (`profiler/templates.go:254-262,284`).
18. **Masuk dengan Google tidak memeriksa 2FA** (`auth/google.go:407-451`, bandingkan `auth/login.go:134-143`). Akun yang mengaktifkan verifikasi dua langkah tetap bisa dimasuki lewat Google tanpa kode.
19. **Kartu 2FA di Konfigurasi bisa mematikan 2FA tanpa sengaja.** Status awal kartu `enabled:false` dan tidak pernah dimuat (`routes/mitra/[partnerId]/profile/+page.svelte:169-186,1004`, `auth/me.go:17-31`). Menekan **Aktifkan** memanggil setup yang langsung mengubah `two_factor_enabled` menjadi `FALSE` dan mengganti secret (`auth/twofa_manage.go:33`). Tombol **Nonaktifkan** tidak terjangkau setelah halaman dimuat ulang. Profil Klien memanggil setup serupa (`routes/klien/[brandId]/profile/+page.svelte:181`).
20. **Rekening penarikan dibaca dari profil saat admin memproses**, bukan salinan saat pengajuan (`platform/misc_finance.go:1547,1605`, `partner/finance.go:289`). Mengubah rekening di tengah proses bisa mengalihkan tujuan transfer.
21. **Harga yang dilihat Klien berbeda dari dana yang ditahan.** Katalog menampilkan harga Klien (harga Mitra ditambah markup, `clientread/helpers.go:114`), tetapi dana yang ditahan saat distribusi dihitung dari harga Mitra tanpa markup (`brandflow/taut_extras.go:1429,1460`). Bila markup lebih dari 0, dana tertahan lebih kecil dari harga yang tampil.
22. **Kebocoran data margin.** API katalog Klien mengirim harga Mitra, markup, serta nama dan ID Mitra (`clientread/media.go:294,311,341`), padahal obrolan dan katalog publik menyembunyikannya. Balasan Mitra di obrolan juga mengirim nama perusahaan ke API Klien (`chat/partner.go:218`, `chat/threads.go:167`).
23. **Peran Viewer bisa mengubah data.** Endpoint profil lama memotong 10 kredit per panggilan untuk Viewer (`clienttaut/profile.go:131,190`). Viewer juga dapat membuat, mengubah, menjadwalkan, dan memvalidasi kampanye (`clienttaut/campaigns.go:176,558`, `campaigns_wizard.go:163,213`) dan memicu pengumpulan data AI (`taut_extras.go:784`).
24. **Biaya analisis Profiler jalur lama tidak dikembalikan** bila AI gagal (`clienttaut/analyze.go:168-181,335-348`). Kampanye dari katalog tidak memeriksa media yang ditangguhkan atau disembunyikan, dan media tanpa harga dilewati diam-diam (`clienttaut/stats.go:136`).
25. **Persetujuan syarat integrasi otomatis.** Menyimpan profil di Konfigurasi ikut menyetujui Syarat & Ketentuan Integrasi (`profile/+page.svelte:288`). Panduan Integrasi menjelaskan persetujuan sebagai langkah terpisah.
26. **Kirim URL pada media Manual menangkap saldo Klien dengan URL apa pun.** Status langsung Terbit tanpa pemeriksaan domain (`partner/publications.go:891-936`). Ekspor CSV Publikasi tidak menetralkan awalan `=`, `+`, `-`, dan `@` (formula injection, `publications.go:516`). Status **Menunggu URL** tidak punya batas waktu dan Klien tidak bisa membatalkannya (`brandflow/helpers.go:165`).
27. **Mengembalikan publikasi ke persetujuan** tidak menahan saldo lagi dan tidak mengulang jam 3×24 (`brandflow/publications.go:661-711`, `scheduler.go:648`). Suntingan draf lama oleh Mitra bisa menimpa revisi Klien (`publication_draft.go:184`, `job_publication_execution.go:205`). Menerima revisi pada media Manual langsung kembali Terbit tanpa pengiriman (`job_publication_execution.go:249-262`), padahal pesan UI menyebut artikel dikirim ulang.
28. **Pengingat "Terlambat menyetujui publikasi" tidak pernah berjalan.** Penyapu pengingat hanya dipanggil dengan `late=false` (`brandflow/scheduler.go:111`). Dokumen tidak menjanjikannya.
29. **Saldo tidak lagi hangus (selesai).** Kebijakan penghangusan saldo akun pasif 12 bulan dihapus dari kode (penyapu scheduler, fungsi penghangusan, dua email, dan dua baris matriks notifikasi) karena saldo klien tidak boleh hangus. Sebelum dihapus, peringatan 7 harinya memang tidak pernah terkirim karena kueri ke tabel `brand_members` gagal, sedangkan penghangusannya tetap berjalan. Di produksi belum pernah ada saldo yang dihanguskan. Tabel `balance_forfeitures` dan jenis ledger `forfeit` dibiarkan di skema (kosong) supaya migrasi lama tetap utuh. Dokumen menyatakan saldo tidak punya masa berlaku.
30. **Analisis Profiler jalur lama bisa salah tagih.** Biaya tetap 10 unit (`clienttaut/analyze.go:19`, `profile.go:191`) kini setara Rp10 sejak saldo dihitung dalam rupiah (migrasi 233), sementara galatnya masih berbunyi "Butuh 10 TC". PPN 11% tampil di Total Top Up (`topup-modal.svelte:64`), tetapi tagihan yang dibuat tanpa PPN (`brandflow/finance.go:190`). Kunci idempotensi Top Up dipakai ulang sehingga nominal baru bisa membuka pesanan lama (`klien/[brandId]/+layout.svelte:161`, `finance.go:218,1615`). Minimum Top Up Rp10.000 hanya diperiksa di browser (`topup-modal.svelte:42`, `finance.go:134`).
31. **Reset password dan email.** Permintaan reset kedua dalam 10 menit mencabut tautan lama, tetapi email baru tidak terkirim karena dedupe (`auth/passwords.go:165-181`, `email/email.go:157-170`). Password polos disimpan sementara di `sessionStorage` saat alur daftar (`auth-panel.svelte:289-297`, `register-role/+page.svelte:152-170`). Pesan masuk membedakan akun terkunci atau ditangguhkan dan waktu responsnya berbeda, sehingga akun bisa dienumerasi (`auth/login.go:92-117`). Gerbang baca notifikasi WhatsApp dan email tidak pernah menahan karena kunci dedupe tidak cocok, sehingga biaya pesan bisa membengkak (`notify/wa_user.go:104-106`, `notify/gate.go:32-43`).
32. **Teks yang menyesatkan di aplikasi.** Email penolakan Mitra menyebut Saldo kembali hanya bila Klien membatalkan (`email/catalog.go:190`), padahal Saldo langsung dilepas (`partner/publications.go:868`). Tujuan "Kembali ke Beranda" di halaman galat membuka halaman Masuk walau sesi aktif (`+error.svelte:33`). `returnTo` diabaikan saat masuk (`auth-panel.svelte:370-405`, `google-callback/+page.svelte:32`). Label status berbeda antar halaman: Beranda dan kamus memakai "Menunggu Mitra", Status Publikasi memakai "Menunggu persetujuan media" (`web/src/lib/i18n/dictionaries.ts:102`, `publications/+page.svelte:174`).

### B. Perilaku yang membingungkan (dokumen menjelaskan apa adanya)

- **Beranda Klien:** kartu **Tauthub AI Insights** berisi pesan tetap, bukan hasil hitungan akun. Tautan **Klaim PPh 23** membuka **Invoice & Pajak**, yang hanya memuat daftar invoice dan tidak punya fitur klaim.
- **Konfigurasi Mitra:** kartu **Verifikasi dua langkah** selalu mulai dari **Nonaktif** karena tidak membaca status tersimpan, sehingga Mitra yang sudah mengaktifkan bisa salah menekan tombolnya. Keterangan di bawah judul menyebut "dokumen legalitas" padahal tidak ada bagiannya.
- **Katalog Media:** kolom berjudul **GEO** sebenarnya berisi AI Trust Score, dan keterangannya menyebut centang biru yang tidak tampil di tabel. Filter status publikasi **Menunggu review** tidak pernah menampilkan hasil.
- **Riwayat saldo Klien:** baris "Pengembalian saldo klaim Link Guardian (garansi tayang)" bertanda minus walau saldo bertambah.
- **Penghasilan Mitra:** tabel **Riwayat Penghasilan** tidak menandai baris yang sudah ditarik. Tabel dan filter hanya bekerja pada 20 baris halaman aktif.
- **Daftar Media Mitra:** kartu verifikasi domain menampilkan kode mentah (misalnya `token_missing`). Skor "Retensi tautan" dan "Uptime" dihitung dari penolakan Mitra dan status Aktif, dan bernilai 0 tanpa riwayat. Izin subjudul tidak diproses saat menambah media dan tidak bisa dikembalikan ke bawaan. Hapus contoh artikel di formulir tidak berefek. EmDash tidak terdeteksi otomatis sebagai platform.
- **Pesan galat berbahasa Inggris atau teknis** tampil ke pengguna: "display_name must be 2-120 characters", "Current password is incorrect", "Too big: expected …", dan "Brand not found" untuk peran yang kurang hak akses.
- **Penyedia Konten:** status **Perlu Revisi** tampil sebagai "Menunggu Artikel" di beberapa tempat. Tombol **Tulis Sendiri** disebut "Unggah Artikel" di pembaruan lama.
- **Hak akses brand:** halaman anggota brand tidak tertaut dari UI, sehingga tidak ada cara di dashboard untuk mengatur peran anggota (Viewer, Writer, Admin).
- **Ringkasan Distribusi:** kartu **Biaya Distribusi Media** menjadi Rp0 setelah distribusi (`brandflow/taut_extras.go:963-971`). Teks di panel Terbitkan Kampanye menyebut artikel terjadwal masih bisa diedit dan dijadwal ulang dari Ringkasan, padahal halaman itu menolak dan server mengunci (`ringkasan/+page.svelte:425-426`, `taut_extras.go:2087-2117`). Dokumen mengikuti perilaku kode.
- **Link Guardian dan Status Publikasi (Klien):** kartu dan filter **Bermasalah** hanya menghitung 404 dan tidak menghitung **Tautan dihapus** (`clientread/sections.go:139-155,194-203`). Baris **Verifikasi URL** di dialog Detail tidak pernah tampil (`publications/+page.svelte:782`). Penghitung **Tracking** `link_click` selalu 0 karena yang tercatat `tracking_link_click` (`brandflow/publications.go:1424`).
- **Katalog Media:** "Server Uptime" berasal dari status media (`brandflow/scheduler.go:1123`) dan "Link Permanence" dari skor penolakan, bukan dari pengukuran sebenarnya. Media tanpa riwayat berlabel "KONDISI RENDAH" (`scheduler.go:1169`). Kotak **pilih semua** menimpa seluruh pilihan, urutan SLA menaruh media tanpa data di tengah, lencana corong di HP menghitung kategori dua kali, dan "Belum ada media aktif" muncul juga saat pencarian tidak menemukan apa pun.
- **Kartu kampanye dan Berita Acara:** **Anggaran terserap** dan **Terpakai** kemungkinan selalu Rp0 karena query memakai referensi `taut:<id>:` yang tidak pernah ditulis (`clienttaut/stats.go:38-40,88-92`). Berita Acara publik menampilkan "Saldo terpakai" dalam kredit yang dicetak sebagai rupiah, "Artikel dibuat" membaca tabel yang tidak diisi Taut Pintar (`clientread/campaign_report.go:59-62`), dan tautannya tidak bisa dicabut maupun kedaluwarsa (`brandflow/campaigns.go:335`).
- **Integrasi Mitra:** contoh isian di Publishing API merusak uji ping (`_publishing-api.svelte:444`), **Buat ulang API key** tidak menampilkan key (`partner/media_detail.go:1745`), Blogspot berstatus **Terhubung** sebelum blog dipilih dan galat OAuth dibuang (`partner/blogger.go:459,529`), lencana Spam Score tampak terbalik (`seo-tool/+page.svelte:252`), dan Akses Crawler tidak punya tombol membuat kredensial baru setelah dinonaktifkan atau dicabut (`crawler/+page.svelte:197`). EmDash dan Publishing API tidak mengisi **URL Terbit**, sehingga artikelnya tidak dipantau Link Guardian (`brandflow/emdash.go:420`).
- **Halaman publik `/docs` berbeda dari kode:** halaman itu menyebut percobaan ulang pada menit ke-1, 5, dan 15 dengan `event_id` yang sama, sedangkan kode memakai `event_id` baru tiap percobaan dengan jeda paling lama 30 detik (`brandflow/content.go:1602`, `jobs/runner.go:77`). Dokumen `mitra/13-integrasi/06-webhook-bertanda-tangan.md` mengikuti kode. Dua halaman teknis di folder Integrasi (Publishing API dan Webhook bertanda tangan) ditujukan untuk developer Mitra dan memuat nama literal seperti `X-Tulisio-*`.
- **Konfigurasi Mitra:** **Email Tambahan 1/2** hanya disimpan, dan pengiriman email memakai email login (`partner/misc.go:271-298`, `notify/deliver.go:183`). Tidak ada jalur UI untuk mencabut izin atau nomor WhatsApp. Keluar akun tidak melepas notifikasi perangkat (`auth/logout.go:12`).
- **Taut Brief:** panel **Riwayat perubahan** selalu kosong karena API mengirim `entries` sementara UI membaca `items` (`profiler/history.go:117`, `profiler/[profileId]/+page.svelte:1766`). Gambar Timeline bisa rusak karena masa simpan gambar lebih pendek daripada masa post, 3 bulan (`uploadquota.go:61`, `timeline.go:34`). Batas unggah, kuota, dan masa simpan berasal dari pengaturan server (`profiler/uploadquota.go:52-66`), sedangkan angka 10 MB dan 5 berkas/25 MB di dokumen diambil dari konstanta tampilan.

### C. Teks dan label

- **Nama lama masih tampil:** label jenis media **Tulisio API** di Daftar Media (`routes/mitra/[partnerId]/media/+page.svelte:540`) dan notifikasi "Admin Tulisio" (`api/internal/domains/platform/dewan_pers.go:152`).
- **Pesan WhatsApp** menyebut "Profil → tab Notifikasi", padahal kartu nomor WhatsApp ada di tab Profil (`partner/wa.go:70`).
- **Banner impor** menyebut tombol "Verifikasi" yang tidak ada (`media/+page.svelte:1436`). **Tooltip Kualitas** berisi JSON mentah (`:1840`). Salah ketik "kerasahasiaan" (`:2673`).
- **Satu hal, banyak nama:** VVIP juga tertulis Brand Ambassador. Halaman Konfigurasi juga bernama Profil dan Konfigurasi Akun. Kampanye juga bernama Taut Pintar. Penyedia Konten juga bernama The Writer dan Dapur Produksi AI. Dashboard Klien juga bernama Client Workspace. Sebaiknya diseragamkan.
- **Catatan usang:** dialog di Penyedia Konten masih menyebut "The Matchmaker" (`writer/+page.svelte:2806`), dan catatan di panel QC usang (`writer/_qc-panel.svelte:537`).
- **Label menyesatkan:** pembatalan oleh admin tampil "Dibatalkan klien:" (`publications/+page.svelte:211-224`). Nama status di Beranda berbeda dari Status Publikasi (`web/src/lib/i18n/dictionaries.ts:89-107`). Taut Brief menyebut ringkasan "diringkas otomatis" padahal manual (`profiler/[profileId]/+page.svelte:105`) dan "dikoreksi lewat Ringkasan" padahal tab Entitas hanya baca (`:3570`). Peran yang kurang memunculkan "Brand not found" (`brands/brands.go:53`). Di sisi Mitra: Menunggu/Selesai versus Masa tunggu/Siap ditarik, Regular versus Reguler, dan baris **Rekening Tujuan** di Invoice menampilkan catatan (`settlements/+page.svelte:329,369`).

## Memperbarui dokumentasi

- Saat sebuah fitur berubah, perbarui halaman yang menjelaskannya, lalu periksa halaman yang menaut ke sana lewat bagian "Lihat juga".
- Untuk aturan yang bernilai tetap (3×24 jam, 2 hari, 12 bulan, 25%), cari angkanya di seluruh folder sebelum mengubahnya, karena disebut di beberapa halaman.
- Bila Anda memperbaiki salah satu keanehan di bagian B, sederhanakan kalimat penjelasnya di halaman terkait.
