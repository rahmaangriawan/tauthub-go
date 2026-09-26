-- 108 — The Profiler: persist pilihan channel pada draft submission.
-- Sebelumnya `channels` hanya disimpan sebagai snapshot per submission saat
-- analyze (taut_profiler_submission_channels), sehingga selection draft yang
-- sedang diedit hilang setelah refresh. Tambah kolom jsonb untuk menyimpan
-- selection draft (channel slug + angle_slug) seperti kolom draft lain.
BEGIN;

ALTER TABLE public.taut_profiler_submissions
  ADD COLUMN IF NOT EXISTS channels jsonb NOT NULL DEFAULT '[]'::jsonb;

COMMIT;
