-- 225_pubs_seen_at.sql
-- Tanda waktu terakhir mitra membuka halaman Publikasinya. Dipakai badge
-- merah "Publikasi" di sidebar portal mitra: publikasi yang dibuat SETELAH
-- tanda waktu ini terhitung belum dibaca (menu jadi berangka), dan angkanya
-- hilang begitu mitra membukanya (mark-seen).

ALTER TABLE public.partners ADD COLUMN IF NOT EXISTS pub_seen_at timestamptz;
