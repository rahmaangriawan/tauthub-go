-- Migration: 028_auto_replace_config
-- Phase 10 follow-up: admin-controlled automatic replacement.
--
-- 1. auto_replace_configs — per-brand override with a global (brand_id NULL)
--    row as the default. `enabled` turns the feature on/off; `max_replaces`
--    caps how many consecutive auto-replacements are allowed per original
--    publication (default 3). Exactly one global row is enforced by a partial
--    unique index.
--
-- 2. publications.replaces_id — self-reference: a replacement publication
--    points back to the rejected original it was created for, so client and
--    admin timelines can render "original -> replacement -> latest status".
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/028_auto_replace_config.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.auto_replace_configs (
    id character(36) NOT NULL,
    brand_id character(36),
    enabled boolean NOT NULL DEFAULT TRUE,
    max_replaces smallint NOT NULL DEFAULT 3,
    created_by character(36),
    updated_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT auto_replace_configs_pkey PRIMARY KEY (id),
    CONSTRAINT ck_auto_replace_max_replaces CHECK (max_replaces >= 1 AND max_replaces <= 10),
    CONSTRAINT fk_auto_replace_configs_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id) ON DELETE CASCADE,
    CONSTRAINT fk_auto_replace_configs_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT fk_auto_replace_configs_updater FOREIGN KEY (updated_by) REFERENCES public.users(id) ON DELETE SET NULL
);

-- At most one global configuration row.
CREATE UNIQUE INDEX IF NOT EXISTS uq_auto_replace_global ON public.auto_replace_configs ((TRUE)) WHERE brand_id IS NULL;

-- One row per brand (global excluded by the partial index above).
CREATE UNIQUE INDEX IF NOT EXISTS uq_auto_replace_brand ON public.auto_replace_configs (brand_id) WHERE brand_id IS NOT NULL;

ALTER TABLE public.publications ADD COLUMN IF NOT EXISTS replaces_id character(36);

ALTER TABLE ONLY public.publications
    ADD CONSTRAINT fk_publications_replaces FOREIGN KEY (replaces_id) REFERENCES public.publications(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS ix_publications_replaces ON public.publications (replaces_id) WHERE replaces_id IS NOT NULL;

COMMIT;
