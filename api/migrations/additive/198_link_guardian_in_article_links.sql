-- 198 — Link Guardian: kolom hasil pemeriksaan tautan DI DALAM artikel.
--
-- Selama ini publication_link_checks hanya mencatat status URL artikelnya
-- (http_status, state). Kalau media mencabut tautan yang ditanam di badan
-- artikel, tidak ada satu pun kolom yang bisa menyimpan temuan itu, sehingga
-- status "Tautan Dihapus" tidak mungkin ditampilkan.
--
-- Kolom baru (additive, idempoten):
--   in_article_state       'present' | 'removed' | 'unknown' (NULL = belum diperiksa)
--   in_article_anchor      anchor yang diharapkan ada di artikel (snapshot saat cek)
--   in_article_found       jumlah tautan yang ditemukan di halaman artikel
--   in_article_checked_at  kapan pemeriksaan isi artikel terakhir dijalankan
--   in_article_error       sebab kegagalan (mis. halaman tidak bisa dibaca)
--
-- Pemeriksaan isi artikel butuh membaca halaman (bukan hanya status HTTP), dan
-- banyak situs mitra memblokir IP server kita (lihat state 'unreachable').
-- Karena itu hasilnya boleh 'unknown' dan tidak pernah dipaksa jadi 'removed'.

ALTER TABLE public.publication_link_checks
  ADD COLUMN IF NOT EXISTS in_article_state      text,
  ADD COLUMN IF NOT EXISTS in_article_anchor     text,
  ADD COLUMN IF NOT EXISTS in_article_found      integer,
  ADD COLUMN IF NOT EXISTS in_article_checked_at timestamptz,
  ADD COLUMN IF NOT EXISTS in_article_error      text;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'publication_link_checks_in_article_state_check'
  ) THEN
    ALTER TABLE public.publication_link_checks
      ADD CONSTRAINT publication_link_checks_in_article_state_check
      CHECK (in_article_state IS NULL OR in_article_state IN ('present', 'removed', 'unknown'));
  END IF;
END $$;

-- Query pemakaian: baris "Tautan Dihapus" dan baris yang belum pernah diperiksa.
CREATE INDEX IF NOT EXISTS publication_link_checks_in_article_state_idx
  ON public.publication_link_checks (in_article_state);
