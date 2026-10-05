-- Drift recovery (2026-10-05): kolom announcements/announcement_reads ada di
-- staging tapi tidak pernah masuk ledger migrasi (dibuat sebelum baseline
-- schema-full.sql di-regenerate). Idempoten — aman dijalankan di DB yang
-- sudah punya kolomnya (staging) maupun yang belum (production/env baru).
ALTER TABLE announcements
  ADD COLUMN IF NOT EXISTS badge character varying(50) NOT NULL DEFAULT 'Pengumuman',
  ADD COLUMN IF NOT EXISTS target_role character varying(20) NOT NULL DEFAULT 'all',
  ADD COLUMN IF NOT EXISTS is_active boolean NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS show_popup boolean NOT NULL DEFAULT true,
  ADD COLUMN IF NOT EXISTS updated_at timestamp with time zone NOT NULL DEFAULT now();

ALTER TABLE announcement_reads
  ADD COLUMN IF NOT EXISTS created_at timestamp with time zone NOT NULL DEFAULT now(),
  ADD COLUMN IF NOT EXISTS popup_dismissed_at timestamp with time zone;
