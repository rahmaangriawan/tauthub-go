-- Google Sign-In OAuth config stored in DB (admin-managed).
-- Separate from Google Search Console config (GSC keeps its own tables).
-- Additive only: no DROP/DELETE/ALTER existing tables.
-- secret is sealed (sealSecret + APP_SECRET) — never plaintext.
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/075_oauth_config.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.platform_oauth_configs (
    id character varying(20) NOT NULL DEFAULT 'google' PRIMARY KEY,
    provider character varying(20) NOT NULL DEFAULT 'google',
    client_id character varying(500),
    sealed_client_secret text,
    redirect_uri character varying(500),
    updated_by character(36),
    updated_at timestamp with time zone NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_platform_oauth_id CHECK (id = 'google')
);

COMMIT;