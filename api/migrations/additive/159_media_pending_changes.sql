-- 159: Keterangan perubahan media yang menunggu review admin.
--
-- Sebelumnya admin hanya melihat "Menunggu Review" tanpa tahu APA yang diubah
-- mitra (kategori? harga? deskripsi?), jadi approval menjadi menebak-nebak.
-- Saat mitra mengubah media aktif (yang memicu status pending_review), sistem
-- kini menyimpan ringkasan perubahan per field: dari apa menjadi apa.
--
-- Dibersihkan (NULL) saat admin memutuskan (approve/reject).
--
-- Aditif: satu kolom baru. Tidak mengubah kolom yang sudah ada.

ALTER TABLE public.media
    ADD COLUMN IF NOT EXISTS pending_changes jsonb;
