-- Migration: 064_media_review
-- Review media oleh Admin sebelum media Mitra masuk marketplace.
--
-- Flow baru: media yang diajukan Mitra masuk status 'pending_review' (bukan
-- langsung 'active'); Admin menyetujui ('active') atau menolak ('rejected')
-- dengan alasan wajib. Data media TIDAK pernah dihapus saat ditolak.
--
-- Additive & backward compatible:
--   - Perluas CHECK constraint media.status (pola sama seperti
--     ck_notifications_type): menambah nilai tidak menghapus data.
--   - Kolom baru rejection_reason / reviewed_by / reviewed_at nullable.
--   - Media existing (active/inactive/...) tidak tersentuh.
-- Idempotent: ADD COLUMN IF NOT EXISTS + DO block untuk constraint.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/064_media_review.sql

ALTER TABLE public.media
    ADD COLUMN IF NOT EXISTS rejection_reason character varying(1000),
    ADD COLUMN IF NOT EXISTS reviewed_by character(36),
    ADD COLUMN IF NOT EXISTS reviewed_at timestamp(6) with time zone;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_media_status'
          AND conrelid = 'public.media'::regclass
    ) THEN
        ALTER TABLE public.media
            ADD CONSTRAINT ck_media_status
            CHECK (status IN ('draft', 'active', 'inactive', 'archived', 'hidden', 'pending_review', 'rejected'));
    ELSE
        ALTER TABLE public.media DROP CONSTRAINT ck_media_status;
        ALTER TABLE public.media
            ADD CONSTRAINT ck_media_status
            CHECK (status IN ('draft', 'active', 'inactive', 'archived', 'hidden', 'pending_review', 'rejected'));
    END IF;
END $$;
