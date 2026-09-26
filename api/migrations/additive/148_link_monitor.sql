-- 148 — Pelacak 404: pantau artikel tayang yang hilang + kesehatan domain mitra.
-- Additive: tabel & kolom baru, tanpa DROP/ubah data. Dua tipe notifikasi baru
-- ditambahkan dengan pola DROP+ADD constraint (sama seperti migrasi 088).
--
-- Konteks: ada garansi tayang 12 bulan, jadi artikel yang sudah published perlu
-- dicek ulang secara berkala — bukan hanya sekali saat terbit (perilaku pr_verify
-- yang sekarang). Domain mitra juga perlu dipantau "hidup atau tidak", terpisah
-- dari status verifikasi token kepemilikan.

-- 1) Hasil pengecekan liveness per artikel tayang.
--    Tabel terpisah dari pr_publication_verifications: tabel PR itu berisi skor
--    compliance dan ditimpa tiap analisis PR, sedangkan di sini kita menyimpan
--    status HTTP mentah + hitungan gagal berturut-turut untuk hindari alarm palsu.
CREATE TABLE IF NOT EXISTS public.publication_link_checks (
    publication_id      character(36) NOT NULL,
    url                 character varying(500),
    final_url           character varying(500),
    http_status         integer,
    -- live         = 2xx, artikel masih tayang
    -- gone         = 404/410, artikel benar-benar hilang
    -- unreachable  = 4xx lain (403/robots), 5xx, atau timeout — TIDAK dianggap hilang
    -- no_url       = belum ada published_url untuk dicek
    state               character varying(20) NOT NULL DEFAULT 'no_url',
    consecutive_failures integer NOT NULL DEFAULT 0,
    first_detected_gone_at timestamp(6) with time zone,
    last_checked_at     timestamp(6) with time zone,
    error_message       character varying(500),
    created_at          timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at          timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_publication_link_checks_state
      CHECK (state IN ('live', 'gone', 'unreachable', 'no_url')),
    CONSTRAINT ck_publication_link_checks_failures
      CHECK (consecutive_failures >= 0)
);

ALTER TABLE ONLY public.publication_link_checks
    DROP CONSTRAINT IF EXISTS publication_link_checks_pkey;
ALTER TABLE ONLY public.publication_link_checks
    ADD CONSTRAINT publication_link_checks_pkey PRIMARY KEY (publication_id);

ALTER TABLE ONLY public.publication_link_checks
    DROP CONSTRAINT IF EXISTS fk_publication_link_checks_publication;
ALTER TABLE ONLY public.publication_link_checks
    ADD CONSTRAINT fk_publication_link_checks_publication
    FOREIGN KEY (publication_id) REFERENCES public.publications(id) ON DELETE CASCADE;

-- Sweep scheduler memfilter (state, last_checked_at); halaman admin memfilter state.
CREATE INDEX IF NOT EXISTS ix_publication_link_checks_state_checked
    ON public.publication_link_checks (state, last_checked_at);

-- 2) Kesehatan domain mitra — dipisah dari domain_check_fail_count yang sekarang
--    mencampur "situs tak terjangkau" dengan "token verifikasi tidak ditemukan".
ALTER TABLE public.media
    ADD COLUMN IF NOT EXISTS domain_health character varying(20) NOT NULL DEFAULT 'unknown';

ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_domain_health;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_domain_health
    CHECK (domain_health IN ('unknown', 'live', 'unreachable', 'token_missing'));

-- 3) Tipe notifikasi baru untuk pelacak 404.
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
