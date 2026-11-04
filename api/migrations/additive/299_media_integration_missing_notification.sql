-- 299 — tipe notifikasi "media disembunyikan dari katalog karena belum tersambung".
--
-- Media non-manual (WordPress/Blogspot/Custom/webhook) tanpa koneksi aktif
-- tidak lagi tampil di katalog klien dan tidak bisa dipesan. Scheduler
-- memberi tahu mitra sekali per media (dedupe_key media.integration_missing:<id>)
-- supaya mereka menyambungkan situsnya di halaman Integrasi.
--
-- Tipe baru ditambahkan ke constraint yang berlaku (definisi terakhir di 298)
-- tanpa menghapus nilai lama, supaya baris historis tetap valid.

ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type
  CHECK (((type)::text = ANY ((ARRAY[
    'announcement.new', 'billing.balance_forfeited', 'billing.balance_forfeit_warning',
    'billing.topup_needed', 'changelog.new', 'chat.message',
    'media.approved', 'media.crawl_blocked', 'media.dewan_pers.approved',
    'media.dewan_pers.rejected', 'media.dewan_pers.submitted', 'media.domain_down',
    'media.integration_missing',
    'media.negotiated', 'media.rejected', 'media.resubmitted', 'media.submitted',
    'partner.wa_required',
    'pr.score_updated', 'pr.verification_completed',
    'publication.auto_replaced', 'publication.cancelled_by_admin',
    'publication.client_approved', 'publication.failed',
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
