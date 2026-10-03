-- 177_ai_image_config.sql
-- Konfigurasi khusus "Generate Gambar AI" (Base URL + API Key + Model) di
-- platform_ai_configuration, terpisah dari konfigurasi AI chat.
--
-- Semua kolom nullable: baris NULL/kosong berarti "ikuti konfigurasi AI chat".
-- Idempoten: aman dijalankan berulang kali.

ALTER TABLE public.platform_ai_configuration
    ADD COLUMN IF NOT EXISTS image_base_url text;
ALTER TABLE public.platform_ai_configuration
    ADD COLUMN IF NOT EXISTS image_sealed_api_key text;
ALTER TABLE public.platform_ai_configuration
    ADD COLUMN IF NOT EXISTS image_api_key_hint character varying(20);
ALTER TABLE public.platform_ai_configuration
    ADD COLUMN IF NOT EXISTS image_model character varying(200);
