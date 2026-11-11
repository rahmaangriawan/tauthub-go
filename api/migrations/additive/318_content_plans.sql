-- 318 — Rencana Bulanan: rencana konten beberapa minggu yang disetujui sekali.
--
-- content_plans: satu rencana (brief, anggaran, jumlah minggu, status).
-- content_plan_items: satu baris per artikel (minggu, tanggal jalan, media,
--   topik, perkiraan biaya, status, dan kampanye draf yang dibuat saat jalan).
-- Saat jalan, sistem hanya membuat KAMPANYE DRAF dengan media dan topik itu lalu
-- memberi tahu klien. Tidak ada saldo yang dipotong dan tidak ada distribusi
-- otomatis; klien menulis, memeriksa mutu, dan mendistribusikan sendiri.
-- Tipe notifikasi baru: plan.week_ready, plan.needs_attention, plan.renewal_draft.
-- Additive dan idempoten.

BEGIN;

CREATE TABLE IF NOT EXISTS public.content_plans (
  id          character(36) PRIMARY KEY,
  brand_id    character(36) NOT NULL REFERENCES public.brands (id) ON DELETE CASCADE,
  created_by  character(36),
  name        character varying(160) NOT NULL,
  brief       text NOT NULL,
  category    character varying(120),
  budget_idr  integer NOT NULL,
  weeks       integer NOT NULL,
  per_week    integer NOT NULL DEFAULT 1,
  start_date  date NOT NULL,
  status      character varying(20) NOT NULL DEFAULT 'draft',
  auto_renew  boolean NOT NULL DEFAULT FALSE,
  approved_at timestamptz,
  created_at  timestamptz NOT NULL DEFAULT NOW(),
  updated_at  timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT content_plans_status_check CHECK (status IN ('draft', 'approved', 'running', 'done', 'cancelled'))
);
CREATE INDEX IF NOT EXISTS content_plans_brand_idx ON public.content_plans (brand_id, created_at DESC);

CREATE TABLE IF NOT EXISTS public.content_plan_items (
  id             character(36) PRIMARY KEY,
  plan_id        character(36) NOT NULL REFERENCES public.content_plans (id) ON DELETE CASCADE,
  week           integer NOT NULL,
  run_on         date NOT NULL,
  media_id       character(36) NOT NULL REFERENCES public.media (id),
  topic          text NOT NULL,
  est_media_tc   integer NOT NULL DEFAULT 0,
  est_writer_tc  integer NOT NULL DEFAULT 0,
  status         character varying(20) NOT NULL DEFAULT 'pending',
  campaign_id    character(36),
  note           text,
  created_at     timestamptz NOT NULL DEFAULT NOW(),
  updated_at     timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT content_plan_items_status_check CHECK (status IN ('pending', 'running', 'ready', 'needs_attention', 'skipped'))
);
CREATE INDEX IF NOT EXISTS content_plan_items_due_idx ON public.content_plan_items (status, run_on);
CREATE INDEX IF NOT EXISTS content_plan_items_plan_idx ON public.content_plan_items (plan_id, week);

COMMIT;
