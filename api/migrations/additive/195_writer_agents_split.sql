-- 195 — Pisahkan agent The Writer per tipe media: subagent `writer_mainstream`
-- dan `writer_blogger`, supaya admin bisa mengatur model/system prompt berbeda
-- untuk media mainstream dan blogger di Admin > Agent Config.
--
-- Konfigurasi KEDUA agent baru DISALIN dari agent `writer` yang sudah ada, jadi
-- perilaku produksi tidak berubah sampai admin menyesuaikannya. Agent `writer`
-- tetap hidup sebagai cadangan: mesin writer memakai agent spesifik lebih dulu,
-- dan kembali ke `writer` bila agent spesifik belum dikonfigurasi.
--
-- Aditif & idempoten (dijalankan oleh cmd/migrate).

-- 1. Perluas CHECK daftar agent pada log run. Tanpa ini pencatatan run agent
--    baru akan gagal (constraint lama hanya mengenal 4 agent).
ALTER TABLE public.taut_agent_runs DROP CONSTRAINT IF EXISTS ck_taut_runs_agent;

DO $$ BEGIN
  ALTER TABLE public.taut_agent_runs ADD CONSTRAINT ck_taut_runs_agent
    CHECK (agent IN ('profiler','matchmaker','writer','writer_mainstream','writer_blogger','gatekeeper'));
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- 2. Salin konfigurasi agent `writer` ke dua subagent baru.
INSERT INTO public.taut_agent_configs
  (agent, base_url, sealed_api_key, model, temperature, max_tokens, timeout_ms,
   system_prompt, enabled, rate_limit_per_min, settings, updated_by, updated_at)
SELECT v.agent, c.base_url, c.sealed_api_key, c.model, c.temperature, c.max_tokens, c.timeout_ms,
       c.system_prompt, c.enabled, c.rate_limit_per_min, c.settings, c.updated_by, c.updated_at
FROM (VALUES ('writer_mainstream'), ('writer_blogger')) AS v(agent)
CROSS JOIN public.taut_agent_configs c
WHERE c.agent = 'writer'
ON CONFLICT (agent) DO NOTHING;
