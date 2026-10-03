-- 182_ai_image_credit_cost.sql
-- Harga kredit per gambar AI yang bisa diatur admin lewat tab "Atur Harga" di
-- /admin/taut-pintar/ai-image, menggantikan ketergantungan pada env
-- AI_IMAGE_CREDIT_COST (env tetap jadi fallback bila baris/kolom belum ada).
-- Default 2 TC, sama dengan ai.DefaultImageCreditCost.
ALTER TABLE public.platform_ai_configuration
    ADD COLUMN IF NOT EXISTS image_credit_cost integer NOT NULL DEFAULT 2;

ALTER TABLE public.platform_ai_configuration DROP CONSTRAINT IF EXISTS ck_pac_image_credit_cost;
ALTER TABLE public.platform_ai_configuration ADD CONSTRAINT ck_pac_image_credit_cost
    CHECK (image_credit_cost >= 0);
