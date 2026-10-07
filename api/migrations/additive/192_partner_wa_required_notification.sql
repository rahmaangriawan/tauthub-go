-- 192_partner_wa_required_notification.sql
-- Tipe notifikasi baru: partner.wa_required — pengingat admin ke mitra yang
-- punya lebih dari 2 media tetapi belum mengisi nomor WhatsApp (dipakai aksi
-- "Kirim pengingat isi WA" di /admin/partners).
-- Idempoten: DROP IF EXISTS lalu ADD dengan daftar lengkap (migrasi 190) + tipe baru.

ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type CHECK (
    type = ANY (ARRAY[
        'announcement.new',
        'changelog.new',
        'media.approved',
        'media.crawl_blocked',
        'media.dewan_pers.approved',
        'media.dewan_pers.rejected',
        'media.dewan_pers.submitted',
        'media.domain_down',
        'media.negotiated',
        'media.rejected',
        'media.resubmitted',
        'media.submitted',
        'partner.wa_required',
        'pr.score_updated',
        'pr.verification_completed',
        'publication.auto_replaced',
        'publication.client_approved',
        'publication.failed',
        'publication.late_approval',
        'publication.link_lost',
        'publication.link_replacement_requested',
        'publication.partner_approved',
        'publication.partner_rejected',
        'publication.published',
        'publication.reminder',
        'publication.revision_accepted',
        'publication.revision_rejected',
        'publication.revision_requested',
        'publication.url_requested',
        'publication.waiting_partner',
        'referral.commission.credited',
        'referral.commission.reversed',
        'referral.cpa.credited',
        'referral.domain.approved',
        'referral.domain.rejected',
        'variant_batch.failed',
        'vip.disabled',
        'vip.enabled',
        'withdrawal.paid',
        'withdrawal.processing',
        'withdrawal.rejected',
        'withdrawal.requested'
    ]::varchar[])
);
