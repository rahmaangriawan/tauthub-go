-- Migration: 045_notification_expiry
-- Notifikasi publikasi: tambah referensi publikasi (deep-link presisi + expiry
-- per media) dan penanda kedaluwarsa (expired_at).
--
-- expired_at diisi otomatis saat media mitra diarsipkan/dilepas sehingga
-- notifikasi publication.waiting_partner yang sudah tidak relevan ditandai
-- kedaluwarsa (tidak dihitung sebagai unread, tampil dengan badge di UI).
--
-- Apply with: psql $DATABASE_URL -f apps/web/src/db/migrations/045_notification_expiry.sql

BEGIN;

ALTER TABLE public.notifications
  ADD COLUMN IF NOT EXISTS publication_id VARCHAR(36),
  ADD COLUMN IF NOT EXISTS expired_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS ix_notifications_publication
  ON public.notifications (publication_id);

CREATE INDEX IF NOT EXISTS ix_notifications_user_read_expired
  ON public.notifications (user_id, expired_at, read_at, created_at DESC);

COMMIT;
