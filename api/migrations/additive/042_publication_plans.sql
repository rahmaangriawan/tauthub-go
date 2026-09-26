-- Migration: 042_publication_plans
-- Recurring distribution plans ("jadwal berulang"): a client picks one approved
-- article + a list of media; the plan distributes the article to ONE selected
-- media per day (cadence = daily) until every media is scheduled or end_at is
-- reached. The schedule only handles DISTRIBUTION to the partner/media —
-- actual publication timing on the media site stays with the Mitra.
--
-- Occurrences are materialized up-front (cheap rows, one per media/day).
-- TC is NOT reserved at plan creation: the scheduler worker materializes a due
-- occurrence into a real publication + reserves credit at due time, so a plan
-- can span months/years without locking the client wallet.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/042_publication_plans.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.publication_plans (
  id          VARCHAR(36) PRIMARY KEY,
  brand_id    VARCHAR(36) NOT NULL REFERENCES public.brands(id) ON DELETE CASCADE,
  name        VARCHAR(160) NOT NULL,
  document_id VARCHAR(36) NOT NULL REFERENCES public.content_documents(id),
  cadence     VARCHAR(20) NOT NULL DEFAULT 'daily' CHECK (cadence IN ('daily')),
  start_at    DATE NOT NULL,
  end_at      DATE,
  time_of_day TIME NOT NULL DEFAULT '09:00',
  timezone    VARCHAR(64) NOT NULL DEFAULT 'Asia/Jakarta',
  status      VARCHAR(20) NOT NULL DEFAULT 'active'
              CHECK (status IN ('draft','active','paused','completed','cancelled')),
  created_by  VARCHAR(36),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT ck_publication_plans_dates CHECK (end_at IS NULL OR end_at >= start_at)
);

CREATE INDEX IF NOT EXISTS ix_publication_plans_brand ON public.publication_plans (brand_id, status);
CREATE INDEX IF NOT EXISTS ix_publication_plans_due ON public.publication_plans (status, start_at);

-- Media targeted by the plan (ordered by position → day 1, day 2, …).
CREATE TABLE IF NOT EXISTS public.publication_plan_media (
  plan_id  VARCHAR(36) NOT NULL REFERENCES public.publication_plans(id) ON DELETE CASCADE,
  media_id VARCHAR(36) NOT NULL REFERENCES public.media(id) ON DELETE CASCADE,
  position INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (plan_id, media_id)
);

-- One row per scheduled day/media. status:
--   scheduled   → not yet due
--   materialized → a real publication row exists (publication_id set)
--   skipped     → client skipped this day
--   failed      → materialization failed (e.g. insufficient credit / standard FAIL)
CREATE TABLE IF NOT EXISTS public.publication_plan_occurrences (
  id             VARCHAR(36) PRIMARY KEY,
  plan_id        VARCHAR(36) NOT NULL REFERENCES public.publication_plans(id) ON DELETE CASCADE,
  media_id       VARCHAR(36) NOT NULL REFERENCES public.media(id),
  scheduled_at   TIMESTAMPTZ NOT NULL,
  status         VARCHAR(20) NOT NULL DEFAULT 'scheduled'
                 CHECK (status IN ('scheduled','materialized','skipped','failed')),
  publication_id VARCHAR(36),
  error_message  TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (plan_id, media_id, scheduled_at)
);

CREATE INDEX IF NOT EXISTS ix_plan_occurrences_due
  ON public.publication_plan_occurrences (status, scheduled_at);

COMMIT;
