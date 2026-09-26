-- Migration: 029_pr_calculator
-- PR Calculator / PR Performance (foundation §PR): data model + publication
-- performance foundation. Scope: data model + Level 1 metrics + analysis
-- storage. No fake metrics — unavailable data stays unavailable.
--
-- 1. pr_scoring_profiles     versioned scoring configuration (weights must sum
--                            to 100, enforced app-side); old versions never
--                            change so campaign history stays reproducible.
-- 2. pr_campaign_scores      per-campaign snapshot: profile version + weights
--                            + status (not_ready/calculating/partial/ready/
--                            final/failed) + breakdown + input snapshot.
-- 3. pr_metric_results       precomputed Level 1 metrics per campaign
--                            (publication performance), source-labelled.
-- 4. media quality           media_tier + media_quality_score + notes on the
--                            existing media entity (no duplicate media table).
-- 5. content_intakes.key_messages  key messages captured during intake.
-- 6. pr_message_analyses     idempotent (unique publication_id) message
--                            consistency analysis results.
-- 7. pr_sentiment_analyses   idempotent per-publication coverage tone.
-- 8. publications.auto_replace_disabled  per-publication kill switch.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/029_pr_calculator.sql

BEGIN;

-- ─── Media quality (integrated into the existing media entity) ───────────
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS media_tier character varying(20) DEFAULT 'tier3';
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS media_quality_score smallint;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS media_quality_notes character varying(500);

ALTER TABLE public.media ADD CONSTRAINT ck_media_tier CHECK (media_tier IN ('tier1','tier2','tier3'));
ALTER TABLE public.media ADD CONSTRAINT ck_media_quality_score CHECK (media_quality_score IS NULL OR (media_quality_score >= 0 AND media_quality_score <= 100));

-- ─── Per-publication auto-replace kill switch ─────────────────────────────
ALTER TABLE public.publications ADD COLUMN IF NOT EXISTS auto_replace_disabled boolean NOT NULL DEFAULT FALSE;

-- ─── Key messages captured during content intake ──────────────────────────
ALTER TABLE public.content_intakes ADD COLUMN IF NOT EXISTS key_messages jsonb NOT NULL DEFAULT '[]';

-- ─── PR scoring profiles (versioned) ──────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.pr_scoring_profiles (
    id character(36) NOT NULL,
    version integer NOT NULL,
    name character varying(120) NOT NULL,
    weights jsonb NOT NULL,
    thresholds jsonb NOT NULL DEFAULT '{}'::jsonb,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_scoring_profiles_pkey PRIMARY KEY (id),
    CONSTRAINT uq_pr_scoring_profile_version UNIQUE (version),
    CONSTRAINT ck_pr_scoring_profile_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'active'::character varying, 'archived'::character varying])::text[]))),
    CONSTRAINT fk_pr_scoring_profiles_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL
);

-- ─── Per-campaign score snapshot ──────────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.pr_campaign_scores (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    profile_id character(36) NOT NULL,
    profile_version integer NOT NULL,
    weights jsonb NOT NULL,
    status character varying(20) DEFAULT 'not_ready'::character varying NOT NULL,
    score numeric(5,2),
    label character varying(40),
    breakdown jsonb NOT NULL DEFAULT '{}'::jsonb,
    input_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb,
    calculated_by character(36),
    calculated_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_campaign_scores_pkey PRIMARY KEY (id),
    CONSTRAINT uq_pr_campaign_profile UNIQUE (campaign_id, profile_version),
    CONSTRAINT ck_pr_campaign_score_status CHECK (((status)::text = ANY ((ARRAY['not_ready'::character varying, 'calculating'::character varying, 'partial'::character varying, 'ready'::character varying, 'final'::character varying, 'failed'::character varying])::text[]))),
    CONSTRAINT fk_pr_campaign_scores_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE,
    CONSTRAINT fk_pr_campaign_scores_profile FOREIGN KEY (profile_id) REFERENCES public.pr_scoring_profiles(id),
    CONSTRAINT fk_pr_campaign_scores_calculator FOREIGN KEY (calculated_by) REFERENCES public.users(id) ON DELETE SET NULL
);

-- ─── Precomputed Level 1 metrics per campaign ─────────────────────────────
CREATE TABLE IF NOT EXISTS public.pr_metric_results (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    metric_key character varying(60) NOT NULL,
    metric_value numeric NOT NULL,
    source character varying(120),
    calculated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_metric_results_pkey PRIMARY KEY (id),
    CONSTRAINT uq_pr_metric_campaign_key UNIQUE (campaign_id, metric_key),
    CONSTRAINT fk_pr_metric_results_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE
);

-- ─── Message consistency analysis (idempotent per publication) ────────────
CREATE TABLE IF NOT EXISTS public.pr_message_analyses (
    id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    campaign_id character(36),
    key_messages jsonb NOT NULL DEFAULT '[]'::jsonb,
    results jsonb NOT NULL DEFAULT '[]'::jsonb,
    consistency_score numeric(5,2),
    analysis_version character varying(60) NOT NULL,
    status character varying(20) DEFAULT 'completed'::character varying NOT NULL,
    error_message character varying(500),
    analyzed_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_message_analyses_pkey PRIMARY KEY (id),
    CONSTRAINT uq_pr_message_analysis_publication UNIQUE (publication_id),
    CONSTRAINT ck_pr_message_analysis_status CHECK (((status)::text = ANY ((ARRAY['queued'::character varying, 'running'::character varying, 'completed'::character varying, 'failed'::character varying])::text[]))),
    CONSTRAINT fk_pr_message_analyses_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE,
    CONSTRAINT fk_pr_message_analyses_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL
);

-- ─── Coverage tone / sentiment analysis (idempotent per publication) ──────
CREATE TABLE IF NOT EXISTS public.pr_sentiment_analyses (
    id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    campaign_id character(36),
    sentiment character varying(20),
    confidence numeric(5,2),
    analysis_version character varying(60) NOT NULL,
    status character varying(20) DEFAULT 'completed'::character varying NOT NULL,
    error_message character varying(500),
    analyzed_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pr_sentiment_analyses_pkey PRIMARY KEY (id),
    CONSTRAINT uq_pr_sentiment_analysis_publication UNIQUE (publication_id),
    CONSTRAINT ck_pr_sentiment_analysis_status CHECK (((status)::text = ANY ((ARRAY['queued'::character varying, 'running'::character varying, 'completed'::character varying, 'failed'::character varying])::text[]))),
    CONSTRAINT ck_pr_sentiment_value CHECK ((sentiment IS NULL OR sentiment IN ('positive','neutral','negative'))),
    CONSTRAINT fk_pr_sentiment_analyses_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE,
    CONSTRAINT fk_pr_sentiment_analyses_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL
);

COMMIT;
