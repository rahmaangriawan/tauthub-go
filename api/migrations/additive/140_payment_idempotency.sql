-- 140 — Proteksi double request & double payment (idempotency).
-- - payment_transactions.idempotency_key: kunci intent dari frontend; unique
--   per brand, jadi satu intent = satu transaksi walau request masuk bersamaan
--   (konflik unique ditangani server dengan mengembalikan transaksi existing).
-- - payment_webhook_events: log dedup webhook — kombinasi provider+transaksi+
--   status hanya diproses satu kali (replay webhook = no-op).

BEGIN;

ALTER TABLE public.payment_transactions
  ADD COLUMN IF NOT EXISTS idempotency_key varchar(100);

CREATE UNIQUE INDEX IF NOT EXISTS uq_payment_transactions_brand_idem
  ON public.payment_transactions (brand_id, idempotency_key)
  WHERE idempotency_key IS NOT NULL;

CREATE TABLE IF NOT EXISTS public.payment_webhook_events (
    id                      character(36) NOT NULL,
    gateway_provider        character varying(20) NOT NULL,
    external_transaction_id character varying(120) NOT NULL,
    status                  character varying(20) NOT NULL,
    payload                 jsonb,
    created_at              timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT payment_webhook_events_pkey PRIMARY KEY (id),
    CONSTRAINT uq_payment_webhook_event UNIQUE (gateway_provider, external_transaction_id, status)
);

COMMIT;
