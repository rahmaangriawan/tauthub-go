-- 162: Jejak persetujuan mitra untuk akses crawler (migrasi 161).
--
-- Membuat kredensial crawler = memberi akses otomatis ke website mitra, jadi
-- wajib ada persetujuan eksplisit yang tercatat: kapan disetujui dan versi
-- dokumen mana yang disetujui (dokumen bisa berubah versi di kemudian hari).
--
-- Aditif: dua kolom baru pada tabel crawler_credentials (migrasi 161).

ALTER TABLE public.crawler_credentials
    ADD COLUMN IF NOT EXISTS consent_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS consent_doc_version TEXT;