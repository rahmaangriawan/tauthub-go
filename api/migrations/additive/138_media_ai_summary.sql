-- 138 — Kesimpulan Website per media (profil deskriptif hasil AI untuk Taut Pintar).
-- - media.ai_summary: teks kesimpulan 3-6 kalimat (karakter, audiens, topik, gaya).
-- - media.ai_summary_generated_at: penanda kapan dibuat (freshness, best-effort).
-- Diisi otomatis oleh job "media_ai_summary" (antrean global, lihat lib/media/ai-summary.ts).

BEGIN;

ALTER TABLE public.media
  ADD COLUMN IF NOT EXISTS ai_summary text;

ALTER TABLE public.media
  ADD COLUMN IF NOT EXISTS ai_summary_generated_at timestamp(6) with time zone;

COMMIT;
