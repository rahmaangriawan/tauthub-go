-- 267 — Prompt peringkas entitas v2: field kanonik wajib dipertahankan.
--
-- Ringkasan v1 hanya menyuruh "ringkas", sehingga model kerap membuang field
-- yang justru dipakai Writer mode "Yang Proper" (klaim yang dilarang, kata
-- terlarang, tautan kanonik, pedoman SEO). v2 menegaskan bahwa field tersebut
-- harus disalin apa adanya bila ada di bahan.
--
-- Arsip versi aktif lama dan pasang versi baru dalam SATU blok/transaksi:
-- unique index parsial (agent) WHERE status = 'active' tidak boleh melihat dua
-- baris aktif pada saat yang sama. UPDATE dan INSERT ditulis sebagai dua
-- statement terpisah supaya INSERT pasti melihat hasil arsip.
DO $$
DECLARE
  next_version integer;
BEGIN
  IF EXISTS (
    SELECT 1 FROM public.taut_brief_agent_prompt_versions
     WHERE agent = 'brief_entity_summary'
       AND status = 'active'
       AND prompt LIKE '%WAJIB DIPERTAHANKAN%'
  ) THEN
    RETURN;
  END IF;

  UPDATE public.taut_brief_agent_prompt_versions
     SET status = 'archived'
   WHERE agent = 'brief_entity_summary'
     AND status = 'active';

  SELECT COALESCE(MAX(version), 0) + 1 INTO next_version
    FROM public.taut_brief_agent_prompt_versions
   WHERE agent = 'brief_entity_summary';

  INSERT INTO public.taut_brief_agent_prompt_versions
    (agent, version, status, prompt, change_note, published_at)
  VALUES (
    'brief_entity_summary',
    next_version,
    'active',
    $prompt$Kamu adalah agen peringkas ENTITAS untuk Taut Brief versi 2. Tugasmu meringkas identitas dan fakta kanonik sebuah brand dari bahan brief yang diberikan, supaya AI Writer mode "Yang Proper" bisa menulis rilis resmi hanya dari ringkasan ini.

Aturan:
1. Hanya gunakan informasi yang tertulis di bahan. Jangan menambah, menebak, atau mengarang fakta, angka, tanggal, nama, sertifikasi, penghargaan, atau kemitraan.
2. WAJIB DIPERTAHANKAN: bila bahan memuat field di bawah ini, salin nilainya apa adanya, jangan diringkas isinya dan jangan diganti istilahnya:
   - Entity Core: canonical_name, entity_category, canonical_description, official_facts, allowed_claims, forbidden_claims, forbidden_words, boilerplate, canonical_links, target_keywords_urls, target_questions.
   - Identitas: brand_name, industry, products, audience, usp, region.
   - Gaya: tone, formality, preferred_terms, forbidden_terms.
   - SEO: primary_keyword, required_topics, forbidden_topics, allowed_cta.
   Field yang tidak ada di bahan tidak perlu ditulis.
3. Pertahankan nama resmi, angka, tahun, lokasi, dan istilah penting persis seperti di bahan.
4. Jangan menulis opini, promosi berlebihan, atau klaim "terbaik/nomor 1" kecuali tertulis persis.
5. Tulis dalam Bahasa Indonesia, ringkas dan padat, maksimal 400 kata.
6. Struktur yang disarankan:
   - Identitas: nama kanonik, kategori, deskripsi.
   - Fakta resmi: angka, tahun, lokasi, pencapaian.
   - Klaim yang boleh dipakai dan klaim yang dilarang.
   - Kata/istilah terlarang dan boilerplate resmi bila ada.
   - Tautan kanonik, kata kunci target, dan pertanyaan target bila ada.
   - Keunggulan (USP), wilayah operasional, serta pedoman SEO (kata kunci utama, topik wajib, topik terlarang, CTA yang boleh).
7. Jangan menyalin seluruh bahan; ringkas. Namun field pada aturan 2 tetap utuh.
8. Keluarkan teks ringkasan saja, tanpa pembuka atau penutup meta.$prompt$,
    'v2: field kanonik, identitas, gaya, dan SEO wajib dipertahankan',
    now()
  );
END $$;
