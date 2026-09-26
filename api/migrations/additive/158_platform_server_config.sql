-- 158: Konfigurasi server (IP publik) — diatur dari dashboard admin.
--
-- Sebelumnya IP publik server hanya bisa diisi lewat env SERVER_PUBLIC_IP. Itu
-- berarti rahasia infrastruktur tertulis di berkas env dan menyebar ke setiap
-- salinan (VPS, CI, laptop). Sekarang disimpan di DB dengan enkripsi at-rest,
-- diatur dari halaman admin, dan env hanya jadi nilai AWAL kalau baris ini kosong.
--
-- `sealed_public_ip` memakai helper seal/unseal yang sudah dipakai kredensial
-- CMS (kunci: CMS_ENCRYPTION_SECRET / APP_SECRET), jadi tidak ada kunci baru.
--
-- Aditif: satu tabel baru. Tidak mengubah tabel/kolom yang sudah ada.

CREATE TABLE IF NOT EXISTS platform_server_config (
  id               character varying(36) PRIMARY KEY,
  sealed_public_ip text,
  updated_by       character varying(36),
  updated_at       timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP
);
