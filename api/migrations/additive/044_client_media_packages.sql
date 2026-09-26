-- Migration: 044_client_media_packages
-- Paket media custom per brand (dibuat klien sendiri, maksimal 5 media).
-- Paket global (buatan admin) tetap di media_packages dengan source='admin'.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/044_client_media_packages.sql

BEGIN;

ALTER TABLE public.media_packages
  ADD COLUMN IF NOT EXISTS source VARCHAR(20) NOT NULL DEFAULT 'admin'
    CHECK (source IN ('admin','client')),
  ADD COLUMN IF NOT EXISTS brand_id VARCHAR(36) REFERENCES public.brands(id) ON DELETE CASCADE;

CREATE INDEX IF NOT EXISTS ix_media_packages_source_brand
  ON public.media_packages (source, brand_id);

-- Paket custom klien dibatasi maksimal 5 media. Trigger menjaga batas ini
-- baik saat insert maupun update item.
CREATE OR REPLACE FUNCTION enforce_client_package_media_limit()
RETURNS TRIGGER AS $$
DECLARE
  pkg_source VARCHAR(20);
  media_count INTEGER;
BEGIN
  SELECT source INTO pkg_source FROM public.media_packages WHERE id = NEW.package_id;
  IF pkg_source = 'client' THEN
    SELECT COUNT(*) INTO media_count FROM public.media_package_items WHERE package_id = NEW.package_id;
    IF media_count >= 5 THEN
      RAISE EXCEPTION 'Paket media custom maksimal 5 media (client package limit)';
    END IF;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_client_package_media_limit ON public.media_package_items;
CREATE TRIGGER trg_client_package_media_limit
  BEFORE INSERT ON public.media_package_items
  FOR EACH ROW EXECUTE FUNCTION enforce_client_package_media_limit();

COMMIT;
