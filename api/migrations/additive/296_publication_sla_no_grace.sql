-- 296 — SLA 3×24 jam tanpa masa perpanjangan.
--
-- Kebijakan berubah: pesanan `waiting_partner` yang melewati
-- COALESCE(scheduled_at, created_at) + 3 hari LANGSUNG dibatalkan otomatis.
-- Tidak ada lagi perpanjangan 1 hari (keputusan bulat pemilik produk).
--
-- Scheduler (schSweepPublicationSLA) sudah disesuaikan: fase perpanjangan
-- dihapus dan ambang pembatalan memakai +3 hari. Migrasi ini hanya merapikan
-- teks alasan yang terlanjur tersimpan dengan frasa perpanjangan supaya
-- riwayat pesanan tidak menampilkan kebijakan yang sudah tidak berlaku.
--
-- Tipe notifikasi `publication.sla_extended` tidak dikirim lagi. Nilainya
-- dibiarkan ada di constraint notifications agar baris historis tetap valid.

UPDATE public.publications
SET cancel_reason = replace(cancel_reason, ' (termasuk 1 hari perpanjangan)', '')
WHERE cancel_reason LIKE '%(termasuk 1 hari perpanjangan)%';
