-- Migration: 067_notification_types
-- Tipe notifikasi baru untuk Media Review dan Withdrawal:
--   media.*        → pengajuan/persetujuan/penolakan media oleh Admin
--   withdrawal.*   → lifecycle permintaan withdraw Mitra
-- Plus 'publication.url_requested' yang sudah dipakai kode
-- (notifyPartnerUrlRequested) namun belum masuk constraint — ditambahkan
-- agar union TS (db/models.ts) dan CHECK constraint sinkron.
--
-- Pola sama seperti migration 022/025/031/032: DROP + ADD constraint
-- (hanya mengubah daftar nilai, tidak menyentuh data).
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/067_notification_types.sql

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

COMMIT;
