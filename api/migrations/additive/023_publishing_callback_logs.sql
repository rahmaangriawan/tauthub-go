-- Migration: 023_publishing_callback_logs
-- Audit + rate limiting for the Publishing API callback endpoint
-- (/v1/publishing/callbacks). Every callback attempt is recorded; the table
-- doubles as the rate-limit counter (per API key and per publication) and as
-- an audit trail for adapter activity.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/023_publishing_callback_logs.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.publishing_callback_logs (
    id character(36) NOT NULL,
    api_key_id character(36),
    partner_id character(36),
    publication_id character(36),
    ip_address character varying(64),
    user_agent character varying(300),
    status_code integer,
    accepted boolean,
    error_message character varying(500),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT publishing_callback_logs_pkey PRIMARY KEY (id),
    CONSTRAINT fk_publishing_callback_logs_key FOREIGN KEY (api_key_id) REFERENCES public.publishing_api_keys(id) ON DELETE SET NULL,
    CONSTRAINT fk_publishing_callback_logs_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS ix_callback_logs_key_time ON public.publishing_callback_logs (api_key_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_callback_logs_pub_time ON public.publishing_callback_logs (publication_id, created_at DESC);

COMMIT;
