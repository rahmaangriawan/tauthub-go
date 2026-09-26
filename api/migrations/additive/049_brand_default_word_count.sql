-- 049: default_target_words per brand (admin-managed).
-- Klien tidak lagi input jumlah kata; admin atur di /admin/content-settings (brand_profiles).
BEGIN;
ALTER TABLE public.brand_profiles
  ADD COLUMN IF NOT EXISTS default_target_words integer DEFAULT 1000 NOT NULL;
-- clamp admin ke 300-10000 (validasi ada di Zod), tapi jaga DB agar tidak aneh:
DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_brand_profile_words') THEN
    ALTER TABLE public.brand_profiles
      ADD CONSTRAINT ck_brand_profile_words CHECK (default_target_words BETWEEN 300 AND 10000);
  END IF;
END $$;
COMMIT;
