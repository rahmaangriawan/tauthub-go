-- 199 — Link Guardian: tabel klaim garansi tayang (SLA).
--
-- Klaim sebelumnya hanya menghasilkan notifikasi ke admin, tanpa catatan yang
-- bisa dilacak. Tabel ini menyimpan siklus klaimnya: cek kondisi domain,
-- menghubungi pemilik domain, lalu pengembalian saldo bila tidak bisa
-- diselesaikan (atau penolakan beserta alasannya).
--
-- Satu publikasi = satu klaim (UNIQUE publication_id): pengajuan ulang tidak
-- membuat baris baru, cukup memperbarui statusnya, sehingga pengembalian saldo
-- tidak mungkin terjadi dua kali untuk artikel yang sama.

CREATE TABLE IF NOT EXISTS public.link_guardian_claims (
  -- publications.id dan brands.id bertipe character(36) di skema ini (bukan
  -- uuid), jadi kunci asing dan id tabel ini mengikuti tipe yang sama. Id dibuat
  -- di aplikasi (uuid string), karena tidak ada default gen_random_uuid() yang
  -- cocok untuk kolom character.
  id                 character(36) PRIMARY KEY,
  publication_id     character(36) NOT NULL REFERENCES public.publications (id) ON DELETE CASCADE,
  brand_id           character(36) NOT NULL REFERENCES public.brands (id) ON DELETE CASCADE,
  -- Siklus kerja admin; 'submitted' = klien baru mengajukan.
  status             text NOT NULL DEFAULT 'submitted',
  -- Kondisi yang dilaporkan saat pengajuan: 'gone' (artikel 404/hilang) atau
  -- 'link_removed' (tautan di dalam artikel dicabut media).
  detection_state    text,
  anchor_expected    text,
  domain_check_note  text,
  owner_contact_note text,
  resolution_note    text,
  rejection_reason   text,
  refund_tc          integer,
  refund_idr         integer,
  refunded_at        timestamptz,
  created_by         character(36),
  updated_by         character(36),
  created_at         timestamptz NOT NULL DEFAULT NOW(),
  updated_at         timestamptz NOT NULL DEFAULT NOW(),
  CONSTRAINT link_guardian_claims_status_check
    CHECK (status IN ('submitted', 'checking', 'contacted', 'refunded', 'rejected')),
  CONSTRAINT link_guardian_claims_detection_check
    CHECK (detection_state IS NULL OR detection_state IN ('gone', 'link_removed'))
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint WHERE conname = 'link_guardian_claims_publication_key'
  ) THEN
    ALTER TABLE public.link_guardian_claims
      ADD CONSTRAINT link_guardian_claims_publication_key UNIQUE (publication_id);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS link_guardian_claims_brand_idx
  ON public.link_guardian_claims (brand_id, status);
CREATE INDEX IF NOT EXISTS link_guardian_claims_status_idx
  ON public.link_guardian_claims (status, updated_at DESC);
