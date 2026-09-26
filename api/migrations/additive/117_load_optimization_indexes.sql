-- 117 -- Optimasi beban DB: index untuk sweep scheduler yang jalan tiap 60 detik.
-- Sebelumnya: 3 query sweep full-scan publications (kolom scheduled_at tanpa
-- index) + refresh health media full-scan tabel media -> jutaan baris dibaca
-- per hari hanya untuk sweep yang umumnya tidak menemukan apa-apa.

-- 1. Sweep publikasi due (publish-scheduler.ts: safety + reminder + late sweep).
CREATE INDEX IF NOT EXISTS ix_publications_scheduled_due
  ON public.publications (scheduled_at)
  WHERE status IN ('pending','waiting_partner','partner_approved','retrying')
    AND scheduled_at IS NOT NULL;

-- 2. refreshStaleMediaHealth (media/intelligence.ts, dipanggil tiap cron).
CREATE INDEX IF NOT EXISTS ix_media_stale_health
  ON public.media (health_computed_at, id)
  WHERE status IN ('active','draft');

-- 3. Anti-join jobs JSONB di safety sweep (payload->>'publicationId').
CREATE INDEX IF NOT EXISTS ix_jobs_live_publication
  ON public.jobs ((payload->>'publicationId'))
  WHERE kind = 'publication_execution' AND status NOT IN ('completed','failed');
