-- 086 - Program Referral Mitra (direct 1%, single-level).
-- Additive: ADD COLUMN / CREATE TABLE IF NOT EXISTS. No DROP/TRUNCATE/DELETE.
-- Apply: psql $DATABASE_URL -f apps/web/src/db/migrations/086_referral_program.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.referral_settings (
  key character varying(80) PRIMARY KEY,
  value character varying(200) NOT NULL,
  updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO public.referral_settings (key, value) VALUES ('commission_rate_bps','100')
ON CONFLICT (key) DO NOTHING;

ALTER TABLE public.users ADD COLUMN IF NOT EXISTS referred_by_partner_id character(36);
DO $$ BEGIN
  ALTER TABLE public.users ADD CONSTRAINT fk_users_referred_by_partner
    FOREIGN KEY (referred_by_partner_id) REFERENCES public.partners(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;
CREATE INDEX IF NOT EXISTS ix_users_referred_by_partner ON public.users(referred_by_partner_id);

ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS referral_code character varying(32);
ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS referral_code_normalized character varying(32);
DO $$ BEGIN
  CREATE UNIQUE INDEX uq_partners_referral_code_norm
    ON public.partners(referral_code_normalized) WHERE referral_code_normalized IS NOT NULL;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

UPDATE public.partners SET
  referral_code = public_id || '-' || substring(replace(id::text, '-', ''), 1, 4),
  referral_code_normalized = lower(public_id || '-' || substring(replace(id::text, '-', ''), 1, 4))
WHERE referral_code IS NULL;

ALTER TABLE public.partner_earnings ADD COLUMN IF NOT EXISTS kind character varying(20) DEFAULT 'publication' NOT NULL;
ALTER TABLE public.partner_earnings ADD COLUMN IF NOT EXISTS source_earning_id character(36);
DO $$ BEGIN
  CREATE INDEX ix_partner_earnings_kind ON public.partner_earnings(kind);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS public.partner_referral_commissions (
  id character(36) PRIMARY KEY,
  referrer_partner_id character(36) NOT NULL REFERENCES public.partners(id) ON DELETE CASCADE,
  referee_partner_id character(36) NOT NULL REFERENCES public.partners(id) ON DELETE CASCADE,
  referee_user_id character(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  earning_id character(36) NOT NULL REFERENCES public.partner_earnings(id) ON DELETE CASCADE,
  amount numeric(14,2) NOT NULL CHECK (amount >= 0),
  rate_bps integer NOT NULL CHECK (rate_bps >= 0 AND rate_bps <= 10000),
  status character varying(20) NOT NULL CHECK (status IN ('pending','available','paid','reversed','review_needed')),
  reversal_of character(36) REFERENCES public.partner_referral_commissions(id) ON DELETE SET NULL,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now(),
  paid_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_refcomm_earning_referrer
  ON public.partner_referral_commissions(earning_id, referrer_partner_id);
CREATE INDEX IF NOT EXISTS ix_refcomm_referrer_status
  ON public.partner_referral_commissions(referrer_partner_id, status);
CREATE INDEX IF NOT EXISTS ix_refcomm_referee
  ON public.partner_referral_commissions(referee_partner_id);
CREATE INDEX IF NOT EXISTS ix_refcomm_earning
  ON public.partner_referral_commissions(earning_id);

CREATE TABLE IF NOT EXISTS public.referral_attributions (
  id character(36) PRIMARY KEY,
  code character varying(32) NOT NULL,
  referrer_partner_id character(36) NOT NULL REFERENCES public.partners(id) ON DELETE CASCADE,
  referee_user_id character(36) REFERENCES public.users(id) ON DELETE SET NULL,
  ip inet,
  user_agent text,
  notes text,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_referral_attributions_code_time
  ON public.referral_attributions(code, created_at DESC);

COMMIT;
