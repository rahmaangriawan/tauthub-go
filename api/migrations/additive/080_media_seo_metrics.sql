-- 080 — Media SEO metrics cache + platform RapidAPI credential
--
-- DA/PA tersimpan per media agar tombol "Cek sekarang" tidak perlu memanggil
-- provider berulang. Credential RapidAPI disimpan terenkripsi oleh server;
-- nilai plaintext tidak pernah masuk database.

CREATE TABLE IF NOT EXISTS public.media_seo_metrics (
  media_id    character(36) NOT NULL,
  da          smallint,
  pa          smallint,
  checked_at  timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  source      character varying(40) NOT NULL DEFAULT 'rapidapi-domain-metrics',
  created_at  timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at  timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT media_seo_metrics_pkey PRIMARY KEY (media_id),
  CONSTRAINT fk_media_seo_metrics_media FOREIGN KEY (media_id)
    REFERENCES public.media(id) ON DELETE CASCADE,
  CONSTRAINT ck_media_seo_metrics_da CHECK (da IS NULL OR da BETWEEN 0 AND 100),
  CONSTRAINT ck_media_seo_metrics_pa CHECK (pa IS NULL OR pa BETWEEN 0 AND 100),
  CONSTRAINT ck_media_seo_metrics_value CHECK (da IS NOT NULL OR pa IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS ix_media_seo_metrics_checked
  ON public.media_seo_metrics (checked_at DESC);

CREATE TABLE IF NOT EXISTS public.platform_seo_config (
  id              text NOT NULL DEFAULT 'default',
  sealed_api_key  text,
  api_key_hint    character varying(16),
  updated_by      character(36),
  updated_at      timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT platform_seo_config_pkey PRIMARY KEY (id),
  CONSTRAINT ck_platform_seo_config_id CHECK (id = 'default')
);
