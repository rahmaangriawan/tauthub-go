-- 200 — Taut Pintar: jenis artikel per media pada satu kampanye.
--
-- Writer hanya punya satu prompt per agent (per tipe media), padahal jenis
-- artikel bisa berbeda untuk media yang sama: berita/editorial, opini, atau
-- advertorial. Kolom ini menyimpan pilihan jenis artikel per baris media pada
-- sebuah kampanye (satu kampanye boleh menulis ke banyak media sekaligus),
-- sehingga Writer bisa memilih varian prompt yang tepat.
--
-- Nilainya adalah taut_article_types.code (dibuat di migrasi 201): daftar jenis
-- artikel beserta prompt berversinya yang dikelola di Taut Prompt Studio.
-- NULL = perilaku lama (prompt agent apa adanya), sehingga artikel yang sudah
-- berjalan tidak berubah.

ALTER TABLE public.taut_campaign_media
  ADD COLUMN IF NOT EXISTS content_type varchar(50);

COMMENT ON COLUMN public.taut_campaign_media.content_type IS
  'Kode jenis artikel (taut_article_types.code) untuk media ini pada kampanye ini. NULL = pakai prompt agent apa adanya.';

CREATE INDEX IF NOT EXISTS taut_campaign_media_content_type_idx
  ON public.taut_campaign_media (content_type);
