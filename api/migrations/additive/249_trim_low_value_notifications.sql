-- 249 — Pangkas notifikasi bernilai rendah (legacy + noise).
--
-- Temuan audit production (24 Okt): 19.384 baris notifikasi menahan ~14 MB,
-- dan 94%-nya berasal dari dua sumber yang tidak berguna:
--
--   1. media.domain_down dengan dedupe_key NULL — 16.504 baris. Kode lama
--      belum memakai dedupe sehingga alert domain yang sama ditulis ribuan
--      kali (badai 2-3 Okt: 16.462 baris untuk hanya 23 media). Versi kode
--      sekarang sudah berdedupe (46 baris ber-key = 23 media x 2 admin);
--      hanya sisa legacy ini yang dibersihkan.
--   2. changelog.new — 1.555 baris, satu per user per entri rilis; 1.493
--      tidak pernah dibaca. Fan-out-nya dihentikan di kode; halaman
--      "Apa yang Baru" tetap menjadi kanal resmi.
--   3. pr.score_updated — 91 baris. Skor PR dihitung dari aksi user sendiri
--      dan tetap terlihat di halaman PR Calculator; notifikasinya dihentikan.
--
-- Baris dengan dedupe_key terisi TIDAK disentuh.

DELETE FROM public.notifications
WHERE type = 'media.domain_down'
  AND dedupe_key IS NULL;

DELETE FROM public.notifications
WHERE type IN ('changelog.new', 'pr.score_updated');
