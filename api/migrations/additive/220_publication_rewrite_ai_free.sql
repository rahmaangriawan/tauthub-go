-- Rewrite AI mitra kini GRATIS dengan batas maksimal 2 kali generate per
-- artikel (aturan pemilik produk). Penghitungnya disimpan di publikasi supaya
-- tidak bisa diperkelirukan dengan refresh halaman.
ALTER TABLE public.publications ADD COLUMN IF NOT EXISTS rewrite_ai_count integer NOT NULL DEFAULT 0;
