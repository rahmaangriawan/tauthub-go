-- 124 — Taut Pintar Writer: mode pelacakan + tautan utama/cadangan per kampanye,
-- batas link per media. Kolom di campaigns dibaca Gatekeeper saat menanam link;
-- max_links di media dipakai saat menghitung <a> per HTML artikel.

BEGIN;

ALTER TABLE public.campaigns
  ADD COLUMN IF NOT EXISTS tracking_mode text NOT NULL DEFAULT 'direct';
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'campaigns_tracking_mode_check'
  ) THEN
    ALTER TABLE public.campaigns
      ADD CONSTRAINT campaigns_tracking_mode_check
      CHECK (tracking_mode IN ('direct', 'tauthub'));
  END IF;
END $$;

ALTER TABLE public.campaigns
  ADD COLUMN IF NOT EXISTS primary_url text;
ALTER TABLE public.campaigns
  ADD COLUMN IF NOT EXISTS primary_anchor varchar(120);
ALTER TABLE public.campaigns
  ADD COLUMN IF NOT EXISTS backup_url text;
ALTER TABLE public.campaigns
  ADD COLUMN IF NOT EXISTS backup_anchor varchar(120);

ALTER TABLE public.media
  ADD COLUMN IF NOT EXISTS max_links integer;

COMMIT;
