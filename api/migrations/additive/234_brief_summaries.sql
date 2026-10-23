-- 234 — Ringkasan Brief (entitas & file) untuk hemat token Writer "Yang Proper".
--
-- Taut Brief sebelumnya menyerahkan seluruh isi profil (entity_core, identity,
-- style, seo) ke Writer mode proper. Sekarang tiap brief punya dua ringkasan
-- yang dihasilkan dua agent terpisah: ringkasan entitas dan ringkasan file.
-- Writer hanya membaca ringkasan ini, jadi prompt jauh lebih pendek.
--
-- Ringkasan diperbarui otomatis saat data brief berubah (job `brief_summary`),
-- dan tiap agent punya koneksi AI + prompt berversi sendiri di admin.

-- ---------- konfigurasi agent ----------
CREATE TABLE IF NOT EXISTS public.taut_brief_agent_configs (
  agent varchar(40) PRIMARY KEY,
  base_url text,
  sealed_api_key text,
  model text,
  temperature double precision NOT NULL DEFAULT 0.2,
  max_tokens integer,
  timeout_ms integer NOT NULL DEFAULT 120000,
  enabled boolean NOT NULL DEFAULT true,
  settings jsonb NOT NULL DEFAULT '{}'::jsonb,
  updated_by character(36),
  updated_at timestamptz NOT NULL DEFAULT now()
);

COMMENT ON TABLE public.taut_brief_agent_configs IS
  'Koneksi & model per agent Taut Brief (brief_entity_summary, brief_file_summary). Kolom kosong mewarisi taut_brief_config.';

-- ---------- prompt berversi per agent ----------
CREATE TABLE IF NOT EXISTS public.taut_brief_agent_prompt_versions (
  id character(36) PRIMARY KEY DEFAULT (gen_random_uuid()::text),
  agent varchar(40) NOT NULL,
  version integer NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'draft',
  prompt text NOT NULL DEFAULT '',
  change_note text,
  created_by character(36),
  created_at timestamptz NOT NULL DEFAULT now(),
  published_by character(36),
  published_at timestamptz,
  CONSTRAINT ck_taut_brief_agent_prompt_status
    CHECK (status IN ('draft', 'active', 'archived'))
);

CREATE UNIQUE INDEX IF NOT EXISTS taut_brief_agent_prompt_agent_version_uidx
  ON public.taut_brief_agent_prompt_versions (agent, version);

CREATE UNIQUE INDEX IF NOT EXISTS taut_brief_agent_prompt_active_uidx
  ON public.taut_brief_agent_prompt_versions (agent)
  WHERE status = 'active';

-- ---------- hasil ringkasan ----------
CREATE TABLE IF NOT EXISTS public.profiler_brief_summaries (
  profile_id character(36) NOT NULL REFERENCES public.profiler_profiles(id) ON DELETE CASCADE,
  kind varchar(16) NOT NULL,
  status varchar(16) NOT NULL DEFAULT 'idle',
  content text,
  source_chars integer,
  source_hash text,
  model text,
  token_input integer,
  token_output integer,
  error text,
  generated_at timestamptz,
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (profile_id, kind),
  CONSTRAINT ck_profiler_brief_summaries_kind CHECK (kind IN ('entity', 'file')),
  CONSTRAINT ck_profiler_brief_summaries_status
    CHECK (status IN ('idle', 'queued', 'running', 'ready', 'failed'))
);

COMMENT ON TABLE public.profiler_brief_summaries IS
  'Ringkasan entitas/file per brief; dibaca Writer mode proper sebagai satu-satunya memori brand.';

CREATE INDEX IF NOT EXISTS profiler_brief_summaries_status_idx
  ON public.profiler_brief_summaries (status);

-- ---------- log run: tandai agent ----------
ALTER TABLE public.taut_brief_runs
  ADD COLUMN IF NOT EXISTS agent varchar(40) NOT NULL DEFAULT 'chat';

-- ---------- seed agent ----------
INSERT INTO public.taut_brief_agent_configs (agent, enabled)
VALUES ('brief_entity_summary', true), ('brief_file_summary', true)
ON CONFLICT (agent) DO NOTHING;

