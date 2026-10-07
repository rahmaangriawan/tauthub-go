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

## v0.3.0 — 2026-10-07

### Fitur
- Katalog media: bar seleksi kini menampilkan saldo kredit, lengkap dengan peringatan "kurang kredit" dan tautan Top Up.
- Katalog media: pilihan media tersimpan antar sesi — tidak hilang lagi saat halaman di-refresh atau berpindah halaman.
- Katalog media: modal konfirmasi menyebut total biaya, saldo, dan langkah setelahnya sebelum kampanye dibuat.
- Katalog media: peringatan bila media yang dipilih sudah dipesan di kampanye aktif lain.
- Katalog media: pencarian pilihan media dari CSV hasil "Ekspor Proposal" tetap bisa dilanjutkan (nama kampanye opsional, dibuat otomatis bila dikosongkan).
- Taut Pintar: kampanye yang sudah selesai (terkirim/terjadwal/terdistribusi) kini mode lihat saja — Matchmaker, Writer, Kilat, dan Ringkasan Profiler tetap bisa dibaca, tapi tidak bisa diubah.
- The Writer: tombol "Generate Konten" membuka Pengelola Naskah di tab "Generate dengan AI", jadi topik, cover, dan kategori diisi dulu sebelum artikel ditulis.
- Admin: peran akun bisa diubah Klien ⇄ Mitra dari halaman Pengguna dan halaman Mitra.
- Halaman "Apa yang Baru" ini sendiri: catatan rilis bisa dibaca dari dashboard klien & mitra.

### Perbaikan
- Advance kampanye yang sudah melewati tahapnya tidak lagi berakhir dengan error 422 — aplikasi mengarahkan ke Ringkasan.
- Kolom pengaju di halaman Pending admin media tidak lagi menampilkan "—": nama mitra diambil langsung dari data media, dan bisa diklik untuk memeriksa mitranya.
- Request yang melewati batas 12 detik kini memberi pesan jelas ("Server terlalu lama merespons"), dan error AbortError yang mengotori konsol tidak lagi muncul.
- Setelah rilis baru, aplikasi mengenali versi terbaru dan memuat ulang sendiri, sehingga navigasi tidak lagi mentok "Failed to fetch dynamically imported module".

### Performa
- Daftar media dan daftar mitra memakai cache singkat (stale-while-revalidate): halaman terasa instan saat dibuka ulang, dan cache dibersihkan otomatis setiap kali ada aksi simpan.
