-- 278 — Aturan judul di system prompt Writer (semua varian).
--
-- Judul artikel sempat jatuh ke frasa industri generik tanpa menyebut brand
-- walaupun anchor/brief sudah menamainya. Aturan keras juga ditanam di prompt
-- user (clienttaut/writer.go); migrasi ini menambahkan versi system prompt agar
-- arahan tetap ada walau admin menyunting prompt-nya kelak.
--
-- Aditif & idempoten: teks prompt admin TIDAK diganti, hanya ditambah di akhir,
-- dan hanya bila penanda belum ada.

UPDATE taut_agent_configs
SET system_prompt = COALESCE(system_prompt, '') ||
  E'\n\nATURAN JUDUL (WAJIB): Judul artikel HARUS menyebut nama subjek/brand yang menjadi fokus (dari anchor, brief, atau bahan) — DILARANG judul yang hanya berisi frasa industri generik. Pola: [Nama brand/entitas] + [aksi/topik]. Panjang 8-15 kata, jelas, tanpa tanda kutip. Contoh BENAR: "Giovani Barbershop Perluas Layanan lewat Rebranding Barbershop Modern". Contoh SALAH: "Transformasi Model Bisnis dan Rebranding Pacu Ekspansi Pasar Berkelanjutan" (tidak menyebut subjeknya).',
    updated_at = NOW()
WHERE agent IN ('writer', 'writer_mainstream', 'writer_blogger')
  AND COALESCE(system_prompt, '') NOT LIKE '%ATURAN JUDUL (WAJIB)%';
