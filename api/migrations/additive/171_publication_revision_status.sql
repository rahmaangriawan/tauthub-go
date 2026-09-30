-- 171 — Status "revision" untuk publikasi + ACC revisi oleh mitra.
--
-- Alur: klien meminta revisi pada artikel yang SUDAH TERBIT → status publikasi
-- berubah menjadi 'revision' dan mitra melihatnya di daftar publikasi dengan
-- tombol Terima/Tolak. Terima mengantrekan ulang publikasi supaya konten + cover
-- versi revisi benar-benar dikirim ke CMS mitra (worker hanya melewati status
-- 'published'/'cancelled', jadi status harus mundur ke 'queued' dulu). Tolak
-- TIDAK mengubah status: klien memperbaiki lalu mengirim ulang, dan mitra boleh
-- menolak maksimal 3 kali (dihitung di publications.revision_reject_count).
--
-- Pola sama seperti 169/079: drop + add constraint, hanya daftar nilai. Aditif,
-- tanpa backfill.

BEGIN;

-- ── 1. publications.status menerima 'revision' ─────────────────────────────
ALTER TABLE public.publications DROP CONSTRAINT IF EXISTS ck_publication_status;

ALTER TABLE public.publications ADD CONSTRAINT ck_publication_status CHECK (
  status IN (
    'pending', 'waiting_partner', 'partner_approved', 'partner_rejected',
    'queued', 'processing', 'published', 'failed', 'retrying', 'cancelled',
    'revision'
  )
);

-- ── 2. Penghitung penolakan revisi (batas 3 kali) ──────────────────────────
ALTER TABLE public.publications
  ADD COLUMN IF NOT EXISTS revision_reject_count integer NOT NULL DEFAULT 0;

-- ── 3. Tipe notifikasi untuk hasil ACC revisi ─────────────────────────────
ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;

ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type CHECK (
  type IN (
    'media.approved', 'media.dewan_pers.approved', 'media.dewan_pers.rejected',
    'media.dewan_pers.submitted', 'media.domain_down', 'media.negotiated',
    'media.rejected', 'media.resubmitted', 'media.submitted',
    'pr.score_updated', 'pr.verification_completed',
    'publication.auto_replaced', 'publication.client_approved', 'publication.failed',
    'publication.late_approval', 'publication.link_lost',
    'publication.link_replacement_requested', 'publication.partner_approved',
    'publication.partner_rejected', 'publication.published', 'publication.reminder',
    'publication.revision_requested', 'publication.revision_accepted',
    'publication.revision_rejected', 'publication.url_requested',
    'publication.waiting_partner',
    'referral.commission.credited', 'referral.commission.reversed',
    'referral.cpa.credited', 'referral.domain.approved', 'referral.domain.rejected',
    'variant_batch.failed', 'vip.disabled', 'vip.enabled',
    'withdrawal.paid', 'withdrawal.processing', 'withdrawal.rejected',
    'withdrawal.requested'
  )
);

COMMIT;
