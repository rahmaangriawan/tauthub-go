-- Migration: 066_partner_draft_edit
-- Snapshot edit draf oleh Mitra untuk media mode Draf.
--
-- Saat Mitra mengubah judul/isi artikel yang masih berupa DRAFT di WordPress
-- (melalui PUT /v1/partner/publications/{id}/draft), perubahan dikirim ke
-- WordPress DAN disimpan di sini sebagai snapshot {title?, content_html?,
-- edited_at} agar preview di portal Mitra konsisten. Dokumen klien
-- (content_documents/content_versions) tidak pernah diubah oleh Mitra.
--
-- Additive & backward compatible: kolom jsonb nullable; publikasi existing
-- tidak tersentuh.
-- Idempotent: ADD COLUMN IF NOT EXISTS.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/066_partner_draft_edit.sql

ALTER TABLE public.publications
    ADD COLUMN IF NOT EXISTS partner_draft_edits jsonb;
