-- Migration: 034_sandbox_payment_gateway
-- Adds a local "sandbox" payment provider used for dev/E2E testing of the
-- top-up flow without external gateway accounts (Xendit/Midtrans sandbox
-- still require real merchant credentials). The sandbox adapter issues a
-- payment_url to /payments/sandbox/{tx} where the payment can be simulated;
-- its webhook uses an HMAC-SHA256 signature (x-sandbox-signature).
--
-- Only the provider CHECK constraints change — no new tables.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/034_sandbox_gateway.sql

BEGIN;

-- Catatan: 'sumopod' dimasukkan di sini (walau secara resmi ditambahkan oleh
-- migration 053) supaya migration ini bisa di-replay aman pada database yang
-- sudah punya baris provider='sumopod' — tanpa itu, ADD CONSTRAINT di bawah
-- gagal dengan "check constraint is violated by some row".
ALTER TABLE public.payment_gateway_configs DROP CONSTRAINT IF EXISTS ck_payment_gateway_provider;
ALTER TABLE public.payment_gateway_configs
  ADD CONSTRAINT ck_payment_gateway_provider CHECK (
    provider IN ('xendit','duitku','midtrans','tripay','sandbox','sumopod')
  );

ALTER TABLE public.payment_transactions DROP CONSTRAINT IF EXISTS ck_payment_transaction_provider;
ALTER TABLE public.payment_transactions
  ADD CONSTRAINT ck_payment_transaction_provider CHECK (
    gateway_provider IN ('xendit','duitku','midtrans','tripay','sandbox','sumopod')
  );

COMMIT;
