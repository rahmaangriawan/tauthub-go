-- 176_article_images.sql
-- Galeri gambar hasil AI (cover & inline) untuk Taut Pintar Writer.
--
-- Isolasi wajib: satu baris hanya boleh tampil untuk scope (brand, campaign,
-- media, article) yang sama — gambar milik artikel A tidak pernah muncul di
-- pustaka artikel B. scope_key disimpan agar filter bisa satu indeks.
--
-- Idempoten: aman dijalankan berulang kali.

CREATE TABLE IF NOT EXISTS public.article_images (
    id text PRIMARY KEY DEFAULT gen_random_uuid()::text,
    brand_id character(36) NOT NULL,
    campaign_id character(36) NOT NULL,
    media_id character(36) NOT NULL,
    article_id character(36),
    scope_key text NOT NULL,
    url text NOT NULL,
    provider character varying(40) NOT NULL DEFAULT 'placeholder',
    model character varying(120),
    prompt text NOT NULL DEFAULT '',
    heading text,
    role character varying(16) NOT NULL DEFAULT 'inline',
    width integer NOT NULL DEFAULT 0,
    height integer NOT NULL DEFAULT 0,
    cost_credits integer NOT NULL DEFAULT 0,
    created_by character(36),
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT ck_article_images_role CHECK (role IN ('cover', 'inline'))
);

CREATE INDEX IF NOT EXISTS idx_article_images_scope
    ON public.article_images (brand_id, campaign_id, media_id, article_id);
CREATE INDEX IF NOT EXISTS idx_article_images_scope_key
    ON public.article_images (scope_key);
CREATE INDEX IF NOT EXISTS idx_article_images_created
    ON public.article_images (created_at DESC);

-- credit_transactions.reason semula dibatasi 4 nilai. Tambahkan
-- 'ai.image.generate' agar pemakaian kredit paket untuk gambar AI tercatat.
ALTER TABLE public.credit_transactions DROP CONSTRAINT IF EXISTS ck_credit_tx_reason;
ALTER TABLE public.credit_transactions
    ADD CONSTRAINT ck_credit_tx_reason
    CHECK (reason IN ('package.grant', 'publication.usage', 'publication.refund', 'adjustment', 'ai.image.generate'));
