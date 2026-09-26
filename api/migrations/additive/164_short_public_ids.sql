-- 164 — Public id pendek (6 karakter acak) untuk partners & brands.
--
-- Latar: public_id lama berformat `{slug-nama}-{5digit}` (mis. `mitra-smoke-001`)
-- sehingga URL /mitra/... dan /klien/... membocorkan nama pemiliknya.
-- Baru: 6 karakter acak dari alfabet tanpa karakter mudah tertukar (tanpa 0/o,
-- 1/l/i, u) → mis. `k7m2x9`. Alfabet & panjang harus SAMA dengan
-- `buildShortPublicId()` di apps/web/src/lib/public-id.ts.
--
-- Aditif & idempoten: tidak ada ALTER/DROP kolom, hanya menimpa nilai id.
-- Link lama tidak resolve lagi — pemilik yang membuka URL lamanya akan
-- di-redirect ke URL baru oleh halaman /mitra/[partnerId].

BEGIN;

-- Alfabet: 31 karakter (huruf kecil + angka, tanpa 0/1/i/l/o/u).
CREATE OR REPLACE FUNCTION public._random_short_id()
RETURNS text LANGUAGE sql VOLATILE AS $$
  SELECT string_agg(substr('23456789abcdefghjkmnpqrstuvwxyz',
                           (floor(random() * 31) + 1)::int, 1), '')
    FROM generate_series(1, 6);
$$;

-- Backfill partners.public_id → 6 karakter, retry saat collision.
-- Sengaja tanpa WHERE: semua baris termasuk yang public_id-nya NULL.
DO $$
DECLARE
  rec RECORD;
  candidate text;
  attempts int;
BEGIN
  FOR rec IN SELECT id FROM public.partners LOOP
    attempts := 0;
    LOOP
      candidate := public._random_short_id();
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

-- Backfill brands.public_id → 6 karakter, retry saat collision.
-- Sengaja tanpa WHERE: semua baris termasuk yang public_id-nya NULL.
DO $$
DECLARE
  rec RECORD;
  candidate text;
  attempts int;
BEGIN
  FOR rec IN SELECT id FROM public.brands LOOP
    attempts := 0;
    LOOP
      candidate := public._random_short_id();
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

COMMIT;
