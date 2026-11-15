-- 331 — (1) Jejak asal "Ulangi kampanye": campaigns.duplicated_from_campaign_id,
-- dipakai Dapur Produksi kampanye baru untuk menampilkan artikel kampanye lama
-- sebagai rujukan baca-saja. (2) Tipe notifikasi email.bounce_spike: peringatan
-- ke admin saat bounce email melonjak. Additive dan idempoten.

BEGIN;

ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS duplicated_from_campaign_id character(36);

ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type
  CHECK (((type)::text = ANY ((ARRAY[
    'announcement.new', 'billing.balance_forfeited', 'billing.balance_forfeit_warning',
    'billing.balance_low', 'billing.topup_needed', 'changelog.new', 'chat.message',
    'media.approved', 'media.crawl_blocked', 'media.dewan_pers.approved',
    'media.dewan_pers.rejected', 'media.dewan_pers.submitted', 'media.domain_down',
    'media.integration_missing',
    'media.negotiated', 'media.rejected', 'media.resubmitted', 'media.submitted',
    'partner.guarantee_reversed', 'partner.wa_required',
    'pr.score_updated', 'pr.verification_completed',
    'publication.auto_replaced', 'publication.cancelled_by_admin',
    'publication.client_approved', 'publication.failed',
    'publication.guarantee_update', 'publication.guarantee_triggered',
    'publication.late_approval', 'publication.link_lost',
    'publication.link_replacement_requested', 'publication.partner_approved',
    'publication.partner_rejected', 'publication.published', 'publication.reminder',
    'publication.revision_accepted', 'publication.revision_rejected',
    'publication.revision_requested', 'publication.sla_cancelled',
    'publication.sla_extended', 'publication.url_requested',
    'publication.waiting_partner', 'publication.waiting_long', 'writer.batch_done', 'email.bounce_spike',
    'referral.commission.credited', 'referral.commission.reversed',
    'referral.cpa.credited', 'referral.domain.approved', 'referral.domain.rejected',
    'report.monthly',
    'secure_direct.approved', 'secure_direct.requested',
    'seller.order_expiring', 'seller.traffic_drop',
    'ticket.closed', 'ticket.created', 'ticket.replied',
    'variant_batch.failed', 'vip.disabled', 'vip.enabled',
    'withdrawal.paid', 'withdrawal.processing', 'withdrawal.rejected',
    'withdrawal.requested'
  ])::text[])));

COMMIT;
