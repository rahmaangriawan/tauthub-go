-- Tab "Syarat & Ketentuan" Integrasi mitra: tanggal persetujuan (null = belum setuju).
ALTER TABLE public.partners
  ADD COLUMN IF NOT EXISTS integration_terms_accepted_at timestamp with time zone;
