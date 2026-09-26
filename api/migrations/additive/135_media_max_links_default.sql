-- 135 — Kapasitas link per media ditentukan MITRA (bukan ngasal).
-- Default 1 saat media terdaftar; nilai lama yang ngasal (2 dari seed manual /
-- NULL dari fallback kode) direset ke 1 supaya data mulai dari kebijakan mitra.
BEGIN;

ALTER TABLE public.media ALTER COLUMN max_links SET DEFAULT 1;

UPDATE public.media SET max_links = 1 WHERE max_links IS NULL OR max_links = 2;

COMMIT;
