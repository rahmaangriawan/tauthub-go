-- 272 — Slot artikel per media: satu domain boleh punya beberapa artikel.
--
-- Sampai migrasi ini, `taut_campaign_media` memakai UNIQUE
-- (campaign_id, media_id) dan tiap baris hanya menyimpan satu
-- article_document_id. Akibatnya klien tidak bisa memesan dua artikel untuk
-- satu domain dalam satu kampanye: INSERT kedua ditolak index, dan jalur
-- addManualMedia menelan error itu sehingga media tampak "diabaikan".
--
-- Solusinya BUKAN kolom quantity, melainkan kolom `slot`: setiap artikel
-- menjadi baris sendiri. Semua state yang dibutuhkan satu artikel
-- (article_document_id, article_status, regen_count, angle_slug, writer_topic,
-- proper_*) sudah ada di baris itu, jadi biaya, QC, editor, dan publikasi ikut
-- otomatis per artikel tanpa tabel paralel.
--
-- Index unik sengaja TETAP tanpa predikat status: baris yang statusnya
-- 'removed' masih menempati slotnya, dan dihidupkan lagi lewat
-- UPDATE status='selected' (perilaku TautPlacementPut sejak awal). Menambah
-- slot baru selalu mulai dari MAX(slot)+1, jadi slot yang pernah dilepas tidak
-- dipakai ulang dan artikel lama tidak pernah tertimpa.

BEGIN;

ALTER TABLE public.taut_campaign_media
  ADD COLUMN IF NOT EXISTS slot smallint NOT NULL DEFAULT 1;

-- Baris lama semuanya artikel pertama.
UPDATE public.taut_campaign_media SET slot = 1 WHERE slot IS NULL OR slot < 1;

DROP INDEX IF EXISTS public.uq_taut_campaign_media;

CREATE UNIQUE INDEX IF NOT EXISTS uq_taut_campaign_media
  ON public.taut_campaign_media (campaign_id, media_id, slot);

-- Audit Gatekeeper menilai isi satu artikel, bukan satu media. Tanpa slot,
-- empat artikel satu domain akan berbagi satu hasil audit.
ALTER TABLE public.taut_gatekeeper_audits
  ADD COLUMN IF NOT EXISTS slot smallint NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS ix_taut_gatekeeper_audits_placement
  ON public.taut_gatekeeper_audits (campaign_id, media_id, slot, created_at DESC);

-- Log eksekusi agent yang terikat media (writer/gatekeeper) juga perlu slot
-- supaya bisa dibedakan artikel mana yang menghasilkan run itu.
ALTER TABLE public.taut_agent_runs
  ADD COLUMN IF NOT EXISTS slot smallint NOT NULL DEFAULT 1;

COMMIT;
