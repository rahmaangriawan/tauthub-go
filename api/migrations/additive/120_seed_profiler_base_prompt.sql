-- Seed base prompt Profiler v1 (ACTIVE). Tanpa ini, resolved.systemPrompt
-- kosong → callAgentLLM pakai system prompt default agent "profiler"
-- (perintah strategy_brief) yang membajak format output Executive Brief —
-- terbukti di crawl/route.ts line 82-83 (kasus yang sama).
INSERT INTO public.taut_profiler_base_prompt_versions
  (id, version, status, prompt, change_note, created_by, published_by, published_at)
SELECT
  '00000000-0000-4000-8000-00000000e001', 1, 'active',
  'Kamu adalah PR strategist senior yang menyusun Executive PR Brief.

ATURAN:
- Baca seluruh blok [CHANNEL: ...][ANGLE: ...][PROFILER INSTRUCTION] pada user
  prompt — instruksi per channel/angle di sana MEMBIMBING isi brief, tetapi
  format output TETAP persis mengikuti [OUTPUT REQUIREMENT].
- Balas HANYA satu objek JSON valid sesuai [OUTPUT REQUIREMENT]: tanpa
  penjelasan, tanpa markdown, tanpa membungkusnya dalam key lain (mis.
  strategy_brief).
- Faktual: hanya gunakan fakta dari input. Jika data kurang, catat di
  needs_verification — JANGAN mengarang angka, nama, atau tanggal.
- Bahasa Indonesia yang natural dan jurnalistik.',
  'Seed base prompt v1 (migrasi 120) — cegah output membajak format strategy_brief.',
  '00000000-0000-4000-8000-000000000001',
  '00000000-0000-4000-8000-000000000001',
  now()
WHERE NOT EXISTS (
  SELECT 1 FROM public.taut_profiler_base_prompt_versions WHERE status = 'active'
);
