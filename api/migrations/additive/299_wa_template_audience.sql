-- 299 — Template WA dipisah per audiens (mitra, klien, admin).
--
-- Sebelumnya `wa_templates` hanya punya PRIMARY KEY (event). Akibatnya satu
-- event hanya bisa punya SATU template, padahal event publikasi dikirim ke
-- mitra DAN klien dengan tautan portal yang berbeda. Kolom `audience` membuat
-- tiap audiens punya baris template sendiri untuk event yang sama.
--
-- Aditif & backward compatible:
--   * kolom baru diberi DEFAULT 'partner', jadi seluruh baris lama tetap sah
--     dan langsung terbaca sebagai template mitra.
--   * PRIMARY KEY lama diganti PRIMARY KEY (event, audience).
--   * tidak ada DROP TABLE / DELETE data.

ALTER TABLE public.wa_templates
  ADD COLUMN IF NOT EXISTS audience character varying(16) NOT NULL DEFAULT 'partner';

-- Batasi nilai audiens supaya salah ketik tidak membuat baris yatim yang tidak
-- pernah dibaca pengirim.
ALTER TABLE public.wa_templates DROP CONSTRAINT IF EXISTS ck_wa_templates_audience;
ALTER TABLE public.wa_templates ADD CONSTRAINT ck_wa_templates_audience
  CHECK (audience IN ('partner', 'client', 'admin'));

-- Ganti PK tunggal (event) menjadi gabungan (event, audience).
ALTER TABLE public.wa_templates DROP CONSTRAINT IF EXISTS wa_templates_pkey;
ALTER TABLE public.wa_templates
  ADD CONSTRAINT wa_templates_pkey PRIMARY KEY (event, audience);

CREATE INDEX IF NOT EXISTS ix_wa_templates_audience
  ON public.wa_templates(audience);
