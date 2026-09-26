-- Phase 10 — RSS install verification (additive)
-- Mitra wajib memasang feed URL di websitenya; Tulisio memverifikasi saat
-- mitra mengklik "Sudah dipasang". Kolom ini mencatat hasil verifikasi.
-- =====================================================================

ALTER TABLE rss_feeds
  ADD COLUMN IF NOT EXISTS install_status VARCHAR(20) NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS install_error TEXT,
  ADD COLUMN IF NOT EXISTS installed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS verified_by CHAR(36);
