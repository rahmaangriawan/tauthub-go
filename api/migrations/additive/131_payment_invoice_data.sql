-- 131 — data faktur pajak (NPWP) dari modal topup: modal mengirim invoice
-- (company_name/npwp/npwp_address) tetapi backend membuangnya. Simpan di
-- payment_transactions supaya Finance bisa menerbitkan faktur.
BEGIN;

ALTER TABLE public.payment_transactions ADD COLUMN IF NOT EXISTS invoice_data jsonb;

COMMIT;
