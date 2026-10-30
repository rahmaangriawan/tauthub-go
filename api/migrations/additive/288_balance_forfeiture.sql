-- 288 — Saldo hangus untuk akun pasif + jejak auditnya.
--
-- Kebijakan: akun yang TIDAK punya transaksi saldo DAN tidak punya publikasi
-- selama 12 bulan dianggap pasif. Saldonya dihanguskan dan TIDAK bisa
-- dipulihkan — tetapi setiap penghangusan WAJIB tercatat agar bisa
-- dipertanggungjawabkan dan dilaporkan.
--
-- Pencatatan memakai dua lapis:
--   1. credit_ledger_entries dengan ref_type='forfeit' dan amount negatif —
--      muncul di riwayat saldo klien, jadi klien melihat sendiri kapan dan
--      berapa saldo yang hangus.
--   2. tabel balance_forfeitures — satu baris per penghangusan, menyimpan
--      nominal, alasan, dasar perhitungan (aktivitas terakhir), dan siapa
--      pelakunya. Tabel ini yang dipakai laporan admin.
--
-- Tidak ada mekanisme pembatalan/pemulihan: sesuai kebijakan, hangus bersifat
-- final.

-- 1) Tipe ledger baru.
ALTER TABLE public.credit_ledger_entries DROP CONSTRAINT IF EXISTS ck_credit_ledger_type;
ALTER TABLE public.credit_ledger_entries ADD CONSTRAINT ck_credit_ledger_type
  CHECK (ref_type::text = ANY (ARRAY[
    'topup','reserve','release','capture','refund','adjustment','forfeit'
  ]::text[]));

-- 2) Catatan penghangusan (untuk laporan & audit).
CREATE TABLE IF NOT EXISTS public.balance_forfeitures (
    id                character(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    brand_id          character(36) NOT NULL REFERENCES public.brands(id) ON DELETE CASCADE,
    amount_tc         bigint NOT NULL CHECK (amount_tc > 0),
    reason            character varying(40) NOT NULL DEFAULT 'akun_pasif_12_bulan',
    -- Dasar perhitungan: aktivitas terakhir yang ditemukan saat penghangusan.
    -- Disimpan supaya laporan bisa menjelaskan MENGAPA akun dianggap pasif.
    last_activity_at  timestamptz,
    -- Snapshot saldo sebelum hangus, untuk rekonsiliasi.
    balance_before_tc bigint NOT NULL,
    ledger_entry_id   character(36),
    created_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_balance_forfeitures_brand
    ON public.balance_forfeitures (brand_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_balance_forfeitures_created
    ON public.balance_forfeitures (created_at DESC);

-- 3) Tipe notifikasi peringatan H-7 sebelum hangus.
ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type
  CHECK (((type)::text = ANY ((ARRAY[
    'announcement.new', 'billing.balance_forfeited', 'billing.balance_forfeit_warning',
    'billing.topup_needed', 'changelog.new', 'chat.message',
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
