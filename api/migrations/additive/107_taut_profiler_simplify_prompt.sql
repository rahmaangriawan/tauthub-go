-- 107 — The Profiler: sederhanakan angle prompt menjadi satu field.
-- Satu angle = satu prompt utuh yang ditulis langsung (analisis Profiler +
-- acuan Matchmaker). Kolom terpisah untuk system/matchmaker/writer/dll.
-- tidak dipakai lagi.
BEGIN;

ALTER TABLE public.taut_content_angle_prompt_versions ADD COLUMN IF NOT EXISTS prompt text;
UPDATE public.taut_content_angle_prompt_versions SET prompt = profiler_prompt WHERE prompt IS NULL AND profiler_prompt IS NOT NULL;

ALTER TABLE public.taut_content_angle_prompt_versions DROP COLUMN IF EXISTS system_instruction;
ALTER TABLE public.taut_content_angle_prompt_versions DROP COLUMN IF EXISTS profiler_prompt;
ALTER TABLE public.taut_content_angle_prompt_versions DROP COLUMN IF EXISTS matchmaker_instruction;
ALTER TABLE public.taut_content_angle_prompt_versions DROP COLUMN IF EXISTS writer_instruction;
ALTER TABLE public.taut_content_angle_prompt_versions DROP COLUMN IF EXISTS mandatory_requirements;
ALTER TABLE public.taut_content_angle_prompt_versions DROP COLUMN IF EXISTS forbidden_rules;
ALTER TABLE public.taut_content_angle_prompt_versions DROP COLUMN IF EXISTS seo_instruction;
ALTER TABLE public.taut_content_angle_prompt_versions DROP COLUMN IF EXISTS recommended_structure;
ALTER TABLE public.taut_content_angle_prompt_versions DROP COLUMN IF EXISTS tone_instruction;
ALTER TABLE public.taut_content_angle_prompt_versions DROP COLUMN IF EXISTS quote_instruction;
ALTER TABLE public.taut_content_angle_prompt_versions DROP COLUMN IF EXISTS cta_instruction;
ALTER TABLE public.taut_content_angle_prompt_versions DROP COLUMN IF EXISTS source_instruction;

ALTER TABLE public.taut_profiler_base_prompt_versions ADD COLUMN IF NOT EXISTS prompt text;
UPDATE public.taut_profiler_base_prompt_versions SET prompt = profiler_prompt WHERE prompt IS NULL AND profiler_prompt IS NOT NULL;
ALTER TABLE public.taut_profiler_base_prompt_versions DROP COLUMN IF EXISTS system_instruction;
ALTER TABLE public.taut_profiler_base_prompt_versions DROP COLUMN IF EXISTS profiler_prompt;

COMMIT;
