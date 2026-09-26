-- 113 -- Per-entity encryption key version (v3 sealed secrets).
-- enc_key_ver = 0 (default): data lama, di-seal dengan kunci global (t2:).
-- enc_key_ver = 1: di-seal dengan key turunan per-entity (t3:, APP_SECRET + entity id).
-- Kolom baru default 0 sehingga data lama tetap terbaca tanpa backfill.

ALTER TABLE public.cms_connections
  ADD COLUMN IF NOT EXISTS enc_key_ver integer NOT NULL DEFAULT 0;

ALTER TABLE public.media
  ADD COLUMN IF NOT EXISTS enc_key_ver integer NOT NULL DEFAULT 0;
