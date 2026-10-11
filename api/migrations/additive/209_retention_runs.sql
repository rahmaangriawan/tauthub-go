-- 209 — Buku besar pelacak eksekusi retensi.
--
-- Job retensi harian (brandflow/retention.go) klaim hari ini lewat INSERT ON
-- CONFLICT DO NOTHING: proses pertama yang berhasil klaim menjalankan
-- pemangkasan, proses lain (API + scheduler berjalan berdampingan) melewati
-- hari itu. Barisnya juga menjadi jejak audit kapan retensi terakhir dijalankan
-- beserta statistik jumlah baris yang dipangkas/diarsipkan.

CREATE TABLE IF NOT EXISTS public.retention_runs (
    day date PRIMARY KEY,
    stats jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
