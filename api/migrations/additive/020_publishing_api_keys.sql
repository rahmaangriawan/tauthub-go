-- Migration: 020_publishing_api_keys
-- Tulisio Publishing API: partner custom websites (adapters) authenticate with
-- an API key (bearer) + secret (HMAC signature). The raw key is shown once at
-- creation; only its sha256 hash is stored, while the secret is sealed
-- (encrypted at rest) so the server can recompute HMAC signatures.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/020_publishing_api_keys.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.publishing_api_keys (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    media_id character(36),
    label character varying(120) DEFAULT 'Default'::character varying NOT NULL,
    api_key_hash character(64) NOT NULL,
    api_key_sealed text NOT NULL,
    secret_sealed text NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    last_used_at timestamp(6) with time zone,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT publishing_api_keys_pkey PRIMARY KEY (id),
    CONSTRAINT uq_publishing_api_key_hash UNIQUE (api_key_hash),
    CONSTRAINT uq_publishing_api_key_partner_label UNIQUE (partner_id, label),
    CONSTRAINT ck_publishing_api_key_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'revoked'::character varying])::text[]))),
    CONSTRAINT fk_publishing_api_keys_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE,
    CONSTRAINT fk_publishing_api_keys_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE,
    CONSTRAINT fk_publishing_api_keys_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL
);

CREATE INDEX ix_publishing_api_keys_partner ON public.publishing_api_keys (partner_id);

COMMIT;
