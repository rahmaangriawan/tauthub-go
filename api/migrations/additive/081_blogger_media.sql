-- 081 — Integrasi Google Blogger sebagai platform media (provider='blogger').
-- Additive: kolom baru di media + perluasan ck_media_integration_type.
-- Tidak DROP TABLE, tidak destructive.

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS blogger_credentials_sealed text;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS blogger_blog_id character varying(200);
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS blogger_blog_name character varying(300);
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS blogger_status character varying(20) NOT NULL DEFAULT 'disconnected';
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS blogger_last_test_at timestamp with time zone;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS blogger_last_test_message character varying(500);

ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_integration_type;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_integration_type
    CHECK (integration_type IN ('manual','wordpress','rss','webhook','tulisio_api','custom_web','blogger'));

-- State OAuth Blogger (mirror gsc_oauth_states; satu tabel untuk semua media partner).
CREATE TABLE IF NOT EXISTS public.blogger_oauth_states (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    state character varying(200) NOT NULL,
    code_verifier character varying(200),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT blogger_oauth_states_pkey PRIMARY KEY (id),
    CONSTRAINT uq_blogger_oauth_states_state UNIQUE (state),
    CONSTRAINT fk_blogger_oauth_states_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE,
    CONSTRAINT fk_blogger_oauth_states_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_blogger_oauth_states_created ON public.blogger_oauth_states USING btree (created_at DESC);