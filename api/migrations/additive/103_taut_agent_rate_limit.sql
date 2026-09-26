-- 103 — Taut Pintar: hardening Fase 3 — rate limit per agent.
ALTER TABLE public.taut_agent_configs ADD COLUMN IF NOT EXISTS rate_limit_per_min integer DEFAULT 3;