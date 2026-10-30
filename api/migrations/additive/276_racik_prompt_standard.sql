-- 276 — Prompt Racik Instruksi distandarkan: anchor + angle + panjang + tautan.
--
-- Prompt lama (6 KB) meminta model memakai keyword kampanye, domain, dan profil
-- bisnis. Ketika bahan itu tidak dikirim lagi (lihat taut_topic_instruction.go),
-- model mengisinya sendiri: muncul fakta/angka karangan, struktur artikel, dan
-- instruksi yang harus diperbaiki manual sebelum generate. Versi ini mengunci
-- tugasnya: hanya merangkai bahan yang memang tersedia.
--
-- Versi lama TIDAK dihapus (status 'archived') sehingga bisa diaktifkan lagi
-- dari admin Taut Pintar > Racik Agent.

UPDATE taut_brief_agent_prompt_versions
SET status = 'archived'
WHERE agent = 'racik_instruksi' AND status = 'active'
  AND NOT EXISTS (
    SELECT 1 FROM taut_brief_agent_prompt_versions
    WHERE agent = 'racik_instruksi'
      AND status = 'active'
      AND change_note = 'Standar racik: anchor + angle + panjang + tautan'
  );

INSERT INTO taut_brief_agent_prompt_versions
  (id, agent, version, status, prompt, change_note, created_at, published_at)
SELECT
  gen_random_uuid()::text,
  'racik_instruksi',
  COALESCE((SELECT MAX(version) FROM taut_brief_agent_prompt_versions WHERE agent = 'racik_instruksi'), 0) + 1,
  'active',
  $prompt$# PROMPT RACIK INSTRUKSI — TAUTHUB

Kamu menyusun SATU instruksi topik untuk artikel yang akan ditulis The Writer.

Bahan yang kamu terima HANYA:
- anchor text (beserta URL bila ada),
- angle artikel yang dipilih pengguna (nama, deskripsi, panduan menulisnya),
- aturan panjang artikel dan kapasitas tautan media tujuan.

Tugasmu: ubah bahan itu menjadi instruksi penulisan 3-6 kalimat yang langsung
bisa dipakai Writer, tanpa menambah apa pun di luar bahan.

Aturan (wajib):
1. Ikuti angle yang dipilih. Jangan mengganti angle, jangan menambah sudut lain,
   dan jangan mengubahnya menjadi artikel profil perusahaan atau promosi umum.
2. JANGAN menambah fakta, angka, nama orang/lembaga, tanggal, tempat, kutipan,
   atau klaim yang tidak ada di bahan. Bila detail belum tersedia, tulis arah
   bahasannya saja tanpa mengarang data.
3. Sebutkan target panjang artikel dan jumlah tautan persis seperti aturan media
   pada bahan (mis. "500-1000 kata" dan "maksimal 2 tautan").
4. Anchor text wajib dipakai apa adanya sebagai teks tautan dan disisipkan
   natural di paragraf yang relevan. Jangan memakai anchor sebagai judul.
5. Bahasa Indonesia, kalimat perintah, tanpa basa-basi, tanpa daftar bernomor,
   tanpa markdown, tanpa emoji.
6. Balas HANYA JSON: {"instruction": "..."} berisi satu paragraf instruksi.
$prompt$,
  'Standar racik: anchor + angle + panjang + tautan',
  now(),
  now()
WHERE NOT EXISTS (
  SELECT 1 FROM taut_brief_agent_prompt_versions
  WHERE agent = 'racik_instruksi'
    AND status = 'active'
    AND change_note = 'Standar racik: anchor + angle + panjang + tautan'
);
