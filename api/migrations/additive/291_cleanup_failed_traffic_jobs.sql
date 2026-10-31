-- 291 — Bersihkan job cek trafik yang gagal karena kredensial kosong.
--
-- Latar: provider Similarweb API Pro menolak semua permintaan tanpa DataBee
-- Token, sementara sweep scheduler tetap meng-enqueue job tiap siklus. Hasilnya
-- 184 baris `failed` dengan pesan identik. Baris-baris itu:
--
--   - tidak bisa berhasil bila di-retry (penyebabnya konfigurasi, bukan
--     gangguan sesaat),
--   - menutupi kegagalan baru di indikator admin,
--   - memakan ruang tabel jobs yang seharusnya untuk riwayat berguna.
--
-- Yang dihapus HANYA job yang gagal karena pesan kredensial tersebut. Job
-- `failed` dengan sebab lain (domain mati, timeout penyedia) dibiarkan supaya
-- masih bisa ditelusuri. Retensi harian tetap memangkas job selesai/gagal
-- berumur >3 hari, jadi tidak perlu penanganan khusus setelah ini.
DELETE FROM public.jobs
WHERE kind = 'media_similarweb_refresh'
  AND status = 'failed'
  AND (
        last_error ILIKE '%DataBee Token%'
     OR last_error ILIKE '%Similarweb API Pro mewajibkan%'
     OR last_error ILIKE '%belum dikonfigurasi%'
  );

-- Sisa job trafik yang masih `failed` setelah pembersihan di atas dijadwalkan
-- ulang oleh sweep berikutnya (cooldown 24 jam di query sweep tetap berlaku),
-- jadi tidak ada langkah manual tambahan.
