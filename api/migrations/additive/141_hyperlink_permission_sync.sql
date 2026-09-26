-- 141 — Izin backlink mengikuti keputusan mitra (max_links).
-- allow_hyperlink (default FALSE, migrasi 102) tidak pernah ditulis aplikasi,
-- jadi media mainstream yang mitranya sudah memberi izin (max_links 1-2) tetap
-- tampil "Tidak Tersedia Backlink" dan link tidak ditanam ke artikel.
-- Baris dengan max_links terisi → allow_hyperlink ikut nilai itu.
BEGIN;

UPDATE public.media SET allow_hyperlink = TRUE WHERE max_links > 0 AND allow_hyperlink IS NOT TRUE;
UPDATE public.media SET allow_hyperlink = FALSE WHERE max_links = 0 AND allow_hyperlink IS NOT FALSE;

COMMIT;