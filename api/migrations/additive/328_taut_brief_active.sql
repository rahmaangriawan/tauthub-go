-- 328 — Brief aktif per pengguna per brand. Dipakai Pengelola Naskah supaya
-- brief yang terakhir dipilih lewat tombol "Pakai untuk Artikel" di Taut Brief
-- terpilih otomatis di tiap artikel, tanpa memilih ulang. Per pengguna (bukan
-- satu untuk seluruh tim) agar dua anggota tidak saling menimpa pilihan.
-- Additive dan idempoten.

BEGIN;

CREATE TABLE IF NOT EXISTS public.taut_brief_active (
  brand_id   character(36) NOT NULL,
  user_id    character(36) NOT NULL,
  profile_id character(36) NOT NULL REFERENCES public.profiler_profiles(id) ON DELETE CASCADE,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (brand_id, user_id)
);

COMMIT;
