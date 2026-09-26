-- 098 — Riwayat Cek DA/PA mitra (Tool SEO). Juga dasar rate limit 24 jam/domain.
-- Additive, backward compatible.

BEGIN;

CREATE TABLE IF NOT EXISTS public.partner_seo_check_history (
  id character(36) PRIMARY KEY,
  partner_id character(36) NOT NULL REFERENCES public.partners(id) ON DELETE CASCADE,
  domain varchar(190) NOT NULL,
  da smallint,
  pa smallint,
  spam_score smallint,
  checked_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_partner_seo_history_partner_domain
  ON public.partner_seo_check_history(partner_id, domain, checked_at DESC);

CREATE INDEX IF NOT EXISTS ix_partner_seo_history_partner_time
  ON public.partner_seo_check_history(partner_id, checked_at DESC);

GRANT SELECT, INSERT, DELETE ON public.partner_seo_check_history TO ai_writer;

COMMIT;