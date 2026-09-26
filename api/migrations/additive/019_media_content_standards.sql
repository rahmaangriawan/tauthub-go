-- Migration: 019_media_content_standards
-- Phase 8-9: per-media editorial standards managed by Admin. The final
-- standard check runs after the client selects media, before publish.
-- Also extends media.integration_type with 'tulisio_api' (pull model for
-- custom partner websites via the Tulisio Publishing API).
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/019_media_content_standards.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.media_content_standards (
    id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    min_words integer DEFAULT 400 NOT NULL,
    max_words integer DEFAULT 1500 NOT NULL,
    journalistic_style character varying(20) DEFAULT 'news'::character varying NOT NULL,
    hard_selling_tolerance character varying(10) DEFAULT 'low'::character varying NOT NULL,
    quote_required boolean DEFAULT false NOT NULL,
    min_quotes integer DEFAULT 0 NOT NULL,
    image_required boolean DEFAULT false NOT NULL,
    category_required boolean DEFAULT false NOT NULL,
    required_info jsonb DEFAULT '[]'::jsonb NOT NULL,
    forbidden_content jsonb DEFAULT '[]'::jsonb NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT media_content_standards_pkey PRIMARY KEY (id),
    CONSTRAINT uq_media_content_standard_media UNIQUE (media_id),
    CONSTRAINT ck_media_standard_style CHECK (((journalistic_style)::text = ANY ((ARRAY['news'::character varying, 'feature'::character varying, 'magazine'::character varying, 'blog'::character varying])::text[]))),
    CONSTRAINT ck_media_standard_tolerance CHECK (((hard_selling_tolerance)::text = ANY ((ARRAY['none'::character varying, 'low'::character varying, 'medium'::character varying, 'high'::character varying])::text[]))),
    CONSTRAINT ck_media_standard_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'archived'::character varying])::text[]))),
    CONSTRAINT ck_media_standard_words CHECK ((min_words >= 0 AND max_words >= min_words)),
    CONSTRAINT ck_media_standard_quotes CHECK (min_quotes >= 0),
    CONSTRAINT fk_media_content_standards_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE,
    CONSTRAINT fk_media_content_standards_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL
);

-- Tulisio Publishing API (pull model) is a valid media integration type.
DO $$
BEGIN
  ALTER TABLE public.media DROP CONSTRAINT ck_media_integration_type;
EXCEPTION WHEN undefined_object THEN NULL;
END $$;

DO $$
BEGIN
  ALTER TABLE public.media ADD CONSTRAINT ck_media_integration_type
    CHECK (integration_type IN ('manual','wordpress','rss','webhook','tulisio_api'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

COMMIT;
