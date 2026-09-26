-- 128 — Taut Pintar Writer: batas regenerasi artikel.
-- Generate pertama = 1 kredit. Generate ulang = 2 kredit, maksimal 2 kali per media
-- (regen_count 0..2; tolak bila sudah 2).

BEGIN;

ALTER TABLE public.taut_campaign_media
  ADD COLUMN IF NOT EXISTS regen_count integer NOT NULL DEFAULT 0;

COMMIT;
