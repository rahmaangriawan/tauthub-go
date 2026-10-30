-- 283 — Putaran kampanye: satu kampanye bisa dipakai untuk beberapa kali posting.
--
-- Sebelumnya kampanye Taut Pintar hanya sekali jalan: setelah distribusi,
-- taut_status='submitted' dan seluruh halaman jadi lihat-saja. Klien yang mau
-- memakai proyek/kampanye yang sama untuk posting berikutnya (beda waktu)
-- harus membuat kampanye baru dengan nama yang sama.
--
-- Kolom ini memisahkan "putaran" tanpa mengubah tabel publications: publikasi
-- putaran baru tetap memakai campaign_id yang sama, jadi tagihan otomatis
-- bergabung ke order kampanye (kode CAM-…) yang sudah ada.
--
-- taut_round    : nomor putaran aktif kampanye (mulai 1).
-- taut_campaign_media.round : putaran tempat satu slot artikel dibuat, supaya
--                artikel putaran lama bisa dikenali dan dikunci.
ALTER TABLE public.campaigns
  ADD COLUMN IF NOT EXISTS taut_round integer NOT NULL DEFAULT 1;

ALTER TABLE public.taut_campaign_media
  ADD COLUMN IF NOT EXISTS round smallint NOT NULL DEFAULT 1;

CREATE INDEX IF NOT EXISTS ix_taut_cm_campaign_round
  ON public.taut_campaign_media (campaign_id, round);
