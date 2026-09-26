-- 092 — VIP masa kontrak (additive, backward compatible).
-- Tidak ada DROP/TRUNCATE/DELETE. Semua ALTER IF NOT EXISTS.
--
-- vip_contract_months : durasi kontrak VIP dalam bulan (NULL = tanpa batas/lifetime).
-- vip_expires_at      : waktu berakhir kontrak = vip_enabled_at + (bulan).
-- Saat vip_expires_at <= NOW() dan is_vip = true, entitlement otomatis
-- dianggap nonaktif (auto-switch ke akun mitra biasa) oleh
-- lib/vip/entitlement.ts (lazy persist).

BEGIN;

ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS vip_contract_months integer;

ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS vip_expires_at timestamp with time zone;

CREATE INDEX IF NOT EXISTS ix_partners_vip_expires
  ON public.partners(vip_expires_at)
  WHERE is_vip = true;

COMMIT;
