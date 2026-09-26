-- 150 — Revisi & ACC klien per domain di ringkasan kampanye.
-- Aditif: kolom baru + DROP+ADD constraint tipe notifikasi. Tanpa DROP tabel,
-- tanpa mengubah data yang ada.
--
-- Alur yang didukung:
--   * Klien mengedit artikel di popup ringkasan → disimpan sebagai VERSI BARU
--     (content_versions, sudah ada) + komentar + lampiran screenshot.
--   * ACC klien dicatat terpisah dan TIDAK mengubah publications.status —
--     status terbit tetap digerakkan persetujuan mitra (waiting_partner →
--     partner_approved), supaya pemilik situs tetap punya suara.

-- 1) Lampiran pada komentar review (screenshot bukti dari klien).
ALTER TABLE public.review_comments
  ADD COLUMN IF NOT EXISTS attachment_url character varying(500);
ALTER TABLE public.review_comments
  ADD COLUMN IF NOT EXISTS attachment_name character varying(255);

-- 2) Persetujuan klien atas artikel, terpisah dari persetujuan mitra.
ALTER TABLE public.publications
  ADD COLUMN IF NOT EXISTS client_approved_at timestamp(6) with time zone;
ALTER TABLE public.publications
  ADD COLUMN IF NOT EXISTS client_approved_by character(36);

-- 3) Dua tipe notifikasi baru. Daftar lengkap ditulis ulang (pola DROP+ADD yang
--    sama seperti migrasi 069/088/148) — kalau ada nilai yang tertinggal,
--    constraint ini akan menolak tipe notifikasi lain yang sudah berjalan.
DO $$
BEGIN
  ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
EXCEPTION WHEN others THEN NULL;
END $$;

ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type
  CHECK (type IN (
    'publication.waiting_partner',
    'publication.partner_approved',
    'publication.partner_rejected',
    'publication.auto_replaced',
    'publication.published',
    'publication.failed',
    'publication.url_requested',
    'publication.reminder',
    'publication.late_approval',
    'publication.link_lost',
    'publication.link_replacement_requested',
    'publication.revision_requested',
    'publication.client_approved',
    'variant_batch.failed',
    'pr.score_updated',
    'pr.verification_completed',
    'media.submitted',
    'media.approved',
    'media.rejected',
    'media.resubmitted',
    'media.negotiated',
    'media.domain_down',
    'media.dewan_pers.submitted',
    'media.dewan_pers.approved',
    'media.dewan_pers.rejected',
    'withdrawal.requested',
    'withdrawal.processing',
    'withdrawal.paid',
    'withdrawal.rejected',
    'referral.domain.approved',
    'referral.domain.rejected',
    'referral.cpa.credited',
    'referral.commission.credited',
    'referral.commission.reversed',
    'vip.enabled',
    'vip.disabled'
  ));
