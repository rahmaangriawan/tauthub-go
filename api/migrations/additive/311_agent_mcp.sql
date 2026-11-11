-- 311_agent_mcp.sql
-- Fondasi untuk MCP admin yang dijalankan dari Hermes Agent + Claude Code.
-- Lihat rencana di plans/ini-aku-baru-ngide-gentle-bunny.md.
--
-- Tiga perubahan aditif:
--   1. agent_clients: satu baris per instance agent (Hermes VPS, Hermes Mac
--      mini, Claude Code). Semua baca di MCP dijalankan dengan identitas
--      read_as_user_id (harus admin). Tulis selalu melewati persetujuan
--      Telegram dan dieksekusi sebagai admin yang menekan tombol Setujui.
--   2. agent_actions: ledger tunggal untuk semua aksi tulis yang diusulkan
--      agent. Status melintasi proposed → executing → executed/failed/rejected
--      /expired. UPDATE compare-and-set di status menjamin tepat-sekali.
--   3. platform_admins.telegram_user_id: pemetaan akun Telegram ke admin.

BEGIN;

CREATE TABLE IF NOT EXISTS agent_clients (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name               TEXT NOT NULL,
    token_hash         TEXT NOT NULL UNIQUE,
    -- users.id adalah CHAR(36) (ULID/UUID sebagai string), bukan UUID.
    read_as_user_id    CHAR(36) NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at       TIMESTAMPTZ,
    revoked_at         TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS agent_clients_active_idx
    ON agent_clients (revoked_at) WHERE revoked_at IS NULL;

CREATE TABLE IF NOT EXISTS agent_actions (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code                 TEXT NOT NULL UNIQUE,
    client_id            UUID REFERENCES agent_clients(id) ON DELETE SET NULL,
    tool                 TEXT NOT NULL,
    params               JSONB NOT NULL,
    summary              TEXT NOT NULL,
    required_permission  TEXT NOT NULL DEFAULT '',
    status               TEXT NOT NULL DEFAULT 'proposed'
        CHECK (status IN ('proposed','executing','executed','failed','rejected','expired')),
    approved_by          CHAR(36) REFERENCES users(id) ON DELETE SET NULL,
    telegram_chat_id     BIGINT,
    telegram_message_id  BIGINT,
    result               JSONB,
    error                TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at           TIMESTAMPTZ,
    executed_at          TIMESTAMPTZ,
    expires_at           TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS agent_actions_status_idx
    ON agent_actions (status, created_at DESC);
CREATE INDEX IF NOT EXISTS agent_actions_client_idx
    ON agent_actions (client_id, created_at DESC);
-- Pencarian via tombol inline Telegram (callback_data memuat kode pendek).
CREATE INDEX IF NOT EXISTS agent_actions_message_idx
    ON agent_actions (telegram_chat_id, telegram_message_id);

ALTER TABLE platform_admins
    ADD COLUMN IF NOT EXISTS telegram_user_id BIGINT;

CREATE UNIQUE INDEX IF NOT EXISTS platform_admins_telegram_user_id_key
    ON platform_admins (telegram_user_id) WHERE telegram_user_id IS NOT NULL;

COMMIT;
