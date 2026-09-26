-- 095 — partner_service_charges: biaya layanan yang didebet dari saldo IDR mitra.
-- Additive, backward compatible. Tidak mengubah partner_earnings.
-- Contoh kasus: AI rewrite artikel (Rp 1.000 per pemakaian) oleh Mitra.
-- Saldo tersedia Mitra = SUM(partner_earnings.amount WHERE status='settled' AND settlement_id IS NULL)
--                       - SUM(partner_service_charges.amount_idr)
-- (dijepit >= 0). Lihat lib/finance/service.ts -> partnerFinanceSummary.

BEGIN;

CREATE TABLE IF NOT EXISTS public.partner_service_charges (
  id character(36) PRIMARY KEY,
  partner_id character(36) NOT NULL REFERENCES public.partners(id) ON DELETE CASCADE,
  service character varying(40) NOT NULL,
  amount_idr numeric(14,2) NOT NULL CHECK (amount_idr >= 0),
  reference_id character varying(100),
  reference_type character varying(40),
  notes text,
  created_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_partner_service_charges_partner
  ON public.partner_service_charges(partner_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_partner_service_charges_service
  ON public.partner_service_charges(service, created_at DESC);

GRANT SELECT, INSERT ON public.partner_service_charges TO ai_writer;

COMMIT;
