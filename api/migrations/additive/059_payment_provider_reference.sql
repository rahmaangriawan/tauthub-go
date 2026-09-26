-- Migration: 059_payment_provider_reference
-- Simpan referensi pembayaran dari sisi provider (mis. payment_id SumoPod)
-- supaya status pembayaran bisa diverifikasi server-side (GET
-- /api/v1/public/payments/{payment_id}/status) bila webhook tidak sampai.
--
-- Additive: tidak menghapus/mengubah kolom atau data yang ada.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/059_payment_provider_reference.sql

BEGIN;

ALTER TABLE public.payment_transactions
  ADD COLUMN IF NOT EXISTS provider_reference VARCHAR(120);

COMMIT;
