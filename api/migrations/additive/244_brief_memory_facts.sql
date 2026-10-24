-- 244 — Memori fakta lintas brief (chat Taut Brief).
--
-- Chat brief untuk satu profile kini bisa mengingat fakta dari profile/brief
-- LAIN di brand yang sama: satu baris per field terisi, embedding lokal
-- 128-dim (ai.LocalEmbedding, pola knowledge_chunks) disimpan sebagai JSONB
-- dan dicocokkan dengan cosine di Go. Tanpa pgvector dan tanpa API embedding
-- eksternal — skala fakta per brand (puluhan-ratusan) jauh di bawah titik
-- di mana indeks ANN dibutuhkan.

CREATE TABLE IF NOT EXISTS public.brief_memory_facts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  brand_id varchar(36) NOT NULL,
  profile_id varchar(36) NOT NULL,
  profile_name varchar(200) NOT NULL DEFAULT '',
  field_key varchar(80) NOT NULL,
  text text NOT NULL,
  embedding jsonb NOT NULL,
  source_status varchar(20) NOT NULL DEFAULT 'filled',
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (profile_id, field_key)
);

CREATE INDEX IF NOT EXISTS brief_memory_facts_brand_idx
  ON public.brief_memory_facts (brand_id);

-- Backfill sekali: antre reindex untuk semua profile yang masih hidup,
-- disebar 2 detik agar tidak menghantam DB sekaligus (pola 234).
INSERT INTO public.jobs (id, kind, payload, status, attempts, max_attempts, run_at)
SELECT gen_random_uuid()::text,
       'brief_memory_reindex',
       jsonb_build_object('profile_id', p.id),
       'pending', 0, 2,
       now() + (row_number() OVER ()) * interval '2 seconds'
FROM public.profiler_profiles p
WHERE p.deleted_at IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM public.jobs j
    WHERE j.kind = 'brief_memory_reindex'
      AND j.status IN ('pending', 'running')
      AND j.payload->>'profile_id' = p.id
  );
