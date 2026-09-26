-- 127 — Short Link TautHub: setting domain + mapping link per campaign.
-- Kode maks 5 char dihasilkan aplikasi; domain custom dikonfigurasi admin
-- (fallback: domain utama via /t/[kode]).

BEGIN;

CREATE TABLE IF NOT EXISTS public.platform_settings (
  key varchar(64) PRIMARY KEY,
  value text NOT NULL DEFAULT '',
  updated_at timestamptz NOT NULL DEFAULT NOW()
);

ALTER TABLE public.tracking_links
  ADD COLUMN IF NOT EXISTS campaign_id varchar(36) NULL;
ALTER TABLE public.tracking_links
  ADD COLUMN IF NOT EXISTS kind varchar(20) NULL;
-- Short link Writer ada di level campaign (tanpa publikasi) → publication_id
-- harus boleh NULL. UNIQUE(publication_id) tetap aman: Postgres mengizinkan
-- banyak NULL di kolom UNIQUE.
ALTER TABLE public.tracking_links
  ALTER COLUMN publication_id DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tracking_links_campaign
  ON public.tracking_links (campaign_id, kind);

COMMIT;
