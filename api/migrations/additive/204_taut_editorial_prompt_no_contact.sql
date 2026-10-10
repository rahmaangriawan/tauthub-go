-- 204 — Editorial: buang instruksi blok "Tentang Perusahaan" + "Media Contact".
--
-- Master prompt editorial (seed migrasi 201) menyuruh model melampirkan
-- paragraf "Tentang [Nama Perusahaan]" dan blok "Media Contact" (nama PIC,
-- surel, kontak resmi, tautan portal) di akhir artikel, dan menandai bagian
-- yang belum ada dengan [Placeholder: ...]. Hasilnya artikel ditutup baris
-- kontak berisi placeholder — bukan bagian dari isi berita dan mengganggu.
--
-- Migrasi ini mengganti kalimat lama pada prompt AKTIF milik tipe "editorial",
-- HANYA bila teks lamanya masih ada persis (replace). Jadi suntingan manual
-- admin lewat Taut Prompt Studio tidak ikut tertimpa, dan migrasi aman diulang.

UPDATE public.taut_article_type_prompt_versions v
SET prompt = replace(
        replace(
            v.prompt,
            'E. Proyeksi ke depan & dokumen pendukung: tutup dengan rencana ekspansi, komitmen jangka panjang, atau kutipan penutup manajemen. Lalu lampirkan paragraph "Tentang [Nama Perusahaan]" (boilerplate 50-70 kata) dan blok Media Contact (nama PIC, email korporat, kontak resmi, tautan portal resmi). JANGAN mengarang email/telepon: pakai yang ada di bahan, atau tandai [Placeholder: ...] bila belum ada.',
            'E. Proyeksi ke depan & penutup: tutup dengan rencana ekspansi, komitmen jangka panjang, atau kutipan penutup manajemen. JANGAN menambahkan paragraf "Tentang Perusahaan", blok kontak media (Media Contact), boilerplate perusahaan, nama PIC, surel, telepon, tautan portal, atau penanda seperti [Placeholder: ...] di akhir artikel — cukup akhiri dengan isi berita.'
        ),
        '- Jangan menulis ketidaktahuan yang melemahkan kredibilitas (hindari "perusahaan belum menyediakan rincian biaya"); gunakan estimasi industri yang logis lalu tandai [Placeholder: masukkan ...].',
        '- Jangan menulis ketidaktahuan yang melemahkan kredibilitas (hindari "perusahaan belum menyediakan rincian biaya"); gunakan estimasi industri yang logis dan tulis wajar tanpa penanda placeholder apa pun.'
    )
FROM public.taut_article_types t
WHERE v.article_type_id = t.id
  AND t.code = 'editorial'
  AND v.status = 'active';
