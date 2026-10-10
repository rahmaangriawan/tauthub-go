-- 207 — Opini: protokol identitas penulis adaptif (dynamic byline synthesis).
--
-- Menyambung 204–206 (editorial). Master prompt opini (seed 201) menyuruh
-- memakai penanda [Nama Penulis] bila identitas penulis belum ada. Protokol
-- baru menggantinya: SINTESIS nama personal Indonesia + atribusi kepakaran
-- yang selaras domain bahasan, tanpa placeholder, sebagai satu kalimat penutup
-- mandiri di akhir naskah.
--
-- Menambal prompt AKTIF milik tipe "opini": merevisi butir byline + butir
-- penanganan input minim, lalu menyisipkan blok protokol (hanya bila belum
-- ada). Semua lewat replace()/append sehingga suntingan manual admin tidak
-- tertimpa dan migrasi aman diulang.

UPDATE public.taut_article_type_prompt_versions v
SET prompt = replace(
        replace(
            v.prompt,
            '7. Bio penulis (byline): satu kalimat lugas di akhir isi artikel yang menegaskan otoritas penulis tanpa embel-embel jualan. Format: "[Nama Penulis] adalah [Keahlian/Peran Profesional] yang berbasis di [Kota]." Bila nama penulis belum ada, pakai penanda [Nama Penulis].',
            '7. Bio penulis (byline — DYNAMIC BYLINE): satu kalimat lugas di akhir isi artikel yang menegaskan otoritas penulis tanpa embel-embel jualan. Bila klien tidak menyediakan nama/profil penulis, SINTESIS nama personal Indonesia yang profesional dan atribusi kepakaran yang selaras dengan domain bahasan (lihat Protokol Identitas Penulis Adaptif di bawah), lalu tulis mengalir sebagai satu kalimat penutup mandiri. DILARANG memakai penanda seperti [Nama Penulis] atau [Placeholder: ...].'
        ),
        '- Bila identitas penulis tidak tersedia, pakai penanda [Nama Penulis] pada bio.',
        '- Bila identitas penulis tidak tersedia, sintesis nama personal Indonesia yang kredibel beserta atribusi kepakaran yang selaras dengan domain bahasan (lihat Protokol Identitas Penulis Adaptif di bawah).'
    )
FROM public.taut_article_types t
WHERE v.article_type_id = t.id
  AND t.code = 'opini'
  AND v.status = 'active';

UPDATE public.taut_article_type_prompt_versions v
SET prompt = v.prompt || E'\n\n' || 'PROTOKOL IDENTITAS PENULIS ADAPTIF (DYNAMIC BYLINE SYNTHESIS):
DILARANG meniru contoh kalimat tertentu atau memakai formula teks yang seragam. Eksplorasi susunan kata, gelar keahlian, dan gaya penyampaian identitas penulis secara mandiri dan dinamis.
1. ATURAN PENULISAN BERSIH (ZERO PLACEHOLDER): DILARANG memunculkan kata "Placeholder", tanda kurung kaku [...], atau format draf yang terputus. Seluruh teks pada baris identitas harus mengalir utuh dan siap tayang layaknya catatan kaki esai opini di surat kabar nasional.
2. PENALARAN IDENTITAS FIGURATIF (ORGANIC IDENTITY GENERATION): bila klien tidak menyediakan nama dan profil penulis, ciptakan nama personal Indonesia yang terdengar profesional dan kredibel; rangkai atribusi kepakaran yang selaras dengan domain pembahasan (mis. tata kelola digital, arsitektur informasi, kebijakan publik, sosiologi teknologi, atau ekonomi kreatif); padukan profesi, fokus minat, dan domisili kota secara variatif tanpa terpaku pada satu pola susunan kalimat. Letakkan identitas ini sebagai satu kalimat penutup mandiri di baris paling akhir naskah.'
FROM public.taut_article_types t
WHERE v.article_type_id = t.id
  AND t.code = 'opini'
  AND v.status = 'active'
  AND v.prompt NOT LIKE '%PROTOKOL IDENTITAS PENULIS%';
