-- 303 — Domain email yang diblokir dari pendaftaran (kelolaan admin, melengkapi
-- daftar temp-mail bawaan di internal/auth/disposable_domains.txt), index
-- hitungan pendaftaran per domain, dan batas akun per domain/hari. Additive.

BEGIN;

CREATE TABLE IF NOT EXISTS public.blocked_email_domains (
  domain     text PRIMARY KEY,
  reason     text NOT NULL DEFAULT 'manual',
  created_by character(36),
  created_at timestamp with time zone NOT NULL DEFAULT now()
);

-- Guard menghitung pendaftaran per domain dalam 24 jam (DOMAIN_MANY_REGISTRATIONS).
CREATE INDEX IF NOT EXISTS ix_registration_history_domain
  ON public.registration_history (domain, registered_at DESC);

-- Batas akun Google/Workspace per domain per hari (gmail.com dikecualikan).
UPDATE public.registration_guard_config
SET rate_limits = COALESCE(rate_limits, '{}'::jsonb) || '{"domain_per_day": 3}'::jsonb
WHERE id = 1 AND NOT (COALESCE(rate_limits, '{}'::jsonb) ? 'domain_per_day');

COMMIT;
