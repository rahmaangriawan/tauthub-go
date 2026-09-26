-- Progress persen untuk animasi distribusi publikasi (UI polling).
-- Kolom baru di publication_jobs; default 0, di-update worker per tahap.
ALTER TABLE ONLY public.publication_jobs
  ADD COLUMN IF NOT EXISTS progress integer DEFAULT 0;
