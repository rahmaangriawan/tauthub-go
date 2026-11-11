-- 312_mcp_config.sql
-- Pindahkan konfigurasi MCP dari .env ke tabel yang dapat diedit lewat
-- dashboard admin. Service key dan token bot Telegram tersegel (pola sama
-- dengan platform_wa_config / payment_gateway_configs); kolom *_hint
-- menyimpan potongan terakhir agar UI dapat menampilkannya tanpa membuka
-- seal.
--
-- Hanya satu baris (id='default') — konfigurasi ini global.

BEGIN;

CREATE TABLE IF NOT EXISTS mcp_config (
    id                         TEXT PRIMARY KEY DEFAULT 'default' CHECK (id = 'default'),
    sealed_service_key         TEXT,
    service_key_hint           TEXT,
    sealed_telegram_bot_token  TEXT,
    telegram_bot_hint          TEXT,
    telegram_chat_ids          BIGINT[] NOT NULL DEFAULT '{}'::bigint[],
    writes_enabled             BOOLEAN  NOT NULL DEFAULT TRUE,
    max_credit_adjust_tc       BIGINT   NOT NULL DEFAULT 50000000,
    updated_by                 UUID REFERENCES users(id),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO mcp_config (id) VALUES ('default') ON CONFLICT DO NOTHING;

COMMIT;
