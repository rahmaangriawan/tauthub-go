-- 271 — Riwayat perubahan field profiler (audit nilai lama -> nilai baru).
--
-- `writeUpdates` selama ini menggabungkan jsonb dengan `||` sehingga nilai
-- lama hilang tanpa jejak. Tabel ini menyimpan satu baris per field yang
-- nilainya benar-benar berubah, supaya perubahan bisa ditelusuri per field.
--
-- Sengaja TANPA foreign key cascade ke profiler_profiles: riwayat harus tetap
-- ada walau brief di-soft-delete (profiler memang tidak pernah hard-delete).
CREATE TABLE IF NOT EXISTS public.profiler_field_history (
  id char(36) PRIMARY KEY,
  profile_id char(36) NOT NULL,
  brand_id char(36) NOT NULL,
  field_key varchar(80) NOT NULL,
  old_value text,
  new_value text,
  status varchar(20),
  source varchar(20),
  changed_by char(36),
  field_version integer,
  changed_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_profiler_field_history_lookup
  ON public.profiler_field_history (profile_id, field_key, changed_at DESC);
