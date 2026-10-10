-- 208 — Editorial: protokol standar redaksi nasional tier-1 (zero-link).
--
-- Menyambung 204–207. Menambal prompt AKTIF milik tipe "editorial":
--   1) butir penutup (E) direvisi — boilerplate "Tentang Perusahaan" + blok
--      "Narahubung Media" kini WAJIB (bukan opsional), tanpa placeholder dan
--      tanpa mengarang surel/nomor telepon;
--   2) menyisipkan blok PROTOKOL KHUSUS STANDAR REDAKSI NASIONAL TIER-1
--      (zero-link, hard news hook, dynamic quote synthesis, anti buying-guide,
--      penutup formal pers, zero-placeholder) — hanya bila belum ada.
-- Sisi kode menegakkan zero-link: articleType 'editorial' memaksa
-- allowsLink=false sehingga finalize mengupas <a> apa pun (stripAllLinks).
-- Suntingan manual admin tidak tertimpa (replace hanya kena teks lama persis).

UPDATE public.taut_article_type_prompt_versions v
SET prompt = replace(
        v.prompt,
        'E. Proyeksi ke depan & penutup: tutup dengan rencana ekspansi, komitmen jangka panjang, atau kutipan penutup manajemen. Bila bahan menyediakan data resmi perusahaan, artikel boleh ditutup dengan boilerplate singkat dan kontak media resmi YANG BERSIH; DILARANG menyisipkan penanda placeholder, tanda kurung kaku, atau catatan pengeditan di badan artikel.',
        'E. Proyeksi ke depan & penutup: tutup dengan rencana ekspansi, komitmen jangka panjang, atau kutipan penutup manajemen. WAJIB ditutup dua komponen baku pers: (a) paragraf "Tentang Perusahaan" (boilerplate 50-70 kata: identitas, fokus industri, basis operasional) dan (b) blok "Narahubung Media" (divisi hubungan pers untuk verifikasi wartawan). Narahubung ditulis tanpa mengarang surel/nomor telepon; DILARANG menyisipkan penanda placeholder, tanda kurung kaku, atau catatan pengeditan di badan artikel.'
    )
FROM public.taut_article_types t
WHERE v.article_type_id = t.id
  AND t.code = 'editorial'
  AND v.status = 'active';

UPDATE public.taut_article_type_prompt_versions v
SET prompt = v.prompt || E'\n\n' || 'PROTOKOL KHUSUS STANDAR REDAKSI NASIONAL TIER-1 (KOMPAS, DETIK, CNN INDONESIA):
Terapkan aturan ketat berikut agar naskah layak melewati kurasi meja redaksi berita nasional.
1. STERILISASI ANCHOR TEXT & BACKLINK (ZERO-LINK): DILARANG KERAS menyematkan tag <a href>, format markdown link, atau hyperlink tersembunyi pada kata apa pun di dalam badan berita. Penautan kata kunci otomatis diklasifikasikan redaktur sebagai materi iklan komersial. Alamat web hanya boleh muncul sebagai teks polos tanpa tautan aktif di blok narahubung pers paling akhir.
2. AKTUASI MOMENTUM BERITA (HARD NEWS HOOK): paragraf pertama (lead) wajib diawali subjek aksi dan momentum peristiwa aktual yang jelas (peresmian sistem baru, integrasi infrastruktur data, pencapaian metrik, atau inisiatif strategis industri) dengan dateline standar. DILARANG membuka berita dengan pengantar esai konseptual yang bertele-tele tanpa menyebut subjek dan peristiwanya di kalimat pembuka.
3. EKSPLORASI DINAMIS KUTIPAN RESMI (DYNAMIC QUOTE SYNTHESIS): DILARANG meniru contoh kalimat kutipan atau pola susunan kata tertentu secara harfiah — eksplorasi pilihan kata, struktur atribusi, dan ritme kalimat pernyataan secara mandiri. Pernyataan narasumber berfokus pada visi industri, efisiensi operasional, pergeseran tren pasar, atau penyelesaian tantangan publik — bukan pujian klise terhadap produk sendiri; nadanya berbobot dan wajar layaknya wawancara langsung figur profesional. Variasikan penempatan nama dan jabatan (di akhir kutipan, di awal kalimat pembuka, atau memecah kutipan di tengah jeda gagasan) serta kata kerja pelapor yang proporsional sesuai konteks (menjelaskan, menegaskan, menuturkan, memaparkan, menggarisbawahi, menandaskan). Jabatan disesuaikan secara logis dengan substansi yang dibahas (Founder, CEO, CTO, atau System Architect).
4. PEMBERSIHAN GAYA PANDUAN KONSUMEN (ELIMINATE BUYING GUIDE TONE): DILARANG memakai kalimat rekomendasi bersyarat untuk pengguna (mis. "Pilih skema A bila...", "Disarankan untuk...", "Jika Anda membutuhkan..."). Ganti dengan ulasan objektif mengenai mitigasi risiko sistem, kepatuhan data/privasi, rencana kerja sama lanjutan, atau proyeksi ekspansi ekosistem ke depan.
5. PENUTUP FORMAL PERS: badan berita ditutup gagasan konklusif terlebih dahulu, baru dua komponen baku pers — (a) Tentang Perusahaan: ringkasan formal entitas 50-70 kata yang menjelaskan identitas, fokus industri, dan basis operasional; (b) Narahubung Media: informasi divisi hubungan pers untuk keperluan verifikasi berita oleh wartawan.
6. PENULISAN BERSIH TANPA PLACEHOLDER: DILARANG memunculkan kata "Placeholder", tanda kurung kaku [...], atau instruksi pengeditan di dalam badan naskah. Bila data masukan tidak menyertakan nama narasumber atau angka riil, susun nama figur profesional Indonesia yang realistis serta estimasi metrik industri yang masuk akal secara mengalir langsung di dalam naskah — tanpa penanda apa pun.'
FROM public.taut_article_types t
WHERE v.article_type_id = t.id
  AND t.code = 'editorial'
  AND v.status = 'active'
  AND v.prompt NOT LIKE '%PROTOKOL KHUSUS STANDAR REDAKSI NASIONAL%';
