-- Migration: 041_media_package_discount
-- Media packages can carry a percent discount (0-100) that is REALLY applied
-- when the client publishes the whole bundle: each member media's credit cost
-- is reduced by discount_percent at billing time (reservation), so the client
-- sees "Hemat X TC" on the package button.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/041_media_package_discount.sql

BEGIN;

ALTER TABLE public.media_packages
  ADD COLUMN IF NOT EXISTS discount_percent INTEGER NOT NULL DEFAULT 0;

-- Keep it sane (0 = no discount, 100 = free). Dropped old constraint if any.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'ck_media_packages_discount'
  ) THEN
    ALTER TABLE public.media_packages DROP CONSTRAINT ck_media_packages_discount;
  END IF;
END $$;

ALTER TABLE public.media_packages
  ADD CONSTRAINT ck_media_packages_discount CHECK (discount_percent BETWEEN 0 AND 100);

COMMIT;
