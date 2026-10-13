-- 214 — Pin brief (Profiler brand) di daftar sidebar.
--
-- `pinned_at` diisi saat brief disematkan dan dikosongkan saat dilepas, jadi
-- urutan daftar bisa menaruh yang terpin di atas tanpa tabel terpisah.
-- Sifatnya per-brand (bukan per-user): seluruh anggota brand melihat pin yang sama.

ALTER TABLE public.profiler_profiles
    ADD COLUMN IF NOT EXISTS pinned_at timestamp with time zone;
