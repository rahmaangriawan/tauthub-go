-- 142 — Simpan daftar narasumber lengkap (plural) di draft Profiler.
-- Sebelumnya hanya kolom `spokesperson` (tunggal, legacy) yang dipersist —
-- array `spokepersons` dari form divalidasi lalu dibuang, sehingga setelah
-- reload hanya narasumber pertama yang tersisa (state "lama" untuk validasi).
BEGIN;

ALTER TABLE public.taut_profiler_submissions
  ADD COLUMN IF NOT EXISTS spokepersons jsonb NOT NULL DEFAULT '[]'::jsonb;

-- Backfill: baris lama yang punya narasumber tunggal -> jadikan satu-satunya
-- entri list (hanya bila list masih kosong).
UPDATE public.taut_profiler_submissions
SET spokepersons = jsonb_build_array(spokesperson)
WHERE spokepersons = '[]'::jsonb
  AND jsonb_typeof(spokesperson) = 'object'
  AND COALESCE(spokesperson->>'name', '') <> '';

COMMIT;
