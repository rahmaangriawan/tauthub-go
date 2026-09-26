-- 106 — The Profiler v3: domain model lengkap Executive PR Brief.
-- Menambah channel media, content angle, prompt version immutable, submission,
-- result, dan campaign files. Semua additive. Kontrak lama (profiler_summary,
-- category_input) tidak dihapus di sini; flow baru tidak membacanya lagi.
-- Referensi: docs/Flow Taut Pintar/flow/the-profiler-master-ui-ux-prompt.md

BEGIN;

-- ── 1. taut_media_channels ──────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.taut_media_channels (
  id character(36) PRIMARY KEY,
  name character varying(120) NOT NULL,
  slug character varying(80) NOT NULL,
  description text,
  active boolean NOT NULL DEFAULT TRUE,
  sort_order integer NOT NULL DEFAULT 0,
  created_by character(36),
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_by character(36),
  updated_at timestamp with time zone NOT NULL DEFAULT now()
);


CREATE UNIQUE INDEX IF NOT EXISTS uq_taut_channels_slug ON public.taut_media_channels(slug);
CREATE INDEX IF NOT EXISTS ix_taut_channels_active_order ON public.taut_media_channels(active, sort_order);

-- ── 2. taut_content_angles ──────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.taut_content_angles (
  id character(36) PRIMARY KEY,
  channel_id character(36) NOT NULL REFERENCES public.taut_media_channels(id) ON DELETE CASCADE,
  name character varying(160) NOT NULL,
  slug character varying(100) NOT NULL,
  description text,
  active boolean NOT NULL DEFAULT TRUE,
  sort_order integer NOT NULL DEFAULT 0,
  created_by character(36),
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_by character(36),
  updated_at timestamp with time zone NOT NULL DEFAULT now()
);


CREATE UNIQUE INDEX IF NOT EXISTS uq_taut_angles_channel_slug ON public.taut_content_angles(channel_id, slug);
CREATE INDEX IF NOT EXISTS ix_taut_angles_channel_active ON public.taut_content_angles(channel_id, active, sort_order);

-- ── 3. taut_content_angle_prompt_versions ───────────────────────────────
-- Immutable per version. Satu version ACTIVE per angle (partial unique index).
CREATE TABLE IF NOT EXISTS public.taut_content_angle_prompt_versions (
  id character(36) PRIMARY KEY,
  content_angle_id character(36) NOT NULL REFERENCES public.taut_content_angles(id) ON DELETE CASCADE,
  version integer NOT NULL,
  status character varying(20) NOT NULL DEFAULT 'draft',
  system_instruction text,
  profiler_prompt text,
  matchmaker_instruction text,
  writer_instruction text,
  mandatory_requirements jsonb DEFAULT '[]'::jsonb NOT NULL,
  forbidden_rules jsonb DEFAULT '[]'::jsonb NOT NULL,
  seo_instruction text,
  recommended_structure jsonb DEFAULT '[]'::jsonb NOT NULL,
  tone_instruction text,
  quote_instruction text,
  cta_instruction text,
  source_instruction text,
  model_overrides jsonb,
  change_note character varying(255),
  created_by character(36) NOT NULL,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  published_by character(36),
  published_at timestamp with time zone
);


CREATE UNIQUE INDEX IF NOT EXISTS uq_taut_angle_versions_version
  ON public.taut_content_angle_prompt_versions(content_angle_id, version);
-- Satu versi ACTIVE per angle.
CREATE UNIQUE INDEX IF NOT EXISTS uq_taut_angle_versions_active
  ON public.taut_content_angle_prompt_versions(content_angle_id) WHERE (status = 'active');

