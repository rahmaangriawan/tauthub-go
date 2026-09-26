-- =====================================================================
-- Migration 004 - Delivery Hardening + RSS (Phase 4)
--
-- Additive migration for databases that already ran 001-003.
--
--   * media.publish_defaults      JSONB: per-media publishing defaults
--     { author_id, categories[], tags[], featured_image_url,
--       custom_fields{}, term_action }
--   * rss_feeds                   RSS channel per partner/media with a
--     secure token (pull mechanism, foundation §13)
--   * rss_access_logs             access log + rate limit per feed
--
-- PostgreSQL has no `ADD CONSTRAINT IF NOT EXISTS`, so new constraints
-- are inline in CREATE TABLE IF NOT EXISTS; new columns use
-- ADD COLUMN IF NOT EXISTS. Idempotent.
--
-- Apply with:
--   psql "$DATABASE_URL" -f apps/web/src/db/migrations/004_delivery_hardening.sql
-- =====================================================================

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS publish_defaults jsonb;

CREATE TABLE IF NOT EXISTS public.rss_feeds (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    media_id character(36),
    name character varying(160) NOT NULL,
    slug character varying(160),
    token character(64) NOT NULL,
    status character varying(20) DEFAULT 'active' NOT NULL,
    description text,
    item_status character varying(20) DEFAULT 'published' NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT rss_feeds_pkey PRIMARY KEY (id),
    CONSTRAINT uq_rss_feed_token UNIQUE (token),
    CONSTRAINT uq_rss_feed_partner_name UNIQUE (partner_id, name),
    CONSTRAINT fk_rss_feeds_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE,
    CONSTRAINT fk_rss_feeds_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE SET NULL,
    CONSTRAINT fk_rss_feeds_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_rss_feed_status CHECK (status IN ('active','revoked')),
    CONSTRAINT ck_rss_feed_item_status CHECK (item_status IN ('published','draft'))
);

CREATE TABLE IF NOT EXISTS public.rss_access_logs (
    id character(36) NOT NULL,
    feed_id character(36) NOT NULL,
    ip_address character varying(64),
    user_agent character varying(300),
    status smallint,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT rss_access_logs_pkey PRIMARY KEY (id),
    CONSTRAINT fk_rss_access_logs_feed FOREIGN KEY (feed_id) REFERENCES public.rss_feeds(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_rss_feed_partner ON public.rss_feeds USING btree (partner_id, status);

CREATE INDEX IF NOT EXISTS ix_rss_feed_media ON public.rss_feeds USING btree (media_id);

CREATE INDEX IF NOT EXISTS ix_rss_access_logs_feed_time ON public.rss_access_logs USING btree (feed_id, created_at);
