-- 313 — Pencatatan penarikan profit Tauthub dari rekening escrow ke rekening
-- operasional. Dashboard admin keuangan memakai jumlah tercatat di sini untuk
-- menghitung "Profit Bersih yang siap ditarik": gross komisi 25% dari semua
-- publikasi 'published' dikurangi jumlah yang sudah tercatat tertarik.
--
-- Aditif dan idempoten.

BEGIN;

CREATE TABLE IF NOT EXISTS public.admin_profit_withdrawals (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    amount        bigint      NOT NULL CHECK (amount > 0),
    note          text        NOT NULL DEFAULT '',
    bank_from     text        NOT NULL DEFAULT '',
    bank_to       text        NOT NULL DEFAULT '',
    reference     text        NOT NULL DEFAULT '',
    receipt_url   text        NOT NULL DEFAULT '',
    status        text        NOT NULL DEFAULT 'paid' CHECK (status IN ('paid', 'void')),
    -- created_by tidak di-FK ke users.id (type uuid vs character(36)); konsisten
    -- dengan pola media_violations (migrasi 292): hanya catatan auditable.
    created_by    uuid,
    created_at    timestamptz NOT NULL DEFAULT now(),
    voided_at     timestamptz,
    void_reason   text        NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_admin_profit_withdrawals_created_at
    ON public.admin_profit_withdrawals (created_at DESC);

CREATE INDEX IF NOT EXISTS idx_admin_profit_withdrawals_status
    ON public.admin_profit_withdrawals (status);

COMMIT;
