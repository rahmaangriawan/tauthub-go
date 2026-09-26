-- =====================================================================
-- Migration 001 - Publication Domain (Phase 1)
--
-- Additive migration for EXISTING databases. Unlike `schema.pg.sql`
-- (which is applied by `bun run migrate` and DROPS all tables), this
-- file only ADDS the Phase 1 publication tables. Safe to run against
-- production PostgreSQL via psql or the Neon SQL editor.
--
-- Apply with:
--   psql "$DATABASE_URL" -f apps/web/src/db/migrations/001_publication_domain.sql
--
-- Idempotent: re-running is a no-op (CREATE TABLE IF NOT EXISTS /
-- CREATE INDEX IF NOT EXISTS). All constraints are defined inline so
-- no ALTER TABLE ADD CONSTRAINT is needed (PostgreSQL has no
-- `ADD CONSTRAINT IF NOT EXISTS`).
-- =====================================================================

CREATE TABLE IF NOT EXISTS public.media (
    id character(36) NOT NULL,
    name character varying(160) NOT NULL,
    slug character varying(160),
    category character varying(120),
    description text,
    website_url character varying(500),
    price numeric(14,2) DEFAULT 0 NOT NULL,
    status character varying(20) DEFAULT 'active' NOT NULL,
    integration_type character varying(20) DEFAULT 'manual' NOT NULL,
    notes text,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT media_pkey PRIMARY KEY (id),
    CONSTRAINT uq_media_slug UNIQUE (slug),
    CONSTRAINT fk_media_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_media_status CHECK (status IN ('draft','active','inactive','archived','hidden')),
    CONSTRAINT ck_media_integration_type CHECK (integration_type IN ('manual','wordpress','rss','webhook'))
);

CREATE TABLE IF NOT EXISTS public.publications (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    document_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    status character varying(20) DEFAULT 'pending' NOT NULL,
    client_price numeric(14,2) DEFAULT 0 NOT NULL,
    scheduled_at timestamp(6) with time zone,
    published_at timestamp(6) with time zone,
    published_url character varying(500),
    notes character varying(1000),
    cancel_reason character varying(500),
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT publications_pkey PRIMARY KEY (id),
    CONSTRAINT uq_publication_document_media UNIQUE (document_id, media_id),
    CONSTRAINT fk_publications_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id),
    CONSTRAINT fk_publications_document FOREIGN KEY (document_id) REFERENCES public.content_documents(id),
    CONSTRAINT fk_publications_media FOREIGN KEY (media_id) REFERENCES public.media(id),
    CONSTRAINT fk_publications_creator FOREIGN KEY (created_by) REFERENCES public.users(id),
    CONSTRAINT ck_publication_status CHECK (status IN ('pending','waiting_partner','partner_approved','partner_rejected','queued','processing','published','failed','retrying','cancelled'))
);

CREATE TABLE IF NOT EXISTS public.publication_attempts (
    id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    attempt_number integer NOT NULL,
    status character varying(20) NOT NULL,
    error_code character varying(80),
    error_message character varying(500),
    started_at timestamp(6) with time zone,
    completed_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT publication_attempts_pkey PRIMARY KEY (id),
    CONSTRAINT uq_publication_attempt_number UNIQUE (publication_id, attempt_number),
    CONSTRAINT fk_publication_attempts_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_media_status ON public.media USING btree (status, name);

CREATE INDEX IF NOT EXISTS ix_publication_brand_created ON public.publications USING btree (brand_id, created_at);

CREATE INDEX IF NOT EXISTS ix_publication_brand_status ON public.publications USING btree (brand_id, status);

CREATE INDEX IF NOT EXISTS ix_publication_media_status ON public.publications USING btree (media_id, status);

CREATE INDEX IF NOT EXISTS ix_publication_attempt_publication ON public.publication_attempts USING btree (publication_id, attempt_number);
