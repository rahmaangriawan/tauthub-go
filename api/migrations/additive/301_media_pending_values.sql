-- Perubahan non-sensitif pada media aktif tidak lagi menyembunyikan media dari
-- katalog: nilai barunya diparkir di pending_values (nilai mentah, siap
-- diterapkan saat admin menyetujui), baris live tetap dipakai klien sampai
-- keputusan admin. pending_changes (migrasi 159) tetap dipakai untuk tampilan
-- "dari → ke" di panel admin.
ALTER TABLE public.media
  ADD COLUMN IF NOT EXISTS pending_values jsonb,
  ADD COLUMN IF NOT EXISTS pending_requested_at timestamp with time zone,
  ADD COLUMN IF NOT EXISTS pending_reminded_at timestamp with time zone;

CREATE INDEX IF NOT EXISTS ix_media_pending_values
  ON public.media USING btree (pending_requested_at)
  WHERE pending_values IS NOT NULL;