DO $$ BEGIN
  ALTER TABLE public.taut_content_angle_prompt_versions ADD CONSTRAINT ck_taut_angle_versions_status
    CHECK (status IN ('draft','active','archived'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ── 4. taut_profiler_base_prompt_versions ───────────────────────────────
-- Base prompt global The Profiler, versioned sama seperti angle prompt.
CREATE TABLE IF NOT EXISTS public.taut_profiler_base_prompt_versions (
  id character(36) PRIMARY KEY,
  version integer NOT NULL,
  status character varying(20) NOT NULL DEFAULT 'draft',
  system_instruction text,
  profiler_prompt text,
  change_note character varying(255),
  created_by character(36) NOT NULL,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  published_by character(36),
  published_at timestamp with time zone
);


CREATE UNIQUE INDEX IF NOT EXISTS uq_taut_base_prompt_version ON public.taut_profiler_base_prompt_versions(version);
CREATE UNIQUE INDEX IF NOT EXISTS uq_taut_base_prompt_active
  ON public.taut_profiler_base_prompt_versions(version) WHERE (status = 'active');

DO $$ BEGIN
  ALTER TABLE public.taut_profiler_base_prompt_versions ADD CONSTRAINT ck_taut_base_prompt_status
    CHECK (status IN ('draft','active','archived'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ── 5. taut_profiler_submissions ────────────────────────────────────────
-- Draft mutable per campaign; revision yang dianalisis dibekukan jadi
-- analyzing/completed/failed/superseded.
CREATE TABLE IF NOT EXISTS public.taut_profiler_submissions (
  id character(36) PRIMARY KEY,
  campaign_id character(36) NOT NULL REFERENCES public.campaigns(id) ON DELETE CASCADE,
  revision integer NOT NULL DEFAULT 1,
  status character varying(20) NOT NULL DEFAULT 'draft',
  input_schema_version integer NOT NULL DEFAULT 1,
  objective jsonb NOT NULL DEFAULT '{}'::jsonb,
  business_profile jsonb NOT NULL DEFAULT '{}'::jsonb,
  audience jsonb NOT NULL DEFAULT '{}'::jsonb,
  spokesperson jsonb,
  guardrails jsonb NOT NULL DEFAULT '{"rules":[]}'::jsonb,
  backlink jsonb NOT NULL DEFAULT '{}'::jsonb,
  materials jsonb NOT NULL DEFAULT '{}'::jsonb,
  budget jsonb NOT NULL DEFAULT '{}'::jsonb,
  crawl_result jsonb,
  input_hash character(64),
  created_by character(36) NOT NULL,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_by character(36),
  updated_at timestamp with time zone NOT NULL DEFAULT now()
);


CREATE UNIQUE INDEX IF NOT EXISTS uq_taut_submissions_campaign_active
  ON public.taut_profiler_submissions(campaign_id) WHERE (status = 'draft');
CREATE INDEX IF NOT EXISTS ix_taut_submissions_campaign
  ON public.taut_profiler_submissions(campaign_id, created_at DESC);

DO $$ BEGIN
  ALTER TABLE public.taut_profiler_submissions ADD CONSTRAINT ck_taut_submissions_status
    CHECK (status IN ('draft','analyzing','completed','failed','superseded'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ── 6. taut_profiler_submission_channels ────────────────────────────────
-- Snapshot per channel yang dipilih pada submission (channel+angle+prompt).
CREATE TABLE IF NOT EXISTS public.taut_profiler_submission_channels (
  id character(36) PRIMARY KEY,
  submission_id character(36) NOT NULL REFERENCES public.taut_profiler_submissions(id) ON DELETE CASCADE,
  channel_id character(36) NOT NULL REFERENCES public.taut_media_channels(id) ON DELETE CASCADE,
  content_angle_id character(36) REFERENCES public.taut_content_angles(id) ON DELETE SET NULL,
  prompt_version_id character(36) REFERENCES public.taut_content_angle_prompt_versions(id) ON DELETE SET NULL,
  created_at timestamp with time zone NOT NULL DEFAULT now()
);


CREATE UNIQUE INDEX IF NOT EXISTS uq_taut_submission_channels
  ON public.taut_profiler_submission_channels(submission_id, channel_id);

-- ── 7. taut_profiler_results ────────────────────────────────────────────
-- Executive PR Brief immutable per run.
CREATE TABLE IF NOT EXISTS public.taut_profiler_results (
  id character(36) PRIMARY KEY,
  submission_id character(36) NOT NULL REFERENCES public.taut_profiler_submissions(id) ON DELETE CASCADE,
  agent_run_id character(36),
  result_schema_version integer NOT NULL DEFAULT 1,
  executive_brief jsonb NOT NULL,
  writing_instructions jsonb NOT NULL DEFAULT '[]'::jsonb,
  strategic_notes jsonb NOT NULL DEFAULT '[]'::jsonb,
  compiled_prompt_manifest jsonb NOT NULL DEFAULT '{}'::jsonb,
  model_snapshot jsonb,
  credit_ledger_entry_id character(36),
  created_at timestamp with time zone NOT NULL DEFAULT now()
);


CREATE INDEX IF NOT EXISTS ix_taut_profiler_results_submission
  ON public.taut_profiler_results(submission_id, created_at DESC);

-- ── 8. taut_campaign_files ──────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.taut_campaign_files (
  id character(36) PRIMARY KEY,
  campaign_id character(36) NOT NULL REFERENCES public.campaigns(id) ON DELETE CASCADE,
  submission_id character(36) REFERENCES public.taut_profiler_submissions(id) ON DELETE SET NULL,
  original_filename character varying(255) NOT NULL,
  storage_key text NOT NULL,
  mime_type character varying(120),
  size_bytes integer,
  checksum character varying(64),
  parse_status character varying(20) NOT NULL DEFAULT 'pending',
  extracted_text text,
  parse_error text,
  uploaded_by character(36) NOT NULL,
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  deleted_at timestamp with time zone
);


CREATE INDEX IF NOT EXISTS ix_taut_campaign_files_campaign
  ON public.taut_campaign_files(campaign_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_taut_campaign_files_active
  ON public.taut_campaign_files(campaign_id) WHERE (deleted_at IS NULL);
CREATE INDEX IF NOT EXISTS ix_taut_campaign_files_checksum
  ON public.taut_campaign_files(checksum) WHERE (checksum IS NOT NULL);

DO $$ BEGIN
  ALTER TABLE public.taut_campaign_files ADD CONSTRAINT ck_taut_campaign_files_parse
    CHECK (parse_status IN ('pending','parsing','ready','failed'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ── Seed master data (idempotent) ───────────────────────────────────────
-- Prompt version tetap DRAFT: super admin wajib publish sebelum angle
-- tampil ke klien (lihat /admin/taut-prompts).

INSERT INTO public.taut_media_channels (id, name, slug, description, sort_order, created_by) VALUES
  ('00000000-0000-4000-8000-00000000b001', 'Media Mainstream / Nasional', 'mainstream',
   'Pendekatan jurnalistik, piramida terbalik, otoritatif.', 1, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000b002', 'Media Blogger / Niche', 'blogger',
   'Pendekatan komunitas, naratif, solusi praktis.', 2, '00000000-0000-4000-8000-000000000001')
ON CONFLICT (slug) DO NOTHING;

INSERT INTO public.taut_content_angles (id, channel_id, name, slug, description, sort_order, created_by) VALUES
  -- Mainstream (13)
  ('00000000-0000-4000-8000-00000000c101', '00000000-0000-4000-8000-00000000b001', 'Peluncuran Produk / Fitur Baru', 'product_launch', 'Pengumuman peluncuran produk atau fitur baru.', 1, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c102', '00000000-0000-4000-8000-00000000b001', 'Promosi Bisnis & Penawaran Spesial (Advertorial)', 'special_offer_advertorial', 'Konten advertorial promosi dan penawaran spesial.', 2, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c103', '00000000-0000-4000-8000-00000000b001', 'Opini Pakar / Pemikiran Strategis (Thought Leadership)', 'thought_leadership', 'Opini pakar dan pemikiran strategis.', 3, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c104', '00000000-0000-4000-8000-00000000b001', 'Laporan Riset, Data & Tren Industri', 'research_data_trends', 'Laporan riset, data, dan tren industri.', 4, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c105', '00000000-0000-4000-8000-00000000b001', 'Kemitraan Strategis & Ekspansi B2B', 'strategic_partnership_b2b', 'Kemitraan strategis dan ekspansi B2B.', 5, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c106', '00000000-0000-4000-8000-00000000b001', 'Pencapaian, Penghargaan & Milestone Perusahaan', 'milestone_awards', 'Pencapaian, penghargaan, dan milestone perusahaan.', 6, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c107', '00000000-0000-4000-8000-00000000b001', 'Pengumuman Pendanaan & Investasi', 'funding_announcement', 'Pengumuman pendanaan dan investasi.', 7, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c108', '00000000-0000-4000-8000-00000000b001', 'Inisiatif CSR & Keberlanjutan Lingkungan', 'csr_sustainability', 'Inisiatif CSR dan keberlanjutan lingkungan.', 8, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c109', '00000000-0000-4000-8000-00000000b001', 'Rebranding & Transformasi Bisnis', 'rebranding_transformation', 'Rebranding dan transformasi bisnis.', 9, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c10a', '00000000-0000-4000-8000-00000000b001', 'Pergantian Manajemen Puncak (C-Level)', 'c_level_change', 'Pergantian manajemen puncak perusahaan.', 10, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c10b', '00000000-0000-4000-8000-00000000b001', 'Kisah Inspiratif / Tokoh Pemberdayaan', 'inspirational_story', 'Kisah inspiratif dan tokoh pemberdayaan.', 11, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c10c', '00000000-0000-4000-8000-00000000b001', 'Klarifikasi Isu Publik & Manajemen Reputasi', 'reputation_clarification', 'Klarifikasi isu publik dan manajemen reputasi.', 12, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c10d', '00000000-0000-4000-8000-00000000b001', 'Liputan Event / Konferensi Skala Besar', 'large_event_coverage', 'Liputan event dan konferensi skala besar.', 13, '00000000-0000-4000-8000-000000000001'),
  -- Blogger (9)
  ('00000000-0000-4000-8000-00000000c201', '00000000-0000-4000-8000-00000000b002', 'Review Produk & Pengalaman Pengguna', 'product_review', 'Review produk dan pengalaman pengguna.', 1, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c202', '00000000-0000-4000-8000-00000000b002', 'Perbandingan Produk (Versus / Komparasi)', 'comparison', 'Perbandingan antar produk.', 2, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c203', '00000000-0000-4000-8000-00000000b002', 'Artikel Tips & Trik / How-to', 'how_to', 'Tips dan trik praktis.', 3, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c204', '00000000-0000-4000-8000-00000000b002', 'Listicle (Rekomendasi Terbaik / Top 10)', 'listicle', 'Rekomendasi daftar terbaik.', 4, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c205', '00000000-0000-4000-8000-00000000b002', 'Cerita Pengalaman / Studi Kasus Keberhasilan', 'success_case', 'Cerita pengalaman dan studi kasus.', 5, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c206', '00000000-0000-4000-8000-00000000b002', 'Unboxing & Impresi Pertama', 'unboxing', 'Unboxing dan impresi pertama.', 6, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c207', '00000000-0000-4000-8000-00000000b002', 'Promo & Info Penawaran Spesial (Soft-sell)', 'soft_sell_promo', 'Promo dan penawaran spesial gaya soft-sell.', 7, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c208', '00000000-0000-4000-8000-00000000b002', 'Edukasi Niche & Penjelasan Konsep', 'niche_education', 'Edukasi niche dan penjelasan konsep.', 8, '00000000-0000-4000-8000-000000000001'),
  ('00000000-0000-4000-8000-00000000c209', '00000000-0000-4000-8000-00000000b002', 'Liputan Acara / Event Komunitas', 'community_event', 'Liputan acara dan event komunitas.', 9, '00000000-0000-4000-8000-000000000001')
ON CONFLICT (channel_id, slug) DO NOTHING;

COMMIT;
