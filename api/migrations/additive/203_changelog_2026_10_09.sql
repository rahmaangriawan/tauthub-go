-- 203 — Entri changelog (Apa yang Baru) untuk rilis 2026-10-09.
-- Ditulis dengan bahasa awam (bukan istilah teknis) karena dibaca klien.
-- Terjemahan Inggris dibuat otomatis oleh sistem (kolom *_en, migrasi 191).

INSERT INTO public.changelog_entries
  (version, released_on, category, title, summary, body, source_key, published)
VALUES
  (
    '2026.10.09', DATE '2026-10-09', 'fitur',
    'Profiler: isi profil brand sekali, dipakai untuk semua artikel',
    'Simpan identitas brand, gaya bahasa, pedoman konten, dan referensi di satu tempat. Artikel AI berikutnya otomatis mengikuti acuan itu.',
    '<p>Sekarang ada menu <strong>Profiler</strong> untuk menyimpan "buku panduan brand" Anda: nama dan bidang usaha, gaya bahasa, kata yang dilarang, pedoman SEO, contoh artikel, sampai dokumen dan tautan referensi.</p><p>Isi sekali, lalu tiap kali membuat artikel AI, pilih Profiler yang sesuai — tulisan akan mengikuti gaya brand Anda. Satu akun boleh punya banyak Profiler (misalnya untuk beberapa brand berbeda), dan bisa diganti nama, disalin, atau dihapus.</p>',
    'changelog-2026-10-09-profiler', TRUE
  ),
  (
    '2026.10.09', DATE '2026-10-09', 'fitur',
    'Pilih jenis artikel: Editorial, Advertorial, atau Opini',
    'Saat membuat naskah dengan AI, Anda bisa memilih mau ditulis sebagai artikel editorial, advertorial, atau opini.',
    '<p>Di jendela <strong>Pengelola Naskah</strong> (tab Generate dengan AI) kini ada pilihan <strong>Jenis Artikel</strong>: Editorial, Advertorial, atau Opini. Masing-masing punya gaya penulisan sendiri, jadi hasilnya lebih pas dengan kebutuhan Anda.</p><p>Jenis artikel ini tersedia untuk media mainstream. Untuk media blog, gaya tulisannya sudah otomatis menyesuaikan.</p>',
    'changelog-2026-10-09-jenis-artikel', TRUE
  ),
  (
    '2026.10.09', DATE '2026-10-09', 'fitur',
    'Minta gambar, AI menyisipkannya langsung ke dalam artikel',
    'Cukup tulis permintaan gambar di kolom topik. AI akan membuat gambarnya dan menaruhnya di dalam artikel. Perkiraan biayanya ditampilkan lebih dulu.',
    '<p>Kalau Anda minta gambar (misalnya "sertakan 3 gambar ilustrasi") saat membuat naskah, AI akan membuat gambar tersebut dan memasukkannya ke dalam artikel secara otomatis.</p><p>Sebelum diproses, Anda melihat perkiraan jumlah gambar dan biayanya lebih dulu, supaya tidak kaget. Gambar utamanya tetap Anda atur sendiri seperti biasa.</p>',
    'changelog-2026-10-09-gambar-artikel', TRUE
  ),
  (
    '2026.10.09', DATE '2026-10-09', 'fitur',
    'Unggah dokumen Word atau PDF, gambarnya ikut terbawa',
    'Dokumen yang Anda unggah dipakai apa adanya — termasuk gambarnya — dan gambarnya otomatis disimpan agar ikut terbit di media tujuan.',
    '<p>Saat mengunggah dokumen di tab <strong>Unggah Dokumen</strong>, sekarang gambar di dalam dokumen ikut dipindahkan, bukan cuma teksnya. Dokumen PDF juga sudah bisa dibaca.</p><p>Dokumen dipakai apa adanya (tidak ditulis ulang AI). Sistem tetap memeriksa tautan dan memastikan isinya aman untuk diterbitkan.</p>',
    'changelog-2026-10-09-unggah-dokumen', TRUE
  ),
  (
    '2026.10.09', DATE '2026-10-09', 'perbaikan',
    'Tabel media lebih rapi di HP & kategori lebih berwarna',
    'Tampilan katalog media kini menyesuaikan layar HP, dan setiap kategori punya warna sendiri agar mudah dibedakan.',
    '<p>Buka katalog media dari HP: kolom yang kurang penting otomatis disembunyikan, jadi yang terlihat tinggal nama media, harga, dan tombolnya — tidak perlu digeser ke samping.</p><p>Nama kategori juga kini berwarna-warni (masing-masing beda) supaya lebih cepat dikenali.</p>',
    'changelog-2026-10-09-tabel-media-hp', TRUE
  ),
  (
    '2026.10.09', DATE '2026-10-09', 'perbaikan',
    'Animasi halus saat memilih media dan menyimpan media',
    'Ada gerakan singkat sebagai tanda bahwa pilihan Anda tercatat — dibuat halus dan tidak mengganggu.',
    '<p>Saat Anda mencentang media, muncul animasi kecil seolah pilihan itu terbang menuju tombol <strong>Buat Kampanye</strong>, dan tombolnya berdenyut sebentar.</p><p>Begitu juga saat menandai media sebagai simpanan: ikonnya terbang ke tombol <strong>Bookmark</strong>. Semua animasi otomatis dimatikan bila perangkat Anda diatur untuk mengurangi gerakan.</p>',
    'changelog-2026-10-09-animasi-pilih', TRUE
  ),
  (
    '2026.10.09', DATE '2026-10-09', 'perbaikan',
    'Modal naskah kini menampilkan jenis media target',
    'Di jendela pengelola naskah terlihat apakah media tujuan termasuk media mainstream atau blog.',
    '<p>Saat membuka pengelola naskah, di bagian atas tercantum jenis medianya: <strong>Media Mainstream</strong> atau <strong>Media Blogger</strong>. Jadi Anda tahu gaya tulisan mana yang akan dipakai AI.</p>',
    'changelog-2026-10-09-jenis-media', TRUE
  )
ON CONFLICT (source_key) DO NOTHING;
