-- Migration: 078_media_negotiated_notification
-- Tipe notifikasi baru 'media.negotiated' untuk pemberitahuan negosiasi harga
-- media dari Admin ke Mitra (sebelumnya salah pakai media.rejected).
--
-- Pola sama seperti migration 067: DROP + ADD constraint
-- (hanya mengubah daftar nilai, tidak menyentuh data).

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
    'media.negotiated',
    'media.dewan_pers.submitted',
    'media.dewan_pers.approved',
    'media.dewan_pers.rejected',
    'withdrawal.requested',
    'withdrawal.processing',
    'withdrawal.paid',
    'withdrawal.rejected'
  )
);

COMMIT;
