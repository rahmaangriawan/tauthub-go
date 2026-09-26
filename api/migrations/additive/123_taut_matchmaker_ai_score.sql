-- 123 — Taut Pintar: simpan skor match + urutan dari LLM Matchmaker.
-- Skor AI disimpan agar listPlacement/UI tidak recompute formula deterministik.

BEGIN;

ALTER TABLE public.taut_campaign_media ADD COLUMN IF NOT EXISTS ai_score integer;
ALTER TABLE public.taut_campaign_media ADD COLUMN IF NOT EXISTS match_rank integer;

COMMIT;
