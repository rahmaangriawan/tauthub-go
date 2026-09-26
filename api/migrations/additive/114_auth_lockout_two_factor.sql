-- 114 -- Hardening auth: lockout percobaan login gagal + 2FA TOTP.
-- Lockout: counter per user, reset saat sukses; lock 15 menit setelah 5 gagal.
-- 2FA: secret TOTP ter-seal (sealSecretV3 per-user) + recovery codes ter-hash.

ALTER TABLE public.users
  ADD COLUMN IF NOT EXISTS failed_login_count integer NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS locked_until timestamptz,
  ADD COLUMN IF NOT EXISTS two_factor_secret text,
  ADD COLUMN IF NOT EXISTS two_factor_enabled boolean NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS two_factor_recovery_codes jsonb;
