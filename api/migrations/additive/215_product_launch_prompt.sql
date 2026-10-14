-- 215 -- Master prompt khusus angle "Peluncuran Produk / Fitur Baru"
-- (product_launch, kanal mainstream). Mengganti seed generik migrasi 118
-- dengan prompt editorial hasil penyesuaian ke pipeline Taut.
--
-- Kenapa disusun begini:
--   * The Profiler memakai prompt ini UTUH sebagai [PROFILER INSTRUCTION]
--     (profiler.go: resolveProfilerPrompt) untuk menyusun Executive PR Brief.
--   * The Writer memakai prompt ini sebagai blok [PROMPT ANGLE ...] tapi
--     DIPOTONG 1500 karakter (matchmaker.go: loadAnglePromptBlockBySlug dan
--     taut_writer.go: loadTautChosenAnglePrompt). Karena itu seluruh aturan
--     penulisan ditaruh di 1500 karakter pertama, dan panduan Profiler
--     diletakkan setelahnya.
--
-- Aman & idempoten:
--   * Hanya menimpa saat versi aktif masih seed generik migrasi 118 —
--     kalau admin sudah mengedit sendiri di Taut Prompt Studio, migrasi ini
--     tidak menyentuh apa pun.
--   * Kalau versi dengan change_note penanda sudah ada, migrasi dilewati.

DO $mig$
DECLARE
  v_angle_id  text;
  v_next_ver  int;
  v_active_note text;
  v_prompt text := $prompt$Anda redaktur senior desk teknologi dan bisnis media nasional tier-1 Indonesia untuk angle "Peluncuran Produk / Fitur Baru". Naskah harus layak terbit sebagai berita peluncuran yang wibawa — bukan brosur promosi, daftar fitur, atau changelog.

PRINSIP WAJIB:
1. Dampak di atas spesifikasi: jangan mendaftar fitur; uraikan masalah yang diselesaikan, cara kerjanya, dan dampaknya pada efisiensi alur kerja.
2. Bingkai sebelum-sesudah: posisikan peluncuran sebagai jawaban atas keterbatasan cara lama — tanpa menjelekkan kompetitor.
3. Anti-brosur: tanpa superlatif kosong ("tercanggih", "nomor satu"), tanpa ajakan membeli; sudut pandang orang ketiga yang netral.
4. Kredibilitas selektif: klaim keandalan arsitektur, keamanan data, atau integrasi hanya bila didukung bahan.

FAKTA & KUTIPAN: angka, tanggal, nama, dan kutipan hanya dari bahan/brief/narasumber yang tersedia — dilarang mengarang. Kutipan narasumber dipakai bila ada, dengan atribusi bervariasi dan kata pelapor proporsional (ujar, menjelaskan, menegaskan, memaparkan). Tanpa narasumber: tanpa kutipan. Tanpa angka pendukung: jelaskan manfaat secara kualitatif. Dilarang menulis tanda kurung siku, placeholder, atau instruksi pengeditan di dalam naskah.

STRUKTUR: pembuka langsung ke peristiwa (apa yang diluncurkan, siapa pengembangnya, urgensinya); tubuh = keterbatasan lama, mekanisme fitur, dampak konkret; penutup = arah pengembangan berikutnya bila ada di bahan. Judul 8-13 kata, menonjolkan momentum peluncuran dan dampaknya.

PETUNJUK THE PROFILER (Executive PR Brief angle ini):
- Susun brief dari seluruh input (profil bisnis, objective, fakta crawl, dokumen, jawaban dinamis, guardrails) agar penulis menghasilkan naskah sesuai prinsip angle di atas.
- writing_instruction per channel: pertegas sudut peluncuran (dampak lebih dulu, spesifikasi menyusul), urutan bahasan, fitur yang wajib dijelaskan mekanismenya, dan larangan gaya brosur.
- strategic_notes: 3-5 saran actionable — penajaman framing sebelum-sesudah, penempatan kutipan, atau klaim yang wajib dijaga.
- Klaim yang belum terbukti di materi masuk needs_verification, jangan diperlakukan sebagai fakta.
- Blok [OUTPUT REQUIREMENT], [SPOKESPERSON], dan [GUARDRAILS] dari pipeline selalu menang atas instruksi prompt ini.$prompt$;
  v_change_note text := 'Master prompt editorial "Peluncuran Produk / Fitur Baru" (migrasi 215) — disesuaikan agar utuh dipakai Profiler (teks penuh) dan Writer (1500 karakter pertama).';
  v_marker text := 'Master prompt editorial "Peluncuran Produk / Fitur Baru" (migrasi 215)%';
BEGIN
  SELECT id INTO v_angle_id
  FROM public.taut_content_angles
  WHERE slug = 'product_launch'
  LIMIT 1;

  IF v_angle_id IS NULL THEN
    RETURN;
  END IF;

  -- Sudah pernah dipasang? (idempoten)
  IF EXISTS (
    SELECT 1 FROM public.taut_content_angle_prompt_versions
    WHERE content_angle_id = v_angle_id AND change_note LIKE v_marker
  ) THEN
    RETURN;
  END IF;

  -- Admin sudah mengedit sendiri? Jangan sentuh.
  SELECT change_note INTO v_active_note
  FROM public.taut_content_angle_prompt_versions
  WHERE content_angle_id = v_angle_id AND status = 'active'
  LIMIT 1;

  IF v_active_note IS NOT NULL AND v_active_note NOT LIKE 'Seed prompt generik v1 (migrasi 118)%' THEN
    RETURN;
  END IF;

  SELECT COALESCE(MAX(version), 0) + 1 INTO v_next_ver
  FROM public.taut_content_angle_prompt_versions
  WHERE content_angle_id = v_angle_id;

  UPDATE public.taut_content_angle_prompt_versions
  SET status = 'archived'
  WHERE content_angle_id = v_angle_id AND status = 'active';

  INSERT INTO public.taut_content_angle_prompt_versions
    (id, content_angle_id, version, status, prompt, change_note, created_by, published_at, published_by)
  VALUES
    ('00000000-0000-4000-8000-00000000d215', v_angle_id, v_next_ver, 'active', v_prompt,
     v_change_note, '00000000-0000-4000-8000-000000000001', now(), '00000000-0000-4000-8000-000000000001')
  ON CONFLICT (id) DO NOTHING;
END
$mig$;
