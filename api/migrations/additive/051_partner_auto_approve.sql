-- 051: Partner auto-approve — publikasi dari klien otomatis disetujui
-- tanpa menunggu klik "Setujui" di portal mitra.

ALTER TABLE public.partners
  ADD COLUMN IF NOT EXISTS auto_approve boolean DEFAULT false NOT NULL;
