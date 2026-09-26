-- 168 — Isi taut_entry_mode untuk kampanye yang sudah ada, hanya dari bukti.
--
-- Aturan (tanpa tebakan):
--   'ai'     → punya hasil Profiler, ATAU punya media hasil Matchmaker AI.
--   'manual' → punya media, semuanya dari pemilihan manual, tanpa hasil Profiler.
--   NULL     → tidak ada bukti (mis. kampanye kosong) — dibiarkan agar UI jatuh
--              ke query param / perilaku AI seperti sebelumnya.
--
-- Penting: "tanpa hasil Profiler" saja BUKAN bukti jalur manual. Di produksi ada
-- kampanye AI lama (sebelum Profiler v2) yang tidak punya hasil Profiler tetapi
-- media-nya jelas hasil Matchmaker AI — kampanye itu harus tetap 'ai'.

BEGIN;

UPDATE public.campaigns c
SET taut_entry_mode = 'ai'
WHERE c.taut_entry_mode IS NULL
  AND (
    EXISTS (
      SELECT 1 FROM public.taut_profiler_results r
      JOIN public.taut_profiler_submissions s ON s.id = r.submission_id
      WHERE s.campaign_id = c.id
    )
    OR EXISTS (
      SELECT 1 FROM public.taut_campaign_media t
      WHERE t.campaign_id = c.id AND t.source = 'ai'
    )
  );

UPDATE public.campaigns c
SET taut_entry_mode = 'manual'
WHERE c.taut_entry_mode IS NULL
  AND EXISTS (
    SELECT 1 FROM public.taut_campaign_media t
    WHERE t.campaign_id = c.id
  )
  AND NOT EXISTS (
    SELECT 1 FROM public.taut_campaign_media t
    WHERE t.campaign_id = c.id AND t.source <> 'manual'
  )
  AND NOT EXISTS (
    SELECT 1 FROM public.taut_profiler_results r
    JOIN public.taut_profiler_submissions s ON s.id = r.submission_id
    WHERE s.campaign_id = c.id
  );

COMMIT;
