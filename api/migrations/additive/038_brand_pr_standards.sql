-- Brand PR standard: input klien untuk standarisasi perhitungan
-- "Estimated PR value" & "Estimated saving" di Analytics & Reports.
-- Client menentukan rate per 1.000 estimasi reach (Rp); Tulisio memakai
-- rate ini untuk menghitung nilai earned media dari estimasi reach media
-- yang benar-benar terbit. Nilai lama tidak berubah karena rate disimpan
-- per brand dan dipakai saat query dijalankan.
CREATE TABLE IF NOT EXISTS brand_pr_standards (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id      character(36) NOT NULL REFERENCES brands(id) ON DELETE CASCADE,
  rate_per_1k_reach integer NOT NULL DEFAULT 1000,
  updated_by    character(36) REFERENCES users(id),
  created_at    timestamptz NOT NULL DEFAULT NOW(),
  updated_at    timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT uq_brand_pr_standards_brand UNIQUE (brand_id)
);
