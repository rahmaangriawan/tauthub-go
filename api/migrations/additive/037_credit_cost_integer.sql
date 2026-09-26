-- Migration: 037_credit_cost_integer
-- media.credit_cost was smallint (max 32767 TC ≈ Rp32.767.000 at 1 TC = Rp1.000).
-- Media priced above that (e.g. Alodokter Rp82.000.000 → 82.000 TC) overflowed
-- the column. Widen to integer; the ck_media_credit_cost (>= 0) check stays.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/037_credit_cost_integer.sql

BEGIN;

ALTER TABLE public.media ALTER COLUMN credit_cost TYPE integer USING credit_cost::integer;

COMMIT;
