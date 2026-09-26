-- =====================================================================
-- Migration 003 - Media Partner (Phase 3)
--
-- Additive migration for databases that already ran 001 + 002.
--
--   * partners table (platform-level partner role, 1 partner = 1 user)
--   * media.partner_id            (partner owns/manages media)
--   * publications.rejection_reason / reviewed_by / reviewed_at
--     (partner approve/reject flow)
--
-- PostgreSQL has no `ADD CONSTRAINT IF NOT EXISTS`, so new constraints
-- are guarded with DO blocks; new columns use ADD COLUMN IF NOT EXISTS.
-- Idempotent.
--
-- Apply with:
--   psql "$DATABASE_URL" -f apps/web/src/db/migrations/003_media_partner.sql
-- =====================================================================

CREATE TABLE IF NOT EXISTS public.partners (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    name character varying(160),
    status character varying(20) DEFAULT 'active' NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT partners_pkey PRIMARY KEY (id),
    CONSTRAINT uq_partners_user UNIQUE (user_id),
    CONSTRAINT fk_partners_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE,
    CONSTRAINT ck_partner_status CHECK (status IN ('active','inactive'))
);

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS partner_id character(36);

ALTER TABLE public.publications ADD COLUMN IF NOT EXISTS rejection_reason character varying(500);

ALTER TABLE public.publications ADD COLUMN IF NOT EXISTS reviewed_by character(36);

ALTER TABLE public.publications ADD COLUMN IF NOT EXISTS reviewed_at timestamp(6) with time zone;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_media_partner') THEN
        ALTER TABLE public.media ADD CONSTRAINT fk_media_partner FOREIGN KEY (partner_id) REFERENCES public.partners(id) ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_publications_reviewed_by') THEN
        ALTER TABLE public.publications ADD CONSTRAINT fk_publications_reviewed_by FOREIGN KEY (reviewed_by) REFERENCES public.users(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS ix_media_partner ON public.media USING btree (partner_id, status);

CREATE INDEX IF NOT EXISTS ix_publication_status_created ON public.publications USING btree (status, created_at);
