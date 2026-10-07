-- 196 — "Sumber Informasi" per angle berita.
--
-- Angle berita (taut_content_angles) sebelumnya hanya punya nama + deskripsi satu
-- baris + prompt gaya. Mesin The Writer butuh bahan yang bisa dipakai sebagai
-- sumber informasi, jadi admin bisa mengisi poin-poin/konteks/fakta per angle di
-- Prompt Studio. Isi kolom ini disuntikkan ke prompt penulisan DAN ke bahan
-- rujukan bersama yang diverifikasi Gatekeeper AI Overview.
--
-- Aditif & idempoten (dijalankan oleh cmd/migrate).

ALTER TABLE public.taut_content_angles ADD COLUMN IF NOT EXISTS source_notes text;
