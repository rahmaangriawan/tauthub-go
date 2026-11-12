-- 324 — Kolom opt-out laporan bulanan di preferensi notifikasi.
-- Additive dan idempoten.

ALTER TABLE public.notification_preferences
  ADD COLUMN IF NOT EXISTS monthly_report_optout boolean NOT NULL DEFAULT false;
