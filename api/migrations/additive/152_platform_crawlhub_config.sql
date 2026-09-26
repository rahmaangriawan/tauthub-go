-- Konfigurasi Crawlhub (service crawler terpisah, crawl.tauthub.com).
--
-- Endpoint disimpan di DB, bukan hanya di env, supaya domainnya bisa diganti
-- dari halaman admin tanpa deploy ulang. Env CRAWLHUB_* tetap dipakai sebagai
-- nilai awal kalau baris ini belum ada.

CREATE TABLE IF NOT EXISTS platform_crawlhub_config (
  id                   character varying(36) PRIMARY KEY,
  base_url             character varying(500),
  sealed_hmac_secret   text,
  sealed_ingest_secret text,
  updated_by           character varying(36),
  updated_at           timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP
);