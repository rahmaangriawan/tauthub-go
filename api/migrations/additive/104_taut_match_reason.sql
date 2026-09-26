-- 104 — Taut Pintar: simpan alasan rekomendasi Matchmaker per media (dok Agent §2: output ranked {media_id, score, alasan}).
BEGIN;

ALTER TABLE public.taut_campaign_media ADD COLUMN IF NOT EXISTS match_reason text;

COMMIT;
