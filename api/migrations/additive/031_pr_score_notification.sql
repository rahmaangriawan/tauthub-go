-- Migration: 031_pr_score_notification
-- Phase 12: notify brand members when a campaign's PR score is updated
-- (analysis job completed → recalculated). Adds the 'pr.score_updated'
-- notification type to the existing CHECK constraint.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/031_pr_score_notification.sql

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
    'pr.score_updated'
  )
);

COMMIT;
