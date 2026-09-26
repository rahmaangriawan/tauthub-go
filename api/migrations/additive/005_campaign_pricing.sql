-- =====================================================================
-- Migration 005 - Campaign & Pricing (Phase 5)
--
-- Additive migration for databases that already ran 001-004.
--
--   * campaigns                 business container for articles + publications
--     (quota artikel + budget, scoped ke brand/client)
--   * packages                  credit package milik brand (credit_total,
--     credit_used, price)
--   * credit_transactions       ledger: grant (+) / usage (-) / refund (+),
--     dengan balance_after (snapshot audit)
--   * publications.campaign_id  publikasi single/package dalam campaign
--   * media.credit_cost         biaya credit per publikasi ke media
--     (media premium bisa lebih mahal dari 1 credit)
--
-- Kebijakan fase ini: quota campaign + pemotongan credit hanya berlaku untuk
-- publikasi yang memiliki campaign_id. Publikasi non-campaign (legacy) tidak
-- berubah.
--
-- Idempotent (ADD COLUMN IF NOT EXISTS / CREATE TABLE IF NOT EXISTS).
--
-- Apply with:
--   psql "$DATABASE_URL" -f apps/web/src/db/migrations/005_campaign_pricing.sql
-- =====================================================================

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS credit_cost smallint DEFAULT 1 NOT NULL;

ALTER TABLE public.publications ADD COLUMN IF NOT EXISTS campaign_id character(36);

CREATE TABLE IF NOT EXISTS public.campaigns (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    name character varying(160) NOT NULL,
    description text,
    status character varying(20) DEFAULT 'draft' NOT NULL,
    article_quota integer DEFAULT 0 NOT NULL,
    budget numeric(14,2) DEFAULT 0 NOT NULL,
    starts_at timestamp(6) with time zone,
    ends_at timestamp(6) with time zone,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT campaigns_pkey PRIMARY KEY (id),
    CONSTRAINT fk_campaigns_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id),
    CONSTRAINT fk_campaigns_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_campaign_status CHECK (status IN ('draft','active','paused','completed','cancelled')),
    CONSTRAINT ck_campaign_quota CHECK (article_quota >= 0),
    CONSTRAINT ck_campaign_budget CHECK (budget >= 0)
);

CREATE TABLE IF NOT EXISTS public.packages (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    name character varying(160) NOT NULL,
    credit_total integer NOT NULL,
    credit_used integer DEFAULT 0 NOT NULL,
    price numeric(14,2) DEFAULT 0 NOT NULL,
    status character varying(20) DEFAULT 'active' NOT NULL,
    starts_at timestamp(6) with time zone,
    ends_at timestamp(6) with time zone,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT packages_pkey PRIMARY KEY (id),
    CONSTRAINT fk_packages_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id),
    CONSTRAINT fk_packages_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_package_status CHECK (status IN ('active','exhausted','revoked')),
    CONSTRAINT ck_package_credit_total CHECK (credit_total > 0),
    CONSTRAINT ck_package_credit_used CHECK (credit_used >= 0)
);

CREATE TABLE IF NOT EXISTS public.credit_transactions (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    package_id character(36),
    publication_id character(36),
    amount integer NOT NULL,
    reason character varying(50) NOT NULL,
    balance_after integer NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT credit_transactions_pkey PRIMARY KEY (id),
    CONSTRAINT fk_credit_tx_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id),
    CONSTRAINT fk_credit_tx_package FOREIGN KEY (package_id) REFERENCES public.packages(id) ON DELETE SET NULL,
    CONSTRAINT fk_credit_tx_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE SET NULL,
    CONSTRAINT fk_credit_tx_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_credit_tx_reason CHECK (reason IN ('package.grant','publication.usage','publication.refund','adjustment')),
    CONSTRAINT ck_credit_tx_amount CHECK (amount <> 0)
);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_publications_campaign') THEN
        ALTER TABLE public.publications ADD CONSTRAINT fk_publications_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS ix_campaign_brand_status ON public.campaigns USING btree (brand_id, status);

CREATE INDEX IF NOT EXISTS ix_package_brand_status ON public.packages USING btree (brand_id, status);

CREATE INDEX IF NOT EXISTS ix_credit_tx_brand_time ON public.credit_transactions USING btree (brand_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_credit_tx_publication ON public.credit_transactions USING btree (publication_id);

CREATE INDEX IF NOT EXISTS ix_publication_campaign ON public.publications USING btree (campaign_id);
