-- Notification reminder untuk publikasi yang menunggu approval Mitra
-- mendekati deadline (scheduled_at). Additive & backward compatible:
-- hanya menambah tipe notification + kolom nullable baru, tidak menyentuh
-- data existing, tidak DROP.
--
-- Pola sama seperti migration 067: DROP + ADD constraint (hanya mengubah
-- daftar nilai yang diizinkan, tidak menyentuh data), lalu ADD COLUMN IF NOT
-- EXISTS + CREATE INDEX IF NOT EXISTS.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/069_partner_reminder_notifications.sql

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
    'publication.url_requested',
    'publication.reminder',
    'variant_batch.failed',
    'pr.score_updated',
    'pr.verification_completed',
    'media.submitted',
    'media.approved',
    'media.rejected',
    'media.resubmitted',
    'withdrawal.requested',
    'withdrawal.processing',
    'withdrawal.paid',
    'withdrawal.rejected'
  )
);

ALTER TABLE public.notifications
    ADD COLUMN IF NOT EXISTS dedupe_key character varying(200);

CREATE INDEX IF NOT EXISTS idx_notifications_dedupe
    ON public.notifications (user_id, dedupe_key)
    WHERE dedupe_key IS NOT NULL;

COMMIT;
