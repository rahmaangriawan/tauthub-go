-- Migration: 079_publication_late_approval_notification
-- Tipe notifikasi baru 'publication.late_approval' untuk pengingat mitra
-- yang belum menyetujui artikel padahal jadwal terbit sudah lewat.
-- Pola sama seperti 067/078: DROP + ADD constraint (hanya daftar nilai).

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
    'publication.late_approval',
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
