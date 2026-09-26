-- Content Safety / Content Moderation (additive, non-destructive)
-- - content_moderation_results: cache hasil moderation by content_hash + version
-- - content_moderation_audits: audit trail per entity (intake/document/publication)
-- Additive only: IF NOT EXISTS, no DROP/DELETE/TRUNCATE, no data loss.
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/070_content_moderation.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.content_moderation_results (
    id character(36) NOT NULL PRIMARY KEY,
    content_hash character varying(64) NOT NULL,
    moderation_version character varying(20) NOT NULL,
    status character varying(20) NOT NULL,
    risk_score integer NOT NULL,
    category character varying(30) NOT NULL,
    confidence numeric(3,2) NOT NULL DEFAULT 0.85,
    reason_code character varying(80) NOT NULL,
    signals jsonb NOT NULL DEFAULT '[]'::jsonb,
    model character varying(160),
    ai_called boolean NOT NULL DEFAULT false,
    token_input integer NOT NULL DEFAULT 0,
    token_output integer NOT NULL DEFAULT 0,
    token_total integer NOT NULL DEFAULT 0,
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_moderation_result_status CHECK (status IN ('safe','review_required','blocked'))
);

CREATE INDEX IF NOT EXISTS idx_moderation_hash_version
    ON public.content_moderation_results (content_hash, moderation_version);

CREATE INDEX IF NOT EXISTS idx_moderation_created_at
    ON public.content_moderation_results (created_at DESC);

CREATE TABLE IF NOT EXISTS public.content_moderation_audits (
    id character(36) NOT NULL PRIMARY KEY,
    entity_type character varying(20) NOT NULL,
    entity_id character(36) NOT NULL,
    brand_id character(36),
    content_hash character varying(64) NOT NULL,
    result_id character(36),
    status character varying(20) NOT NULL,
    risk_score integer NOT NULL,
    category character varying(30) NOT NULL,
    reason_code character varying(80) NOT NULL,
    signals jsonb NOT NULL DEFAULT '[]'::jsonb,
    moderation_version character varying(20) NOT NULL,
    model character varying(160),
    ai_called boolean NOT NULL DEFAULT false,
    token_input integer NOT NULL DEFAULT 0,
    token_output integer NOT NULL DEFAULT 0,
    token_total integer NOT NULL DEFAULT 0,
    reviewed_by character(36),
    reviewed_at timestamp with time zone,
    review_decision character varying(20),
    review_note text,
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_moderation_audit_status CHECK (status IN ('safe','review_required','blocked')),
    CONSTRAINT ck_moderation_audit_entity CHECK (entity_type IN ('intake','document','publication','content_job')),
    CONSTRAINT ck_moderation_review_decision CHECK (review_decision IS NULL OR review_decision IN ('approved','rejected'))
);

CREATE INDEX IF NOT EXISTS idx_moderation_audits_entity
    ON public.content_moderation_audits (entity_type, entity_id);

CREATE INDEX IF NOT EXISTS idx_moderation_audits_brand_status
    ON public.content_moderation_audits (brand_id, status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_moderation_audits_content_hash
    ON public.content_moderation_audits (content_hash);

CREATE INDEX IF NOT EXISTS idx_moderation_audits_review
    ON public.content_moderation_audits (status, reviewed_at)
    WHERE status = 'review_required';

COMMIT;
