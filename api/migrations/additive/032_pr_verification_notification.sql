-- Migration: 032_pr_verification_notification
-- Phase 13: notify the media partner when the URL verification of their
-- published article completes (passed / failed / unavailable). Adds the
-- 'pr.verification_completed' notification type to the existing CHECK
-- constraint.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/032_pr_verification_notification.sql

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
    'variant_batch.failed',
    'pr.score_updated',
    'pr.verification_completed'
  )
);

COMMIT;
