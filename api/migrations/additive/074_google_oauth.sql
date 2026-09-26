-- Google OAuth account linking (additive, non-destructive)
-- - user_oauth_accounts: provider identity (Google sub) -> existing user
-- - UNIQUE(provider, provider_account_id) prevents duplicate Google claim
-- - No DROP/DELETE/ALTER existing tables; users table untouched.
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/074_google_oauth.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.user_oauth_accounts (
    id character(36) NOT NULL PRIMARY KEY,
    user_id character(36) NOT NULL,
    provider character varying(20) NOT NULL DEFAULT 'google',
    provider_account_id character varying(200) NOT NULL,
    email character varying(320),
    display_name character varying(120),
    avatar_url character varying(500),
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    updated_at timestamp with time zone NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_oauth_provider_account
    ON public.user_oauth_accounts (provider, provider_account_id);

CREATE INDEX IF NOT EXISTS idx_oauth_user
    ON public.user_oauth_accounts (user_id);

CREATE INDEX IF NOT EXISTS idx_oauth_email
    ON public.user_oauth_accounts (email);

COMMIT;