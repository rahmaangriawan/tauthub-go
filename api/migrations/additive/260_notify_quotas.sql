-- Kuota pengiriman notifikasi per user per kanal.
--
-- Kanal berbayar (email lewat Kirim.id, WhatsApp lewat Fonnte) dibatasi per
-- jam supaya satu event berulang tidak menghabiskan biaya. Sebelumnya batas
-- email hanya counter in-memory di proses, jadi api + worker + scheduler
-- masing-masing punya jatah sendiri dan totalnya bisa tiga kali lipat.
-- Tabel ini membuat batasnya tunggal dan tahan restart.
CREATE TABLE IF NOT EXISTS public.notify_quotas (
    id character(36) NOT NULL DEFAULT gen_random_uuid(),
    user_id character(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    channel character varying(16) NOT NULL,
    window_start timestamp with time zone NOT NULL,
    count integer DEFAULT 0 NOT NULL,
    CONSTRAINT notify_quotas_pkey PRIMARY KEY (id),
    CONSTRAINT notify_quotas_unique UNIQUE (user_id, channel, window_start)
);

-- Pemindaian kuota selalu "bucket terbaru untuk user + kanal ini".
CREATE INDEX IF NOT EXISTS ix_notify_quotas_lookup
    ON public.notify_quotas (user_id, channel, window_start DESC);
