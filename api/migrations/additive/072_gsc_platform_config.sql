-- 072 — Platform GSC OAuth config (admin dashboard, DB-first, ENV fallback)
--
-- Tujuan: admin isi GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / REDIRECT_URI
-- lewat UI / admin dashboard; ENV (GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET,
-- GOOGLE_REDIRECT_URI / NEXT_PUBLIC_APP_URL) hanya fallback.
-- Semua credential di-seal server-side (APP_SECRET), tidak plaintext.
--
-- Idempotent: CREATE TABLE IF NOT EXISTS + DO block untuk singleton check.

CREATE TABLE IF NOT EXISTS public.platform_gsc_config (
  id                text NOT NULL DEFAULT 'default',
  client_id         text,
  sealed_client_secret text,
  redirect_uri      text,
  updated_by        character(36),
  updated_at        timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT platform_gsc_config_pkey PRIMARY KEY (id),
  CONSTRAINT ck_platform_gsc_config_id CHECK (id = 'default')
);

-- GSC OAuth state untuk CSRF (authorization code flow)
CREATE TABLE IF NOT EXISTS public.gsc_oauth_states (
  id          character(36) NOT NULL,
  partner_id  character(36) NOT NULL,
  state       character varying(128) NOT NULL,
  code_verifier character varying(128),
  created_at  timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT gsc_oauth_states_pkey PRIMARY KEY (id),
  CONSTRAINT uq_gsc_oauth_state UNIQUE (state),
  CONSTRAINT fk_gsc_oauth_states_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_gsc_oauth_states_created ON public.gsc_oauth_states USING btree (created_at DESC);

-- Per-partner selected property untuk kalkulator (persist pilihan)
ALTER TABLE public.gsc_connections ADD COLUMN IF NOT EXISTS selected_property_id character(36);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_gsc_connections_selected_property') THEN
    ALTER TABLE public.gsc_connections ADD CONSTRAINT fk_gsc_connections_selected_property
      FOREIGN KEY (selected_property_id) REFERENCES public.gsc_properties(id) ON DELETE SET NULL;
  END IF;
END $$;
