-- Backfill notifikasi pengumuman yang belum sampai ke penerimanya.
--
-- Sebab: fan-out `announcement.new` baru aktif setelah fitur pengumuman rilis,
-- sehingga pengumuman yang dibuat sebelum itu tidak punya baris notifikasi
-- (akun admin bahkan nol, karena admin baru ikut disertakan kemudian). Akibatnya
-- tab "Pengumuman" di riwayat notifikasi admin kosong.
--
-- Idempoten: baris yang sudah ada dikenali dari pasangan user + judul notifikasi,
-- jadi pengumuman yang tadinya hanya sampai ke klien/mitra tidak digandakan.
-- created_at disamakan dengan waktu pengumuman dibuat supaya urutan riwayat benar.

INSERT INTO public.notifications (id, user_id, type, title, body, created_at)
SELECT gen_random_uuid()::text,
       u.id,
       'announcement.new',
       'Pengumuman: ' || a.title,
       left(a.body, 500),
       a.created_at
FROM public.announcements a
JOIN public.users u
  ON u.is_active = TRUE
 AND ((a.audience = 'all' AND u.role IN ('client','partner','admin')) OR u.role = a.audience)
WHERE NOT EXISTS (
  SELECT 1
  FROM public.notifications n
  WHERE n.user_id = u.id
    AND n.type = 'announcement.new'
    AND n.title = 'Pengumuman: ' || a.title
);
