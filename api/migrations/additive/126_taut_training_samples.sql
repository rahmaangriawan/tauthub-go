-- 126 — Taut Pintar: gudang data training AI (input user → output agent).
-- Setiap run agent (sukses/gagal) menyimpan pasangan input→output sebagai bahan
-- bacaan RAG platform. Tidak menyimpan secret; teks dipotong agar hemat.

BEGIN;

CREATE TABLE IF NOT EXISTS public.taut_training_samples (
  id varchar(36) PRIMARY KEY,
  campaign_id varchar(36) NULL,
  media_id varchar(36) NULL,
  agent varchar(32) NOT NULL,
  status varchar(16) NOT NULL,              -- done | failed
  input_text text NULL,                     -- prompt/ringkasan input user
  output_text text NULL,                    -- hasil agent / pesan error
  model varchar(120) NULL,
  token_input integer NULL,
  token_output integer NULL,
  created_at timestamptz NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_taut_training_agent
  ON public.taut_training_samples (agent, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_taut_training_campaign
  ON public.taut_training_samples (campaign_id);

COMMIT;
