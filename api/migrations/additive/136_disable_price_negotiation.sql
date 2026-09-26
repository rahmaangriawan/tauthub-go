-- 136 — Nonaktifkan fitur negosiasi harga (sementara): harga ditentukan mitra.
-- Reset media yang tertahan di status "negosiasi": kembali menunggu review
-- dengan harga milik mitra sendiri (tawaran admin dibuang).
BEGIN;

UPDATE public.media
SET price_status = 'menunggu_review',
    negotiated_price = NULL,
    updated_at = NOW()
WHERE price_status = 'negosiasi';

COMMIT;
