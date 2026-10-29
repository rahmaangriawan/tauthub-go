-- Stuck WA recovery: kembalikan baris 'sending' yang pemiliknya mati.
--
-- Klaim antrean sekarang menandai baris 'sending' sebelum gateway dipanggil.
-- Kalau proses mati di tengah pengiriman (deploy/restart), baris itu tidak akan
-- pernah kembali ke 'queued' dan pesannya hilang tanpa jejak. Fungsi di bawah
-- mengembalikannya setelah 10 menit, sama seperti reaper job di jobs/runner.go.
--
-- Tidak dipanggil otomatis dari SQL; pemanggilnya ada di kode Go.
CREATE OR REPLACE FUNCTION public.wa_release_stuck(stale_after interval)
RETURNS integer
LANGUAGE plpgsql
AS $$
DECLARE
    affected integer;
BEGIN
    UPDATE public.wa_deliveries
       SET status = 'queued',
           claimed_at = NULL,
           next_attempt_at = now()
     WHERE status = 'sending'
       AND claimed_at IS NOT NULL
       AND claimed_at < now() - stale_after;
    GET DIAGNOSTICS affected = ROW_COUNT;
    RETURN affected;
END;
$$;
