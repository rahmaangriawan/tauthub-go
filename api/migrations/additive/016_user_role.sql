-- Migration: 016_user_role
-- Adds an authoritative role column to users (client | partner | admin) so
-- registration can store the chosen role and login/redirects read it from the
-- backend instead of deriving it from URL/frontend state. Existing role data
-- (platform_admins / partners) is backfilled; the derived-role queries in the
-- app remain as the source for tenant scoping.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/016_user_role.sql

BEGIN;

ALTER TABLE public.users ADD COLUMN IF NOT EXISTS role character varying(20) NOT NULL DEFAULT 'client';

-- One CHECK constraint, added idempotently.
DO $$
BEGIN
  ALTER TABLE public.users ADD CONSTRAINT ck_users_role CHECK (role IN ('client', 'partner', 'admin'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Backfill from the existing role tables (platform admin wins, then partner).
UPDATE public.users SET role = 'admin'
  WHERE role = 'client' AND id IN (SELECT user_id FROM public.platform_admins);

UPDATE public.users SET role = 'partner'
  WHERE role = 'client' AND id IN (SELECT user_id FROM public.partners);

COMMIT;
