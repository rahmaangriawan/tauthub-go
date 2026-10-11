-- 210 — Rapikan urutan notifikasi "Apa yang Baru" yang sudah terlanjur terkirim.
--
-- Notifikasi changelog lama dibuat tanpa created_at eksplisit: satu batch impor
-- memberi waktu yang (hampir) sama, sehingga ORDER BY created_at DESC di lonceng
-- bisa menaruh versi lama di atas versi baru (mis. v0.5.0 muncul sebelum v0.5.1).
-- Penulisan ke depan sudah diperbaiki di AdminChangelogImport (created_at
-- eksplisit per entri); migrasi ini merapikan baris yang sudah ada.
--
-- Cara: untuk tiap pengguna, urutkan notifikasi changelog menurut versi (ambil
-- pola vX.Y.Z dari judul) DESC, lalu beri created_at berjarak 1 detik di bawah
-- timestamp terbaru milik pengguna itu — urutan jadi benar tanpa mengubah
-- rentang waktu historisnya.

WITH base AS (
    SELECT user_id, MAX(created_at) AS mx
      FROM notifications
     WHERE type = 'changelog.new'
     GROUP BY user_id
), ordered AS (
    SELECT n.id, n.user_id,
           row_number() OVER (
               PARTITION BY n.user_id
               ORDER BY substring(n.title from 'v[0-9]+\.[0-9]+\.[0-9]+') DESC NULLS LAST,
                        n.title
           ) AS rn
      FROM notifications n
     WHERE n.type = 'changelog.new'
)
UPDATE notifications n
SET created_at = b.mx - ((o.rn - 1) * INTERVAL '1 second')
FROM ordered o
JOIN base b ON b.user_id = o.user_id
WHERE n.id = o.id;
