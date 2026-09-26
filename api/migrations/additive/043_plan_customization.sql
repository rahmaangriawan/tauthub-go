-- Migration: 043_plan_customization
-- Extend recurring publication plans with scheduling customization:
--   media_per_day  → how many media are distributed per scheduled day (default 1)
--   date_mode      → how the schedule dates are chosen:
--                     'consecutive' (default)  = start_at + 1 day each step
--                     'random'                 = dates shuffled inside the range
--                     'custom'                 = explicit dates chosen by client
--   custom_dates   → JSON array of "YYYY-MM-DD" when date_mode = 'custom'
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/043_plan_customization.sql

BEGIN;

ALTER TABLE public.publication_plans
  ADD COLUMN IF NOT EXISTS media_per_day INTEGER NOT NULL DEFAULT 1,
  ADD COLUMN IF NOT EXISTS date_mode VARCHAR(16) NOT NULL DEFAULT 'consecutive'
    CHECK (date_mode IN ('consecutive','random','custom')),
  ADD COLUMN IF NOT EXISTS custom_dates JSONB;

COMMIT;
