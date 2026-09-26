-- 112 -- Kontak notifikasi partner: email tambahan (maks 2) + nomor WA + consent.

ALTER TABLE public.notification_preferences
  ADD COLUMN IF NOT EXISTS email_alt1 character varying(320),
  ADD COLUMN IF NOT EXISTS email_alt2 character varying(320),
  ADD COLUMN IF NOT EXISTS wa_number character varying(30),
  ADD COLUMN IF NOT EXISTS wa_consent boolean NOT NULL DEFAULT false;
