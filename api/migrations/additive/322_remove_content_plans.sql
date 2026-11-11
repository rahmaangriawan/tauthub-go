-- 322 — Hapus fitur Rencana Bulanan. Tabel content_plans dan content_plan_items
-- dibuang (migrasi 318), dan daftar tipe notifikasi dikembalikan tanpa plan.*
-- (migrasi 319). Migrasi 318 dan 319 tetap ada sebagai riwayat. Additive dan idempoten.

BEGIN;

DROP TABLE IF EXISTS public.content_plan_items;
DROP TABLE IF EXISTS public.content_plans;

CREATE TABLE IF NOT EXISTS public.brand_invitations (
  id character(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
  brand_id character(36) NOT NULL REFERENCES public.brands(id) ON DELETE CASCADE,
  email varchar(255) NOT NULL,
  role varchar(20) NOT NULL,
  token_hash varchar(64) NOT NULL,
  invited_by character(36) NOT NULL REFERENCES public.users(id),
  expires_at timestamptz NOT NULL,
  accepted_at timestamptz,
  accepted_by character(36) REFERENCES public.users(id),
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_brand_invitations_token ON public.brand_invitations (token_hash);
CREATE UNIQUE INDEX IF NOT EXISTS uq_brand_invitations_open
  ON public.brand_invitations (brand_id, lower(email))
  WHERE accepted_at IS NULL AND revoked_at IS NULL;

DELETE FROM public.notifications WHERE type LIKE 'plan.%';

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
    'publication.waiting_partner',
    'referral.commission.credited', 'referral.commission.reversed',
    'referral.cpa.credited', 'referral.domain.approved', 'referral.domain.rejected',
    'secure_direct.approved', 'secure_direct.requested',
    'seller.order_expiring', 'seller.traffic_drop',
    'ticket.closed', 'ticket.created', 'ticket.replied',
    'variant_batch.failed', 'vip.disabled', 'vip.enabled',
    'withdrawal.paid', 'withdrawal.processing', 'withdrawal.rejected',
    'withdrawal.requested'
  ])::text[])));

COMMIT;
