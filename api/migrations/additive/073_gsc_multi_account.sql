-- 073 — GSC multi-account per partner
--
-- Sebelumnya: gsc_connections ada UNIQUE(partner_id) → 1 partner = 1 akun Google.
-- Masalah: mitra dengan 10 domain di 5 akun berbeda akan ke-overwrite tiap OAuth.
-- Fix: drop UNIQUE(partner_id), ganti jadi UNIQUE(partner_id, email) atau
-- biarkan multi-row; properti per koneksi tetap terisolasi.
-- Backward compatible: data existing (1 row per partner) tetap aman.

-- Drop constraint lama bila ada (nama bisa berbeda per DB) — best-effort
DO $$
DECLARE r record;
BEGIN
  FOR r IN SELECT conname FROM pg_constraint WHERE conrelid = 'public.gsc_connections'::regclass AND contype = 'u'
  LOOP
    IF r.conname LIKE '%partner%' THEN
      EXECUTE format('ALTER TABLE public.gsc_connections DROP CONSTRAINT %I', r.conname);
    END IF;
  END LOOP;
END $$;

-- Pastikan email terisi untuk dedupe; fallback ke provider
ALTER TABLE public.gsc_connections ADD COLUMN IF NOT EXISTS email character varying(320);

-- Index baru untuk dedupe per (partner, email) — bukan UNIQUE strict agar
-- migrasi tidak gagal jika data lama ada email NULL; pakai partial unique
CREATE UNIQUE INDEX IF NOT EXISTS uq_gsc_connections_partner_email
  ON public.gsc_connections (partner_id, COALESCE(email, id::text));

-- Pastikan kolom yang dipakai kalkulator ada
ALTER TABLE public.gsc_connections ADD COLUMN IF NOT EXISTS selected_property_id character(36);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_gsc_connections_selected_property') THEN
    ALTER TABLE public.gsc_connections ADD CONSTRAINT fk_gsc_connections_selected_property
      FOREIGN KEY (selected_property_id) REFERENCES public.gsc_properties(id) ON DELETE SET NULL;
  END IF;
END $$;
