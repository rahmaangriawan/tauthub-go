-- Migration: 018_content_intake
-- Content Intake layer (Bahan vs Artikel separation).
--
-- content_intake_types : admin-managed questionnaire/brief schemas per content
--   type. `questions` is a JSONB array of { key, label, type, required,
--   weight, helper?, placeholder?, options?[] } where weight drives the
--   configurable Content Readiness Score (sum of weights = 100).
-- content_intakes     : one intake session per brand. `mode` distinguishes
--   BAHAN (source material) from ARTIKEL (existing article) from keduanya
--   from rekomendasi (no material yet). Answers are stored separately from
--   the article so the two concepts never mix; a `job_id` links the intake
--   to the existing content_jobs pipeline once the user proceeds.
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/018_content_intake.sql

BEGIN;

CREATE TABLE IF NOT EXISTS public.content_intake_types (
    id character(36) NOT NULL,
    code character varying(50) NOT NULL,
    name character varying(120) NOT NULL,
    description text,
    questions jsonb DEFAULT '[]'::jsonb NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT content_intake_types_pkey PRIMARY KEY (id),
    CONSTRAINT uq_content_intake_type_code UNIQUE (code),
    CONSTRAINT ck_content_intake_type_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'archived'::character varying])::text[])))
);

CREATE TABLE IF NOT EXISTS public.content_intakes (
    id character(36) NOT NULL,
    brand_id character(36) NOT NULL,
    mode character varying(20) NOT NULL,
    content_type_code character varying(50),
    topic character varying(300),
    answers jsonb DEFAULT '{}'::jsonb NOT NULL,
    article_text text,
    article_filename character varying(255),
    readiness_score integer,
    readiness_checks jsonb DEFAULT '[]'::jsonb NOT NULL,
    recommendation jsonb DEFAULT '{}'::jsonb NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    job_id character(36),
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT content_intakes_pkey PRIMARY KEY (id),
    CONSTRAINT ck_content_intake_mode CHECK (((mode)::text = ANY ((ARRAY['bahan'::character varying, 'artikel'::character varying, 'keduanya'::character varying, 'rekomendasi'::character varying])::text[]))),
    CONSTRAINT ck_content_intake_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'ready'::character varying, 'job_created'::character varying])::text[]))),
    CONSTRAINT fk_content_intakes_brand FOREIGN KEY (brand_id) REFERENCES public.brands(id),
    CONSTRAINT fk_content_intakes_creator FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS ix_content_intakes_brand_created ON public.content_intakes (brand_id, created_at DESC);

-- Seed: default questionnaire schemas (weights sum to 100 per type).
INSERT INTO public.content_intake_types (id, code, name, description, questions, created_by)
SELECT '00000000-0000-4000-8000-000000000101', 'press_release', 'Siaran Pers Umum',
  'Pengumuman/rilis untuk media: siapa, apa, kapan, di mana, mengapa, plus data dan kutipan.',
  '[{"key":"topic","label":"Topik utama pemberitaan","type":"text","required":true,"weight":10,"placeholder":"Contoh: Peluncuran layanan baru PT Nusantara"},
    {"key":"who","label":"Siapa yang terlibat?","type":"text","required":true,"weight":15,"placeholder":"Nama perusahaan, tokoh, atau pihak terkait"},
    {"key":"what","label":"Apa yang terjadi?","type":"textarea","required":true,"weight":20,"placeholder":"Jelaskan inti kejadian/pengumuman"},
    {"key":"when","label":"Kapan?","type":"date","required":true,"weight":10,"helper":"Tanggal pelaksanaan/pengumuman"},
    {"key":"where","label":"Di mana?","type":"text","required":true,"weight":10,"placeholder":"Lokasi fisik, virtual, atau link"},
    {"key":"why","label":"Mengapa hal ini penting?","type":"textarea","required":true,"weight":15,"placeholder":"Dampak, nilai berita, atau alasan publikasi"},
    {"key":"data","label":"Data/fakta pendukung","type":"textarea","required":false,"weight":10,"placeholder":"Angka, statistik, atau fakta yang memperkuat"},
    {"key":"quote","label":"Kutipan narasumber","type":"textarea","required":false,"weight":10,"placeholder":"Kutipan resmi tokoh/representatif"}]'::jsonb,
  NULL
WHERE NOT EXISTS (SELECT 1 FROM public.content_intake_types WHERE code = 'press_release');

INSERT INTO public.content_intake_types (id, code, name, description, questions, created_by)
SELECT '00000000-0000-4000-8000-000000000102', 'event', 'Acara / Event',
  'Liputan atau rilis acara: jenis acara, waktu, lokasi, dan siapa yang terlibat.',
  '[{"key":"topic","label":"Topik acara","type":"text","required":true,"weight":10,"placeholder":"Nama acara yang diliput/diiklankan"},
    {"key":"event_type","label":"Jenis acara","type":"select","required":true,"weight":10,"options":["Rilis media","Konferensi pers","Seminar","Workshop","Launching","Webinar","Lainnya"]},
    {"key":"who","label":"Siapa yang terlibat?","type":"text","required":true,"weight":10,"placeholder":"Penyelenggara, pembicara, atau peserta kunci"},
    {"key":"what","label":"Apa yang terjadi?","type":"textarea","required":true,"weight":15,"placeholder":"Agenda, rangkaian, atau momen utama"},
    {"key":"when","label":"Kapan acaranya?","type":"date","required":true,"weight":15},
    {"key":"where","label":"Di mana?","type":"text","required":true,"weight":10,"placeholder":"Lokasi, venue, atau link pendaftaran"},
    {"key":"why","label":"Mengapa penting?","type":"textarea","required":true,"weight":10,"placeholder":"Manfaat bagi peserta/pembaca"},
    {"key":"data","label":"Data/fakta pendukung","type":"textarea","required":false,"weight":10,"placeholder":"Kapasitas, jumlah peserta, statistik acara"},
    {"key":"quote","label":"Kutipan narasumber","type":"textarea","required":false,"weight":10}]'::jsonb,
  NULL
