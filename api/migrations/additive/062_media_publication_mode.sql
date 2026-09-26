-- Mode Publikasi Media (per media/domain, bukan per profil mitra).
--
-- publication_mode menentukan bagaimana artikel dari Client diproses saat
-- didistribusikan ke media ini:
--   draft   → artikel dikirim ke WordPress sebagai DRAFT (menunggu tindakan Mitra)
--   publish → artikel langsung dipublikasikan tanpa persetujuan manual Mitra
--
-- Default 'draft' = aman untuk media existing: media lama TIDAK tiba-tiba
-- auto-publish tanpa persetujuan Mitra. Nilai ini disnapshot ke
-- publications.publish_status saat publikasi dibuat, jadi mengubah mode
-- media tidak mengubah publikasi lama yang sudah diproses.
--
-- Additive & backward compatible: tidak menghapus/mengubah data existing.
-- Idempotent: ADD COLUMN IF NOT EXISTS + DO block untuk CHECK constraint.

ALTER TABLE public.media
    ADD COLUMN IF NOT EXISTS publication_mode character varying(20) NOT NULL DEFAULT 'draft';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'ck_media_publication_mode'
          AND conrelid = 'public.media'::regclass
    ) THEN
        ALTER TABLE public.media
            ADD CONSTRAINT ck_media_publication_mode
            CHECK (publication_mode IN ('draft', 'publish'));
    END IF;
END $$;
