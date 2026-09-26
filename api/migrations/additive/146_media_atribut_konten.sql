-- 146 — Atribut konten media mitra: bahasa artikel, label sponsor,
-- topik terlarang, dan URL contoh artikel.
-- Additive: 4 kolom baru di media. Tidak DROP TABLE, tidak destructive.
-- bahasa_artikel & topik_terlarang disimpan sebagai JSON array string
-- (seragam dengan kolom JSON lain, mis. health_breakdown).

ALTER TABLE public.media ADD COLUMN IF NOT EXISTS bahasa_artikel text;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS label_sponsor character varying(20) NOT NULL DEFAULT 'tidak_ada';
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS topik_terlarang text;
ALTER TABLE public.media ADD COLUMN IF NOT EXISTS contoh_artikel_url character varying(500);

ALTER TABLE ONLY public.media DROP CONSTRAINT IF EXISTS ck_media_label_sponsor;
ALTER TABLE ONLY public.media ADD CONSTRAINT ck_media_label_sponsor
    CHECK (label_sponsor IN ('tidak_ada', 'ada_sponsor'));
