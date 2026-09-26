-- Phase 13 — RSS (Sindikasi Konten) opt-in flag untuk mitra.
-- Admin yg mengaktifkan (partners.rss_enabled); mitra hanya bisa mematikan.
-- =====================================================================

ALTER TABLE partners
  ADD COLUMN IF NOT EXISTS rss_enabled BOOLEAN NOT NULL DEFAULT false;
