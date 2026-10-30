-- 289 — Tagih kredit publikasi yang sudah terbit tanpa pernah di-capture.
--
-- Penyebab: pesanan masuk antrean publikasi SEBELUM saldo ditahan. Pada media
-- auto-publish, artikel bisa terbit dalam hitungan detik — sebelum reserve
-- tercatat. Saat captureCredit berjalan, outstanding masih 0 sehingga capture
-- dilewati tanpa suara; reserve baru ditulis belakangan setelah klien topup,
-- dan tidak ada yang memicu capture lagi.
--
-- Perbaikan urutan sudah dilakukan di kode (reserve dijalankan di dalam
-- createPublicationRecord, sebelum enqueue). Migrasi ini membersihkan
-- publikasi yang sudah telanjur bocor: artikelnya terbit, saldo belum ditagih.
--
-- Nominal diambil dari reservasi yang benar-benar ada (bukan angka tetap), dan
-- idempotency key memakai pola yang sama dengan captureCredit sehingga
-- menjalankan ulang tidak menggandakan tagihan.
--
-- Ditulis satu-per-satu (bukan CTE gabungan) supaya setiap publikasi pasti
-- diproses: CTE dengan UPDATE ... FROM hanya mengembalikan baris yang ikut
-- ter-update, dan satu baris bisa terlewat tanpa jejak.

DO $$
DECLARE
  rec       RECORD;
  v_rate    BIGINT;
  v_balance BIGINT;
BEGIN
  SELECT rate_idr INTO v_rate FROM tc_config WHERE id = 1;
  IF v_rate IS NULL THEN
    v_rate := 1;
  END IF;

  FOR rec IN
    SELECT p.id AS publication_id, p.brand_id,
           (-SUM(l.amount_tc))::BIGINT AS outstanding
    FROM publications p
    JOIN credit_ledger_entries l
      ON l.ref_id = p.id AND l.ref_type IN ('reserve','release','capture')
    WHERE p.status = 'published'
      AND p.id IN (
        '84a1465f-14ee-41d3-8a88-16abad45130d',
        'c9b9b2df-7c9f-422f-bdbf-828966837767'
      )
      AND NOT EXISTS (
        SELECT 1 FROM credit_ledger_entries c
        WHERE c.ref_id = p.id AND c.ref_type = 'capture'
      )
    GROUP BY p.id, p.brand_id
    HAVING -SUM(l.amount_tc) > 0
  LOOP
    -- Pindahkan reserved → captured pada wallet yang bersangkutan.
    UPDATE credit_wallets
       SET reserved_tc = GREATEST(reserved_tc - rec.outstanding, 0),
           captured_tc = captured_tc + rec.outstanding,
           updated_at  = NOW()
     WHERE brand_id = rec.brand_id;

    SELECT available_tc + reserved_tc + captured_tc
      INTO v_balance
      FROM credit_wallets WHERE brand_id = rec.brand_id;

    INSERT INTO credit_ledger_entries
      (id, brand_id, ref_type, ref_id, amount_tc, balance_after_tc, rate_idr,
       amount_idr, reason, idempotency_key, created_by)
    VALUES (
      gen_random_uuid()::text, rec.brand_id, 'capture', rec.publication_id,
      rec.outstanding, v_balance, v_rate, rec.outstanding * v_rate,
      'Kredit publikasi ditagihkan (artikel terbit) — koreksi tagihan yang terlewat',
      'capture:' || rec.publication_id, 'system'
    )
    ON CONFLICT (idempotency_key) DO NOTHING;
  END LOOP;
END $$;
