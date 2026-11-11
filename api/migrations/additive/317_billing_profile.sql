-- 317 — Profil penagihan per brand, PO, dan invoice otomatis untuk top-up berfaktur.
--
-- billing_profiles: data perusahaan dan NPWP disimpan sekali per brand, supaya
-- klien tidak mengisi ulang di tiap top-up. NPWP adalah data sensitif: hanya
-- peran writer ke atas yang boleh membacanya (dijaga di API).
-- invoices: kolom po_number, bill_to (salinan data penagihan saat invoice dibuat,
-- supaya invoice lama tidak berubah bila profil diganti), dan
-- payment_transaction_id (unik) agar satu pembayaran hanya menghasilkan satu
-- invoice walau webhook datang dua kali.
-- Additive dan idempoten.

BEGIN;

CREATE TABLE IF NOT EXISTS public.billing_profiles (
  brand_id      character(36) PRIMARY KEY REFERENCES public.brands (id) ON DELETE CASCADE,
  company_name  character varying(200) NOT NULL,
  npwp          character varying(30)  NOT NULL,
  npwp_address  character varying(500) NOT NULL,
  billing_email character varying(254),
  updated_by    character(36),
  created_at    timestamptz NOT NULL DEFAULT NOW(),
  updated_at    timestamptz NOT NULL DEFAULT NOW()
);

ALTER TABLE public.invoices ADD COLUMN IF NOT EXISTS po_number character varying(60);
ALTER TABLE public.invoices ADD COLUMN IF NOT EXISTS bill_to jsonb;
ALTER TABLE public.invoices ADD COLUMN IF NOT EXISTS payment_transaction_id character(36);

CREATE UNIQUE INDEX IF NOT EXISTS invoices_payment_transaction_key
  ON public.invoices (payment_transaction_id) WHERE payment_transaction_id IS NOT NULL;

COMMIT;
