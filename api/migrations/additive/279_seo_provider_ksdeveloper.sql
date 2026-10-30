-- 279 — Provider DA/PA pindah ke "Bulk DA PA Checker Semrush" (RapidAPI, KSDeveloper).
--
-- Kontrak API baru: host bulk-da-pa-checker-semrush.p.rapidapi.com,
-- POST /bulk-dapa.php. Host/path/nama field body disimpan sebagai override di
-- platform_seo_config supaya penyesuaian kontrak (mis. nama field domain)
-- cukup lewat DB, tanpa ubah kode. Kolom NULL = default di kode.
--
-- Tidak menyentuh media_seo_metrics: kolom da/pa tetap sumber data yang sama.
ALTER TABLE public.platform_seo_config
  ADD COLUMN IF NOT EXISTS provider_host character varying(120),
  ADD COLUMN IF NOT EXISTS provider_path character varying(200),
  ADD COLUMN IF NOT EXISTS provider_domain_field character varying(40);

UPDATE public.platform_seo_config
SET provider_host = 'bulk-da-pa-checker-semrush.p.rapidapi.com',
    provider_path = '/bulk-dapa.php',
    provider_domain_field = 'domains'
WHERE id = 'default'
  AND provider_host IS NULL AND provider_path IS NULL AND provider_domain_field IS NULL;
