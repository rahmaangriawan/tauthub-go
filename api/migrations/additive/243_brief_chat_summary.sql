-- 243 — Ringkasan percakapan bergulir untuk chat Taut Brief.
--
-- Riwayat yang dikirim ke LLM dibatasi 20 pesan terakhir; pesan lebih tua
-- dari itu selama ini hilang dari konteks sama sekali. Kolom chat_summary
-- menyimpan ringkasan bergulir (satu baris per giliran, dibuang dari depan
-- saat melebihi batas) supaya giliran tetap tahu apa yang terjadi di awal
-- percakapan: koreksi user, preferensi, dan fakta yang sudah disepakati.

ALTER TABLE public.profiler_profiles
  ADD COLUMN IF NOT EXISTS chat_summary TEXT NOT NULL DEFAULT '';
