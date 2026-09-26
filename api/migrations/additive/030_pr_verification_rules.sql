-- Migration: 030_pr_verification_rules
-- PR Calculator follow-up (foundation §PR): publication verification (Level 2
-- compliance) + media quality/relevance rules for per-media performance.
--
-- 1. pr_publication_verifications  idempotent (unique publication_id) result
--                                  of fetching the published URL: status
--                                  passed/failed/unavailable (VERIFICATION_
--                                  UNAVAILABLE when the URL cannot be reached),
--                                  compliance_score, per-check breakdown.
-- 2. pr_media_rules                single-row ('default') configuration:
--                                  tier → default quality, category →
--                                  relevance score (used for the per-media
--                                  table when the media itself has no score).
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/030_pr_verification_rules.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.pr_publication_verifications (
    id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    campaign_id character(36),
    url character varying(500),
    status character varying(20) NOT NULL DEFAULT 'unavailable',
    compliance_score numeric(5,2),
    checks jsonb NOT NULL DEFAULT '[]'::jsonb,
    error_message character varying(500),
    verified_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_publication_verifications_pkey PRIMARY KEY (id),
    CONSTRAINT uq_pr_verification_publication UNIQUE (publication_id),
    CONSTRAINT ck_pr_verification_status CHECK (((status)::text = ANY ((ARRAY['passed'::character varying, 'failed'::character varying, 'unavailable'::character varying])::text[]))),
    CONSTRAINT fk_pr_verifications_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE,
    CONSTRAINT fk_pr_verifications_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_pr_verifications_campaign ON public.pr_publication_verifications (campaign_id);

CREATE TABLE IF NOT EXISTS public.pr_media_rules (
    id character(36) NOT NULL,
    tier_default_quality jsonb NOT NULL DEFAULT '{"tier1": 90, "tier2": 80, "tier3": 70}'::jsonb,
    category_relevance jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_media_rules_pkey PRIMARY KEY (id),
    CONSTRAINT fk_pr_media_rules_updater FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL
);

COMMIT;
