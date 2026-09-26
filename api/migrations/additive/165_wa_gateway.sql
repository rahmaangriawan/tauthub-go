-- 165 — WA Gateway (Fonnte): konfigurasi token, template per event, antrean kirim.
--
-- Aditif & backward compatible. Tidak ada DROP data / TRUNCATE / DELETE.
--
-- Nomor WA penerima sudah tersedia di notification_preferences.wa_number
-- (migrasi 112) dengan flag persetujuan wa_consent — tabel ini melengkapinya
-- dengan sisi pengiriman.

BEGIN;

-- 1) Konfigurasi gateway (singleton). Token disegel pakai CMS_ENCRYPTION_SECRET
--    dan tidak pernah dikembalikan ke browser.
CREATE TABLE IF NOT EXISTS public.platform_wa_config (
  id              text NOT NULL DEFAULT 'default',
  sealed_token    text,
  token_hint      character varying(16),
  sender_name     character varying(120),
  country_code    character varying(6) NOT NULL DEFAULT '62',
  enabled         boolean NOT NULL DEFAULT false,
  updated_by      character(36) REFERENCES public.users(id) ON DELETE SET NULL,
  updated_at      timestamp with time zone NOT NULL DEFAULT now(),
  CONSTRAINT platform_wa_config_pkey PRIMARY KEY (id),
  CONSTRAINT ck_platform_wa_config_id CHECK (id = 'default')
);

-- 2) Template pesan per event. `body` memakai variabel {nama} {judul} {link}
--    {status} {jumlah}; baris dibuat saat admin menyimpan, dan selama belum ada
--    baris, pengiriman memakai default di lib/wa/templates.ts.
CREATE TABLE IF NOT EXISTS public.wa_templates (
  event        character varying(64) PRIMARY KEY,
  body         text NOT NULL,
  enabled      boolean NOT NULL DEFAULT true,
  updated_by   character(36) REFERENCES public.users(id) ON DELETE SET NULL,
  updated_at   timestamp with time zone NOT NULL DEFAULT now()
);

-- 3) Antrean kirim (pola sama dengan email_deliveries): worker mengambil baris
--    queued, mengirim, lalu menandai sent/failed dengan retry terbatas.
CREATE TABLE IF NOT EXISTS public.wa_deliveries (
  id            character(36) PRIMARY KEY,
  user_id       character(36) REFERENCES public.users(id) ON DELETE CASCADE,
  target        character varying(30) NOT NULL,
  event         character varying(64) NOT NULL,
  rendered_body text NOT NULL,
  status        character varying(20) NOT NULL DEFAULT 'queued'
    CHECK (status IN ('queued', 'sent', 'failed')),
  attempts      integer NOT NULL DEFAULT 0,
  last_error    text,
  dedupe_key    character varying(200),
  reference_id  character varying(120),
  sent_at       timestamp with time zone,
  created_at    timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_wa_deliveries_status ON public.wa_deliveries(status, created_at);
CREATE INDEX IF NOT EXISTS ix_wa_deliveries_user ON public.wa_deliveries(user_id, created_at DESC);

-- Dedupe: satu pesan per (event, referensi, tujuan) — cegah kiriman ganda saat
-- event yang sama terpicu berulang.
CREATE UNIQUE INDEX IF NOT EXISTS uq_wa_deliveries_dedupe
  ON public.wa_deliveries(dedupe_key)
  WHERE dedupe_key IS NOT NULL;

COMMIT;
