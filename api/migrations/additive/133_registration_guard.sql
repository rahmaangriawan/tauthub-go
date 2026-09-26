-- 133 — Registration Abuse Guard: kolom sumber registrasi + riwayat pendaftaran
-- (hash, tanpa PII) + tabel config singleton. Additive, style 132.
-- - users.registration_source: asal pembuatan akun ('public'|'admin'|'google').
-- - registration_history: sinyal anti-abuse (email/sub/ip/device ter-hash HMAC).
-- - registration_guard_config: skor & threshold guard (baris tunggal id=1).

BEGIN;

ALTER TABLE public.users
  ADD COLUMN IF NOT EXISTS registration_source varchar(20) NOT NULL DEFAULT 'public';

-- Tempat reason_codes/risk untuk audit registration_blocked (spec C).
ALTER TABLE public.audit_logs
  ADD COLUMN IF NOT EXISTS detail jsonb;

-- Constraint CHECK ditambah kondisional (idempotent, tidak ada ADD CONSTRAINT IF NOT EXISTS).
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'users_registration_source_check'
      AND conrelid = 'public.users'::regclass
  ) THEN
    ALTER TABLE public.users
      ADD CONSTRAINT users_registration_source_check
      CHECK (registration_source IN ('public', 'admin', 'google'));
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS public.registration_history (
  id                character(36) NOT NULL,
  user_id           character(36) REFERENCES public.users (id) ON DELETE SET NULL,
  registration_source varchar(20) NOT NULL,
  email_hash        varchar(64),
  google_sub_hash   varchar(64),
  ip_hash           varchar(64),
  device_hash       varchar(64),
  domain            varchar(253),
  registered_at     timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
  risk_score        integer NOT NULL DEFAULT 0,
  risk_level        varchar(20) NOT NULL DEFAULT 'low',
  reason_codes      jsonb NOT NULL DEFAULT '[]'::jsonb,
  CONSTRAINT registration_history_pkey PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS ix_registration_history_email_hash
  ON public.registration_history (email_hash);

CREATE INDEX IF NOT EXISTS ix_registration_history_google_sub_hash
  ON public.registration_history (google_sub_hash);

CREATE INDEX IF NOT EXISTS ix_registration_history_ip_hash_time
  ON public.registration_history (ip_hash, registered_at DESC);

CREATE INDEX IF NOT EXISTS ix_registration_history_device_hash_time
  ON public.registration_history (device_hash, registered_at DESC);

GRANT SELECT, INSERT ON public.registration_history TO ai_writer;

CREATE TABLE IF NOT EXISTS public.registration_guard_config (
  id          integer NOT NULL DEFAULT 1,
  scores      jsonb NOT NULL,
  thresholds jsonb NOT NULL,
  rate_limits jsonb NOT NULL,
  updated_at  timestamp with time zone NOT NULL DEFAULT NOW(),
  CONSTRAINT registration_guard_config_pkey PRIMARY KEY (id),
  CONSTRAINT registration_guard_config_id_check CHECK (id = 1)
);

INSERT INTO public.registration_guard_config (id, scores, thresholds, rate_limits)
VALUES (
  1,
  '{"disposable":50,"known_abusive_domain":70,"very_new_domain":20,"workspace":10,"ip_many_registrations":20,"device_many_registrations":25,"recent_registration":100,"suspicious_pattern":30,"previously_blocked":70,"existing_google_sub":100,"mx_missing":15}'::jsonb,
  '{"verify":30,"review":60,"block":80,"critical":110}'::jsonb,
  '{"register_email_per_hour":5,"register_device_per_hour":10,"register_ip_per_minute":10,"google_callback_ip_per_minute":30}'::jsonb
)
ON CONFLICT (id) DO NOTHING;

GRANT SELECT ON public.registration_guard_config TO ai_writer;

COMMIT;
