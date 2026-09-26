-- Migration: 021_tulisio_publishing_api
-- Tulisio Publishing API (Phase: custom website partners).
--
-- Media with integration_type = 'tulisio_api' delivers articles by POSTing a
-- signed payload to the partner's custom adapter endpoint (adapter_url).
-- Delivery does not use a CMS connection, so publication_jobs.connection_id
-- becomes nullable for these jobs.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/021_tulisio_publishing_api.sql

BEGIN;

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS adapter_url character varying(500);

ALTER TABLE public.publication_jobs ALTER COLUMN connection_id DROP NOT NULL;

COMMIT;
