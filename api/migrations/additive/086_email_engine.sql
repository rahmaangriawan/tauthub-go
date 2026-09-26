-- 086 — Email engine: SMTP settings (DB-only, encrypted) + email_deliveries (queue/log) +
-- notification_preferences. Additive, backward compatible. No DROP/TRUNCATE/DELETE.

BEGIN;

CREATE TABLE IF NOT EXISTS public.smtp_settings (
  id character varying(20) PRIMARY KEY DEFAULT 'default',
  provider character varying(20) NOT NULL DEFAULT 'smtp',
  host character varying(200),
  port integer NOT NULL DEFAULT 587,
  secure boolean NOT NULL DEFAULT false,
  username character varying(200),
  password_sealed character varying(2000),
  from_email character varying(200) NOT NULL DEFAULT 'no-reply@tauthub.com',
  from_name character varying(120) NOT NULL DEFAULT 'Tauthub',
  enabled boolean NOT NULL DEFAULT false,
  last_test_at timestamp with time zone,
  last_test_ok boolean,
  last_test_message character varying(500),
  updated_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_by character(36)
);

DO $$
BEGIN
  ALTER TABLE public.smtp_settings ADD CONSTRAINT fk_smtp_settings_updated_by
    FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$
BEGIN
  ALTER TABLE public.smtp_settings ADD CONSTRAINT ck_smtp_provider
    CHECK (provider IN ('smtp'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$
BEGIN
  ALTER TABLE public.smtp_settings ADD CONSTRAINT ck_smtp_port
    CHECK (port IS NULL OR (port > 0 AND port < 65536));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

INSERT INTO public.smtp_settings (id, provider, port, from_email, from_name, enabled)
VALUES ('default', 'smtp', 587, 'no-reply@tauthub.com', 'Tauthub', false)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS public.email_deliveries (
  id character(36) PRIMARY KEY,
  template character varying(120) NOT NULL,
  recipient_email character varying(200) NOT NULL,
  user_id character(36),
  reference_id character(36),
  event character varying(60),
  status character varying(20) NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','sending','sent','failed','rate_limited','skipped')),
  attempts integer NOT NULL DEFAULT 0,
  max_attempts integer NOT NULL DEFAULT 3,
  next_attempt_at timestamp with time zone NOT NULL DEFAULT now(),
  provider character varying(20) NOT NULL DEFAULT 'smtp',
  provider_message_id character varying(200),
  error_code character varying(40),
  error_message character varying(500),
  sent_at timestamp with time zone,
  created_at timestamp with time zone NOT NULL DEFAULT now()
);

DO $$
BEGIN
  ALTER TABLE public.email_deliveries ADD CONSTRAINT fk_email_deliveries_user
    FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE INDEX IF NOT EXISTS ix_email_deliveries_queue
  ON public.email_deliveries (status, next_attempt_at) WHERE status IN ('queued','failed');
CREATE INDEX IF NOT EXISTS ix_email_deliveries_user
  ON public.email_deliveries (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_email_deliveries_reference
  ON public.email_deliveries (reference_id);

CREATE TABLE IF NOT EXISTS public.notification_preferences (
  user_id character(36) PRIMARY KEY REFERENCES public.users(id) ON DELETE CASCADE,
  email_security boolean NOT NULL DEFAULT true,
  email_payments boolean NOT NULL DEFAULT true,
  email_campaign boolean NOT NULL DEFAULT true,
  email_partner boolean NOT NULL DEFAULT true,
  email_marketing boolean NOT NULL DEFAULT false,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_at timestamp with time zone NOT NULL DEFAULT now()
);

COMMIT;
