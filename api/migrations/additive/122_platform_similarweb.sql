-- SimilarWeb Insights (RapidAPI): key ter-seal + snapshot per media.
-- Key dikelola admin via /admin/konfigurasi (pola platform_seo_config existing).

ALTER TABLE public.platform_seo_config
  ADD COLUMN IF NOT EXISTS sealed_similarweb_api_key text;

CREATE TABLE IF NOT EXISTS public.media_similarweb_snapshots (
  id              character(36) NOT NULL,
  media_id        character(36) NOT NULL,
  insights        jsonb NOT NULL,
  snapshot_date   date,
  created_by      character(36),
  created_at      timestamp(6) with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT media_similarweb_snapshots_pkey PRIMARY KEY (id),
  CONSTRAINT fk_media_similarweb_media FOREIGN KEY (media_id)
    REFERENCES public.media (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_media_similarweb_media
  ON public.media_similarweb_snapshots (media_id, created_at DESC);
