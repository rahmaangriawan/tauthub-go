-- 292 — Pelanggaran media + penangguhan (suspend) domain.
--
-- Dua kebutuhan admin yang belum punya tempat penyimpanan:
--
-- 1. Catatan pelanggaran per media, bertingkat: tidak ada / ringan / berat.
--    Disimpan sebagai RIWAYAT (bukan satu kolom di tabel media) supaya admin
--    bisa melihat kapan dan oleh siapa pelanggaran dicatat, dan supaya
--    pencabutan pelanggaran tidak menghapus jejaknya.
--
-- 2. Suspend domain. Menyembunyikan media dari katalog klien/mitra dan menolak
--    publikasi baru, TANPA menghapus data. Reversible: admin bisa mencabutnya
--    dan media kembali seperti semula. Ini beda dengan arsip (niat pemilik)
--    dan hidden (sembunyi tanpa alasan tercatat) — suspend selalu punya sebab.
--
-- Catatan: kolom `status` yang ada tetap dipakai apa adanya. Suspend TIDAK
-- mengubah status menjadi 'hidden' karena 'hidden' bisa diubah mitra lewat
-- halaman media mereka; suspend harus dikendalikan admin saja. Penyaringan
-- katalog membaca kolom suspended_at di bawah.

CREATE TABLE IF NOT EXISTS public.media_violations (
  id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  media_id      character(36) NOT NULL,
  -- none | ringan | berat
  level         text NOT NULL,
  note          text,
  created_by    uuid,
  created_at    timestamptz NOT NULL DEFAULT NOW(),
  -- Pelanggaran bisa dicabut admin; barisnya tetap ada sebagai riwayat.
  revoked_at    timestamptz,
  revoked_by    uuid,
  CONSTRAINT fk_media_violations_media
    FOREIGN KEY (media_id) REFERENCES public.media(id) ON DELETE CASCADE,
  CONSTRAINT ck_media_violations_level
    CHECK (level IN ('none', 'ringan', 'berat'))
);

CREATE INDEX IF NOT EXISTS idx_media_violations_media
  ON public.media_violations (media_id, created_at DESC);

COMMENT ON TABLE public.media_violations IS
  'Riwayat pelanggaran media (ringan/berat) beserta catatan dan admin pencatat.';

-- Suspend: kolom terpisah, bukan status, supaya tidak bisa diubah mitra dan
-- supaya alasan + waktu selalu tercatat.
ALTER TABLE public.media
  ADD COLUMN IF NOT EXISTS suspended_at timestamptz,
  ADD COLUMN IF NOT EXISTS suspended_reason text,
  ADD COLUMN IF NOT EXISTS suspended_by uuid;

COMMENT ON COLUMN public.media.suspended_at IS
  'Waktu admin menyuspend domain. Selama terisi, media tidak tampil di katalog dan publikasi baru ditolak.';

CREATE INDEX IF NOT EXISTS idx_media_suspended
  ON public.media (suspended_at)
  WHERE suspended_at IS NOT NULL;
