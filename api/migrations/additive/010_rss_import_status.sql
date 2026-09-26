-- Phase 10 — RSS import callback status (foundation §13)
-- Status per publikasi untuk alur pull RSS:
--   Feed Published (publications.status = published) →
--   Partner Imported (callback importer mitra) →
--   Partner Published (callback saat artikel terbit di situs mitra)
-- =====================================================================

ALTER TABLE publications
  ADD COLUMN IF NOT EXISTS rss_import_status VARCHAR(20) NOT NULL DEFAULT 'not_imported',
  ADD COLUMN IF NOT EXISTS rss_imported_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS rss_published_at TIMESTAMPTZ;
