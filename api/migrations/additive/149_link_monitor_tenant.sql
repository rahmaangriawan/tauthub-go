-- 149 — Pelacak 404 di dashboard klien & mitra: satu tipe notifikasi baru.
-- Aditif: hanya DROP+ADD constraint (pola sama seperti migrasi 088/148), tanpa
-- mengubah data. TIDAK ada tabel atau kolom baru — data pelacaknya sudah dibuat
-- di migrasi 148 (publication_link_checks + media.domain_health).
--
-- Dipakai oleh POST /v1/brands/[brandId]/link-monitor/request-replacement:
-- admin platform menerima sinyal eksplisit dari klien bahwa artikel yang
-- hilang perlu diganti. Terpisah dari publication.link_lost, yang dikirim
-- otomatis oleh pemeriksa tautan.

DO $$
BEGIN
  ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
EXCEPTION WHEN others THEN NULL;
END $$;

ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type
  CHECK (type IN (
    'publication.waiting_partner',
    'publication.partner_approved',
    'publication.partner_rejected',
    'publication.auto_replaced',
    'publication.published',
    'publication.failed',
    'publication.url_requested',
    'publication.reminder',
    'publication.late_approval',
    'publication.link_lost',
    'publication.link_replacement_requested',
    'variant_batch.failed',
    'pr.score_updated',
    'pr.verification_completed',
    'media.submitted',
    'media.approved',
    'media.rejected',
    'media.resubmitted',
    'media.negotiated',
    'media.domain_down',
    'media.dewan_pers.submitted',
    'media.dewan_pers.approved',
    'media.dewan_pers.rejected',
    'withdrawal.requested',
    'withdrawal.processing',
    'withdrawal.paid',
    'withdrawal.rejected',
    'referral.domain.approved',
    'referral.domain.rejected',
    'referral.cpa.credited',
    'referral.commission.credited',
    'referral.commission.reversed',
    'vip.enabled',
    'vip.disabled'
  ));
