-- 170 — Public id 6 karakter untuk campaigns (segmen URL /klien/{brand}/taut-pintar/{public_id}/...).
--
-- Latar: URL kampanye memakai primary key UUID 36 karakter, sementara brand dan
-- mitra sudah punya public_id 6 karakter sejak 164. Panjang UUID itu murni soal
-- penampilan (UUID tidak membocorkan apa pun), tapi URL jadi tidak seragam.
--
-- Kolom terpisah, BUKAN menumpang order_code: order_code adalah nomor order
-- keuangan (CAM-YYYYMM-NNNNN), berurutan sehingga mudah ditebak, dan NULL untuk
-- semua kampanye baru karena tidak ada penulisnya yang mengisinya.
--
-- DEFAULT di level DB dipakai sebagai satu-satunya generator supaya insert dari
-- app lama (yang masih hidup selama masa paralel) juga mendapat public_id tanpa
-- perubahan kode di sana. Unik dijaga indeks parsial; kolom tetap nullable agar
-- baris lama yang belum terbackfill tidak menggagalkan migrasi.
--
-- Resolusi public_id -> id dilakukan di lapisan parameter API, jadi seluruh
-- kolom relasi (publications.campaign_id, taut_*, audit_logs.target_id) tetap
-- berisi UUID. Lihat internal/domains/campaigns/resolve.go.

BEGIN;

ALTER TABLE public.campaigns ADD COLUMN IF NOT EXISTS public_id character varying(180);

-- Generator 6 karakter yang sudah dipakai brands/partners (dibuat di 164).
ALTER TABLE public.campaigns ALTER COLUMN public_id SET DEFAULT public._random_short_id();

-- Backfill semua baris, termasuk yang public_id-nya masih NULL. Retry saat
-- tabrakan; pola sama seperti 014 dan 164.
DO $$
DECLARE
  rec RECORD;
  candidate text;
  attempts int;
BEGIN
  FOR rec IN SELECT id FROM public.campaigns LOOP
    IF EXISTS (SELECT 1 FROM public.campaigns WHERE id = rec.id AND public_id IS NOT NULL) THEN
      CONTINUE;
    END IF;
    attempts := 0;
    LOOP
      candidate := public._random_short_id();
      BEGIN
        UPDATE public.campaigns SET public_id = candidate WHERE id = rec.id;
        EXIT;
      EXCEPTION WHEN unique_violation THEN
        attempts := attempts + 1;
        IF attempts > 50 THEN
          RAISE EXCEPTION 'public_id campaign tidak unik setelah 50 percobaan: %', rec.id;
        END IF;
      END;
    END LOOP;
  END LOOP;
END $$;

CREATE UNIQUE INDEX IF NOT EXISTS uq_campaigns_public_id
  ON public.campaigns (public_id) WHERE public_id IS NOT NULL;

COMMIT;
