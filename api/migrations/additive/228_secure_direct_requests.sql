-- 228: Pengajuan fitur Secure Direct (mode terbit otomatis) oleh mitra.
-- Mitra mengajukan lewat popup di Daftar Media; admin menyetujui lewat
-- gerbang "Izinkan mode otomatis" di /admin/partners (PUT auto_publish_enabled
-- menutup pengajuan pending secara otomatis dan memberi tahu mitra).

CREATE TABLE IF NOT EXISTS public.secure_direct_requests (
    id           character(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    partner_id   character(36) NOT NULL REFERENCES public.partners(id) ON DELETE CASCADE,
    status       varchar(20) NOT NULL DEFAULT 'pending',
    note         text,
    requested_at timestamptz NOT NULL DEFAULT now(),
    decided_at   timestamptz,
    decided_by   character(36) REFERENCES public.users(id) ON DELETE SET NULL
);

-- Satu pengajuan aktif per partner; boleh mengajukan ulang setelah penolakan.
CREATE UNIQUE INDEX IF NOT EXISTS secure_direct_requests_one_pending
    ON public.secure_direct_requests (partner_id) WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS secure_direct_requests_partner_idx
    ON public.secure_direct_requests (partner_id, requested_at DESC);

-- Tipe notifikasi baru (pengajuan → admin, persetujuan → mitra).
DO $$
BEGIN
  ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
EXCEPTION WHEN others THEN NULL;
END $$;

ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type
  CHECK (((type)::text = ANY ((ARRAY['announcement.new', 'changelog.new', 'chat.message', 'media.approved', 'media.crawl_blocked', 'media.dewan_pers.approved', 'media.dewan_pers.rejected', 'media.dewan_pers.submitted', 'media.domain_down', 'media.negotiated', 'media.rejected', 'media.resubmitted', 'media.submitted', 'pr.score_updated', 'pr.verification_completed', 'publication.auto_replaced', 'publication.client_approved', 'publication.failed', 'publication.late_approval', 'publication.link_lost', 'publication.link_replacement_requested', 'publication.partner_approved', 'publication.partner_rejected', 'publication.published', 'publication.reminder', 'publication.revision_accepted', 'publication.revision_rejected', 'publication.revision_requested', 'publication.url_requested', 'publication.waiting_partner', 'referral.commission.credited', 'referral.commission.reversed', 'referral.cpa.credited', 'referral.domain.approved', 'referral.domain.rejected', 'secure_direct.approved', 'secure_direct.requested', 'variant_batch.failed', 'vip.disabled', 'vip.enabled', 'withdrawal.paid', 'withdrawal.processing', 'withdrawal.rejected', 'withdrawal.requested'])::text[])));
