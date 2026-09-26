-- Revenue Share default split diubah 65:35 → 75:25 (TULISIO:MITRA).
--
-- Semantik baru: bagi hasil diterapkan ke MARKUP Tulisio
-- (client_price − partner_price), BUKAN dari harga klien. Mitra selalu
-- menerima harga base-nya penuh + bagian mitra dari markup.
-- Default 75:25 hanya fallback — API selalu mengirim persen eksplisit
-- (zod default 75/25), jadi kolom default ini nyaris tidak terpakai,
-- tapi diselaraskan supaya konsisten.
--
-- Additive & backward compatible: tidak menyentuh baris existing,
-- tidak mengubah data rule yang sudah dibuat admin.
-- Idempotent: SET DEFAULT aman dijalankan berulang.

ALTER TABLE public.revenue_shares
    ALTER COLUMN tulisio_percent SET DEFAULT 75;

ALTER TABLE public.revenue_shares
    ALTER COLUMN partner_percent SET DEFAULT 25;
