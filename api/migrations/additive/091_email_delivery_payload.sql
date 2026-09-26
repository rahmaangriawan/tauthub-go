-- 091 — Email delivery payload + dedupe key. Additive only.
-- Needed by the queued email worker; no existing rows are deleted.

BEGIN;

ALTER TABLE public.email_deliveries ADD COLUMN IF NOT EXISTS subject character varying(500);
ALTER TABLE public.email_deliveries ADD COLUMN IF NOT EXISTS html text;
ALTER TABLE public.email_deliveries ADD COLUMN IF NOT EXISTS dedupe_key character varying(255);

CREATE INDEX IF NOT EXISTS ix_email_deliveries_dedupe
  ON public.email_deliveries (template, user_id, dedupe_key, created_at DESC);

COMMIT;
