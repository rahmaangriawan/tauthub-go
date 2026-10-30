-- 284 — Bersihkan writer_topic yang mencampur dua brand.
--
-- Kasus nyata: kampanye 3c952119 slot 1 (media IDNberita.com) punya topik yang
-- menyuruh "mengintegrasikan informasi tentang Giovani Barbershop" padahal
-- subjeknya Salsari Resto. Model menuruti apa adanya sehingga judulnya jadi
-- "Giovani Barbershop dan Salsari Resto: ..." dan isi berpindah topik.
--
-- Topik baru fokus pada SATU subjek. Angka seperti "1.000 orang", "35 menu",
-- "sejak 2018" TIDAK dimasukkan karena itu karangan model, bukan fakta bahan.
-- Struktur artikel tetap ditentukan angle yang dipilih di modal, bukan di sini.
--
-- Idempoten: hanya menyentuh baris yang topiknya masih memuat nama brand lain,
-- jadi menjalankan ulang tidak menimpa topik yang sudah diedit klien.
UPDATE taut_campaign_media
SET writer_topic = $topic$Tulis artikel blog untuk audiens blogger IDNberita.com tentang Salsari Resto, rumah makan keluarga di Playen, Gunungkidul yang letaknya di jalur wisata Gunungkidul–Jogja.

Fokus bahasan: suasana tempat yang luas dan nyaman untuk keluarga maupun rombongan, fasilitas yang ramah anak, serta ragam menu khas Nusantara yang ditawarkan. Kaitkan dengan kebutuhan audiens blogger, misalnya tempat bersantai, pertemuan dengan teman, atau acara keluarga.

Sertakan anchor text "salsari resto" yang mengarah ke https://salsariresto.com secara natural di dalam paragraf yang relevan. Jangan menambah fakta, angka, harga, atau tahun yang tidak disebut di sini.$topic$
WHERE id = '1dd30220-4b06-4f7c-931a-840ef7fe6e73'
  AND writer_topic ILIKE '%giovani%';
