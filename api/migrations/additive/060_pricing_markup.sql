-- Migration: 060_pricing_markup
-- Pricing engine (markup Tulisio di atas harga mitra).
--
-- 1. pricing_config — singleton (id=1) berisi markup_percent (default 15%).
-- 2. publications.partner_price + publications.markup_percent — snapshot harga
--    saat publikasi dibuat (requirement: publikasi lama tidak berubah walau
--    markup diubah kemudian).
--    Backfill: publikasi lama dibuat saat model lama (client_price = harga
--    media, tanpa markup) → partner_price = client_price, markup = 0.
--
-- Additive & backward compatible: tidak menghapus/mengubah data existing.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/060_pricing_markup.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.pricing_config (
    id integer NOT NULL DEFAULT 1,
    markup_percent numeric(5,2) NOT NULL DEFAULT 15,
    updated_by character(36),
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT pricing_config_pkey PRIMARY KEY (id),
    CONSTRAINT ck_pricing_config_singleton CHECK (id = 1),
    CONSTRAINT ck_pricing_config_markup CHECK (markup_percent >= 0)
);

INSERT INTO public.pricing_config (id, markup_percent)
VALUES (1, 15)
ON CONFLICT (id) DO NOTHING;

ALTER TABLE public.publications
    ADD COLUMN IF NOT EXISTS partner_price numeric(14,2),
    ADD COLUMN IF NOT EXISTS markup_percent numeric(5,2);

-- Backfill historis: model lama tidak punya markup → partner_price = harga klien.
UPDATE public.publications
   SET partner_price = COALESCE(partner_price, client_price),
       markup_percent = COALESCE(markup_percent, 0)
 WHERE partner_price IS NULL OR markup_percent IS NULL;

COMMIT;
