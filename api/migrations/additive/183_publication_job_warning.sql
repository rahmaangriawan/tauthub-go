-- 183_publication_job_warning.sql
-- Peringatan non-fatal pada pengiriman publikasi. Dipakai saat gambar artikel
-- gagal diunggah ke media library mitra: artikel tetap terbit, tetapi
-- kegagalannya tidak boleh hilang diam-diam — sebelumnya hanya error_code/
-- error_message yang diisi ketika publikasi benar-benar gagal.
ALTER TABLE public.publication_jobs
    ADD COLUMN IF NOT EXISTS warning_message text;
