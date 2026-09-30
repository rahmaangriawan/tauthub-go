-- Pengumuman broadcast dari admin ke pengguna klien/mitra, plus status dibaca
-- per user. Aditif & idempoten (dijalankan oleh cmd/migrate).
-- Tipe id mengikuti users.id (char(36)) supaya FK bisa dipasang.

CREATE TABLE IF NOT EXISTS public.announcements (
    id varchar(36) CONSTRAINT pk_announcements PRIMARY KEY DEFAULT gen_random_uuid()::text,
    title varchar(120) NOT NULL,
    body text NOT NULL,
    audience varchar(20) NOT NULL DEFAULT 'all',
    created_by varchar(36) NULL,
    created_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_announcements_author FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL,
    CONSTRAINT ck_announcements_audience CHECK (audience IN ('all','client','partner'))
);

CREATE INDEX IF NOT EXISTS ix_announcements_created_at ON public.announcements (created_at DESC);

CREATE TABLE IF NOT EXISTS public.announcement_reads (
    announcement_id varchar(36) NOT NULL,
    user_id varchar(36) NOT NULL,
    read_at timestamptz NOT NULL DEFAULT NOW(),
    CONSTRAINT pk_announcement_reads PRIMARY KEY (announcement_id, user_id),
    CONSTRAINT fk_announcement_reads_announcement FOREIGN KEY (announcement_id) REFERENCES public.announcements(id) ON DELETE CASCADE,
    CONSTRAINT fk_announcement_reads_user FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS ix_announcement_reads_user ON public.announcement_reads (user_id);
