-- 302 — Verifikasi email sebelum akun dibuat (kode OTP 6 digit), blokir email
-- yang tidak menyelesaikan OTP dalam 48 jam, dan jejak perangkat sesi login
-- untuk notifikasi "login baru". Additive.

BEGIN;

-- Kode OTP pendaftaran. Tanpa FK ke users: baris users belum ada saat kode
-- dibuat. code_hash = sha256(pepper || email || kode); ticket_hash dibuat saat
-- kode benar dan ditukar sekali oleh /auth/register.
CREATE TABLE IF NOT EXISTS public.email_verification_codes (
  id           character(36) PRIMARY KEY,
  email        text NOT NULL,
  code_hash    character(64) NOT NULL,
  purpose      text NOT NULL DEFAULT 'register',
  attempts     smallint NOT NULL DEFAULT 0,
  ticket_hash  character(64),
  verified_at  timestamp with time zone,
  expires_at   timestamp with time zone NOT NULL,
  consumed_at  timestamp with time zone,
  request_ip   text,
  created_at   timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_email_verification_codes_email
  ON public.email_verification_codes (email, purpose, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_email_verification_codes_expires
  ON public.email_verification_codes (expires_at);

-- Email yang meminta kode tetapi tidak pernah terverifikasi dalam 48 jam.
-- Diisi sweep scheduler; admin bisa membuka blokir (unblocked_at).
CREATE TABLE IF NOT EXISTS public.registration_email_blocks (
  email        text PRIMARY KEY,
  reason       text NOT NULL DEFAULT 'otp_unanswered_48h',
  blocked_at   timestamp with time zone NOT NULL DEFAULT now(),
  unblocked_at timestamp with time zone,
  unblocked_by character(36)
);

-- Perangkat sesi login: dipakai email/WA "login baru ke akun Anda".
ALTER TABLE public.user_sessions ADD COLUMN IF NOT EXISTS ip character varying(64);
ALTER TABLE public.user_sessions ADD COLUMN IF NOT EXISTS user_agent text;

COMMIT;
