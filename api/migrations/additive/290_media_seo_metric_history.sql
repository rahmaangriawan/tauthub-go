-- 290 — Riwayat metrik SEO per domain + penanda domain pada metrik aktif.
--
-- Dua masalah yang diselesaikan:
--
-- 1. Ketika admin mengganti website_url sebuah media, baris media_seo_metrics
--    lama tetap tersimpan apa adanya. Karena tabel itu satu baris per media
--    (PRIMARY KEY media_id), angka DA/PA/AI Trust milik domain LAMA terus
--    ditampilkan seolah milik domain baru — menyesatkan, dan menahan
--    pemeriksaan otomatis karena baris dianggap "sudah berskor".
--
--    Kolom `domain` di media_seo_metrics mencatat domain mana yang diukur.
--    Saat domain berubah, baris lama dipindahkan ke media_seo_metric_history
--    (riwayat tidak hilang) lalu dihapus dari tabel aktif supaya pemeriksaan
--    berikutnya menulis baris baru untuk domain baru.
--
-- 2. Tidak ada tempat menyimpan riwayat pemeriksaan. Tabel history menampung
--    satu baris per perpindahan domain; riwayat pemeriksaan rutin tetap
--    tercatat di media_seo_metrics.updated_at dan audit_logs.
--
-- Catatan: riwayat trafik (SimilarWeb) tidak butuh tabel baru —
-- media_similarweb_snapshots sudah menyimpan banyak baris per media dan
-- mencatat domain di dalam kolom insights.
ALTER TABLE public.media_seo_metrics
  ADD COLUMN IF NOT EXISTS domain text;

COMMENT ON COLUMN public.media_seo_metrics.domain IS
  'Domain yang diukur saat baris ini ditulis. NULL = baris lama sebelum kolom ini ada.';

CREATE TABLE IF NOT EXISTS public.media_seo_metric_history (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  media_id        character(36) NOT NULL,
  domain          text,
  da              smallint,
  pa              smallint,
  spam_score      smallint,
  ai_trust_score  smallint,
  ai_citation     boolean,
  ai_citation_checked_at timestamptz,
  source          text,
  checked_at      timestamptz,
  -- Alasan baris dipindahkan: 'domain_changed' | 'manual_reset'
  reason          text NOT NULL DEFAULT 'domain_changed',
  archived_at     timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_media_seo_metric_history_media
    FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_media_seo_metric_history_media
  ON public.media_seo_metric_history (media_id, archived_at DESC);

COMMENT ON TABLE public.media_seo_metric_history IS
  'Arsip metrik SEO saat domain media diganti, supaya angka domain lama tidak hilang dan tidak tercampur dengan domain baru.';
