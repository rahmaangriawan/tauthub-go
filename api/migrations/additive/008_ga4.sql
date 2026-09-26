-- =====================================================================
-- Migration 008 - GA4 Integration (Phase 8)
--
-- Additive migration for databases that already ran 001-007.
--
--   * ga4_configs       integrasi GA4 per brand (Measurement Protocol v2):
--     measurement_id (G-XXXX), api_secret, event_mapping (override nama
--     event Tulisio → GA4), consent_mode ('required'|'none').
--     Tulisio Analytics tetap source of truth; GA4 hanya mirror
--     (foundation §15).
--   * analytics_daily.campaign_id + tracking_events.campaign_id
--     dimensi campaign untuk agregasi (foundation §16): date,
--     campaign_id, article_id, media_id, views, clicks, ctr.
--
-- Idempotent (CREATE TABLE IF NOT EXISTS / ADD COLUMN IF NOT EXISTS).
--
-- Apply with:
--   psql "$DATABASE_URL" -f apps/web/src/db/migrations/008_ga4.sql
-- =====================================================================

CREATE TABLE IF NOT EXISTS public.ga4_configs (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    measurement_id character varying(40) NOT NULL,
    api_secret character varying(64),
    event_mapping jsonb,
    enabled boolean DEFAULT false NOT NULL,
    consent_mode character varying(20) DEFAULT 'required' NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ga4_configs_pkey PRIMARY KEY (id),
    CONSTRAINT uq_ga4_config_brand UNIQUE (brand_id),
    CONSTRAINT fk_ga4_configs_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id),
    CONSTRAINT fk_ga4_configs_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_ga4_consent_mode CHECK (consent_mode IN ('required','none')),
    CONSTRAINT ck_ga4_measurement_id CHECK (measurement_id ~ '^G-[A-Z0-9]+$')
);

ALTER TABLE public.tracking_events ADD COLUMN IF NOT EXISTS campaign_id character(36);

ALTER TABLE public.analytics_daily ADD COLUMN IF NOT EXISTS campaign_id character(36);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_tracking_events_campaign') THEN
        ALTER TABLE public.tracking_events ADD CONSTRAINT fk_tracking_events_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_analytics_daily_campaign') THEN
        ALTER TABLE public.analytics_daily ADD CONSTRAINT fk_analytics_daily_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS ix_ga4_config_status ON public.ga4_configs USING btree (brand_id, enabled);

CREATE INDEX IF NOT EXISTS ix_tracking_events_campaign_time ON public.tracking_events USING btree (campaign_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_analytics_daily_campaign_date ON public.analytics_daily USING btree (brand_id, campaign_id, event_date);
