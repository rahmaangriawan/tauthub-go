-- 325 — Loyalty tier (volume discount). Total belanja seumur hidup brand
-- (SUM capture TC) membuka tier dengan diskon markup. Diskon memotong markup
-- Tauthub; harga partner tidak berubah. Terapkan hanya di checkout + saat charge
-- (harga katalog tetap normal). Additive dan idempoten.

BEGIN;

CREATE TABLE IF NOT EXISTS public.loyalty_tiers (
  name             text PRIMARY KEY,
  min_spend_tc     bigint NOT NULL CHECK (min_spend_tc >= 0),
  discount_percent numeric(5,2) NOT NULL CHECK (discount_percent >= 0 AND discount_percent <= 100),
  sort_order       integer NOT NULL DEFAULT 0
);

INSERT INTO public.loyalty_tiers (name, min_spend_tc, discount_percent, sort_order) VALUES
  ('Bronze',   0,         0, 1),
  ('Silver',   10000000,  3, 2),
  ('Gold',     50000000,  6, 3),
  ('Platinum', 200000000, 10, 4)
ON CONFLICT (name) DO NOTHING;

COMMIT;
