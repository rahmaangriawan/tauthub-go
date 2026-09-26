-- Konfigurasi standar PR internal Tulisio (bukan input klien).
-- Klien hanya menerima hasil hitungan; Admin yang mengatur rate di sini.
CREATE TABLE IF NOT EXISTS pr_config (
  id                text PRIMARY KEY DEFAULT 'default',
  rate_per_1k_reach integer NOT NULL DEFAULT 5000,
  updated_by        character(36) REFERENCES users(id),
  updated_at        timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT ck_pr_config_id CHECK (id = 'default'),
  CONSTRAINT ck_pr_config_rate CHECK (rate_per_1k_reach >= 0)
);

INSERT INTO pr_config (id, rate_per_1k_reach)
VALUES ('default', 5000)
ON CONFLICT (id) DO NOTHING;

-- Tabel per-brand yang dibuat saat konsep "input klien" — tidak dipakai lagi
-- karena standarisasi PR adalah engine internal Tulisio.
DROP TABLE IF EXISTS brand_pr_standards;
