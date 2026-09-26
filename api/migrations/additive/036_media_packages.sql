-- Migration: 036_media_packages
-- Media package bundles ("Paket Media Tulisio"): a named set of media that a
-- client can select in one click. The bundle is separate from per-brand
-- credit `packages` (that table is credit allocation, not media bundles).
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/036_media_packages.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.media_packages (
  id          VARCHAR(36) PRIMARY KEY,
  name        VARCHAR(120) NOT NULL,
  slug        VARCHAR(140) NOT NULL,
  description TEXT,
  status      VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive')),
  created_by  VARCHAR(36),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_media_packages_slug ON public.media_packages (slug);

CREATE TABLE IF NOT EXISTS public.media_package_items (
  package_id VARCHAR(36) NOT NULL REFERENCES public.media_packages(id) ON DELETE CASCADE,
  media_id   VARCHAR(36) NOT NULL REFERENCES public.media(id) ON DELETE CASCADE,
  position   INTEGER NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (package_id, media_id)
);

CREATE INDEX IF NOT EXISTS ix_media_package_items_media ON public.media_package_items (media_id);

COMMIT;
