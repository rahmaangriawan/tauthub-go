-- 053: SumoPod payment gateway (sandbox → live).
-- Tambahkan 'sumopod' ke daftar provider yang diizinkan pada
-- payment_gateway_configs.provider dan payment_transactions.gateway_provider.

ALTER TABLE public.payment_gateway_configs
  DROP CONSTRAINT IF EXISTS ck_payment_gateway_provider;

ALTER TABLE public.payment_gateway_configs
  ADD CONSTRAINT ck_payment_gateway_provider CHECK (
    provider IN ('xendit','duitku','midtrans','tripay','sandbox','sumopod')
  );

ALTER TABLE public.payment_transactions
  DROP CONSTRAINT IF EXISTS ck_payment_transactions_gateway;

-- Nama lama (schema.pg.sql / 034_sandbox_gateway.sql) masih dipakai pada
-- database yang dibuat sebelum migration ini; tanpa drop, constraint lama
-- tetap memblokir gateway_provider = 'sumopod' meski constraint baru dibuat.
ALTER TABLE public.payment_transactions
  DROP CONSTRAINT IF EXISTS ck_payment_transaction_provider;

ALTER TABLE public.payment_transactions
  ADD CONSTRAINT ck_payment_transactions_gateway CHECK (
    gateway_provider IN ('xendit','duitku','midtrans','tripay','sandbox','sumopod')
  );
