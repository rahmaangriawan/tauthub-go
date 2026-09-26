-- 080 — Tulisio Publisher Protocol v1 (Custom Web distribution).
-- Additive: new columns on media + extend integration_type CHECK.
-- No DROP TABLE, no destructive migration.

-- 1. New columns for Custom Web configuration.
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS custom_web_endpoint_url character varying(500);
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS custom_web_api_key_sealed text;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS custom_web_status character varying(20) NOT NULL DEFAULT 'disconnected';
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS custom_web_last_test_at timestamp with time zone;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS custom_web_last_test_message character varying(500);

-- 2. Extend integration_type CHECK to include 'custom_web'.
ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_integration_type;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_integration_type
    CHECK (integration_type IN ('manual','wordpress','rss','webhook','tulisio_api','custom_web'));
