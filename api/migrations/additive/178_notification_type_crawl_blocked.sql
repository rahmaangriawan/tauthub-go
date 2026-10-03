-- 178_notification_type_crawl_blocked.sql
-- Tipe notifikasi baru media.crawl_blocked: mitra pemilik media diberi tahu
-- bila situsnya tidak bisa di-crawl Tauthub (gagal >= 3x berturut-turut ATAU
-- diblokir WAF / HTTP 403), supaya mengizinkan akses crawler atau memasang
-- verifikasi domain (meta tag / TXT record).
--
-- Idempoten: DROP IF EXISTS lalu ADD dengan daftar lengkap + tipe baru.

ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type CHECK (
    type = ANY (ARRAY[
        'announcement.new',
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
