-- Standar PR per tier media (Tier 1/2/3) + standar per brand (admin-managed).
-- Prinsip: standarisasi PR adalah engine internal Tulisio — klien hanya
-- menerima angka. Admin yang mengatur rate global, rate per tier, dan
-- override per brand (bila diperlukan). Nilai lama tidak berubah karena
-- rate disimpan dan dipakai saat laporan dihitung.

-- Rate per tier di pr_config (NULL = fallback ke rate global).
ALTER TABLE pr_config ADD COLUMN IF NOT EXISTS rate_tier1 integer;
ALTER TABLE pr_config ADD COLUMN IF NOT EXISTS rate_tier2 integer;
ALTER TABLE pr_config ADD COLUMN IF NOT EXISTS rate_tier3 integer;

-- Standar PR per brand (admin-managed). NULL pada rate = pakai rate global/
-- tier dari pr_config. updated_by/updated_at mencatat siapa terakhir ubah.
CREATE TABLE IF NOT EXISTS brand_pr_standards (
  brand_id           character(36) PRIMARY KEY REFERENCES brands(id) ON DELETE CASCADE,
  rate_per_1k_reach  integer,
  rate_tier1         integer,
  rate_tier2         integer,
  rate_tier3         integer,
  updated_by         character(36) REFERENCES users(id),
  updated_at         timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT ck_brand_pr_rates CHECK (
    (rate_per_1k_reach IS NULL OR rate_per_1k_reach >= 0)
    AND (rate_tier1 IS NULL OR rate_tier1 >= 0)
    AND (rate_tier2 IS NULL OR rate_tier2 >= 0)
    AND (rate_tier3 IS NULL OR rate_tier3 >= 0)
  )
);
