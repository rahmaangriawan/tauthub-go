-- 321 — Bonus top-up. Bonus dicatat sebagai baris ledger terpisah (ref_type 'bonus'),
-- tidak mengubah nominal pembayaran, faktur, maupun pendapatan top-up.
--   promo_codes: kode promo (bonus TC tetap, minimal top-up, batas pakai, masa berlaku)
--   promo_redemptions: satu brand hanya sekali per kode
--   payment_transactions.promo_code: kode yang dipilih klien saat membuat top-up
-- Paket bonus disimpan di tc_config.topup_packages sebagai bonus_tc (opsional).
-- Additive dan idempoten.

BEGIN;

ALTER TABLE public.credit_ledger_entries DROP CONSTRAINT IF EXISTS ck_credit_ledger_type;
ALTER TABLE public.credit_ledger_entries ADD CONSTRAINT ck_credit_ledger_type
  CHECK (((ref_type)::text = ANY ((ARRAY[
    'topup', 'reserve', 'release', 'capture', 'refund', 'adjustment', 'bonus'
  ])::text[])));

ALTER TABLE public.payment_transactions ADD COLUMN IF NOT EXISTS promo_code varchar(40);

CREATE TABLE IF NOT EXISTS public.promo_codes (
  code varchar(40) PRIMARY KEY,
  bonus_tc integer NOT NULL CHECK (bonus_tc > 0 AND bonus_tc <= 1000000),
  min_topup_tc integer NOT NULL DEFAULT 1 CHECK (min_topup_tc >= 1),
  max_uses integer CHECK (max_uses IS NULL OR max_uses > 0),
  used_count integer NOT NULL DEFAULT 0,
  valid_until timestamptz,
  active boolean NOT NULL DEFAULT TRUE,
  note text,
  created_by character(36) REFERENCES public.users(id),
  created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS public.promo_redemptions (
  code varchar(40) NOT NULL REFERENCES public.promo_codes(code),
  brand_id character(36) NOT NULL REFERENCES public.brands(id) ON DELETE CASCADE,
  payment_transaction_id character(36) NOT NULL,
  bonus_tc integer NOT NULL,
  created_at timestamptz NOT NULL DEFAULT NOW(),
  PRIMARY KEY (code, brand_id)
);

COMMIT;
