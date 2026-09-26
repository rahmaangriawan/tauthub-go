-- 050: Knowledge Base editorial global (admin-managed).
-- KB = kumpulan aturan editorial/gaya penulisan yang diinjeksikan ke prompt AI
-- saat content generation. Satu versi ACTIVE per KB (partial unique index).
BEGIN;

-- ─── knowledge_bases ─────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.knowledge_bases (
    id character(36) NOT NULL,
    code character varying(60) NOT NULL,
    name character varying(200) NOT NULL,
    description text,
    scope character varying(20) DEFAULT 'global'::character varying NOT NULL,
    scope_ref character varying(36),
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_by character(36),
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT knowledge_bases_pkey PRIMARY KEY (id),
    CONSTRAINT uq_knowledge_bases_code UNIQUE (code),
    CONSTRAINT ck_knowledge_bases_scope CHECK (((scope)::text = ANY ((ARRAY['global'::character varying, 'media'::character varying, 'brand'::character varying])::text[]))),
    CONSTRAINT ck_knowledge_bases_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'inactive'::character varying])::text[])))
);

-- ─── knowledge_base_versions ─────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.knowledge_base_versions (
    id character(36) NOT NULL,
    knowledge_base_id character(36) NOT NULL,
    version integer NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying NOT NULL,
    change_note character varying(255),
    created_by character(36) NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT knowledge_base_versions_pkey PRIMARY KEY (id),
    CONSTRAINT uq_knowledge_base_versions_version UNIQUE (knowledge_base_id, version),
    CONSTRAINT ck_knowledge_base_versions_status CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'active'::character varying, 'archived'::character varying])::text[])))
);

-- Satu versi ACTIVE per knowledge base.
CREATE UNIQUE INDEX IF NOT EXISTS uq_knowledge_base_versions_active
    ON public.knowledge_base_versions (knowledge_base_id) WHERE (status = 'active'::character varying);

-- ─── knowledge_base_rules ────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS public.knowledge_base_rules (
    id character(36) NOT NULL,
    knowledge_base_id character(36) NOT NULL,
    version_id character(36) NOT NULL,
    category character varying(80) NOT NULL,
    title character varying(255) NOT NULL,
    instruction text NOT NULL,
    examples jsonb DEFAULT '[]'::jsonb NOT NULL,
    forbidden_examples jsonb DEFAULT '[]'::jsonb NOT NULL,
    tags jsonb DEFAULT '[]'::jsonb NOT NULL,
    audience jsonb DEFAULT '[]'::jsonb NOT NULL,
    content_types jsonb DEFAULT '[]'::jsonb NOT NULL,
    priority smallint DEFAULT 5 NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT knowledge_base_rules_pkey PRIMARY KEY (id),
    CONSTRAINT ck_knowledge_base_rules_priority CHECK ((priority BETWEEN 1 AND 8)),
    CONSTRAINT ck_knowledge_base_rules_status CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'inactive'::character varying])::text[])))
);
CREATE INDEX IF NOT EXISTS idx_kb_rules_version ON public.knowledge_base_rules (version_id);
CREATE INDEX IF NOT EXISTS idx_kb_rules_base_status ON public.knowledge_base_rules (knowledge_base_id, status);

-- ─── knowledge_base_rule_chunks (retrieval semantik, embedding local 128-d) ──
CREATE TABLE IF NOT EXISTS public.knowledge_base_rule_chunks (
    id character(36) NOT NULL,
    rule_id character(36) NOT NULL,
    chunk_index integer NOT NULL,
    content text NOT NULL,
    embedding jsonb NOT NULL,
    CONSTRAINT knowledge_base_rule_chunks_pkey PRIMARY KEY (id),
    CONSTRAINT uq_kb_rule_chunks_rule_index UNIQUE (rule_id, chunk_index)
);
CREATE INDEX IF NOT EXISTS idx_kb_rule_chunks_rule ON public.knowledge_base_rule_chunks (rule_id);

