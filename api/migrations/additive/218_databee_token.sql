-- Similarweb API Pro (publisher DataBee, via RapidAPI) mewajibkan token
-- DataBee terpisah dari key RapidAPI; dikirim sebagai
-- `Authorization: Bearer <DATABEE_TOKEN>` pada setiap panggilan provider.
-- Token dikelola admin via /admin/konfigurasi, mengikuti pola seal kolom lain.

ALTER TABLE public.platform_seo_config
  ADD COLUMN IF NOT EXISTS sealed_databee_token text;
