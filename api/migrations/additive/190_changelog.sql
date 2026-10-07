-- 190_changelog.sql
-- Catatan rilis ("Apa yang Baru"): entri changelog hasil impor admin dari
-- web/static/changelog.md, plus status dibaca per user. Aditif & idempoten
-- (dijalankan oleh cmd/migrate). Sumber kebenaran isi entri tetap file repo;
-- tabel ini hanya salinan agar bisa tampil di dashboard dan memicu notifikasi.
-- Tipe id mengikuti users.id (varchar(36)) supaya FK bisa dipasang.

CREATE TABLE IF NOT EXISTS public.changelog_entries (
    id varchar(36) CONSTRAINT pk_changelog_entries PRIMARY KEY DEFAULT gen_random_uuid()::text,
    version varchar(40) NOT NULL,
    released_on date NOT NULL,
    category varchar(20) NOT NULL DEFAULT 'lainnya',
    title varchar(160) NOT NULL,
    summary varchar(400) NOT NULL DEFAULT '',
    body text NOT NULL,
    source_key varchar(120) NOT NULL,
    published boolean NOT NULL DEFAULT TRUE,
    created_by varchar(36) NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    updated_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_changelog_entries_author FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_changelog_entries_category CHECK (category IN ('fitur', 'perbaikan', 'performa', 'lainnya')),
    CONSTRAINT uq_changelog_entries_source_key UNIQUE (source_key)
);

CREATE INDEX IF NOT EXISTS ix_changelog_entries_released ON public.changelog_entries (released_on DESC, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_changelog_entries_published ON public.changelog_entries (published, released_on DESC);

CREATE TABLE IF NOT EXISTS public.changelog_reads (
    entry_id varchar(36) NOT NULL,
    user_id varchar(36) NOT NULL,
    read_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_changelog_reads PRIMARY KEY (entry_id, user_id),
    CONSTRAINT fk_changelog_reads_entry FOREIGN KEY (entry_id) REFERENCES public.changelog_entries(id) ON DELETE CASCADE,
    CONSTRAINT fk_changelog_reads_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_changelog_reads_user ON public.changelog_reads (user_id);

-- Tipe notifikasi baru: changelog.new (lonceng + web push ikut otomatis karena
-- trigger push memantau tabel notifications). Idempoten: DROP IF EXISTS lalu ADD
-- dengan daftar lengkap (migrasi 178) + tipe baru.
ALTER TABLE public.notifications DROP CONSTRAINT IF EXISTS ck_notifications_type;
ALTER TABLE public.notifications ADD CONSTRAINT ck_notifications_type CHECK (
    type = ANY (ARRAY[
        'announcement.new',
        'changelog.new',
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
