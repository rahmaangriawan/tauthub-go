-- 206 — Editorial: prinsip generasi penutup & data adaptif (dynamic reasoning).
--
-- Menyambung migrasi 205. Menyisipkan blok "PRINSIP GENERASI PENUTUP & DATA
-- ADAPTIF" ke prompt editorial AKTIF — melarang peniruan contoh secara harfiah,
-- mengatur skema harga/spek minim tanpa placeholder, mewajibkan penutup
-- konklusif (refleksi nilai + soft-CTA), dan menegaskan badan artikel 100%
-- bersih. Disisipkan HANYA bila belum ada, jadi aman diulang; suntingan manual
-- admin tidak tertimpa karena penambahan hanya muncul sekali.

UPDATE public.taut_article_type_prompt_versions v
SET prompt = v.prompt || E'\n\n' || 'PRINSIP GENERASI PENUTUP & DATA ADAPTIF (DYNAMIC REASONING PRINCIPLE):
DILARANG meniru kalimat contoh atau format teks tertentu secara harfiah. Susun kalimat penutup dan deskripsi secara mandiri, bervariasi, dan kontekstual terhadap industri yang dibahas.
1. SKEMA HARGA & SPESIFIKASI MINIM (ZERO-PLACEHOLDER): DILARANG memunculkan kata "Placeholder", tanda kurung kaku [...], atau instruksi pengeditan di tubuh naskah. Bila data paket/biaya tidak disediakan, sintesis narasi yang luwes tentang fleksibilitas penggunaan, skalabilitas akses, atau skema berjenjang yang lazim berlaku di industri terkait — kalimat deskriptif alami yang mengalir tanpa menimbulkan keraguan pembaca terhadap kesiapan bisnis tersebut.
2. LOGIKA PERAKITAN PENUTUP (DYNAMIC CLOSING): artikel DILARANG berhenti menggantung pada komparasi atau pembahasan fitur. Wajib menutup naskah dengan satu gagasan konklusif yang memadukan: (a) Refleksi Nilai — sintesis singkat peran solusi ini menjawab tantangan era digital/modern; (b) Ajakan Bertindak Halus (Soft-CTA) yang intensitasnya menyesuaikan kampanye — bila entitas punya portal/akses publik, rangkai ajakan mengalir untuk mengeksplorasi layanan, mencoba simulasi, atau berkonsultasi; bila murni reputasi tanpa tautan, tutup dengan kalimat reflektif yang mengukuhkan posisi entitas sebagai opsi rasional di industrinya. Tetap tanpa nada jualan keras dan tanpa seruan transaksi konsumen. DILARANG formula kaku seperti "Kesimpulannya", "Akhir kata", atau "Tunggu apa lagi".
3. BADAN ARTIKEL BERSIH: seluruh badan artikel WAJIB 100% bersih dan siap terbit — tanpa placeholder, tanpa tanda kurung kaku, dan tanpa catatan verifikasi di dalam naskah.'
FROM public.taut_article_types t
WHERE v.article_type_id = t.id
  AND t.code = 'editorial'
  AND v.status = 'active'
  AND v.prompt NOT LIKE '%PRINSIP GENERASI PENUTUP%';
