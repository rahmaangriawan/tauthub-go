-- Unique index untuk dedupe notifikasi.
--
-- Tiga tempat memakai ON CONFLICT (dedupe_key), tetapi index yang ada
-- (idx_notifications_dedupe, migrasi 069) hanya index biasa, bukan unique.
-- Postgres menolak ON CONFLICT tanpa unique index yang cocok, sehingga ketiga
-- insert itu error dan error-nya ditelan pemanggil: dedup tidak pernah jalan.
--
-- Kunci dedupe harus menyertakan user_id karena dedupe_key yang sama boleh
-- dipakai dua user berbeda (mis. "withdrawal.requested:<id>" untuk admin yang
-- berbeda, atau event yang sama pada brand berbeda).
--
-- Sebelum index dibuat, baris duplikat yang sudah terlanjur masuk dibersihkan:
-- per (user_id, dedupe_key) hanya baris paling awal yang dipertahankan.
DELETE FROM public.notifications n
USING public.notifications older
WHERE n.dedupe_key IS NOT NULL
  AND older.dedupe_key IS NOT NULL
  AND n.user_id = older.user_id
  AND n.dedupe_key = older.dedupe_key
  AND (older.created_at < n.created_at
       OR (older.created_at = n.created_at AND older.id < n.id));

CREATE UNIQUE INDEX IF NOT EXISTS uq_notifications_user_dedupe
    ON public.notifications (user_id, dedupe_key)
    WHERE dedupe_key IS NOT NULL;
