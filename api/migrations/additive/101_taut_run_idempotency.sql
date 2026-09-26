-- 101 — Taut Pintar: idempotensi agent run via hash input.
-- Hash (agent + campaign + media + prompt) dipakai dedup: run done dgn hash sama → hasil di-reuse, tidak dobel generate.
ALTER TABLE public.taut_agent_runs ADD COLUMN IF NOT EXISTS input_hash character varying(64);

DO $$ BEGIN
  CREATE INDEX IF NOT EXISTS ix_taut_runs_input_hash ON public.taut_agent_runs(agent, input_hash, status);
EXCEPTION WHEN duplicate_object THEN NULL; END $$;