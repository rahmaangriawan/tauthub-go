-- 299 — Kesimpulan manual/AI yang diisi admin.
--
-- Dua perubahan:
--   1. Status event baru 'manual': admin menimpa/mengisi kesimpulan media
--      langsung dari panel Monitoring Kesimpulan Website (dengan atau tanpa
--      bantuan AI). Status ini tampil sebagai badge "Manual" di monitoring dan
--      tidak diganggu sweep antrean (media.ai_summary sudah terisi).
--   2. Kolom source: membedakan asal kesimpulan — 'auto' (job crawl+AI),
--      'manual' (admin tulis sendiri), 'ai' (admin jalankan AI atas teks yang
--      ditempel). Kolom lama di-backfill 'auto' supaya riwayat tetap bermakna.

ALTER TABLE public.media_ai_summary_events DROP CONSTRAINT IF EXISTS ck_media_ai_summary_events_status;
ALTER TABLE public.media_ai_summary_events ADD CONSTRAINT ck_media_ai_summary_events_status
  CHECK (((status)::text = ANY ((ARRAY[
    'queued'::character varying, 'success'::character varying,
    'failed'::character varying, 'manual'::character varying
  ])::text[])));

ALTER TABLE public.media_ai_summary_events ADD COLUMN IF NOT EXISTS source character varying(12) NOT NULL DEFAULT 'auto';
