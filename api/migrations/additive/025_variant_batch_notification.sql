-- Migration: 025_variant_batch_notification
-- In-app notifications now also carry 'variant_batch.failed' so platform
-- admins get pinged when a multi-article batch ends in failed / partial
-- (generation errors in the queue worker, not distribution errors).
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/025_variant_batch_notification.sql

BEGIN;

ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;

ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type CHECK (
  type IN (
    'publication.waiting_partner',
    'publication.partner_approved',
    'publication.partner_rejected',
    'publication.published',
    'publication.failed',
    'variant_batch.failed'
  )
);

COMMIT;
