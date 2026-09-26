-- Migration: 026_media_intelligence
-- Phase 10 - Intelligence: media health score + smart media recommendation.
--
-- media gains operational/performance fields used by the recommendation &
-- health engine (lib/media/intelligence.ts):
--   traffic_estimate    : admin-provided monthly visitors estimate (reach basis)
--   health_score        : computed 0-100 (success rate, publish time, rejection
--                         rate, availability, traffic, historical performance)
--   health_breakdown    : jsonb component scores + reason strings
--   health_computed_at  : when the score was last refreshed
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/026_media_intelligence.sql

BEGIN;

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS traffic_estimate bigint DEFAULT 0 NOT NULL;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS health_score smallint;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS health_breakdown jsonb;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS health_computed_at timestamp(6) with time zone;

COMMIT;
