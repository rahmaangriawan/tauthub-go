-- 083 — SEO: kolom spam_score untuk media_seo_metrics.
-- Additive: ADD COLUMN IF NOT EXISTS + relaksasi CHECK lama (agar media tanpa
-- nilai apa pun tetap bisa tersimpan saat provider tidak lengkap).

ALTER TABLE public.media_seo_metrics ADD COLUMN IF NOT EXISTS spam_score smallint;
ALTER TABLE public.media_seo_metrics ADD CONSTRAINT ck_media_seo_metrics_spam CHECK (spam_score IS NULL OR spam_score BETWEEN 0 AND 100);

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'ck_media_seo_metrics_value' AND conrelid = 'public.media_seo_metrics'::regclass
  ) THEN
    ALTER TABLE public.media_seo_metrics DROP CONSTRAINT ck_media_seo_metrics_value;
    ALTER TABLE public.media_seo_metrics ADD CONSTRAINT ck_media_seo_metrics_value
      CHECK (da IS NOT NULL OR pa IS NOT NULL OR spam_score IS NOT NULL);
  END IF;
END $$;
