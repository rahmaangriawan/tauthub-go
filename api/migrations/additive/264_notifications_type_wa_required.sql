-- Tambahkan partner.wa_required ke daftar tipe notifikasi.
--
-- Migrasi 192 memperkenalkan tipe ini, tetapi 224, 228, dan 259 masing-masing
-- menulis ulang CHECK dengan daftar lengkap dan tidak menyertakannya. Akibatnya
-- INSERT di partner_wa_reminder.go:90 ditolak constraint, dan karena pemanggil
-- hanya mencatat warning, notifikasinya hilang tanpa jejak.
--
-- Daftar di bawah diambil dari definisi terakhir (259) dan hanya ditambah satu
-- nilai yang hilang, supaya tidak ada tipe lain yang ikut terbuang.
ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type
  CHECK (((type)::text = ANY ((ARRAY[
    'announcement.new', 'changelog.new', 'chat.message',
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
    'publication.revision_requested', 'publication.url_requested',
    'publication.waiting_partner',
    'referral.commission.credited', 'referral.commission.reversed',
    'referral.cpa.credited', 'referral.domain.approved', 'referral.domain.rejected',
    'secure_direct.approved', 'secure_direct.requested',
    'ticket.closed', 'ticket.created', 'ticket.replied',
    'variant_batch.failed', 'vip.disabled', 'vip.enabled',
    'withdrawal.paid', 'withdrawal.processing', 'withdrawal.rejected',
    'withdrawal.requested'
  ])::text[])));
