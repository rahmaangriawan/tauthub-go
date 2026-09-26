-- Dewan Pers per media/domain (bukan per profil mitra).
--
-- Status verifikasi Dewan Pers melekat pada MEDIA: mitra mengisi status
-- (Ya/Tidak) + keterangan + nomor sertifikat + dokumen bukti (opsional)
-- lewat halaman Media mitra. Data lama otomatis dianggap "Tidak"
-- (default false) — tidak ada data yang diubah/dihapus.

ALTER TABLE public.media
    ADD COLUMN IF NOT EXISTS dewan_pers_verified boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS dewan_pers_note text,
    ADD COLUMN IF NOT EXISTS dewan_pers_cert_no character varying(200),
    ADD COLUMN IF NOT EXISTS dewan_pers_doc_key character varying(500);
