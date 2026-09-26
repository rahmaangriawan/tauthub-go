-- 163 — VIP Reguler: tier VIP kedua (bukan Brand Ambassador).
-- Aditif & backward compatible. Tidak ada DROP data / TRUNCATE / DELETE.
--
-- Aturan tier ini:
--   - Fee per domain referral: Rp10.000 untuk domain ke-1..1000 sejak aktivasi,
--     Rp5.000 untuk domain ke-1001 dst.
--   - Kontrak 60 hari (vip_contract_days) — habis masa kontrak, hak VIP mati.
--   - Komisi transaksi tetap 1% (sama seperti mitra biasa) dan TIDAK dibatasi
--     kontrak; karena itu commission_rate_bp diset 100 saat aktivasi.
--   - Tanpa entitlement tambahan (Dashboard Klien/AI Premium/SEO Tools).

BEGIN;

-- 1) Izinkan nilai vip_tier baru. Constraint lama dideklarasikan inline di
--    migrasi 088 dengan nama auto: partners_vip_tier_check.
ALTER TABLE public.partners DROP CONSTRAINT IF EXISTS partners_vip_tier_check;
ALTER TABLE public.partners ADD CONSTRAINT partners_vip_tier_check
  CHECK (vip_tier IN ('regular', 'brand_ambassador', 'vip_reguler'));

-- 2) Masa kontrak dalam hari (eksak). vip_contract_months tetap dipertahankan
--    untuk kontrak berbasis bulan milik Brand Ambassador.
ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS vip_contract_days integer;

-- 3) Rate CPA bertingkat untuk VIP Reguler. Default mengikuti kesepakatan:
--    1000 domain pertama Rp10.000, setelahnya Rp5.000.
ALTER TABLE public.affiliate_settings
  ADD COLUMN IF NOT EXISTS cpa_tier_threshold integer NOT NULL DEFAULT 1000,
  ADD COLUMN IF NOT EXISTS cpa_tier1_rate    integer NOT NULL DEFAULT 10000,
  ADD COLUMN IF NOT EXISTS cpa_tier2_rate    integer NOT NULL DEFAULT 5000;

-- 4) referred_domains.updated_at dipakai route admin
--    (/v1/platform/referred-domains PUT) tapi belum pernah ada kolomnya.
--    Tanpa kolom ini, approve/tolak domain gagal di runtime.
ALTER TABLE public.referred_domains
  ADD COLUMN IF NOT EXISTS updated_at timestamp with time zone NOT NULL DEFAULT now();

COMMIT;