WHERE NOT EXISTS (SELECT 1 FROM public.content_intake_types WHERE code = 'event');

INSERT INTO public.content_intake_types (id, code, name, description, questions, created_by)
SELECT '00000000-0000-4000-8000-000000000103', 'csr', 'CSR / Program Sosial',
  'Kegiatan tanggung jawab sosial perusahaan: dampak, penerima manfaat, dan dukungan data.',
  '[{"key":"topic","label":"Topik program CSR","type":"text","required":true,"weight":10,"placeholder":"Nama program/ kegiatan sosial"},
    {"key":"company","label":"Perusahaan/penyelenggara","type":"text","required":true,"weight":10},
    {"key":"who","label":"Penerima manfaat / pihak terlibat","type":"text","required":true,"weight":10},
    {"key":"what","label":"Apa yang dilakukan?","type":"textarea","required":true,"weight":15},
    {"key":"when","label":"Kapan?","type":"date","required":true,"weight":10},
    {"key":"where","label":"Di mana?","type":"text","required":true,"weight":10},
    {"key":"why","label":"Mengapa program ini penting?","type":"textarea","required":true,"weight":15},
    {"key":"data","label":"Data/fakta pendukung","type":"textarea","required":false,"weight":10,"placeholder":"Jumlah penerima, nilai program, dampak terukur"},
    {"key":"quote","label":"Kutipan narasumber","type":"textarea","required":false,"weight":10}]'::jsonb,
  NULL
WHERE NOT EXISTS (SELECT 1 FROM public.content_intake_types WHERE code = 'csr');

INSERT INTO public.content_intake_types (id, code, name, description, questions, created_by)
SELECT '00000000-0000-4000-8000-000000000104', 'product_launch', 'Launching Produk',
  'Peluncuran produk/layanan: keunggulan, tanggal rilis, dan ajakan bertindak.',
  '[{"key":"topic","label":"Topik peluncuran","type":"text","required":true,"weight":10,"placeholder":"Nama produk/layanan yang diluncurkan"},
    {"key":"product","label":"Produk/layanan","type":"text","required":true,"weight":10,"placeholder":"Nama resmi dan varian jika ada"},
    {"key":"company","label":"Perusahaan/brand","type":"text","required":true,"weight":10},
    {"key":"what","label":"Apa yang ditawarkan?","type":"textarea","required":true,"weight":15,"placeholder":"Fitur utama, keunggulan, atau pembaruan"},
    {"key":"when","label":"Kapan rilisnya?","type":"date","required":true,"weight":10},
    {"key":"where","label":"Di mana bisa didapat?","type":"text","required":true,"weight":10,"placeholder":"Toko, marketplace, atau link pemesanan"},
    {"key":"why","label":"Mengapa produk ini penting?","type":"textarea","required":true,"weight":15,"placeholder":"Masalah yang dipecahkan / value proposition"},
    {"key":"data","label":"Data/fakta pendukung","type":"textarea","required":false,"weight":10,"placeholder":"Harga, spesifikasi, riset pasar"},
    {"key":"cta","label":"Ajakan bertindak (CTA)","type":"textarea","required":false,"weight":10,"placeholder":"Contoh: Daftar sekarang di situs resmi"}]'::jsonb,
  NULL
WHERE NOT EXISTS (SELECT 1 FROM public.content_intake_types WHERE code = 'product_launch');

INSERT INTO public.content_intake_types (id, code, name, description, questions, created_by)
SELECT '00000000-0000-4000-8000-000000000105', 'personal_branding', 'Personal Branding',
  'Profil atau pencitraan personal: latar tokoh, pencapaian, dan pesan utama.',
  '[{"key":"topic","label":"Topik/fokus personal branding","type":"text","required":true,"weight":10,"placeholder":"Contoh: Perjalanan karier founder startup edukasi"},
    {"key":"person","label":"Tokoh utama","type":"text","required":true,"weight":15,"placeholder":"Nama, jabatan, atau peran"},
    {"key":"what","label":"Pencapaian/peran utama","type":"textarea","required":true,"weight":15},
    {"key":"why","label":"Mengapa profil ini relevan?","type":"textarea","required":true,"weight":20,"placeholder":"Nilai berita, visi, atau dampak"},
    {"key":"data","label":"Data/fakta pendukung","type":"textarea","required":false,"weight":15,"placeholder":"Angka, penghargaan, atau pencapaian terukur"},
    {"key":"quote","label":"Kutipan tokoh","type":"textarea","required":false,"weight":15},
    {"key":"cta","label":"Ajakan bertindak (CTA)","type":"textarea","required":false,"weight":10}]'::jsonb,
  NULL
WHERE NOT EXISTS (SELECT 1 FROM public.content_intake_types WHERE code = 'personal_branding');

COMMIT;
