-- Migration: 027_publication_auto_replaced
-- Phase 10: automatic replacement notification type. When a partner rejects
-- a publication and Tulisio auto-creates a replacement to the best media,
-- brand members get a 'publication.auto_replaced' notification.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/027_publication_auto_replaced.sql

BEGIN;

ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;

ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type CHECK (
  type IN (
    'publication.waiting_partner',
    'publication.partner_approved',
    'publication.partner_rejected',
    'publication.auto_replaced',
    'publication.published',
    'publication.failed',
    'variant_batch.failed'
  )
);

COMMIT;
