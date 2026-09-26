-- Laporan topup gagal dari klien (bukan self-claim — hanya pencatatan, saldo
-- tidak berubah sampai admin menyetujui dan menjalankan adjustCredit).
CREATE TABLE IF NOT EXISTS public.credit_topup_reports (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    claimed_tc integer NOT NULL,
    amount_idr integer,
    payment_reference character varying(200),
    description text NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    review_note text,
    reviewed_by character(36),
    reviewed_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT credit_topup_reports_pkey PRIMARY KEY (id),
    CONSTRAINT ck_credit_topup_report_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'approved'::character varying, 'rejected'::character varying])::text[]))),
    CONSTRAINT ck_credit_topup_report_claimed CHECK (claimed_tc > 0),
    CONSTRAINT fk_credit_topup_reports_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE,
    CONSTRAINT fk_credit_topup_reports_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_credit_topup_reports_brand
    ON public.credit_topup_reports (brand_id, created_at DESC);

CREATE INDEX IF NOT EXISTS ix_credit_topup_reports_status
    ON public.credit_topup_reports (status, created_at DESC);
