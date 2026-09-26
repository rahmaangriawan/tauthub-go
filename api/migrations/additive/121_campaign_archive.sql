-- Soft delete kampanye Taut Pintar: arsip & restore.
-- Hard delete (purge) menjalankan DELETE baris campaigns; child FK dengan
-- ON DELETE CASCADE ikut terhapus, sisanya (SET NULL) menjaga riwayat lain.
ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS archived_at timestamptz;
CREATE INDEX IF NOT EXISTS idx_campaigns_brand_archived ON public.campaigns (brand_id, archived_at);
