-- 096 — Cloudflare Turnstile config (login/register/forgot-password security).
-- Additive, backward compatible. site_key publik; secret key disegel server-side.

BEGIN;

CREATE TABLE IF NOT EXISTS public.platform_turnstile_config (
  id character(36) PRIMARY KEY,
  site_key varchar(512) NOT NULL,
  sealed_secret_key text NOT NULL,
  enabled boolean NOT NULL DEFAULT FALSE,
  updated_by character(36),
  updated_at timestamptz NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT, UPDATE ON public.platform_turnstile_config TO ai_writer;

COMMIT;