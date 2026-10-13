-- 213 — Kode pendek Profiler brand untuk URL yang ringkas.
--
-- Workspace chat Taut Brief memakai URL /klien/{brand}/profiler/{kode}, dan
-- siswa diminta maksimal 5 karakter. `id` tetap UUID 36 karakter sebagai kunci
-- internal; `short_code` hanya untuk tampilan URL.
--
-- Backfill memakai md5 dari id (deterministik, stabil walau dijalankan ulang
-- di beberapa DB); kolisi untuk 5 karakter praktis nol pada jumlah row sini.

ALTER TABLE public.profiler_profiles
    ADD COLUMN IF NOT EXISTS short_code character varying(5);

UPDATE public.profiler_profiles
SET short_code = substr(md5('profil:' || id), 1, 5)
WHERE short_code IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_profiler_profiles_short_code
    ON public.profiler_profiles (short_code);
