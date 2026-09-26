-- 093 — Partner speed audits (PageSpeed) for Tool SEO Kecepatan tab.
-- Additive only. Stores minimal normalized PageSpeed results per partner + URL.
-- No raw dump, no API key storage.
BEGIN;

CREATE TABLE IF NOT EXISTS partner_speed_audits (
  id                uuid PRIMARY KEY,
  partner_id        uuid NOT NULL,
  url               text NOT NULL,
  normalized_url    text NOT NULL,
  domain            text NOT NULL,
  strategy          text NOT NULL DEFAULT 'mobile',
  performance_score integer,
  accessibility_score integer,
  best_practices_score integer,
  seo_score         integer,
  lcp_ms            integer,
  inp_ms            integer,
  cls               numeric,
  audits            jsonb NOT NULL DEFAULT '[]',
  provider          text NOT NULL DEFAULT 'pagespeed',
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_psa_partner_url ON partner_speed_audits(partner_id, normalized_url);
CREATE INDEX IF NOT EXISTS idx_psa_created ON partner_speed_audits(partner_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_psa_domain ON partner_speed_audits(domain);

COMMIT;
