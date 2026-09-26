-- 012_notifications.sql
-- Notifikasi in-app untuk klien (brand members) dan mitra (partner user).
-- Dipicu oleh event publikasi: menunggu persetujuan, disetujui/ditolak,
-- dan terbit. Additive — tidak menyentuh tabel existing.
-- =====================================================================

CREATE TABLE IF NOT EXISTS public.notifications (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    type character varying(40) NOT NULL,
    title character varying(200) NOT NULL,
    body character varying(1000),
    link character varying(500),
    read_at timestamp(6) with time zone,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT notifications_pkey PRIMARY KEY (id),
    CONSTRAINT fk_notifications_user FOREIGN KEY (user_id) REFERENCES public.users (id),
    CONSTRAINT ck_notifications_type CHECK (((type)::text = ANY ((ARRAY[
        'publication.waiting_partner'::character varying,
        'publication.partner_approved'::character varying,
        'publication.partner_rejected'::character varying,
        'publication.published'::character varying
    ])::text[])))
);

CREATE INDEX IF NOT EXISTS ix_notifications_user_read
    ON public.notifications (user_id, read_at, created_at DESC);
