-- Migration: 024_publication_variants
-- Auditable multi-variant publication: one source (bahan/article) → N articles,
-- each with a distinct randomized angle (seeded PRNG). The batch row records
-- the seed + mode; each variant row tracks which angle went to which media and
-- the resulting document + publication, so client and admin can audit
-- "artikel mana memakai sudut apa". Generation runs in the job queue worker
-- (job kind `publication_variant_generation`).
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/024_publication_variants.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.publication_variant_batches (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    mode character varying(20) NOT NULL,
    seed integer NOT NULL,
    status character varying(20) DEFAULT 'queued'::character varying NOT NULL,
    source_document_id character(36),
    source_material text,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    completed_at timestamp(6) with time zone,
    CONSTRAINT publication_variant_batches_pkey PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS ix_variant_batch_brand_time ON public.publication_variant_batches (brand_id, created_at DESC);

CREATE TABLE IF NOT EXISTS public.publication_variants (
    id character(36) NOT NULL,
    batch_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    angle_id character varying(60) NOT NULL,
    angle_label character varying(120) NOT NULL,
    status character varying(20) DEFAULT 'queued'::character varying NOT NULL,
    document_id character(36),
    publication_id character(36),
    error_message character varying(1000),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT publication_variants_pkey PRIMARY KEY (id),
    CONSTRAINT uq_variant_batch_media UNIQUE (batch_id, media_id)
);

CREATE INDEX IF NOT EXISTS ix_variant_publication ON public.publication_variants (publication_id);
CREATE INDEX IF NOT EXISTS ix_variant_batch ON public.publication_variants (batch_id, created_at);

COMMIT;
