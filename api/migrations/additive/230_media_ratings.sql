-- 230 — Rating kecepatan terbit per media.
--
-- Sumber rating: bintang 1–5 di dialog "Konfirmasi Sesi Obrolan" (tombol
-- Hubungi pada halaman ringkasan). Sebelumnya nilai itu hanya state tampilan
-- dan hilang saat dialog ditutup; sekarang disimpan di sini lalu ditampilkan
-- sebagai rata-rata di kolom SLA katalog media.
--
-- Satu brand hanya punya satu rating per media (dinilai ulang = menimpa),
-- supaya satu klien tidak bisa menumpuk rating dan menggeser rata-rata.
-- Semua id mengikuti konvensi tabel lama di basis data ini: character(36),
-- bukan uuid.
CREATE TABLE IF NOT EXISTS public.media_ratings (
  id character(36) PRIMARY KEY DEFAULT (gen_random_uuid()::text),
  media_id character(36) NOT NULL REFERENCES public.media(id) ON DELETE CASCADE,
  brand_id character(36) NOT NULL REFERENCES public.brands(id) ON DELETE CASCADE,
  rating smallint NOT NULL CHECK (rating BETWEEN 1 AND 5),
  publication_id character(36) REFERENCES public.publications(id) ON DELETE SET NULL,
  created_by character(36) REFERENCES public.users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS media_ratings_brand_media_uidx
  ON public.media_ratings (brand_id, media_id);

CREATE INDEX IF NOT EXISTS media_ratings_media_idx
  ON public.media_ratings (media_id);

COMMENT ON TABLE public.media_ratings IS
  'Rating kecepatan terbit (1–5) dari klien saat menghubungi mitra; ditampilkan sebagai rata-rata di kolom SLA katalog media.';
