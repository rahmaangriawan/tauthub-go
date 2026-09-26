-- 085 — Jenis Media + taxonomy multi-kategori (global dedup).
-- Additive: ADD COLUMN / CREATE TABLE IF NOT EXISTS. No DROP.
--  * media.media_type         -> blogger | mainstream (default mainstream; backfill blogger dari integration_type)
--  * media_categories         -> katalog kategori global (normalized lower(trim(name)) unique)
--  * media_category_links     -> relasi many-to-many media <-> category (PK composite)
-- Backfill link dari legacy media.category (single varchar) supaya data lama tetap muncul di filter.
-- Sync rule: API tetap tulis media.category = categories[0] (legacy compat).
-- Apply: psql $DATABASE_URL -f apps/web/src/db/migrations/085_media_taxonomy.sql

BEGIN;

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS media_type character varying(20) NOT NULL DEFAULT 'mainstream';

DO $$
BEGIN
  ALTER TABLE public.media ADD CONSTRAINT ck_media_type CHECK (media_type IN ('blogger','mainstream'));
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

UPDATE public.media SET media_type = 'blogger' WHERE integration_type = 'blogger' AND media_type <> 'blogger';
UPDATE public.media SET media_type = 'mainstream' WHERE media_type IS NULL OR TRIM(media_type) = '';

CREATE TABLE IF NOT EXISTS public.media_categories (
  id character(36) NOT NULL,
  name character varying(120) NOT NULL,
  normalized character varying(120) NOT NULL,
  created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
  CONSTRAINT media_categories_pkey PRIMARY KEY (id),
  CONSTRAINT uq_media_categories_normalized UNIQUE (normalized)
);

CREATE TABLE IF NOT EXISTS public.media_category_links (
  media_id character(36) NOT NULL,
  category_id character(36) NOT NULL,
  created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
  CONSTRAINT media_category_links_pkey PRIMARY KEY (media_id, category_id),
  CONSTRAINT fk_mcl_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE,
  CONSTRAINT fk_mcl_category FOREIGN KEY (category_id) REFERENCES public.media_categories(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_media_category_links_media ON public.media_category_links USING btree (media_id);
CREATE INDEX IF NOT EXISTS ix_media_category_links_category ON public.media_category_links USING btree (category_id);
CREATE INDEX IF NOT EXISTS ix_media_categories_name ON public.media_categories USING btree (name);

-- Backfill kategori global dari distinct legacy media.category
INSERT INTO public.media_categories (id, name, normalized)
SELECT gen_random_uuid()::text, TRIM(s.category), LOWER(TRIM(s.category))
FROM (SELECT DISTINCT TRIM(category) AS category FROM public.media WHERE category IS NOT NULL AND TRIM(category) <> '') s
ON CONFLICT (normalized) DO NOTHING;

-- Backfill link media -> category (legacy single category)
INSERT INTO public.media_category_links (media_id, category_id)
SELECT m.id, c.id
FROM public.media m
JOIN public.media_categories c ON c.normalized = LOWER(TRIM(m.category))
WHERE m.category IS NOT NULL AND TRIM(m.category) <> ''
ON CONFLICT DO NOTHING;

COMMIT;
