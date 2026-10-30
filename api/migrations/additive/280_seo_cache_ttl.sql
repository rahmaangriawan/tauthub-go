-- 280 — TTL cache DA/PA bisa dikonfigurasi.
--
-- Sebelumnya umur cache di kode: media 2 bulan (mpSeoIsFresh) dan tool mitra
-- 24 jam. Kolom ini menampung TTL media (jam); NULL = default 1440 (60 hari).
ALTER TABLE public.platform_seo_config
  ADD COLUMN IF NOT EXISTS cache_ttl_hours integer;

UPDATE public.platform_seo_config
SET cache_ttl_hours = 1440
WHERE id = 'default' AND cache_ttl_hours IS NULL;
