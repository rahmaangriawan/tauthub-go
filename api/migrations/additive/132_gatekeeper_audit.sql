-- 132 — Gatekeeper (Agent 4): kunci Smodin, settings agent, tabel hasil audit.
-- Additive. Detail desain: docs/Flow Taut Pintar/Agent 4 Gatekeeper/08-data-dan-migrasi.md
-- - platform_seo_config.sealed_smodin_api_key: kunci RapidAPI Smodin (plagiarism), pola sealed_similarweb_api_key.
-- - taut_agent_configs.settings: preferensi pengecek untuk baris agent='gatekeeper'.
-- - taut_gatekeeper_audits: ringkasan 4 pengecek per (campaign, media, run), tanpa konten artikel.

BEGIN;

ALTER TABLE public.platform_seo_config
  ADD COLUMN IF NOT EXISTS sealed_smodin_api_key text;

ALTER TABLE public.taut_agent_configs
  ADD COLUMN IF NOT EXISTS settings jsonb;

CREATE TABLE IF NOT EXISTS public.taut_gatekeeper_audits (
  id              character(36) NOT NULL,
  campaign_id     character(36) NOT NULL,
  media_id        character(36) NOT NULL,
  agent_run_id    character(36),
  run_revision    integer NOT NULL DEFAULT 1,
  status          character varying(20) NOT NULL,
  checks          jsonb NOT NULL DEFAULT '{}'::jsonb,
  overall_verdict character varying(20) NOT NULL,
  created_by      character(36),
  created_at      timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      timestamp with time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
  CONSTRAINT taut_gatekeeper_audits_pkey PRIMARY KEY (id),
  CONSTRAINT fk_taut_gatekeeper_audits_campaign FOREIGN KEY (campaign_id)
    REFERENCES public.campaigns (id) ON DELETE CASCADE,
  CONSTRAINT fk_taut_gatekeeper_audits_media FOREIGN KEY (media_id)
    REFERENCES public.media (id),
  CONSTRAINT fk_taut_gatekeeper_audits_run FOREIGN KEY (agent_run_id)
    REFERENCES public.taut_agent_runs (id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_taut_gatekeeper_audits_campaign
  ON public.taut_gatekeeper_audits (campaign_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_taut_gatekeeper_audits_media
  ON public.taut_gatekeeper_audits (media_id, created_at DESC);

GRANT SELECT, INSERT, UPDATE, DELETE ON public.taut_gatekeeper_audits TO ai_writer;

COMMIT;