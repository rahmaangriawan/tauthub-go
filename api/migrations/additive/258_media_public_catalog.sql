-- 258 — Katalog media publik (data.tauthub.com).
--
-- Dua kolom untuk membuka direktori media ke publik tanpa membocorkan data
-- internal:
--   * public_listed — hanya admin yang boleh mematikan. Media dengan flag ini
--     tetap tampil di katalog klien; yang berubah hanya keterlihatannya di
--     situs publik.
--   * public_slug   — slug yang dipakai URL publik (data.tauthub.com/media/<slug>).
--     Dipisah dari `slug` internal karena `slug` ikut berubah saat mitra
--     mengganti nama media; URL yang sudah terindeks mesin pencari tidak boleh
--     putus karena rename. Hanya berubah bila admin mengirimkannya eksplisit.
--
-- Backfill memakai public._slugify dari migrasi 014, dengan sufiks angka bila
-- bentrok. Additive + idempoten: kolom baru hanya ditambah bila belum ada, dan
-- baris yang sudah punya public_slug tidak disentuh.

BEGIN;

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS public_listed boolean NOT NULL DEFAULT true;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS public_slug character varying(160);

-- _slugify berasal dari migrasi 014; dideklarasikan ulang di sini (definisi
-- identik) supaya migrasi ini tidak bergantung pada urutan/keberadaan 014.
CREATE OR REPLACE FUNCTION public._slugify(raw text)
RETURNS text LANGUAGE sql IMMUTABLE AS $$
  SELECT trim(BOTH '-' FROM regexp_replace(regexp_replace(
      lower(coalesce(raw, '')),
      '[^a-z0-9\s-]', ' ', 'g'),
      '[\s-]+', '-', 'g'));
$$;

DO $$
DECLARE
  rec RECORD;
  base text;
  candidate text;
  n int;
BEGIN
  FOR rec IN SELECT id, name, slug FROM public.media WHERE public_slug IS NULL LOOP
    base := left(coalesce(nullif(public._slugify(coalesce(rec.slug, rec.name)), ''),
                          nullif(public._slugify(rec.name), ''),
                          'media'), 150);
    candidate := base;
    n := 1;
    WHILE EXISTS (SELECT 1 FROM public.media WHERE public_slug = candidate) LOOP
      n := n + 1;
      candidate := base || '-' || n;
    END LOOP;
    UPDATE public.media SET public_slug = candidate WHERE id = rec.id;
  END LOOP;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_media_public_slug ON public.media (public_slug);
CREATE INDEX IF NOT EXISTS ix_media_public_listing
    ON public.media (media_tier, name)
    WHERE status = 'active' AND public_listed;

-- Trigger: semua jalur INSERT (admin, mitra, impor massal) mengisi public_slug
-- bila belum diisi. Tanpa ini media baru dari portal mitra tidak akan pernah
-- muncul di katalog publik karena query publik mensyaratkan public_slug.
CREATE OR REPLACE FUNCTION public.set_media_public_slug() RETURNS trigger AS $$
DECLARE
  base text;
  candidate text;
  n int;
BEGIN
  IF NEW.public_slug IS NULL OR NEW.public_slug = '' THEN
    base := left(coalesce(nullif(public._slugify(coalesce(NEW.slug, NEW.name)), ''),
                          nullif(public._slugify(NEW.name), ''),
                          'media'), 150);
    candidate := base;
    n := 1;
    WHILE EXISTS (SELECT 1 FROM public.media WHERE public_slug = candidate) LOOP
      n := n + 1;
      candidate := base || '-' || n;
    END LOOP;
    NEW.public_slug := candidate;
  END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_media_public_slug ON public.media;
CREATE TRIGGER trg_media_public_slug
  BEFORE INSERT ON public.media
  FOR EACH ROW
  EXECUTE FUNCTION public.set_media_public_slug();

COMMIT;
