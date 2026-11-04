-- 307 — Platform website media (WordPress / Blogspot / Custom) sebagai info
-- terpisah dari integration_type. integration_type = cara artikel dikirim
-- (manual/API); website_platform = situs itu sendiri dibangun pakai apa.
-- Diisi otomatis oleh scheduler (deteksi dari halaman depan) dan di-reset
-- ke NULL bila website_url berubah supaya dideteksi ulang. Additive.

BEGIN;

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS website_platform text;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS website_platform_checked_at timestamptz;
ALTER TABLE public.media DROP CONSTRAINT IF EXISTS media_website_platform_check;
ALTER TABLE public.media ADD CONSTRAINT media_website_platform_check
  CHECK (website_platform IS NULL OR website_platform IN ('wordpress', 'blogspot', 'custom'));

CREATE OR REPLACE FUNCTION public.media_reset_website_platform() RETURNS trigger AS $$
BEGIN
  IF NEW.website_url IS DISTINCT FROM OLD.website_url THEN
    NEW.website_platform := NULL;
    NEW.website_platform_checked_at := NULL;
  END IF;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_media_reset_website_platform ON public.media;
CREATE TRIGGER trg_media_reset_website_platform
  BEFORE UPDATE OF website_url ON public.media
  FOR EACH ROW EXECUTE FUNCTION public.media_reset_website_platform();

COMMIT;