INSERT INTO public.taut_brief_agent_prompt_versions (agent, version, status, prompt, change_note, published_at)
VALUES
(
  'brief_entity_summary', 1, 'active',
  'Kamu adalah agen peringkas ENTITAS untuk Taut Brief. Tugasmu meringkas identitas dan fakta kanonik sebuah brand dari bahan brief yang diberikan, supaya AI Writer mode "Yang Proper" bisa menulis rilis resmi hanya dari ringkasan ini.

Aturan:
1. Hanya gunakan informasi yang tertulis di bahan. Jangan menambah, menebak, atau mengarang fakta, angka, tanggal, nama, sertifikasi, penghargaan, atau kemitraan.
2. Pertahankan nama resmi, angka, tahun, lokasi, dan istilah penting persis seperti di bahan.
3. Jangan menulis opini, promosi berlebihan, atau klaim "terbaik/nomor 1" kecuali tertulis persis.
4. Tulis dalam Bahasa Indonesia, ringkas dan padat, maksimal 400 kata.
5. Struktur yang disarankan:
   - Identitas: nama kanonik, kategori, deskripsi.
   - Fakta resmi: angka, tahun, lokasi, pencapaian.
   - Klaim yang boleh dipakai.
   - Larangan/klaim yang tidak boleh dipakai.
   - Keunggulan (USP) dan wilayah operasional.
   - Pertanyaan target yang ingin dimenangkan brand, bila ada.
6. Jangan menyalin seluruh bahan; ringkas.
7. Keluarkan teks ringkasan saja, tanpa pembuka atau penutup meta.',
  'Seed awal agen ringkasan entitas',
  now()
),
(
  'brief_file_summary', 1, 'active',
  'Kamu adalah agen peringkas FILE untuk Taut Brief. Tugasmu meringkas dokumen, tautan, dan aset file milik brand dari bahan yang diberikan, supaya AI Writer mode "Yang Proper" cukup membaca ringkasan ini.

Aturan:
1. Hanya gunakan isi bahan. Jangan menambah fakta dari luar bahan.
2. Pertahankan angka, tanggal, nama, lokasi, dan istilah teknis persis seperti tertulis.
3. Abaikan boilerplate navigasi, menu, iklan, atau teks yang tidak informatif.
4. Tulis dalam Bahasa Indonesia, maksimal 500 kata.
5. Struktur:
   - Ringkasan umum: isi berkas-berkas ini dan kegunaannya.
   - Poin fakta penting per berkas/tautan, sebutkan nama berkasnya.
   - Data kuantitatif (angka, tanggal, kapasitas, harga bila ada).
   - Pertentangan atau hal yang perlu dihindari, bila ada.
6. Bila bahan kosong atau tidak informatif, tulis "Tidak ada isi berkas yang bisa diringkas."
7. Keluarkan teks ringkasan saja, tanpa pembuka atau penutup meta.',
  'Seed awal agen ringkasan file',
  now()
)
ON CONFLICT DO NOTHING;

-- ---------- backfill: brief lama ----------
-- Writer mode proper mewajibkan ringkasan entitas. Brief lama diantrekan sekali,
-- tetapi dibatasi supaya deploy tidak memicu ribuan panggilan model:
--   * hanya 300 brief terbaru (updated_at) yang diantrekan;
--   * job disebar 2 detik (worker sudah membatasi konkurensi model);
--   * ringkasan file hanya untuk brief yang benar-benar punya berkas.
-- Brief di luar batas akan dibuat saat datanya berubah atau lewat tombol manual.
INSERT INTO public.jobs (id, kind, payload, status, attempts, max_attempts, run_at)
SELECT gen_random_uuid()::text,
       'brief_summary',
       jsonb_build_object('profileId', p.id, 'brandId', p.brand_id, 'kind', 'entity'),
       'pending', 0, 2,
       now() + (p.rn * interval '2 seconds')
FROM (
  SELECT p.id, p.brand_id,
         row_number() OVER (ORDER BY p.updated_at DESC) AS rn
  FROM public.profiler_profiles p
  WHERE p.deleted_at IS NULL
    AND (
      p.entity_core <> '{}'::jsonb
      OR p.identity <> '{}'::jsonb
      OR p.style <> '{}'::jsonb
      OR p.seo <> '{}'::jsonb
      OR p.instructions IS NOT NULL
      OR EXISTS (SELECT 1 FROM public.profiler_blocks b WHERE b.profile_id = p.id)
    )
  ORDER BY p.updated_at DESC
  LIMIT 300
) p
WHERE NOT EXISTS (
  SELECT 1 FROM public.jobs j
  WHERE j.kind = 'brief_summary' AND j.status = 'pending'
    AND j.payload->>'profileId' = p.id
    AND j.payload->>'kind' = 'entity'
);

INSERT INTO public.jobs (id, kind, payload, status, attempts, max_attempts, run_at)
SELECT gen_random_uuid()::text,
       'brief_summary',
       jsonb_build_object('profileId', p.id, 'brandId', p.brand_id, 'kind', 'file'),
       'pending', 0, 2,
       now() + interval '5 seconds' + (p.rn * interval '2 seconds')
FROM (
  SELECT p.id, p.brand_id,
         row_number() OVER (ORDER BY p.updated_at DESC) AS rn
  FROM public.profiler_profiles p
  WHERE p.deleted_at IS NULL
    AND (
      EXISTS (SELECT 1 FROM public.profiler_documents d WHERE d.profile_id = p.id AND d.deleted_at IS NULL)
      OR EXISTS (SELECT 1 FROM public.profiler_urls u WHERE u.profile_id = p.id AND u.deleted_at IS NULL)
      OR EXISTS (SELECT 1 FROM public.profiler_images i WHERE i.profile_id = p.id AND i.deleted_at IS NULL)
    )
  ORDER BY p.updated_at DESC
  LIMIT 300
) p
WHERE NOT EXISTS (
  SELECT 1 FROM public.jobs j
  WHERE j.kind = 'brief_summary' AND j.status = 'pending'
    AND j.payload->>'profileId' = p.id
    AND j.payload->>'kind' = 'file'
);
