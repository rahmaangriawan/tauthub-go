-- =====================================================================
-- Migration 007 - Tracking & Analytics (Phase 7)
--
-- Additive migration for databases that already ran 001-006.
--
--   * tracking_links   tracking URL per publikasi (go.tulisio.id/{code}).
--     Code unik; destination URL dari client; redirect + click tracking.
--   * tracking_events  event storage: article_view, article_scroll,
--     outbound_click, cta_click, tracking_link_click. Indeks besar:
--     (brand_id, created_at) dan (link_id, created_at).
--   * analytics_daily  rollup harian per (brand, tanggal, event_type,
--     publication). Unique index pakai COALESCE(publication_id,'') agar
--     baris publication NULL tetap dedup (PG memperlakukan NULL sebagai
--     distinct pada UNIQUE biasa).
--
-- Idempotent (CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS).
--
-- Apply with:
--   psql "$DATABASE_URL" -f apps/web/src/db/migrations/007_tracking.sql
-- =====================================================================

CREATE TABLE IF NOT EXISTS public.tracking_links (
    id character(36) NOT NULL,
    code character varying(24) NOT NULL,
    publication_id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    destination_url character varying(2000),
    status character varying(20) DEFAULT 'active' NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT tracking_links_pkey PRIMARY KEY (id),
    CONSTRAINT uq_tracking_link_code UNIQUE (code),
    CONSTRAINT uq_tracking_link_publication UNIQUE (publication_id),
    CONSTRAINT fk_tracking_links_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE,
    CONSTRAINT fk_tracking_links_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id),
    CONSTRAINT fk_tracking_links_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_tracking_link_status CHECK (status IN ('active','disabled'))
);

CREATE TABLE IF NOT EXISTS public.tracking_events (
    id character(36) NOT NULL,
    link_id character(36),
    publication_id character(36),
    brand_id character(36) NOT NULL,
    event_type character varying(30) NOT NULL,
    session_id character varying(80),
    ip_address character varying(64),
    user_agent character varying(300),
    referrer character varying(1000),
    url character varying(1000),
    metadata jsonb,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT tracking_events_pkey PRIMARY KEY (id),
    CONSTRAINT fk_tracking_events_link FOREIGN KEY (link_id) REFERENCES public.tracking_links(id) ON DELETE SET NULL,
    CONSTRAINT fk_tracking_events_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE SET NULL,
    CONSTRAINT fk_tracking_events_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id),
    CONSTRAINT ck_tracking_event_type CHECK (event_type IN ('article_view','article_scroll','outbound_click','cta_click','tracking_link_click'))
);

CREATE TABLE IF NOT EXISTS public.analytics_daily (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    publication_id character(36),
    event_type character varying(30) NOT NULL,
    event_date date NOT NULL,
    count bigint DEFAULT 0 NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT analytics_daily_pkey PRIMARY KEY (id),
    CONSTRAINT fk_analytics_daily_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id),
    CONSTRAINT fk_analytics_daily_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE,
    CONSTRAINT ck_analytics_daily_event_type CHECK (event_type IN ('article_view','article_scroll','outbound_click','cta_click','tracking_link_click'))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_analytics_daily ON public.analytics_daily USING btree (brand_id, event_date, event_type, COALESCE(publication_id, ''));

CREATE INDEX IF NOT EXISTS ix_tracking_link_brand ON public.tracking_links USING btree (brand_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_tracking_events_brand_time ON public.tracking_events USING btree (brand_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_tracking_events_link_time ON public.tracking_events USING btree (link_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_tracking_events_type_time ON public.tracking_events USING btree (event_type, created_at DESC);
