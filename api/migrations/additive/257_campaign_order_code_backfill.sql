-- 257 — CAM- order_code: backfill + trigger BEFORE INSERT.
--
-- Migrasi 160 membackfill campaigns.order_code sekali, tapi tidak ada jalur
-- runtime yang mengisinya (INSERT publikasi punya ORD- di SQL-nya, INSERT
-- kampanye tidak). Kampanye yang dibuat sesudah 160 lari tetap NULL sehingga
-- UI Monitoring Order jatuh ke potongan UUID. Dua hal di sini:
--   1) backfill kronologis baris NULL yang tersisa (format yang sama, 160),
--   2) trigger BEFORE INSERT yang mengisi CAM- dari sequence — semua jalur
--      insert kampanye (baik/legacy/kilat) langsung ber nomor.

BEGIN;

-- 1) Backfill kronologis sisa NULL (idempotent: hanya baris NULL).
WITH ordered AS (
  SELECT id, created_at, row_number() OVER (ORDER BY created_at, id) AS n
  FROM public.campaigns WHERE order_code IS NULL
)
UPDATE public.campaigns c
SET order_code = 'CAM-' || to_char(o.created_at, 'YYYYMM') || '-' || lpad((1000 + o.n)::text, 5, '0')
FROM ordered o WHERE c.id = o.id AND c.order_code IS NULL;

SELECT setval('public.campaign_order_seq',
              GREATEST(
                (SELECT COALESCE(MAX(right(order_code, 5)::int), 1000) FROM public.campaigns WHERE order_code ~ '^CAM-'),
                1000),
              TRUE);

-- 2) Trigger: isi CAM- saat insert jika kolomnya belum diisi.
CREATE OR REPLACE FUNCTION public.set_campaign_order_code() RETURNS trigger AS $$
BEGIN
  IF NEW.order_code IS NULL THEN
    NEW.order_code := 'CAM-' || to_char(COALESCE(NEW.created_at, now()), 'YYYYMM') || '-'
                      || lpad(nextval('public.campaign_order_seq')::text, 5, '0');
  END IF;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_campaign_order_code ON public.campaigns;
CREATE TRIGGER trg_campaign_order_code
  BEFORE INSERT ON public.campaigns
  FOR EACH ROW WHEN (NEW.order_code IS NULL)
  EXECUTE FUNCTION public.set_campaign_order_code();

COMMIT;
