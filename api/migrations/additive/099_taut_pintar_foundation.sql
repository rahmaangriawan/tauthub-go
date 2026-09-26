-- 099 — Taut Pintar: fondasi wizard klien + estafet 4 agent AI.
-- Additive. Reuse tabel campaigns. Detail desain:
-- docs/Flow Taut Pintar/Implementasi Tautpintar.md
-- docs/Flow Taut Pintar/Agent Taut Pintar.md

BEGIN;

-- ── 1. Extend campaigns.media_type agar menerima 'kombinasi' ──────────────
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS business_profile jsonb;
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS hyperlink_config jsonb;
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS trigger_keywords text[];
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS budget_strategy character varying(20);
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS budget_config jsonb;
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS strategy_brief jsonb;
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS timeline_plan jsonb;
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS taut_status character varying(20);

DO $$ BEGIN
  ALTER TABLE public.campaigns DROP CONSTRAINT ck_campaign_media_type;
EXCEPTION WHEN undefined_object THEN NULL; END $$;

DO $$ BEGIN
  ALTER TABLE public.campaigns ADD CONSTRAINT ck_campaign_media_type
    CHECK (media_type IN ('blogger','mainstream','kombinasi'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  ALTER TABLE public.campaigns ADD CONSTRAINT ck_campaigns_budget_strategy
    CHECK (budget_strategy IN ('efisien','optimal','brutal'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  ALTER TABLE public.campaigns ADD CONSTRAINT ck_campaigns_taut_status
    CHECK (taut_status IN ('draft','materials','budget','placement','production','review','scheduled','submitted'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE INDEX IF NOT EXISTS ix_campaigns_taut_status ON public.campaigns(brand_id, taut_status);

-- ── 2. taut_agent_configs — per-agent LLM + knowledge (super admin) ───────
CREATE TABLE IF NOT EXISTS public.taut_agent_configs (
  agent character varying(20) PRIMARY KEY,
  base_url text,
  sealed_api_key text,
  model character varying(120),
  temperature numeric(3,2) DEFAULT 0.7,
  max_tokens integer,
  timeout_ms integer DEFAULT 60000,
  system_prompt text,
  enabled boolean NOT NULL DEFAULT FALSE,
  updated_by character(36),
  updated_at timestamp with time zone NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT, UPDATE, DELETE ON public.taut_agent_configs TO ai_writer;

-- Seed 4 baris: system_prompt terisi draft knowledge, enabled=false (hard stop).
INSERT INTO public.taut_agent_configs (agent, system_prompt) VALUES
('profiler', 'Kamu adalah Strategis Taut Pintar. Tugasmu menerjemahkan profil bisnis klien dan anggaran menjadi brief strategi distribusi konten. Mode Efisien = maksimalkan jumlah media dari nominal; Optimal = sebar durasi harian/bulanan; Brutal = skala penuh tanpa batas. Selalu keluarkan JSON valid strategy_brief berisi angle, keywords, budget_breakdown (estimasi_media, durasi, alokasi_per_media), tone_hint. Jangan menulis artikel.'),
('matchmaker', 'Kamu adalah Matchmaker Taut Pintar. Tugasmu meranking media berdasarkan kecocokan trigger dan metrik media (DA/PA, harga, kategori, health score). Jangan menulis artikel. Beri alasan singkat per media. Keluarkan JSON valid berupa array berurutan dari media terbaik.'),
('writer', 'Kamu adalah Writer Taut Pintar. Tulis artikel jurnalistik Indonesia yang natural, faktual, dan unik untuk setiap media tujuan. Ikuti format sesuai trigger (review, perbandingan, rilis produk, opini). Gaya bahasa selaras profil bisnis klien. Jangan menanam hyperlink — itu tugas Gatekeeper. Hindari judi, SARA, dan ujaran kebencian. Keluarkan JSON valid {title, content_html}.'),
('gatekeeper', 'Kamu adalah Gatekeeper Taut Pintar, auditor independen. Tanam hyperlink secara natural hanya pada media yang mengizinkan, sesuai hyperlink_config klien. Periksa judi, SARA, dan ujaran kebencian. Jangan mengubah fakta artikel. Keluarkan JSON valid {final_html, verdict: layak_tayang|revisi|gagal, catatan}.')
ON CONFLICT (agent) DO NOTHING;

-- ── 3. taut_agent_runs — logging eksekusi agent ───────────────────────────
CREATE TABLE IF NOT EXISTS public.taut_agent_runs (
  id character(36) PRIMARY KEY,
  campaign_id character(36) NOT NULL REFERENCES public.campaigns(id) ON DELETE CASCADE,
  media_id character(36),
  agent character varying(20) NOT NULL,
  status character varying(20) NOT NULL DEFAULT 'pending',
  model character varying(120),
  token_input integer,
  token_output integer,
  duration_ms integer,
  input jsonb,
  output jsonb,
  error text,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  finished_at timestamp with time zone
);

GRANT SELECT, INSERT, UPDATE ON public.taut_agent_runs TO ai_writer;

CREATE INDEX IF NOT EXISTS ix_taut_runs_campaign ON public.taut_agent_runs(campaign_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_taut_runs_agent ON public.taut_agent_runs(agent, created_at DESC);

DO $$ BEGIN
  ALTER TABLE public.taut_agent_runs ADD CONSTRAINT ck_taut_runs_agent
    CHECK (agent IN ('profiler','matchmaker','writer','gatekeeper'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  ALTER TABLE public.taut_agent_runs ADD CONSTRAINT ck_taut_runs_status
    CHECK (status IN ('pending','running','done','failed','skipped'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ── 4. taut_campaign_media — seleksi media per kampanye ──────────────────
CREATE TABLE IF NOT EXISTS public.taut_campaign_media (
  id character(36) PRIMARY KEY,
  campaign_id character(36) NOT NULL REFERENCES public.campaigns(id) ON DELETE CASCADE,
  media_id character(36) NOT NULL REFERENCES public.media(id) ON DELETE CASCADE,
  source character varying(10) NOT NULL DEFAULT 'ai',
  price numeric(14,2),
  status character varying(20) NOT NULL DEFAULT 'selected',
  article_document_id character(36),
  article_status character varying(20) NOT NULL DEFAULT 'none',
  created_at timestamp with time zone NOT NULL DEFAULT now()
);

GRANT SELECT, INSERT, UPDATE, DELETE ON public.taut_campaign_media TO ai_writer;

CREATE UNIQUE INDEX IF NOT EXISTS uq_taut_campaign_media
  ON public.taut_campaign_media(campaign_id, media_id);
CREATE INDEX IF NOT EXISTS ix_taut_campaign_media_campaign
  ON public.taut_campaign_media(campaign_id);

DO $$ BEGIN
  ALTER TABLE public.taut_campaign_media ADD CONSTRAINT ck_taut_cm_source
    CHECK (source IN ('ai','manual'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  ALTER TABLE public.taut_campaign_media ADD CONSTRAINT ck_taut_cm_status
    CHECK (status IN ('selected','removed','replaced'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  ALTER TABLE public.taut_campaign_media ADD CONSTRAINT ck_taut_cm_article_status
    CHECK (article_status IN ('none','generating','draft','hyperlinked','safe','blocked','revision','ready'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  ALTER TABLE public.taut_campaign_media ADD CONSTRAINT fk_taut_cm_document
    FOREIGN KEY (article_document_id) REFERENCES public.content_documents(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

COMMIT;