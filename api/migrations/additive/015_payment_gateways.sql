-- Phase 10 - Payment Gateway (added 2026-08-09)
-- =====================================================================
-- Multi-provider payment gateway configuration + payment transactions
-- with a provider snapshot (a transaction keeps the gateway it was
-- created with, even after the active gateway switches).
--
-- payment_gateway_configs  : one row per provider; exactly ONE row may
--                            have is_active = TRUE (enforced by a partial
--                            unique index, not by the application).
-- payment_transactions     : provider-agnostic status machine + snapshot
--                            of the gateway used when the transaction was
--                            created.
-- =====================================================================

CREATE TABLE public.payment_gateway_configs (
    id character(36) NOT NULL,
    provider character varying(20) NOT NULL,
    environment character varying(10) NOT NULL DEFAULT 'sandbox',
    encrypted_credentials text NOT NULL,
    is_configured boolean NOT NULL DEFAULT FALSE,
    is_active boolean NOT NULL DEFAULT FALSE,
    last_tested_at timestamp(6) with time zone,
    last_test_ok boolean,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pk_payment_gateway_configs PRIMARY KEY (id),
    CONSTRAINT uq_payment_gateway_provider UNIQUE (provider),
    CONSTRAINT ck_payment_gateway_provider CHECK (((provider)::text = ANY ((ARRAY['xendit'::character varying, 'duitku'::character varying, 'midtrans'::character varying, 'tripay'::character varying])::text[]))),
    CONSTRAINT ck_payment_gateway_environment CHECK (((environment)::text = ANY ((ARRAY['sandbox'::character varying, 'production'::character varying])::text[])))
);

-- Database-level guarantee: at most ONE active gateway at any time.
-- The constant expression makes this a partial unique index over the whole
-- table, so two concurrent activation transactions cannot both succeed.
CREATE UNIQUE INDEX uq_payment_gateway_single_active ON public.payment_gateway_configs ((TRUE)) WHERE is_active = TRUE;

CREATE TABLE public.payment_transactions (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    brand_id character(36),
    invoice_id character(36),
    gateway_provider character varying(20) NOT NULL,
    gateway_config_id character(36),
    gateway_environment character varying(10) NOT NULL,
    external_transaction_id character varying(120),
    amount numeric(14,2) NOT NULL,
    currency character varying(8) NOT NULL DEFAULT 'IDR',
    status character varying(20) NOT NULL DEFAULT 'pending',
    payment_method character varying(60),
    payment_url character varying(2000),
    paid_at timestamp(6) with time zone,
    expired_at timestamp(6) with time zone,
    raw_callback jsonb,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT pk_payment_transactions PRIMARY KEY (id),
    CONSTRAINT ck_payment_transaction_provider CHECK (((gateway_provider)::text = ANY ((ARRAY['xendit'::character varying, 'duitku'::character varying, 'midtrans'::character varying, 'tripay'::character varying])::text[]))),
    CONSTRAINT ck_payment_transaction_environment CHECK (((gateway_environment)::text = ANY ((ARRAY['sandbox'::character varying, 'production'::character varying])::text[]))),
    CONSTRAINT ck_payment_transaction_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'processing'::character varying, 'paid'::character varying, 'failed'::character varying, 'expired'::character varying, 'cancelled'::character varying, 'refunded'::character varying])::text[]))),
    CONSTRAINT ck_payment_transaction_amount CHECK (amount > 0)
);

-- One external transaction id per provider (webhook/retry idempotency).
CREATE UNIQUE INDEX uq_payment_transaction_external ON public.payment_transactions (gateway_provider, external_transaction_id) WHERE external_transaction_id IS NOT NULL;

CREATE INDEX ix_payment_transactions_user ON public.payment_transactions USING btree (user_id, created_at DESC);
