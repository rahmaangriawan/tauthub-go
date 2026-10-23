-- 235 — Agent "Racik Instruksi" (racik_instruksi).
--
-- Tombol "Racik Instruksi dari Anchor" di Pengelola Naskah sebelumnya memakai
-- tier economy platform AI dengan prompt yang ditulis di kode. Sekarang agent
-- ini punya koneksi + master prompt berversi sendiri di admin (menu Taut Brief
-- > Racik Agent), mengikuti pola agent ringkasan entitas/file.
--
-- Tanpa konfigurasi yang valid, endpoint /produce/topic-instruction menolak
-- dengan pesan yang mengarahkan admin ke halaman Racik Agent — tidak lagi
-- diam-diam memakai jalur AI lama.

INSERT INTO public.taut_brief_agent_configs (agent, enabled)
VALUES ('racik_instruksi', true)
ON CONFLICT (agent) DO NOTHING;

INSERT INTO public.taut_brief_agent_prompt_versions (agent, version, status, prompt, change_note, published_at)
VALUES (
  'racik_instruksi', 1, 'active',
  'Kamu perancang instruksi penulisan artikel untuk platform Tauthub (bahasa Indonesia).

Bahan yang diberikan berisi daftar tautan/anchor yang diisi user dan jenis media target. Tulis SATU instruksi penulisan (1-2 kalimat) yang memuat:
1. Arah bahasan & tema judul artikel — WAJIB BERBEDA dari anchor text; jangan menyalin anchor persis sebagai judul. Pakai kata kunci semantik yang relevan lalu hubungkan ke niat pencarian (search intent), dan cerminkan kaidah SEO & E-E-A-T (keahlian/pengalaman/otoritas topik).
2. Arahan penyisipan tautan secara natural di bagian yang relevan HANYA bila anchor punya URL; kalau tidak ada URL, tetap sebut mengikuti konteksnya.
3. Gaya mengikuti jenis media: mainstream = berita/feature objektif dengan kaidah berita; blogger = santai mengalir dengan heading.

Aturan:
- Jangan mengarang fakta, angka, harga, atau nama produk yang tidak ada di daftar anchor.
- Jangan menulis pembuka, penjelasan, atau catatan meta.
- Kembalikan JSON valid persis bentuk {"instruction": "..."} tanpa teks lain di luar JSON.',
  'Seed awal master prompt agent racik instruksi',
  now()
)
ON CONFLICT DO NOTHING;
