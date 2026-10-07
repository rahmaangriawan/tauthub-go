-- 191_changelog_translations.sql
-- Terjemahan Inggris entri changelog ("Apa yang Baru"). Diisi otomatis oleh API
-- saat pengguna memakai antarmuka Inggris (GET /v1/changelog?lang=en) lalu
-- dipakai ulang, supaya model AI tidak dipanggil berulang untuk entri yang sama.
-- Aditif & idempoten (dijalankan oleh cmd/migrate).

ALTER TABLE public.changelog_entries ADD COLUMN IF NOT EXISTS title_en text;
ALTER TABLE public.changelog_entries ADD COLUMN IF NOT EXISTS summary_en text;
ALTER TABLE public.changelog_entries ADD COLUMN IF NOT EXISTS body_en text;
