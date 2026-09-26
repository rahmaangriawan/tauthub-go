-- =====================================================================
-- Migration 006 - Finance (Phase 6)
--
-- Additive migration for databases that already ran 001-005.
--
--   * revenue_shares    rule split Tulisio/Mitra (percentage atau fixed).
--     Prioritas resolusi: media-specific > partner-specific > platform
--     default (default 65/35 TULISIO:MITRA, TIDAK dihardcode di kode —
--     admin membuat baris default platform bila ingin mengubahnya).
--   * partner_earnings  pendapatan partner per publikasi yang published
--     (status pending → settled → voided).
--   * settlements       payout batch per partner per periode
--     (pending/approved/processing/paid/failed).
--   * transactions      ledger keuangan: publication.revenue,
--     publication.refund, settlement.payout, adjustment.
--   * invoices          invoice per brand (draft/issued/paid/void).
--
-- Idempotent (ADD COLUMN IF NOT EXISTS / CREATE TABLE IF NOT EXISTS).
--
-- Apply with:
--   psql "$DATABASE_URL" -f apps/web/src/db/migrations/006_finance.sql
-- =====================================================================

CREATE TABLE IF NOT EXISTS public.revenue_shares (
    id character(36) NOT NULL,
    partner_id character(36),
    media_id character(36),
    name character varying(160) NOT NULL,
    share_type character varying(20) DEFAULT 'percentage' NOT NULL,
    tulisio_percent numeric(5,2) DEFAULT 65 NOT NULL,
    partner_percent numeric(5,2) DEFAULT 35 NOT NULL,
    partner_fixed numeric(14,2) DEFAULT 0 NOT NULL,
    status character varying(20) DEFAULT 'active' NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT revenue_shares_pkey PRIMARY KEY (id),
    CONSTRAINT fk_revenue_shares_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE,
    CONSTRAINT fk_revenue_shares_media FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE,
    CONSTRAINT fk_revenue_shares_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_revenue_share_type CHECK (share_type IN ('percentage','fixed')),
    CONSTRAINT ck_revenue_share_status CHECK (status IN ('active','archived')),
    CONSTRAINT ck_revenue_share_percent CHECK (partner_percent >= 0 AND partner_percent <= 100 AND tulisio_percent >= 0 AND tulisio_percent <= 100),
    CONSTRAINT ck_revenue_share_fixed CHECK (partner_fixed >= 0)
);

CREATE TABLE IF NOT EXISTS public.partner_earnings (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    publication_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    rule_id character(36),
    amount numeric(14,2) NOT NULL,
    client_price numeric(14,2) NOT NULL,
    status character varying(20) DEFAULT 'pending' NOT NULL,
    settlement_id character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT partner_earnings_pkey PRIMARY KEY (id),
    CONSTRAINT uq_partner_earnings_publication UNIQUE (publication_id),
    CONSTRAINT fk_partner_earnings_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE,
    CONSTRAINT fk_partner_earnings_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE,
    CONSTRAINT fk_partner_earnings_media FOREIGN KEY (media_id) REFERENCES public.media(id),
    CONSTRAINT fk_partner_earnings_rule FOREIGN KEY (rule_id) REFERENCES public.revenue_shares(id) ON DELETE SET NULL,
    CONSTRAINT ck_partner_earnings_status CHECK (status IN ('pending','settled','voided')),
    CONSTRAINT ck_partner_earnings_amount CHECK (amount >= 0)
);

CREATE TABLE IF NOT EXISTS public.settlements (
    id character(36) NOT NULL,
    partner_id character(36) NOT NULL,
    period_start date NOT NULL,
    period_end date NOT NULL,
    status character varying(20) DEFAULT 'pending' NOT NULL,
    total_amount numeric(14,2) DEFAULT 0 NOT NULL,
    notes text,
    paid_at timestamp(6) with time zone,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT settlements_pkey PRIMARY KEY (id),
    CONSTRAINT fk_settlements_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE CASCADE,
    CONSTRAINT fk_settlements_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_settlement_status CHECK (status IN ('pending','approved','processing','paid','failed')),
    CONSTRAINT ck_settlement_amount CHECK (total_amount >= 0),
    CONSTRAINT ck_settlement_period CHECK (period_end >= period_start)
);

CREATE TABLE IF NOT EXISTS public.transactions (
    id character(36) NOT NULL,
    brand_id character(36),
    partner_id character(36),
    publication_id character(36),
    campaign_id character(36),
    amount numeric(14,2) NOT NULL,
    type character varying(30) NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT transactions_pkey PRIMARY KEY (id),
    CONSTRAINT fk_transactions_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id),
    CONSTRAINT fk_transactions_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id),
    CONSTRAINT fk_transactions_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE SET NULL,
    CONSTRAINT fk_transactions_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL,
    CONSTRAINT fk_transactions_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_transaction_type CHECK (type IN ('publication.revenue','publication.refund','settlement.payout','adjustment')),
    CONSTRAINT ck_transaction_amount CHECK (amount <> 0)
);

CREATE TABLE IF NOT EXISTS public.invoices (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    campaign_id character(36),
    number character varying(40) NOT NULL,
    status character varying(20) DEFAULT 'draft' NOT NULL,
    amount numeric(14,2) NOT NULL,
    issued_at timestamp(6) with time zone,
    due_at timestamp(6) with time zone,
    paid_at timestamp(6) with time zone,
    notes text,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT invoices_pkey PRIMARY KEY (id),
    CONSTRAINT uq_invoice_number UNIQUE (number),
    CONSTRAINT fk_invoices_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id),
    CONSTRAINT fk_invoices_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL,
    CONSTRAINT fk_invoices_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_invoice_status CHECK (status IN ('draft','issued','paid','void')),
    CONSTRAINT ck_invoice_amount CHECK (amount >= 0)
);

ALTER TABLE public.partner_earnings ADD CONSTRAINT fk_partner_earnings_settlement FOREIGN KEY (settlement_id) REFERENCES public.settlements(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS ix_revenue_share_lookup ON public.revenue_shares USING btree (status, media_id, partner_id);

CREATE INDEX IF NOT EXISTS ix_partner_earnings_partner_status ON public.partner_earnings USING btree (partner_id, status, created_at);

CREATE INDEX IF NOT EXISTS ix_settlement_partner_status ON public.settlements USING btree (partner_id, status);

CREATE INDEX IF NOT EXISTS ix_transaction_brand_time ON public.transactions USING btree (brand_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_transaction_partner_time ON public.transactions USING btree (partner_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_invoice_brand_status ON public.invoices USING btree (brand_id, status);
