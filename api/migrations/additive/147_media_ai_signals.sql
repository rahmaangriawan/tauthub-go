-- 147 — Sinyal AI katalog media: ai_trust_score (0-100, kesiapan infrastruktur
-- web untuk LLM) + ai_citation (domain disebut Google AI Overview via SERPens
-- RapidAPI). Key SERPens dikelola admin via /admin/konfigurasi — pola
-- platform_seo_config existing.

ALTER TABLE public.media_seo_metrics ADD COLUMN IF NOT EXISTS ai_trust_score smallint;
ALTER TABLE public.media_seo_metrics ADD COLUMN IF NOT EXISTS ai_citation boolean;
ALTER TABLE public.media_seo_metrics ADD COLUMN IF NOT EXISTS ai_citation_checked_at timestamp(6) with time zone;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'ck_media_seo_metrics_ai_trust'
  ) THEN
    ALTER TABLE public.media_seo_metrics ADD CONSTRAINT ck_media_seo_metrics_ai_trust
      CHECK (ai_trust_score IS NULL OR ai_trust_score BETWEEN 0 AND 100);
  END IF;
END $$;

-- Baris boleh berisi sinyal AI saja (tanpa DA/PA/spam) — cek AI Trust/Citation
-- tidak menuntut key RapidAPI domain-metrics terkonfigurasi.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'ck_media_seo_metrics_value' AND conrelid = 'public.media_seo_metrics'::regclass
  ) THEN
    ALTER TABLE public.media_seo_metrics DROP CONSTRAINT ck_media_seo_metrics_value;
    ALTER TABLE public.media_seo_metrics ADD CONSTRAINT ck_media_seo_metrics_value
      CHECK (da IS NOT NULL OR pa IS NOT NULL OR spam_score IS NOT NULL
             OR ai_trust_score IS NOT NULL OR ai_citation IS NOT NULL);
  END IF;
END $$;

ALTER TABLE public.platform_seo_config ADD COLUMN IF NOT EXISTS sealed_serpens_api_key text;
