-- 287 — SLA 3×24 jam untuk persetujuan media.
--
-- Kebijakan: pesanan publikasi yang masih `waiting_partner` melewati SLA 3×24
-- jam mendapat perpanjangan 1 hari, lalu dibatalkan otomatis dan nilainya
-- dikembalikan ke saldo akun klien (bukan uang tunai — sesuai kebijakan
-- non-refund: restitusi hanya ke Saldo Akun Tauthub).
--
-- Titik mulai tenggat: COALESCE(scheduled_at, created_at). Jadwal terbit dipakai
-- bila ada karena itu yang dijanjikan ke klien; pesanan tanpa jadwal memakai
-- waktu pesanan dibuat supaya tetap punya tenggat. Tenggat TIDAK disimpan di
-- kolom: scheduled_at bisa diubah lewat /publications/{id}/schedule, dan kolom
-- deadline yang basi akan membuat dua sumber kebenaran.
--
-- Dua tipe notifikasi baru (dipakai scheduler schSweepPublicationSLA):
--   publication.sla_extended  — tenggat lewat, ada 1 hari tambahan
--   publication.sla_cancelled — pesanan dibatalkan otomatis, saldo kembali
--
-- Daftar tipe diambil dari definisi terakhir (273) dan hanya ditambah dua nilai,
-- supaya tipe lain tidak ikut terbuang.

ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type
  CHECK (((type)::text = ANY ((ARRAY[
    'announcement.new', 'billing.topup_needed', 'changelog.new', 'chat.message',
    'media.approved', 'media.crawl_blocked', 'media.dewan_pers.approved',
    'media.dewan_pers.rejected', 'media.dewan_pers.submitted', 'media.domain_down',
    'media.negotiated', 'media.rejected', 'media.resubmitted', 'media.submitted',
    'partner.wa_required',
    'pr.score_updated', 'pr.verification_completed',
    'publication.auto_replaced', 'publication.client_approved', 'publication.failed',
    'publication.late_approval', 'publication.link_lost',
    'publication.link_replacement_requested', 'publication.partner_approved',
    'publication.partner_rejected', 'publication.published', 'publication.reminder',
    'publication.revision_accepted', 'publication.revision_rejected',
    'publication.revision_requested', 'publication.sla_cancelled',
    'publication.sla_extended', 'publication.url_requested',
    'publication.waiting_partner',
    'referral.commission.credited', 'referral.commission.reversed',
    'referral.cpa.credited', 'referral.domain.approved', 'referral.domain.rejected',
    'secure_direct.approved', 'secure_direct.requested',
    'ticket.closed', 'ticket.created', 'ticket.replied',
    'variant_batch.failed', 'vip.disabled', 'vip.enabled',
    'withdrawal.paid', 'withdrawal.processing', 'withdrawal.rejected',
    'withdrawal.requested'
  ])::text[])));

-- Index sweep SLA: scheduler hanya menyapu pesanan yang masih menunggu media.
-- Partial index menjaga biaya tetap kecil walau tabel publications membesar.
CREATE INDEX IF NOT EXISTS ix_publications_sla_waiting
    ON public.publications (status, scheduled_at, created_at)
    WHERE status = 'waiting_partner';
