-- 011_partner_connections.sql
-- Media partner dapat memiliki koneksi WordPress/webhook sendiri
-- (cms_connections.partner_id) sehingga publikasi ke media partner
-- dikirim lewat koneksi mitra, bukan koneksi brand.
--
-- Additive: brand connections existing tetap utuh (brand_id tidak diubah).
-- =====================================================================

ALTER TABLE public.cms_connections ADD COLUMN IF NOT EXISTS partner_id character(36);

-- Koneksi milik partner tidak terikat ke brand.
ALTER TABLE public.cms_connections ALTER COLUMN brand_id DROP NOT NULL;

-- Koneksi webhook tidak memakai username (kredensial = shared secret saja).
ALTER TABLE public.cms_connections ALTER COLUMN username DROP NOT NULL;

-- Setiap koneksi harus dimiliki tepat satu pemilik (brand ATAU partner).
ALTER TABLE public.cms_connections
    DROP CONSTRAINT IF EXISTS ck_cms_connections_owner;
ALTER TABLE public.cms_connections
    ADD CONSTRAINT ck_cms_connections_owner
    CHECK ((brand_id IS NOT NULL) <> (partner_id IS NOT NULL));

ALTER TABLE public.cms_connections
    DROP CONSTRAINT IF EXISTS fk_cms_connections_partner;
ALTER TABLE public.cms_connections
    ADD CONSTRAINT fk_cms_connections_partner
    FOREIGN KEY (partner_id) REFERENCES public.partners (id);

-- Nama unik per partner (partner_id NULL tidak ikut serta karena brand
-- connection tidak punya partner_id).
CREATE UNIQUE INDEX IF NOT EXISTS uq_cms_connection_partner_name
    ON public.cms_connections (partner_id, name)
    WHERE partner_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_cms_connection_partner_status
    ON public.cms_connections (partner_id, status);
