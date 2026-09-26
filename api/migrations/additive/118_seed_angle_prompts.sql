-- 118 -- Seed versi prompt generik untuk SEMUA content angle (v1, active).
-- Angle sebelumnya tidak punya versi prompt -> "Jalankan Analisis Profiler"
-- selalu ditolak ("belum memiliki versi prompt aktif"). Prompt generik ini
-- berbasis nama angle + konteks bisnis dari input Profiler; super admin bisa
-- menimpa per angle lewat Taut Prompt Studio (versi baru, version++).
INSERT INTO public.taut_content_angle_prompt_versions
  (id, content_angle_id, version, status, prompt, model_overrides, change_note, created_by, published_by, published_at)
SELECT
  '00000000-0000-4000-8000-00000000d' || lpad((row_number() OVER (ORDER BY a.sort_order, a.id))::text, 3, '0'),
  a.id,
  1,
  'active',
  'Kamu adalah PR strategist untuk angle "' || a.name || '" (' || a.slug || ').
' || COALESCE(NULLIF(a.description, ''), '') || '

TUGAS: Analisis input bisnis yang diberikan (profil bisnis, objective, audiens,
fakta crawl website, materi dokumen klien) lalu susun Executive PR Brief untuk
artikel dengan angle "' || a.name || '".

PEDOMAN PENULISAN:
- Faktual: hanya gunakan fakta dari input. Jika data kurang, catat di
  needs_verification — JANGAN mengarang angka, nama, atau tanggal.
- Sesuai angle: pilih sudut pandang, struktur, dan nada yang khas untuk
  "' || a.name || '".
- Bahasa Indonesia yang natural, gaya jurnalistik yang enak dibaca.
- Sertakan 3-5 saran strategic_notes yang actionable.
- output WAJIB satu objek JSON valid sesuai [OUTPUT REQUIREMENT].',
  NULL,
  'Seed prompt generik v1 (migrasi 118) — sementara sampai dioptimalkan di Taut Prompt Studio.',
  '00000000-0000-4000-8000-000000000001',
  '00000000-0000-4000-8000-000000000001',
  now()
FROM public.taut_content_angles a
WHERE NOT EXISTS (
  SELECT 1 FROM public.taut_content_angle_prompt_versions v WHERE v.content_angle_id = a.id
);
