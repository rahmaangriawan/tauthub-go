-- 094 — Platform Pagespeed config for Tool SEO Kecepatan tab.
-- Stores Google PageSpeed API key sealed, similar to platform_seo_config.
BEGIN;

CREATE TABLE IF NOT EXISTS platform_pagespeed_config (
  id              text PRIMARY KEY,
  sealed_api_key  text,
  api_key_hint    text,
  updated_by      uuid,
  updated_at      timestamptz NOT NULL DEFAULT now()
);

-- Ensure default row exists conceptually (app will INSERT ON CONFLICT)
-- No seed data required; GET returns has_key false if no row.

COMMIT;
