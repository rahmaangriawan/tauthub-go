-- 144_partner_payout_account.sql
-- Kolom payout/rekening di partners (perbaikan PATCH /v1/partner/me 500).
-- Additive + idempoten: aman diulang, tercatat di schema_migrations.
ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS payout_type VARCHAR(16) NOT NULL DEFAULT 'bank';
ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS bank_name VARCHAR(120);
ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS bank_account_number VARCHAR(40);
ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS bank_account_name VARCHAR(160);
