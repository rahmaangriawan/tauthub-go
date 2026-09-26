-- 081 — Berita Acara kampanye: tautan share publik (tanpa login).
-- Satu tautan aktif per kampanye (uq_campaign_report_campaign): POST share
-- bersifat create-or-return sehingga link yang sudah tersebar tetap valid.
-- Additive: CREATE TABLE IF NOT EXISTS saja. No DROP, no destructive change.

CREATE TABLE IF NOT EXISTS public.campaign_report_shares (
    id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    token character(64) NOT NULL,
    created_by character(36),
    revoked_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT campaign_report_shares_pkey PRIMARY KEY (id),
    CONSTRAINT uq_campaign_report_token UNIQUE (token),
    CONSTRAINT uq_campaign_report_campaign UNIQUE (campaign_id),
    CONSTRAINT fk_campaign_report_share_campaign FOREIGN KEY (campaign_id) REFERENCES public.campaigns(id) ON DELETE CASCADE,
    CONSTRAINT fk_campaign_report_share_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE,
    CONSTRAINT fk_campaign_report_share_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS ix_campaign_report_shares_brand ON public.campaign_report_shares USING btree (brand_id);
