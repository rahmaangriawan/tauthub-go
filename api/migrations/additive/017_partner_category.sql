-- Migration: 017_partner_category
-- Adds a media-partner category to partners so the "Daftar sebagai Mitra"
-- registration can capture the partner's media category (berita | lifestyle |
-- lainnya). Existing partners get the default 'lainnya'.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/017_partner_category.sql

BEGIN;

ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS category character varying(60) DEFAULT 'lainnya';

COMMIT;
