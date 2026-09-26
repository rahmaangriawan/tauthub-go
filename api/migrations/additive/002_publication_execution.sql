-- =====================================================================
-- Migration 002 - Publication Execution (Phase 2)
--
-- Additive migration for databases that already ran 001. Wires the
-- publication domain to the existing WordPress/webhook delivery:
--
--   * publications.publish_status  (draft|publish) and connection_id
--   * publication_jobs.publication_id (link execution record to publication)
--   * publication_jobs started_at / completed_at / next_retry_at / error_code
--
-- PostgreSQL has no `ADD CONSTRAINT IF NOT EXISTS`, so new constraints
-- are guarded with DO blocks. `ADD COLUMN IF NOT EXISTS` and
-- `CREATE INDEX IF NOT EXISTS` are supported natively. Idempotent.
--
-- Apply with:
--   psql "$DATABASE_URL" -f apps/web/src/db/migrations/002_publication_execution.sql
-- =====================================================================

ALTER TABLE public.publications ADD COLUMN IF NOT EXISTS publish_status character varying(20) DEFAULT 'draft' NOT NULL;

ALTER TABLE public.publications ADD COLUMN IF NOT EXISTS connection_id character(36);

ALTER TABLE public.publication_jobs ADD COLUMN IF NOT EXISTS publication_id character(36);

ALTER TABLE public.publication_jobs ADD COLUMN IF NOT EXISTS started_at timestamp(6) with time zone;

ALTER TABLE public.publication_jobs ADD COLUMN IF NOT EXISTS completed_at timestamp(6) with time zone;

ALTER TABLE public.publication_jobs ADD COLUMN IF NOT EXISTS next_retry_at timestamp(6) with time zone;

ALTER TABLE public.publication_jobs ADD COLUMN IF NOT EXISTS error_code character varying(80);

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'ck_publication_publish_status') THEN
        ALTER TABLE public.publications ADD CONSTRAINT ck_publication_publish_status CHECK (publish_status IN ('draft','publish'));
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_publications_connection') THEN
        ALTER TABLE public.publications ADD CONSTRAINT fk_publications_connection FOREIGN KEY (connection_id) REFERENCES public.cms_connections(id) ON DELETE SET NULL;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_publication_jobs_publication') THEN
        ALTER TABLE public.publication_jobs ADD CONSTRAINT fk_publication_jobs_publication FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS ix_publication_jobs_publication ON public.publication_jobs USING btree (publication_id);
