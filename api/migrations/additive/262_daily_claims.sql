-- Klaim tugas harian generik.
--
-- Scheduler berjalan tiap 10-60 detik, jadi tugas yang seharusnya sekali
-- sehari perlu penanda agar tidak jalan berkali-kali. Tabel retention_runs
-- hanya bisa menampung satu tugas (day adalah primary key), jadi tabel ini
-- memisahkan nama tugas dari tanggal.
--
-- Pola pemakaian sama dengan retention_runs: INSERT ... ON CONFLICT DO NOTHING,
-- baris yang berhasil masuk berarti proses itu yang menjalankan tugas hari ini.
CREATE TABLE IF NOT EXISTS public.daily_claims (
    name character varying(64) NOT NULL,
    day date NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT daily_claims_pkey PRIMARY KEY (name, day)
);

-- Pemangkasan baris lama: klaim lebih dari 30 hari tidak berguna lagi.
CREATE INDEX IF NOT EXISTS ix_daily_claims_day ON public.daily_claims (day);
