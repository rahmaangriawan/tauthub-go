-- 259 — Tiket dukungan mitra.
--
-- Mitra membuka tiket (subjek, kategori, prioritas) dan berbincang dengan
-- admin di dalamnya. Setelah admin membalas, status otomatis "diproses";
-- mitra atau admin dapat menandai "selesai". Unread dihitung dari read_at
-- pesan lawan bicara, jadi tidak ada kolom penghitung yang bisa basi.
--
-- Aditif: hanya menambah tabel baru + memperluas daftar tipe notifikasi.

CREATE TABLE IF NOT EXISTS public.support_tickets (
    id              character(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    partner_id      character(36) NOT NULL REFERENCES public.partners(id) ON DELETE CASCADE,
    created_by      character(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    subject         character varying(200) NOT NULL,
    category        character varying(20) NOT NULL DEFAULT 'lainnya'
        CHECK (category IN ('teknis', 'pembayaran', 'publikasi', 'lainnya')),
    priority        character varying(10) NOT NULL DEFAULT 'normal'
        CHECK (priority IN ('rendah', 'normal', 'tinggi')),
    status          character varying(12) NOT NULL DEFAULT 'buka'
        CHECK (status IN ('buka', 'diproses', 'selesai')),
    last_message_at timestamptz NOT NULL DEFAULT now(),
    last_message_by character varying(10)
        CHECK (last_message_by IN ('mitra', 'admin')),
    closed_at       timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_support_tickets_partner
    ON public.support_tickets (partner_id, last_message_at DESC);
CREATE INDEX IF NOT EXISTS ix_support_tickets_queue
    ON public.support_tickets (status, last_message_at DESC);

CREATE TABLE IF NOT EXISTS public.support_ticket_messages (
    id             character(36) PRIMARY KEY DEFAULT gen_random_uuid()::text,
    ticket_id      character(36) NOT NULL REFERENCES public.support_tickets(id) ON DELETE CASCADE,
    author_role    character varying(10) NOT NULL
        CHECK (author_role IN ('mitra', 'admin')),
    author_user_id character(36) REFERENCES public.users(id) ON DELETE SET NULL,
    body           text NOT NULL,
    read_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_support_ticket_messages_ticket
    ON public.support_ticket_messages (ticket_id, created_at);

-- Tipe notifikasi tiket. CHECK constraint harus ditulis ulang dengan daftar
-- lengkap (pola yang sama dengan migrasi 228).
ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type
  CHECK (((type)::text = ANY ((ARRAY['announcement.new', 'changelog.new', 'chat.message', 'media.approved', 'media.crawl_blocked', 'media.dewan_pers.approved', 'media.dewan_pers.rejected', 'media.dewan_pers.submitted', 'media.domain_down', 'media.negotiated', 'media.rejected', 'media.resubmitted', 'media.submitted', 'pr.score_updated', 'pr.verification_completed', 'publication.auto_replaced', 'publication.client_approved', 'publication.failed', 'publication.late_approval', 'publication.link_lost', 'publication.link_replacement_requested', 'publication.partner_approved', 'publication.partner_rejected', 'publication.published', 'publication.reminder', 'publication.revision_accepted', 'publication.revision_rejected', 'publication.revision_requested', 'publication.url_requested', 'publication.waiting_partner', 'referral.commission.credited', 'referral.commission.reversed', 'referral.cpa.credited', 'referral.domain.approved', 'referral.domain.rejected', 'secure_direct.approved', 'secure_direct.requested', 'ticket.closed', 'ticket.created', 'ticket.replied', 'variant_batch.failed', 'vip.disabled', 'vip.enabled', 'withdrawal.paid', 'withdrawal.processing', 'withdrawal.rejected', 'withdrawal.requested'])::text[])));
