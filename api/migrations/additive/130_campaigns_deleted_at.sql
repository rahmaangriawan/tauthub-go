-- 130 — soft-delete kolom: schema.pg.sql sudah mendefinisikan deleted_at,
-- tapi DB lama tidak punya; halaman campaign (page.tsx) memfilter deleted_at IS NULL.
BEGIN;

ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS deleted_at timestamp(6) with time zone;
ALTER TABLE public.brands ADD COLUMN IF NOT EXISTS deleted_at timestamp(6) with time zone;

COMMIT;
