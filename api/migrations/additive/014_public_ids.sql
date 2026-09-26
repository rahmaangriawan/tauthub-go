-- Migration: 014_public_ids
-- Adds a stable public-facing id (`{slug}-{5digit}`) to brands and partners so
-- client/partner URLs stop exposing raw UUIDs: /klien/{public_id}, /mitra/{public_id}.
-- Internal relations keep using the UUID primary key.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/014_public_ids.sql

BEGIN;

ALTER TABLE public.brands   ADD COLUMN IF NOT EXISTS public_id character varying(180);
ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS public_id character varying(180);

-- slugify: lowercase, strip accents/punct, collapse spaces & hyphens to single "-",
-- trim leading/trailing hyphens. Mirrors src/lib/public-id.ts slugify().
CREATE OR REPLACE FUNCTION public._slugify(raw text)
RETURNS text LANGUAGE sql IMMUTABLE AS $$
  SELECT trim(BOTH '-' FROM regexp_replace(regexp_replace(
      lower(coalesce(raw, '')),
      '[^a-z0-9\s-]', ' ', 'g'),
      '[\s-]+', '-', 'g'));
$$;

-- Backfill brands.public_id from slug + 5-digit random (retry on collision).
DO $$
DECLARE
  rec RECORD;
  candidate text;
  attempts int;
BEGIN
  FOR rec IN SELECT id, slug FROM public.brands WHERE public_id IS NULL LOOP
    attempts := 0;
    LOOP
      candidate := coalesce(nullif(public._slugify(rec.slug), ''), 'brand') || '-' || lpad(floor(random() * 100000)::text, 5, '0');
      BEGIN
        UPDATE public.brands SET public_id = candidate WHERE id = rec.id;
        EXIT;
      EXCEPTION WHEN unique_violation THEN
        attempts := attempts + 1;
        IF attempts > 50 THEN RAISE EXCEPTION 'public_id collision for brand %', rec.id; END IF;
      END;
    END LOOP;
  END LOOP;
END $$;

-- Backfill partners.public_id from name + 5-digit random (fallback slug 'mitra').
DO $$
DECLARE
  rec RECORD;
  candidate text;
  attempts int;
BEGIN
  FOR rec IN SELECT id, name FROM public.partners WHERE public_id IS NULL LOOP
    attempts := 0;
    LOOP
      candidate := coalesce(nullif(public._slugify(rec.name), ''), 'mitra') || '-' || lpad(floor(random() * 100000)::text, 5, '0');
      BEGIN
        UPDATE public.partners SET public_id = candidate WHERE id = rec.id;
        EXIT;
      EXCEPTION WHEN unique_violation THEN
        attempts := attempts + 1;
        IF attempts > 50 THEN RAISE EXCEPTION 'public_id collision for partner %', rec.id; END IF;
      END;
    END LOOP;
  END LOOP;
END $$;

ALTER TABLE public.brands   ADD CONSTRAINT uq_brands_public_id   UNIQUE (public_id);
ALTER TABLE public.partners ADD CONSTRAINT uq_partners_public_id UNIQUE (public_id);

COMMIT;
