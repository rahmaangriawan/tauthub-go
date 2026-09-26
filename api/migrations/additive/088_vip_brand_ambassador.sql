-- 088 — Partner VIP / Brand Ambassador — additive, backward compatible.
-- No DROP/TRUNCATE/DELETE. All ALTERs are IF NOT EXISTS.

BEGIN;

ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS is_vip boolean NOT NULL DEFAULT false;

ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS vip_tier character varying(32) NOT NULL DEFAULT 'regular'
  CHECK (vip_tier IN ('regular','brand_ambassador'));

ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS vip_enabled_at timestamp with time zone;

CREATE INDEX IF NOT EXISTS ix_partners_is_vip ON public.partners(is_vip) WHERE is_vip = true;

CREATE TABLE IF NOT EXISTS public.affiliate_settings (
  partner_id character(36) PRIMARY KEY REFERENCES public.partners(id) ON DELETE CASCADE,
  commission_rate_bp integer NOT NULL DEFAULT 100
    CHECK (commission_rate_bp >= 0 AND commission_rate_bp <= 10000),
  cpa_rate integer NOT NULL DEFAULT 0
    CHECK (cpa_rate >= 0),
  entitlements jsonb NOT NULL DEFAULT '{"client_workspace":true,"ai_premium":true,"seo_tools":true,"keyword_research":true}'::jsonb,
  updated_by character(36) REFERENCES public.users(id) ON DELETE SET NULL,
  updated_at timestamp with time zone NOT NULL DEFAULT now()
);

GRANT SELECT ON public.affiliate_settings TO ai_writer;

CREATE TABLE IF NOT EXISTS public.referred_domains (
  id character(36) PRIMARY KEY,
  referrer_partner_id character(36) NOT NULL REFERENCES public.partners(id) ON DELETE CASCADE,
  owner_user_id character(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  media_id character(36) UNIQUE REFERENCES public.media(id) ON DELETE CASCADE,
  domain_name character varying(255) NOT NULL,
  domain_age_months integer,
  is_tld boolean NOT NULL DEFAULT true,
  status character varying(20) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','approved','rejected')),
  rejection_reason text,
  cpa_paid boolean NOT NULL DEFAULT false,
  cpa_amount integer,
  created_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_referred_domains_referrer ON public.referred_domains(referrer_partner_id, status);
CREATE INDEX IF NOT EXISTS ix_referred_domains_owner ON public.referred_domains(owner_user_id);
CREATE INDEX IF NOT EXISTS ix_referred_domains_media ON public.referred_domains(media_id) WHERE media_id IS NOT NULL;

DO $$
BEGIN
  ALTER TABLE public.partner_earnings DROP CONSTRAINT IF EXISTS ck_partner_earnings_kind;
EXCEPTION WHEN others THEN NULL;
END $$;

ALTER TABLE public.partner_earnings ADD COLUMN IF NOT EXISTS kind character varying(30) NOT NULL DEFAULT 'publication';
ALTER TABLE public.partner_earnings ADD COLUMN IF NOT EXISTS source_earning_id character(36);
ALTER TABLE public.partner_earnings ADD COLUMN IF NOT EXISTS source_referred_domain_id character(36);

DO $$
BEGIN
  ALTER TABLE public.partner_earnings ADD CONSTRAINT ck_partner_earnings_kind
    CHECK (kind IN ('publication','referral','cpa','transaction_commission'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS ix_partner_earnings_kind ON public.partner_earnings(kind);

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
    'withdrawal.rejected',
    'referral.domain.approved',
    'referral.domain.rejected',
    'referral.cpa.credited',
    'referral.commission.credited',
    'referral.commission.reversed',
    'vip.enabled',
    'vip.disabled'
  ));

COMMIT;
