-- 105 — Taut Pintar Flow 1 Profiler v2 (5 alur, acuan flow 1.md 2026-08-27).
-- Additive. Gap-closure: profiler_summary + category_input + taut_status profiling/profiled.
-- Ref: docs/Flow Taut Pintar/flow/flow 1.md §3, §8 + taut-pintar-flow-4-agent-v2.md §3
BEGIN;

ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS profiler_summary jsonb;
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS category_input jsonb;

DO $$ BEGIN
  ALTER TABLE public.campaigns DROP CONSTRAINT ck_campaigns_taut_status;
EXCEPTION WHEN undefined_object THEN NULL; END $$;

DO $$ BEGIN
  ALTER TABLE public.campaigns ADD CONSTRAINT ck_campaigns_taut_status
    CHECK (taut_status IN ('draft','profiling','profiled','materials','budget','placement','production','review','scheduled','submitted'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE INDEX IF NOT EXISTS ix_campaigns_profiler_status
  ON public.campaigns(brand_id, taut_status) WHERE taut_status IN ('profiling','profiled');

COMMIT;
