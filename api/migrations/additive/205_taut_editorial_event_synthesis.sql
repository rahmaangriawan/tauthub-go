-- 205 — Editorial: protokol sintesis peristiwa + simulasi narasumber, dan
-- penutup "bersih" (boilerplate/kontak resmi boleh, tanpa placeholder).
--
-- Menyambung migrasi 204. Menambal prompt AKTIF milik tipe "editorial" saja:
--   1) merevisi kalimat penutup (menangani DUA varian teks lama: seed asli 201
--      dan hasil 204), supaya boilerplate/kontak resmi YANG BERSIH diizinkan
--      tetapi penanda placeholder tetap dilarang;
--   2) menyisipkan blok PROTOKOL SINTESIS PERISTIWA & SIMULASI NARASUMBER —
--      hanya bila belum ada, jadi aman diulang.
-- Semua lewat replace()/append sehingga suntingan manual admin di Taut Prompt
-- Studio tidak ikut tertimpa (teks lama tak cocok -> no-op).

UPDATE public.taut_article_type_prompt_versions v
SET prompt = replace(
        replace(
            v.prompt,
            'E. Proyeksi ke depan & dokumen pendukung: tutup dengan rencana ekspansi, komitmen jangka panjang, atau kutipan penutup manajemen. Lalu lampirkan paragraph "Tentang [Nama Perusahaan]" (boilerplate 50-70 kata) dan blok Media Contact (nama PIC, email korporat, kontak resmi, tautan portal resmi). JANGAN mengarang email/telepon: pakai yang ada di bahan, atau tandai [Placeholder: ...] bila belum ada.',
            'E. Proyeksi ke depan & penutup: tutup dengan rencana ekspansi, komitmen jangka panjang, atau kutipan penutup manajemen. Bila bahan menyediakan data resmi perusahaan, artikel boleh ditutup dengan boilerplate singkat dan kontak media resmi YANG BERSIH; DILARANG menyisipkan penanda placeholder, tanda kurung kaku, atau catatan pengeditan di badan artikel.'
        ),
        'E. Proyeksi ke depan & penutup: tutup dengan rencana ekspansi, komitmen jangka panjang, atau kutipan penutup manajemen. JANGAN menambahkan paragraf "Tentang Perusahaan", blok kontak media (Media Contact), boilerplate perusahaan, nama PIC, surel, telepon, tautan portal, atau penanda seperti [Placeholder: ...] di akhir artikel — cukup akhiri dengan isi berita.',
        'E. Proyeksi ke depan & penutup: tutup dengan rencana ekspansi, komitmen jangka panjang, atau kutipan penutup manajemen. Bila bahan menyediakan data resmi perusahaan, artikel boleh ditutup dengan boilerplate singkat dan kontak media resmi YANG BERSIH; DILARANG menyisipkan penanda placeholder, tanda kurung kaku, atau catatan pengeditan di badan artikel.'
    )
FROM public.taut_article_types t
WHERE v.article_type_id = t.id
  AND t.code = 'editorial'
  AND v.status = 'active';

UPDATE public.taut_article_type_prompt_versions v
SET prompt = v.prompt || E'\n\n' || 'PROTOKOL SINTESIS PERISTIWA & SIMULASI NARASUMBER (WAJIB bila input klien hanya berupa profil statis, deskripsi umum, atau tanpa peristiwa aktual dan data narasumber):
1. PEMBUATAN MOMENTUM BERITA: DILARANG menulis teks berformat profil deskriptif ("X adalah...", "Platform ini berfungsi..."). WAJIB membingkai naskah sebagai momentum aksi resmi yang paling rasional: (a) Peluncuran Resmi — platform/layanan baru resmi beroperasi penuh menjawab kebutuhan pasar nasional; (b) Pembaruan Sistem — integrasi versi terbaru atau pembaruan arsitektur demi keandalan operasional; (c) Perluasan Ekosistem — langkah strategis memperluas jangkauan dan adopsi digital ke industri terkait.
2. NARASUMBER & KUTIPAN: DILARANG memakai narasumber anonim ("perwakilan manajemen", "pihak internal", "narasumber yang enggan disebutkan namanya"). Tulis nama profesional Indonesia yang realistis dengan jabatan C-Level kredibel secara mengalir di tubuh berita (mis. "Rama Pratama, Chief Executive Officer [Nama Brand]"). Kutipan berfokus pada visi industri, efisiensi operasional, dan penyelesaian masalah publik — bukan memuji keunggulan produk sendiri.
3. METRIK EFISIENSI RASIONAL: bila klien tidak memberi data statistik riil, susun klaim efisiensi yang masuk akal dan lazim di industri teknologi (mis. "memangkas waktu pemrosesan dari beberapa hari kerja menjadi hitungan menit", "mengintegrasikan alur distribusi lintas kanal secara terpusat").
4. ATURAN PENULISAN BERSIH (ZERO-PLACEHOLDER): DILARANG mencantumkan kata "Placeholder", tanda kurung kaku [...], atau catatan pengeditan di dalam badan berita — dari headline, lead, tubuh artikel, hingga boilerplate. Naskah harus mengalir bersih dan siap terbit.'
FROM public.taut_article_types t
WHERE v.article_type_id = t.id
  AND t.code = 'editorial'
  AND v.status = 'active'
  AND v.prompt NOT LIKE '%PROTOKOL SINTESIS PERISTIWA%';
