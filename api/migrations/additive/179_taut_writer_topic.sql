-- 179_taut_writer_topic.sql
-- Topik/konteks klien dari modal Pengelola Naskah (The Writer) disimpan per media
-- supaya Gatekeeper AI Overview Checker memverifikasi klaim ke konteks yang SAMA
-- dengan yang dipakai Writer, bukan hanya brief Profiler. Tanpa kolom ini,
-- Checker menandai nama/angka/kutipan dari topik klien sebagai "tanpa dasar".
ALTER TABLE taut_campaign_media ADD COLUMN IF NOT EXISTS writer_topic text;
