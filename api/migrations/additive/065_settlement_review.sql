-- Migration: 065_settlement_review
-- Review withdrawal oleh Admin: penolakan wajib disertai alasan, dan
-- pembayaran manual dapat mencatat nomor referensi transfer.
--
-- Reject settlement memakai status 'failed' existing (mekanisme
-- transitionSettlement melepas earning kembali ke pending) — alasan
-- penolakan disimpan di kolom baru rejection_reason agar terlihat Mitra.
--
-- Additive & backward compatible: kolom baru nullable; tidak mengubah
-- data/status existing.
-- Idempotent: ADD COLUMN IF NOT EXISTS.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/065_settlement_review.sql

ALTER TABLE public.settlements
    ADD COLUMN IF NOT EXISTS rejection_reason character varying(1000),
    ADD COLUMN IF NOT EXISTS payment_reference character varying(200);
