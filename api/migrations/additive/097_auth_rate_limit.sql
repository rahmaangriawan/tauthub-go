-- 097 — Auth rate limit (DB-backed, tahan restart & multi-instance).
-- Bucket contoh: login:ip:{ip}, login:email:{email}, register:ip:{ip}, forgot:ip:{ip}.
-- Sliding 1 menit: count di-reset otomatis saat window_start kedaluwarsa.

BEGIN;

CREATE TABLE IF NOT EXISTS public.auth_rate_limits (
  id character(36) PRIMARY KEY,
  bucket varchar(120) NOT NULL,
  count integer NOT NULL DEFAULT 0,
  window_start timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_auth_rate_limits_bucket
  ON public.auth_rate_limits(bucket);

CREATE INDEX IF NOT EXISTS ix_auth_rate_limits_window
  ON public.auth_rate_limits(window_start);

GRANT SELECT, INSERT, UPDATE, DELETE ON public.auth_rate_limits TO ai_writer;

COMMIT;