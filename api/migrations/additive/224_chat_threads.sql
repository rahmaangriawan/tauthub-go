-- 224_chat_threads.sql
-- Obrolan klien ↔ mitra per publikasi. Satu forum = satu publikasi
-- (brand + media + campaign). Sesi berlaku 3×24 jam sejak PESAN PERTAMA
-- klien; setelah lewat, riwayat tetap dapat dibaca dua pihak (komposer mati).
--
-- Tipe notifikasi baru 'chat.message' untuk memberi tahu mitra/klien saat ada
-- pesan masuk (idempoten: DROP IF EXISTS lalu ADD daftar lengkap + tipe baru).

CREATE TABLE IF NOT EXISTS public.chat_threads (
    id              character(36) PRIMARY KEY,
    brand_id        character(36) NOT NULL,
    campaign_id     character(36),
    media_id        character(36) NOT NULL,
    partner_id      character(36),
    publication_id  character(36),
    document_id     character(36),
    started_at      timestamptz,
    expires_at      timestamptz,
    last_message_at timestamptz,
    last_message_by varchar(20),
    created_by      character(36),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_chat_threads_author CHECK (last_message_by IS NULL OR last_message_by IN ('client','partner'))
);

-- Satu forum per publikasi per brand; find-or-create aman diklik berulang.
CREATE UNIQUE INDEX IF NOT EXISTS chat_threads_brand_publication_uq
    ON public.chat_threads (brand_id, publication_id)
    WHERE publication_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS chat_threads_brand_idx ON public.chat_threads (brand_id, last_message_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS chat_threads_partner_idx ON public.chat_threads (partner_id, last_message_at DESC NULLS LAST);

CREATE TABLE IF NOT EXISTS public.chat_messages (
    id             character(36) PRIMARY KEY,
    thread_id      character(36) NOT NULL REFERENCES public.chat_threads(id) ON DELETE CASCADE,
    author_role    varchar(20) NOT NULL,
    author_user_id character(36),
    author_name    varchar(160),
    body           text NOT NULL,
    read_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ck_chat_messages_author CHECK (author_role IN ('client','partner'))
);

CREATE INDEX IF NOT EXISTS chat_messages_thread_idx ON public.chat_messages (thread_id, created_at);

ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type CHECK (
    type = ANY (ARRAY[
        'announcement.new',
        'changelog.new',
        'chat.message',
        'media.approved',
        'media.crawl_blocked',
        'media.dewan_pers.approved',
        'media.dewan_pers.rejected',
        'media.dewan_pers.submitted',
        'media.domain_down',
        'media.negotiated',
        'media.rejected',
        'media.resubmitted',
        'media.submitted',
        'pr.score_updated',
        'pr.verification_completed',
        'publication.auto_replaced',
        'publication.client_approved',
        'publication.failed',
        'publication.late_approval',
        'publication.link_lost',
        'publication.link_replacement_requested',
        'publication.partner_approved',
        'publication.partner_rejected',
        'publication.published',
        'publication.reminder',
        'publication.revision_accepted',
        'publication.revision_rejected',
        'publication.revision_requested',
        'publication.url_requested',
        'publication.waiting_partner',
        'referral.commission.credited',
        'referral.commission.reversed',
        'referral.cpa.credited',
        'referral.domain.approved',
        'referral.domain.rejected',
        'variant_batch.failed',
        'vip.disabled',
        'vip.enabled',
        'withdrawal.paid',
        'withdrawal.processing',
        'withdrawal.rejected',
        'withdrawal.requested'
    ]::varchar[])
);
