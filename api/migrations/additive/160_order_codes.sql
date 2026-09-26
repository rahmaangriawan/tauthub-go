-- 160: Nomor order yang bisa dibaca manusia untuk kampanye & publikasi.
--
-- Checkout kampanye klien membuat satu baris publications per media terpilih.
-- Setiap baris itu kini punya nomor order sendiri, dan kampanyenya punya nomor
-- order induk — dipakai klien, mitra, dan admin untuk tracking masalah per
-- order ("yang gagal itu ORD-202609-01042") tanpa menyebarkan UUID mentah.
--
-- Format, mengikuti pola PAY-YYYYMM-NNNNN (migrasi 035, sequence race-free):
--   kampanye   : CAM-YYYYMM-NNNNN
--   publikasi  : ORD-YYYYMM-NNNNN   (per media = satu order distribusi)
--
-- Sequence terpisah per tabel; baris historis di-backfill kronologis.
--
-- Aditif: dua kolom + dua sequence + dua unique index parsial.

BEGIN;

CREATE SEQUENCE IF NOT EXISTS public.publication_order_seq START 1000;
CREATE SEQUENCE IF NOT EXISTS public.campaign_order_seq START 1000;

ALTER TABLE public.publications ADD COLUMN IF NOT EXISTS order_code VARCHAR(40);
ALTER TABLE public.campaigns    ADD COLUMN IF NOT EXISTS order_code VARCHAR(40);

-- Backfill historis (kronologis → penomoran stabil).
WITH ordered AS (
  SELECT id, created_at, row_number() OVER (ORDER BY created_at, id) AS n
  FROM publications WHERE order_code IS NULL
)
UPDATE publications p
SET order_code = 'ORD-' || to_char(o.created_at, 'YYYYMM') || '-' || lpad((1000 + o.n)::text, 5, '0')
FROM ordered o WHERE p.id = o.id;

WITH ordered AS (
  SELECT id, created_at, row_number() OVER (ORDER BY created_at, id) AS n
  FROM campaigns WHERE order_code IS NULL
)
UPDATE campaigns c
SET order_code = 'CAM-' || to_char(o.created_at, 'YYYYMM') || '-' || lpad((1000 + o.n)::text, 5, '0')
FROM ordered o WHERE c.id = o.id;

-- Majukan sequence melewati backfill (setval 3-arg → nextval = max+1).
SELECT setval('public.publication_order_seq',
              (SELECT COALESCE(MAX(right(order_code, 5)::int), 1000) FROM publications WHERE order_code ~ '^ORD-'),
              TRUE);
SELECT setval('public.campaign_order_seq',
              (SELECT COALESCE(MAX(right(order_code, 5)::int), 1000) FROM campaigns WHERE order_code ~ '^CAM-'),
              TRUE);

CREATE UNIQUE INDEX IF NOT EXISTS uq_publication_order_code
  ON public.publications (order_code) WHERE order_code IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_campaign_order_code
  ON public.campaigns (order_code) WHERE order_code IS NOT NULL;

COMMIT;
