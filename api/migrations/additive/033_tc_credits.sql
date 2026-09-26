-- Migration: 033_tc_credits
-- Tulisio Credit (TC) — internal credit unit for Clients (1 TC = Rp1.000 by
-- default). Adds:
--   - tc_config           : denomination rate + top-up package list (admin-editable)
--   - credit_wallets      : per-brand Available / Reserved / Captured balance
--   - credit_ledger_entries: source-of-truth ledger with rate + IDR snapshots
--                            and a unique idempotency key per business action
--   - payment_transactions.purpose / purpose_ref_id: marks Top Up payments so
--                            verified webhooks can credit the wallet once
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/033_tc_credits.sql

BEGIN;

-- ── TC denomination configuration (single row, id always 1) ─────────────
CREATE TABLE IF NOT EXISTS public.tc_config (
  id              INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  rate_idr        INTEGER NOT NULL DEFAULT 1000 CHECK (rate_idr > 0),
  topup_packages  JSONB NOT NULL DEFAULT '[{"tc":500,"idr":500000},{"tc":1000,"idr":1000000},{"tc":5000,"idr":5000000}]',
  updated_by      UUID,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO public.tc_config (id, rate_idr) VALUES (1, 1000)
ON CONFLICT (id) DO NOTHING;

-- ── Client wallet (per brand) ───────────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.credit_wallets (
  brand_id     CHARACTER(36) PRIMARY KEY REFERENCES public.brands(id) ON DELETE CASCADE,
  available_tc INTEGER NOT NULL DEFAULT 0 CHECK (available_tc >= 0),
  reserved_tc  INTEGER NOT NULL DEFAULT 0 CHECK (reserved_tc >= 0),
  captured_tc  INTEGER NOT NULL DEFAULT 0 CHECK (captured_tc >= 0),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ── Credit ledger (source of truth) ─────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.credit_ledger_entries (
  id               CHARACTER(36) PRIMARY KEY,
  brand_id         CHARACTER(36) NOT NULL REFERENCES public.brands(id) ON DELETE CASCADE,
  ref_type         TEXT NOT NULL CHECK (ref_type IN ('topup','reserve','release','capture','refund','adjustment')),
  ref_id           CHARACTER(36),
  amount_tc        INTEGER NOT NULL,
  balance_after_tc INTEGER NOT NULL,
  rate_idr         INTEGER NOT NULL,
  amount_idr       INTEGER,
  reason           TEXT,
  idempotency_key  TEXT NOT NULL UNIQUE,
  created_by       CHARACTER(36),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_credit_ledger_brand ON public.credit_ledger_entries (brand_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_credit_ledger_ref   ON public.credit_ledger_entries (ref_id, ref_type);

-- ── Mark payment transactions that fund a Top Up ────────────────────────
ALTER TABLE public.payment_transactions ADD COLUMN IF NOT EXISTS purpose       VARCHAR(20) NOT NULL DEFAULT 'generic';
ALTER TABLE public.payment_transactions ADD COLUMN IF NOT EXISTS purpose_ref_id CHARACTER(36);

COMMIT;
