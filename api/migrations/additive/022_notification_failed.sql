-- Migration: 022_notification_failed
-- In-app notifications now also carry 'publication.failed' so platform admins
-- get pinged when a distribution fails (with the adapter error code).
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/022_notification_failed.sql

BEGIN;

ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;

ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type CHECK (
  type IN (
    'publication.waiting_partner',
    'publication.partner_approved',
    'publication.partner_rejected',
    'publication.published',
    'publication.failed'
  )
);

COMMIT;
