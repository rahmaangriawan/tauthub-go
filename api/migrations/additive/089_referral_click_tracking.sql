-- 089 — Referral click tracking — additive, backward compatible.
-- No DROP/TRUNCATE/DELETE. Explain attribution by clicks, not just registration header.

BEGIN;

CREATE TABLE IF NOT EXISTS public.referral_clicks (
  id character(36) PRIMARY KEY,
  code character varying(32) NOT NULL,
  referrer_partner_id character(36) NOT NULL REFERENCES public.partners(id) ON DELETE CASCADE,
  fingerprint character varying(200) NOT NULL,
  click_count integer NOT NULL DEFAULT 1,
  first_clicked_at timestamp with time zone NOT NULL DEFAULT now(),
  last_clicked_at timestamp with time zone NOT NULL DEFAULT now(),
  last_ip character varying(64),
  last_user_agent character varying(500),
  UNIQUE (code, fingerprint)
);

CREATE INDEX IF NOT EXISTS ix_referral_clicks_referrer ON public.referral_clicks(referrer_partner_id, last_clicked_at DESC);
CREATE INDEX IF NOT EXISTS ix_referral_clicks_code ON public.referral_clicks(code, last_clicked_at DESC);

ALTER TABLE public.referral_attributions ADD COLUMN IF NOT EXISTS click_id character(36) REFERENCES public.referral_clicks(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS ix_referral_attributions_click ON public.referral_attributions(click_id);

ALTER TABLE public.referral_attributions ADD COLUMN IF NOT EXISTS referer character varying(1000);

COMMIT;
