-- Jadwal percobaan ulang untuk antrean WhatsApp.
--
-- Sebelumnya baris yang gagal retryable langsung dikembalikan ke status
-- 'queued' tanpa jeda, sehingga scheduler (tiap 10 detik di produksi) bisa
-- menembak gateway yang sama berkali-kali dalam hitungan detik dan menghabiskan
-- kuota Fonnte untuk pesan yang jelas-jelas ditolak.
--
-- next_attempt_at memberi jarak antar percobaan, mengikuti pola yang sudah
-- dipakai email_deliveries.
ALTER TABLE public.wa_deliveries
    ADD COLUMN IF NOT EXISTS next_attempt_at timestamp with time zone;

-- Waktu baris diklaim oleh satu job pengirim. Dipakai untuk mengembalikan
-- baris 'sending' yang pemiliknya mati di tengah pengiriman (deploy/restart),
-- supaya tidak tertahan selamanya. Tanpa kolom ini, klaim status akan membuat
-- pesan hilang diam-diam.
ALTER TABLE public.wa_deliveries
    ADD COLUMN IF NOT EXISTS claimed_at timestamp with time zone;

-- Baris lama tidak punya jadwal; anggap siap kirim sekarang.
UPDATE public.wa_deliveries
   SET next_attempt_at = now()
 WHERE next_attempt_at IS NULL;

-- Pemungutan antrean selalu "yang siap kirim, terlama dulu".
CREATE INDEX IF NOT EXISTS ix_wa_deliveries_due
    ON public.wa_deliveries (status, next_attempt_at)
    WHERE status = 'queued';

-- Pencarian baris 'sending' yang macet.
CREATE INDEX IF NOT EXISTS ix_wa_deliveries_sending
    ON public.wa_deliveries (claimed_at)
    WHERE status = 'sending';
