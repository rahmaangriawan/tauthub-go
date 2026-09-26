-- 082 — Penarikan dana mitra: biaya transfer (bank/ewallet), saldo minimal 50rb,
-- dan penyimpanan fee per settlement (untuk invoice).
-- Additive: ADD COLUMN IF NOT EXISTS + UPDATE default. No DROP, no destructive change.

-- 1. Konfigurasi biaya transfer (per 1 penarikan).
ALTER TABLE public.tc_config ADD COLUMN IF NOT EXISTS withdraw_bank_fee_idr integer NOT NULL DEFAULT 2400;
ALTER TABLE public.tc_config ADD COLUMN IF NOT EXISTS withdraw_ewallet_fee_percent numeric(5,2) NOT NULL DEFAULT 0.7;

-- 2. Saldo minimal penarikan: 10.000 -> 50.000 (nilai lama dipertahankan bila
--    admin sudah mengeset lebih tinggi lewat panel konfigurasi).
UPDATE public.tc_config SET minimum_withdraw_idr = 50000 WHERE id = 1 AND minimum_withdraw_idr < 50000;

-- 3. Settlement menyimpan fee & metode pembayaran (untuk invoice penarikan).
ALTER TABLE public.settlements ADD COLUMN IF NOT EXISTS payout_method character varying(10) NOT NULL DEFAULT 'bank';
ALTER TABLE public.settlements ADD COLUMN IF NOT EXISTS transfer_fee_idr integer NOT NULL DEFAULT 0;
ALTER TABLE public.settlements ADD COLUMN IF NOT EXISTS net_amount_idr integer NOT NULL DEFAULT 0;
ALTER TABLE public.settlements ADD COLUMN IF NOT EXISTS invoice_number character varying(40);

CREATE UNIQUE INDEX IF NOT EXISTS uq_settlement_invoice_number ON public.settlements (invoice_number);
