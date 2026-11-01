-- 295 — Template email yang bisa diatur admin + gaya visual email.
--
-- Sebelumnya isi email hardcoded di Go dan tampilan (logo, warna, footer)
-- terkunci di layout.go. Tabel di bawah memindahkan kendali itu ke UI admin:
--   email_branding        : satu baris gaya untuk SELURUH email keluar.
--   email_templates       : subjek/body per audiens (client | partner).
--   email_event_channels  : saklar pengiriman email per event. Default mati;
--                           enabled_at dipakai agar notifikasi lama tidak
--                           ikut terkirim saat kanal baru dinyalakan.
--
-- Isi body disimpan sebagai HTML yang sudah disanitasi server.

CREATE TABLE IF NOT EXISTS public.email_branding (
  id             text PRIMARY KEY DEFAULT 'default',
  brand_name     text NOT NULL DEFAULT 'Tauthub',
  logo_url       text NOT NULL DEFAULT 'https://app.tauthub.com/tauthub-logo.webp',
  primary_color  text NOT NULL DEFAULT '#2b67ec',
  accent_color   text NOT NULL DEFAULT '#2b67ec',
  page_bg        text NOT NULL DEFAULT '#f6f8fc',
  card_bg        text NOT NULL DEFAULT '#ffffff',
  text_color     text NOT NULL DEFAULT '#111827',
  muted_color    text NOT NULL DEFAULT '#667085',
  font_stack     text NOT NULL DEFAULT 'Arial,Helvetica,''Segoe UI'',sans-serif',
  button_radius  integer NOT NULL DEFAULT 10,
  footer_text    text NOT NULL DEFAULT 'Email otomatis dari Tauthub.',
  support_email  text NOT NULL DEFAULT 'support@tauthub.com',
  updated_by     character(36) REFERENCES public.users(id) ON DELETE SET NULL,
  updated_at     timestamp with time zone NOT NULL DEFAULT now()
);

INSERT INTO public.email_branding (id) VALUES ('default')
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS public.email_templates (
  audience    character varying(16) NOT NULL,
  event       character varying(120) NOT NULL,
  subject     character varying(500) NOT NULL,
  body_html   text NOT NULL,
  enabled     boolean NOT NULL DEFAULT true,
  updated_by  character(36) REFERENCES public.users(id) ON DELETE SET NULL,
  updated_at  timestamp with time zone NOT NULL DEFAULT now(),
  PRIMARY KEY (audience, event),
  CONSTRAINT ck_email_templates_audience CHECK (audience IN ('client', 'partner'))
);

CREATE TABLE IF NOT EXISTS public.email_event_channels (
  event       character varying(120) PRIMARY KEY,
  enabled     boolean NOT NULL DEFAULT false,
  enabled_at  timestamp with time zone,
  updated_by  character(36) REFERENCES public.users(id) ON DELETE SET NULL,
  updated_at  timestamp with time zone NOT NULL DEFAULT now()
);
