-- Index untuk gate "sudah dibaca di in-app".
--
-- Sebelum email/WA berbayar dikirim, kita cek apakah notifikasi in-app dengan
-- tipe yang sama sudah dibaca user dalam jendela waktu tertentu. Tanpa index
-- ini query itu memindai seluruh riwayat notifikasi user.
--
-- Partial index pada read_at IS NOT NULL karena baris yang belum dibaca tidak
-- pernah memenuhi syarat gate, jadi tidak perlu ikut disimpan.
CREATE INDEX IF NOT EXISTS ix_notifications_gate
    ON public.notifications (user_id, type, created_at DESC)
    WHERE read_at IS NOT NULL;