-- ─── knowledge_base_sources (referensi normatif KB) ─────────────────────────
CREATE TABLE IF NOT EXISTS public.knowledge_base_sources (
    id character(36) NOT NULL,
    knowledge_base_id character(36) NOT NULL,
    source_name character varying(255) NOT NULL,
    source_url text,
    source_type character varying(30) DEFAULT 'regulation'::character varying NOT NULL,
    notes text,
    accessed_at date,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT knowledge_base_sources_pkey PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS idx_kb_sources_base ON public.knowledge_base_sources (knowledge_base_id);

-- ─── FKs ─────────────────────────────────────────────────────────────────────
ALTER TABLE ONLY public.knowledge_base_versions
    ADD CONSTRAINT fk_kb_versions_base FOREIGN KEY (knowledge_base_id) REFERENCES public.knowledge_bases(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.knowledge_base_rules
    ADD CONSTRAINT fk_kb_rules_base FOREIGN KEY (knowledge_base_id) REFERENCES public.knowledge_bases(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.knowledge_base_rules
    ADD CONSTRAINT fk_kb_rules_version FOREIGN KEY (version_id) REFERENCES public.knowledge_base_versions(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.knowledge_base_rule_chunks
    ADD CONSTRAINT fk_kb_rule_chunks_rule FOREIGN KEY (rule_id) REFERENCES public.knowledge_base_rules(id) ON DELETE CASCADE;
ALTER TABLE ONLY public.knowledge_base_sources
    ADD CONSTRAINT fk_kb_sources_base FOREIGN KEY (knowledge_base_id) REFERENCES public.knowledge_bases(id) ON DELETE CASCADE;
-- =====================================================================
-- Seed: Knowledge Base "Standar Penulisan Jurnalistik Indonesia" (v1)
-- Idempotent (ON CONFLICT DO NOTHING). Chunk embeddings di-backfill aplikasi.
-- =====================================================================
INSERT INTO public.knowledge_bases (id, code, name, description, scope, scope_ref, status, created_by)
VALUES ('00000000-0000-4000-8000-00000000a001', 'journalistic-writing-id', 'Standar Penulisan Jurnalistik Indonesia',
        'Aturan editorial dan gaya penulisan yang digunakan AI Tulisio untuk menghasilkan konten berita/press release yang layak dibaca dalam konteks media Indonesia. Baseline editorial global Tulisio.',
        'global', NULL, 'active', '00000000-0000-4000-8000-000000000001')
ON CONFLICT (code) DO NOTHING;

INSERT INTO public.knowledge_base_versions (id, knowledge_base_id, version, status, change_note, created_by)
VALUES ('00000000-0000-4000-8000-00000000a002', '00000000-0000-4000-8000-00000000a001', 1, 'active', 'Baseline jurnalistik Indonesia', '00000000-0000-4000-8000-000000000001')
ON CONFLICT (knowledge_base_id, version) DO NOTHING;

INSERT INTO public.knowledge_base_sources (id, knowledge_base_id, source_name, source_url, source_type, notes, accessed_at) VALUES
  ('00000000-0000-4000-8000-00000000a101', '00000000-0000-4000-8000-00000000a001', 'Peraturan Dewan Pers Nomor 6/Peraturan-DP/V/2008', 'https://dewanpers.or.id', 'regulation', 'Pengesahan Kode Etik Jurnalistik', '2026-01-01'),
  ('00000000-0000-4000-8000-00000000a102', '00000000-0000-4000-8000-00000000a001', 'Peraturan Dewan Pers Nomor 1/Peraturan-DP/III/2012', 'https://dewanpers.or.id', 'regulation', 'Pedoman Pemberitaan Media Siber', '2026-01-01'),
  ('00000000-0000-4000-8000-00000000a103', '00000000-0000-4000-8000-00000000a001', 'Peraturan Dewan Pers Nomor 03/Peraturan-DP/IV/2024', 'https://dewanpers.or.id', 'regulation', 'Pedoman Perilaku dan Standar Pers Profesional', '2026-01-01'),
  ('00000000-0000-4000-8000-00000000a104', '00000000-0000-4000-8000-00000000a001', 'Peraturan Dewan Pers Nomor 1/PERATURAN-DP/I/2025', 'https://dewanpers.or.id', 'regulation', 'Pedoman Penggunaan Kecerdasan Buatan dalam Karya Jurnalistik', '2026-01-01'),
  ('00000000-0000-4000-8000-00000000a105', '00000000-0000-4000-8000-00000000a001', 'EYD Edisi V', 'https://badanbahasa.kemdikbud.go.id', 'dictionary', 'Pedoman Umum Ejaan Bahasa Indonesia Edisi V (Badan Pengembangan dan Pembinaan Bahasa)', '2026-01-01'),
  ('00000000-0000-4000-8000-00000000a106', '00000000-0000-4000-8000-00000000a001', 'KBBI Daring', 'https://kbbi.kemdikbud.go.id', 'dictionary', 'Referensi kosakata bahasa Indonesia baku', '2026-01-01')
ON CONFLICT DO NOTHING;

INSERT INTO public.knowledge_base_rules
  (id, knowledge_base_id, version_id, category, title, instruction, examples, forbidden_examples, tags, audience, content_types, priority, status)
VALUES
  ('00000000-0000-4000-8000-00000000a201', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Editorial Principles', 'Tulisan penulis berita Indonesia',
   'Konten harus faktual, informatif, ringkas, padat, jelas, lugas, dan natural — seperti naskah yang ditulis penulis/editor media Indonesia. Bukan iklan, bukan artikel SEO generik, bukan opini AI, bukan copywriting korporat yang dipanjangkan.',
   '["Perusahaan meluncurkan platform baru untuk membantu UMKM mengelola transaksi."]', '["Di era digital yang semakin berkembang, kebutuhan akan solusi inovatif menjadi semakin penting."]', '["baseline","editorial"]', '[]', '[]', 2, 'active'),
  ('00000000-0000-4000-8000-00000000a202', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Journalistic Structure', 'Inverted pyramid',
   'Susun informasi dengan prinsip piramida terbalik: informasi paling penting di awal, lalu fakta utama, detail pendukung, kutipan, konteks, dan informasi tambahan. Paragraf awal harus membuat pembaca memahami inti tanpa membaca seluruh artikel.',
   '["JAKARTA, KOMPAS.com - PT XYZ meluncurkan platform digital baru untuk 500 pelaku UMKM di Indonesia pada Senin (10/8/2026)."]', '["Perusahaan yang telah berdiri sejak 1990 dan melalui berbagai perjalanan panjang ini, kini akhirnya meluncurkan sebuah platform yang diharapkan dapat membantu masyarakat."]', '["struktur","pyramid"]', '[]', '[]', 4, 'active'),
  ('00000000-0000-4000-8000-00000000a203', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Lead', 'Lead langsung ke inti peristiwa',
   'Lead/teras berita harus langsung menuju inti peristiwa: informatif, tidak bertele-tele, dapat dipahami tanpa paragraf berikutnya. Prioritaskan WHAT + WHO + WHEN/WHERE; tambahkan WHY/HOW jika tersedia dan relevan. Hindari kalimat pembuka generik atau klise.',
   '["PT ABC meluncurkan layanan pembayaran digital baru pada Senin di Jakarta untuk mempercepat transaksi UMKM."]', '["PT ABC dengan bangga mengumumkan sebuah terobosan luar biasa yang akan mengubah masa depan Indonesia."]', '["lead","teras"]', '[]', '[]', 4, 'active'),
  ('00000000-0000-4000-8000-00000000a204', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Headline', 'Judul informatif dan spesifik',
   'Judul harus informatif, spesifik, ringkas, dan menggambarkan isi. Jangan clickbait, jangan berlebihan, jangan menipu ekspektasi pembaca, jangan ALL CAPS (kecuali nama resmi), jangan terlalu banyak tanda seru. Judul tidak boleh memuat informasi yang tidak ada di isi.',
   '["XYZ Meluncurkan Platform Baru untuk Mendukung UMKM"]', '["TEROBOSAN LUAR BIASA! XYZ UBAH MASA DEPAN INDONESIA"]', '["headline","judul"]', '[]', '[]', 4, 'active'),
  ('00000000-0000-4000-8000-00000000a205', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', '5W+1H', 'Manfaatkan seluruh unsur yang tersedia',
   'Analisis bahan dengan 5W+1H: What, Who, When, Where, Why, How. Manfaatkan semua unsur yang tersedia dari brief/bahan. Jika ada unsur yang tidak tersedia, jangan mengarang, menebak, atau membuat fakta sintetis — gunakan formulasi jujur atau tandai sebagai informasi yang belum tersedia.',
   '["Berdasarkan keterangan perusahaan, jumlah peserta mencapai 500 orang, namun detail lokasi belum dirilis."]', '["Acara dihadiri ribuan peserta dari berbagai daerah di Indonesia."]', '["5w1h","fakta"]', '[]', '[]', 2, 'active'),
  ('00000000-0000-4000-8000-00000000a206', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Language Style', 'Bahasa jurnalistik yang efisien',
   'Gunakan bahasa Indonesia baku dengan gaya jurnalistik: singkat, padat, jelas, lugas, sederhana, komunikatif, dan menarik melalui informasi bukan hiperbola. Prioritaskan "jelas dan cepat dipahami" daripada "terdengar canggih".',
   '["Pemerintah menaikkan subsidi energi untuk 10 juta keluarga berpenghasilan rendah."]', '["Pemerintah mengambil langkah strategis yang monumental dalam rangka memberikan kontribusi signifikan bagi kesejahteraan masyarakat luas."]', '["bahasa","gaya"]', '[]', '[]', 6, 'active'),
  ('00000000-0000-4000-8000-00000000a207', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Paragraph', 'Paragraf pendek, satu ide',
   'Gunakan paragraf pendek (satu ide utama per paragraf untuk konten digital). Hindari paragraf sangat panjang dan jangan memaksakan heading. Untuk press release/news pendek, gunakan struktur artikel tanpa heading secara default; gunakan heading hanya jika konten panjang atau diminta.',
   '["Perusahaan menggelar pelatihan digital bagi 500 UMKM. Pelatihan berlangsung selama tiga hari di Jakarta."]', '["Dalam kegiatan yang diselenggarakan oleh perusahaan tersebut yang bertujuan untuk memberikan berbagai macam manfaat kepada para peserta yang hadir, perusahaan menyampaikan bahwa program ini merupakan wujud nyata dari komitmen perusahaan dalam mendorong transformasi digital di Indonesia yang lebih maju dan berkelanjutan."]', '["paragraf","struktur"]', '[]', '[]', 5, 'active'),
  ('00000000-0000-4000-8000-00000000a208', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Fact & Opinion', 'Pisahkan fakta dan opini',
   'AI WAJIB membedakan fakta dan opini. Opini (terutama dari narasumber/perusahaan) harus diatribusikan, misalnya "Menurut CEO XYZ...". Jangan mengubah opini menjadi fakta dan jangan menghapus atribusi dari klaim subjektif.',
   '["Menurut CEO XYZ, produk tersebut merupakan solusi terbaik di pasar saat ini."]', '["Produk XYZ adalah yang terbaik di Indonesia."]', '["fakta","opini","atribusi"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a209', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Attribution', 'Pertahankan atribusi sumber',
   'Jika informasi berasal dari narasumber, perusahaan, lembaga, laporan, penelitian, dokumen, atau pernyataan resmi, gunakan atribusi yang sesuai ("Menurut data...", "Berdasarkan laporan...", "Dalam keterangannya...", "CEO XYZ mengatakan..."). Jangan menghapus sumber demi membuat tulisan lebih singkat.',
   '["Berdasarkan laporan Bank Indonesia, inflasi inti tercatat 2,8 persen pada Juli 2026."]', '["Inflasi inti tercatat 2,8 persen."]', '["atribusi","sumber"]', '[]', '[]', 3, 'active'),
  ('00000000-0000-4000-8000-00000000a210', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Quote', 'Jangan mengarang kutipan',
   'DILARANG membuat direct quote yang tidak diberikan user. Jika hanya tersedia informasi/parafrasa, gunakan bentuk atribusi parafrasa ("CEO XYZ mengatakan langkah tersebut merupakan bagian dari strategi perusahaan"), bukan tanda kutip langsung. Jangan mengubah makna kutipan yang memang diberikan.',
   '["CEO XYZ mengatakan langkah tersebut merupakan bagian dari strategi ekspansi perusahaan."]', '["“Ini merupakan langkah strategis terbesar kami,” kata CEO XYZ."]', '["kutipan","quote"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a211', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Data & Numbers', 'Pertahankan angka sesuai bahan',
   'Pertahankan angka, tanggal, dan satuan sesuai bahan. Jangan mengubah, membulatkan tanpa alasan, membuat persentase, atau menghitung statistik baru. Gunakan format mata uang natural Indonesia: Rp5 juta, Rp5,2 juta, Rp1,5 miliar.',
   '["Perusahaan menginvestasikan Rp5,2 miliar untuk program tersebut."]', '["Perusahaan menginvestasikan sekitar Rp5 miliar untuk program tersebut."]', '["angka","data"]', '[]', '[]', 2, 'active'),
  ('00000000-0000-4000-8000-00000000a212', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Press Release', 'Press release bergaya berita',
   'Tulis press release dengan gaya news/jurnalistik: informasi dari pihak/klien, tujuan menyampaikan informasi ke media, informatif, bukan iklan terang-terangan. Boleh menyebut brand/produk/layanan, tetapi klaim subjektif tetap diatribusikan. Struktur default: Headline, Lead, Detail utama, Kutipan narasumber, Detail/konteks, Informasi penutup. Tanpa heading H2/H3 secara default. Target maksimal 600 kata.',
   '["PT XYZ meluncurkan aplikasi baru untuk petani pada Rabu (12/8/2026) di Bandung. Aplikasi tersebut menyediakan data harga komoditas secara real-time."]', '["PT XYZ, perusahaan terdepan yang telah dipercaya jutaan pelanggan, dengan bangga mempersembahkan inovasi revolusioner yang akan mengubah wajah pertanian Indonesia selamanya."]', '["press-release","siaran-pers"]', '[]', '["press_release","event","csr","product_launch"]', 4, 'active'),
  ('00000000-0000-4000-8000-00000000a213', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'News Article', 'Straight news yang faktual',
   'Untuk artikel berita: sajikan straight news yang faktual dengan piramida terbalik, tanpa opini AI, tanpa menambahkan sudut pandang pribadi. Informasi utama di paragraf pertama.',
   '["Dinas Pendidikan DKI Jakarta mengumumkan pembukaan pendaftaran sekolah negeri tahap kedua pada 15 Agustus."]', '["Pendaftaran sekolah negeri yang dinanti-nantikan akhirnya dibuka. Ini kabar baik bagi orang tua yang selama ini menunggu dengan harapan besar."]', '["berita","news"]', '[]', '["news","article"]', 4, 'active'),
  ('00000000-0000-4000-8000-00000000a214', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'SEO', 'SEO mengikuti kualitas editorial',
   'SEO hanya digunakan setelah struktur editorial benar. Jika keyword diberikan, masukkan secara natural; jangan keyword stuffing, jangan memasukkan keyword jika merusak kalimat, jangan membuat judul aneh hanya untuk keyword. Prioritas: READABILITY > ACCURACY > RELEVANCE > SEO.',
   '["Artikel tentang pelatihan UMKM menyebut kata kunci “pelatihan UMKM digital” dua kali secara natural."]', '["Pelatihan UMKM digital adalah solusi pelatihan UMKM digital terbaik. Dengan pelatihan UMKM digital, pelaku UMKM bisa berkembang. Pelatihan UMKM digital..."]', '["seo"]', '[]', '[]', 7, 'active'),
  ('00000000-0000-4000-8000-00000000a215', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Anti Clickbait', 'Tanpa judul berlebihan',
   'Judul dan konten tidak boleh melebih-lebihkan atau menjanjikan sesuatu yang tidak ada di isi. Jangan gunakan hiperbola, kata berlebihan (luar biasa, spektakuler, revolusioner), atau tanda seru berlebihan.',
   '["Bank XYZ Meluncurkan Layanan Pinjaman untuk Nelayan"]', '["RAHASIA BESAR TERBONGKAR! BANK XYZ SELAMATKAN NELAYAN INDONESIA"]', '["clickbait","judul"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a216', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Anti Hallucination', 'Dilarang mengarang fakta',
   'AI TIDAK BOLEH mengarang nama orang, jabatan, tanggal, lokasi, angka, statistik, jumlah peserta, omzet, revenue, market share, penghargaan, pelanggan, partner, sumber, kutipan, hasil penelitian, data survei, atau klaim performa (nomor satu, terbesar, pertama, terbaik, paling inovatif) jika tidak diberikan. Jika informasi penting tidak tersedia, gunakan formulasi netral atau minta informasi tambahan.',
   '["Jumlah investasi akan diumumkan perusahaan pada konferensi pers, Jumat (14/8/2026)."]', '["Perusahaan menginvestasikan Rp100 miliar untuk program ini."]', '["anti-hallucination","fakta"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a217', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Ethics', 'Etika dan bahasa yang hati-hati',
   'Jangan menghasilkan konten fitnah, bohong, diskriminatif, menghakimi, menyerang kelompok berdasarkan SARA, atau merendahkan kelompok tertentu. Untuk kasus sensitif, gunakan bahasa hati-hati dan faktual. Jangan menyimpulkan seseorang bersalah hanya karena dituduh, dilaporkan, diperiksa, ditangkap, atau menjadi tersangka — gunakan status hukum yang tepat sesuai sumber.',
   '["Tersangka berinisial A kini ditahan di Polres Jakarta Selatan untuk pemeriksaan lebih lanjut."]', '["A terbukti bersalah melakukan penipuan terhadap puluhan korbannya."]', '["etika"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a218', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Privacy', 'Lindungi data pribadi',
   'Jangan memasukkan data pribadi yang tidak diperlukan untuk memahami berita: nomor telepon, alamat pribadi, email pribadi, NIK, nomor identitas, atau data sensitif lainnya — kecuali relevan, diberikan secara sah, dan diperlukan oleh konteks.',
   '["Seorang warga melaporkan kehilangan sepeda motor ke Polsek Kebayoran Baru."]', '["Korban bernama Budi (45), warga Jalan Mawar No. 12, RT 05/RW 03, dengan nomor HP 0812xxxxxxx, melaporkan kehilangan sepeda motor."]', '["privasi","privacy"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a219', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'AI Governance', 'Jangan mengklaim yang tidak dilakukan',
   'AI dilarang mengklaim telah mewawancarai seseorang, membaca dokumen tertentu, menghubungi narasumber, melakukan konfirmasi, fact-check, mengecek website/data resmi, atau verifikasi lapangan jika tidak dilakukan sistem. Konten AI bukan hasil verifikasi jurnalistik; jangan menulis seolah-olah data telah diverifikasi.',
   '["Menurut data yang diberikan perusahaan, jumlah pengguna mencapai 2 juta."]', '["Setelah melakukan verifikasi langsung, jumlah pengguna mencapai 2 juta."]', '["ai-governance","verifikasi"]', '[]', '[]', 1, 'active'),
  ('00000000-0000-4000-8000-00000000a220', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Quality Control', 'Editorial check sebelum output',
   'Lakukan pemeriksaan editorial internal sebelum output final: pastikan semua fakta berasal dari input/bahan, tidak ada fakta yang dikarang, lead menyampaikan inti berita, seluruh unsur 5W+1H yang tersedia dimanfaatkan, fakta dan opini terpisah, klaim subjektif ber-atribusi, tidak ada promosi berlebihan, tidak ada clickbait, tidak ada jargon yang tidak perlu, artikel tidak terlalu panjang, bahasa mudah dipahami, ejaan dan tanda baca benar, tidak ada kutipan yang dibuat-buat, dan hasil terasa seperti berita/press release. Perbaiki masalah apa pun sebelum menyerahkan output.',
   '["(Setelah menulis draft, periksa ulang setiap klaim terhadap bahan baku sebelum mengirim.)"]', '["(Mengirim draft langsung tanpa pemeriksaan ulang.)"]', '["quality","kontrol"]', '[]', '[]', 5, 'active'),
  ('00000000-0000-4000-8000-00000000a221', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Corporate Language', 'Hindari buzzword korporat',
   'Kurangi corporate buzzwords: "hadir sebagai solusi inovatif", "langkah strategis", "komitmen nyata", "transformasi digital", "ekosistem berkelanjutan", "memberikan dampak positif", "mendorong pertumbuhan", "memperkuat sinergi". Kata-kata ini boleh dipakai jika relevan dan berasal dari sumber, tetapi jangan otomatis hanya karena artikel membahas perusahaan. Utamakan fakta konkret.',
   '["Program tersebut menyediakan pelatihan digital bagi 500 pelaku UMKM."]', '["Langkah strategis ini menjadi bentuk komitmen nyata perusahaan dalam mendorong transformasi digital."]', '["jargon","buzzword"]', '[]', '[]', 6, 'active'),
  ('00000000-0000-4000-8000-00000000a222', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Names & Titles', 'Nama dan jabatan yang benar',
   'Pertahankan nama resmi dan jangan membuat variasi sembarangan. Penyebutan pertama: Nama Lengkap, Jabatan (mis. "Rama Angriawan, Direktur PT XYZ, mengatakan..."). Selanjutnya boleh "Rama" atau "dia" jika konteks jelas. Jangan salah mengganti gender/pronomina.',
   '["Rama Angriawan, Direktur PT XYZ, mengatakan program ini akan berjalan selama satu tahun."]', '["Bapak Rama, direktur perusahaan tersebut, bilang programnya jalan satu tahun."]', '["nama","jabatan"]', '[]', '[]', 3, 'active'),
  ('00000000-0000-4000-8000-00000000a223', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Foreign Terms', 'Utamakan padanan Indonesia',
   'Jika ada istilah asing dengan padanan Indonesia yang umum digunakan, prioritaskan bahasa Indonesia. Istilah teknis yang lazim boleh dipertahankan; jangan dipaksakan diterjemahkan jika hasilnya tidak natural. Jika istilah asing penting dan baru pertama kali digunakan, boleh diberi padanan dalam tanda kurung.',
   '["Perusahaan membuka pusat layanan (customer service) baru di Surabaya."]', '["Perusahaan membuka customer service center baru di Surabaya untuk memberikan best experience kepada customer."]', '["istilah"]', '[]', '[]', 6, 'active'),
  ('00000000-0000-4000-8000-00000000a224', '00000000-0000-4000-8000-00000000a001', '00000000-0000-4000-8000-00000000a002', 'Digital Style', 'Gaya untuk pembaca digital',
   'Tulis untuk pembaca media digital Indonesia: inti cepat dipahami, paragraf relatif pendek, informasi utama lebih awal, hindari intro panjang, hindari repetisi, jangan mengulang judul di setiap paragraf, jangan memaksakan keyword, jangan membuat artikel terasa seperti SEO article generik. Variasikan struktur paragraf secara natural.',
   '["Kementerian ESDM menargetkan 10.000 SPKLU baru pada 2027. Hingga Juli 2026, baru 4.200 unit terpasang."]', '["Di era digital yang serba cepat ini, banyak hal berubah dengan pesat. Perubahan ini membawa berbagai dampak yang signifikan..."]', '["digital","gaya"]', '[]', '[]', 5, 'active')
ON CONFLICT (id) DO NOTHING;

COMMIT;