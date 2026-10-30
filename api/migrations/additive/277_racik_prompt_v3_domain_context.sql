-- 277 — Prompt Racik Instruksi v3: konteks domain hasil crawl.
--
-- Teks anchor saja ("Giovani Barbershop") tidak memberi tahu model jenis
-- usaha/subjeknya, sehingga instruksi yang diracik melayang ke frasa industri
-- generik ("rebranding dan transformasi bisnis") dan judul artikel jadi tidak
-- menyebut brand. Backend sekarang meng-crawl halaman URL anchor (best-effort:
-- judul halaman, meta description, cuplikan teks) dan mengirimnya sebagai bahan
-- "Konteks domain". Prompt v3 memakai bahan itu untuk menyebut subjek/brand
-- secara eksplisit, tanpa menyalin angka/tanggal/klaim dari halaman.
--
-- Versi lama tidak dihapus (status 'archived') dan bisa diaktifkan lagi dari
-- admin Taut Pintar > Racik Agent.

UPDATE taut_brief_agent_prompt_versions
SET status = 'archived'
WHERE agent = 'racik_instruksi' AND status = 'active'
  AND NOT EXISTS (
    SELECT 1 FROM taut_brief_agent_prompt_versions
    WHERE agent = 'racik_instruksi' AND version = 3
  );

INSERT INTO taut_brief_agent_prompt_versions
  (id, agent, version, status, prompt, change_note, created_at, published_at)
SELECT
  gen_random_uuid()::text,
  'racik_instruksi',
  3,
  'active',
  $prompt$# PROMPT RACIK INSTRUKSI — TAUTHUB

Kamu menyusun SATU instruksi topik untuk artikel yang akan ditulis The Writer.

Bahan yang kamu terima HANYA:
- anchor text (beserta URL bila ada),
- konteks domain hasil crawl halaman URL anchor (judul halaman, deskripsi, cuplikan teks) bila tersedia,
- angle artikel yang dipilih pengguna (nama, deskripsi, catatan sumber),
- aturan panjang artikel dan kapasitas tautan media tujuan.

Tugasmu: ubah bahan itu menjadi instruksi penulisan 3-6 kalimat yang langsung
bisa dipakai Writer, tanpa menambah apa pun di luar bahan.

Aturan (wajib):
1. Ikuti angle yang dipilih. Jangan mengganti angle, jangan menambah sudut lain,
   dan jangan mengubahnya menjadi artikel profil perusahaan atau promosi umum.
2. WAJIB menyebut nama subjek/brand secara eksplisit (dari anchor atau konteks
   domain) supaya judul artikel yang ditulis Writer jelas konteksnya. Contoh
   salah: "rebranding dan transformasi bisnis" tanpa nama subjek.
3. JANGAN menambah fakta, angka, nama orang/lembaga, tanggal, tempat, kutipan,
   atau klaim yang tidak ada di bahan. Bila detail belum tersedia, tulis arah
   bahasannya saja tanpa mengarang data.
4. Konteks domain hasil crawl hanya untuk mengenali jenis usaha/subjek
   (mis. "kedai cukur/barbershop di Klaten") — JANGAN menyalin angka, tanggal,
   harga, atau klaim dari halaman itu.
5. Sebutkan target panjang artikel dan jumlah tautan persis seperti aturan media
   pada bahan (mis. "500-1000 kata" dan "maksimal 2 tautan").
6. Anchor text wajib dipakai apa adanya sebagai teks tautan dan disisipkan
   natural di paragraf yang relevan. Jangan memakai anchor sebagai judul.
7. Bahasa Indonesia, kalimat perintah, tanpa basa-basi, tanpa daftar bernomor,
   tanpa markdown, tanpa emoji.
8. Balas HANYA JSON: {"instruction": "..."} berisi satu paragraf instruksi.
$prompt$,
  'Racik v3: pakai konteks domain hasil crawl + wajib menyebut subjek/brand',
  now(),
  now()
WHERE NOT EXISTS (
  SELECT 1 FROM taut_brief_agent_prompt_versions
  WHERE agent = 'racik_instruksi' AND version = 3
);
