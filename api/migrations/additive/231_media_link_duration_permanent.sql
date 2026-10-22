-- 231 — Semua media yang masih memakai durasi bawaan 12 bulan jadi permanen.
--
-- Kolom link_duration_months dibuat (229) dengan DEFAULT 12 supaya perilaku
-- media lama tidak berubah. Akibatnya media yang didaftarkan sesudahnya ikut
-- mewarisi 12 bulan tanpa pernah dipilih mitra — padahal tautan media di sini
-- dijaga tetap tayang. Karena itu:
--   - semua baris yang masih 12 bulan (nilai bawaan, bukan pilihan mitra) → 0,
--   - DEFAULT kolom ikut 0 supaya media baru tidak lagi terlanjur 12 bulan.
-- Media yang memang diset 6/24/36 bulan tidak disentuh.
UPDATE public.media
   SET link_duration_months = 0
 WHERE link_duration_months = 12;

ALTER TABLE public.media
  ALTER COLUMN link_duration_months SET DEFAULT 0;
