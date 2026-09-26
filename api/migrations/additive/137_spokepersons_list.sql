-- 137 — Narasumber jadi daftar (multi-entry) per kampanye.
-- Kolom campaigns.spokepersons jsonb: [{name, role, quote}] — diisi draft Profiler,
-- dipakai The Writer (mainstream) sebagai acuan kutipan wajib.
BEGIN;

ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS spokepersons jsonb;

COMMIT;
