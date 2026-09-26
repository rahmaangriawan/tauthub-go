-- 090: Media-First Content Flow — additive, backward compatible.
-- Media menjadi editorial context sejak awal. Tidak DROP/TRUNCATE apapun.

BEGIN;

-- Campaign media context (media-first flow)
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS media_type character varying(20);
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS selected_media_id character(36);
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS editorial_standard_snapshot jsonb;
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS media_locked_at timestamp(6) with time zone;

DO $$
BEGIN
  ALTER TABLE public.campaigns ADD CONSTRAINT ck_campaign_media_type
    CHECK (media_type IN ('blogger','mainstream'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$
BEGIN
  ALTER TABLE public.campaigns ADD CONSTRAINT fk_campaigns_selected_media
    FOREIGN KEY (selected_media_id) REFERENCES public.media(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS ix_campaigns_selected_media ON public.campaigns(selected_media_id);

-- AI analysis results (editorial score, diagnosis, strengths, violations, headlines, revised draft)
CREATE TABLE IF NOT EXISTS public.content_ai_analyses (
  id character(36) PRIMARY KEY,
  content_intake_id character(36) NOT NULL REFERENCES public.content_intakes(id) ON DELETE CASCADE,
  campaign_id character(36) REFERENCES public.campaigns(id) ON DELETE SET NULL,
  media_id character(36) REFERENCES public.media(id) ON DELETE SET NULL,
  media_type character varying(20),
  editorial_score integer CHECK (editorial_score >= 0 AND editorial_score <= 100),
  diagnosis text,
  strengths jsonb DEFAULT '[]'::jsonb NOT NULL,
  violations jsonb DEFAULT '[]'::jsonb NOT NULL,
  content_type_detected character varying(80),
  editorial_fit_score integer CHECK (editorial_fit_score >= 0 AND editorial_fit_score <= 100),
  readability integer CHECK (readability >= 0 AND readability <= 100),
  clarity integer CHECK (clarity >= 0 AND clarity <= 100),
  search_intent character varying(120),
  structure_score integer CHECK (structure_score >= 0 AND structure_score <= 100),
  potential_value character varying(200),
  headline_suggestions jsonb DEFAULT '[]'::jsonb NOT NULL,
  revised_draft text,
  standard_source character varying(200),
  standard_version character varying(80),
  fallback_type character varying(50),
  content_hash character varying(64),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_content_ai_analyses_intake ON public.content_ai_analyses(content_intake_id);
CREATE INDEX IF NOT EXISTS ix_content_ai_analyses_campaign ON public.content_ai_analyses(campaign_id);
CREATE INDEX IF NOT EXISTS ix_content_ai_analyses_media ON public.content_ai_analyses(media_id);
CREATE INDEX IF NOT EXISTS ix_content_ai_analyses_hash ON public.content_ai_analyses(content_hash);

-- Fallback standards per media type (created if missing; content diisi di app layer)
INSERT INTO public.content_type_standards (id, content_type_code, status, created_at, updated_at)
SELECT '00000000-0000-4000-8000-000000000301', 'mainstream_fallback', 'active', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM public.content_type_standards WHERE content_type_code = 'mainstream_fallback');

INSERT INTO public.content_type_standards (id, content_type_code, status, created_at, updated_at)
SELECT '00000000-0000-4000-8000-000000000302', 'blogger_fallback', 'active', NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM public.content_type_standards WHERE content_type_code = 'blogger_fallback');

COMMIT;