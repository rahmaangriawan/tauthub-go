-- 304 — Pisahkan "label sponsor di artikel" (label_sponsor, tampilan/disclosure)
-- dari "rel tautan" (link_rel: dofollow | nofollow | sponsored). Sebelumnya
-- label_sponsor = ada_sponsor ikut memaksa rel="sponsored" pada tautan, padahal
-- ada media yang memakai rel=sponsored tanpa label sponsor di artikel. Additive.

BEGIN;

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS link_rel text;

-- Backfill dari data lama: label sponsor → sponsored, nofollow → nofollow, sisanya dofollow.
UPDATE public.media SET link_rel = CASE
  WHEN label_sponsor = 'ada_sponsor' THEN 'sponsored'
  WHEN allow_hyperlink = FALSE THEN 'nofollow'
  ELSE 'dofollow' END
WHERE link_rel IS NULL;

ALTER TABLE public.media ALTER COLUMN link_rel SET DEFAULT 'dofollow';

COMMIT;
