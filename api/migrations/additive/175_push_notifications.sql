-- 175_push_notifications.sql
-- Antrean dan langganan Web Push per pengguna (desktop & mobile).
-- Idempoten: aman dijalankan berulang kali.

CREATE TABLE IF NOT EXISTS push_subscriptions (
    id text PRIMARY KEY DEFAULT gen_random_uuid()::text,
    user_id char(36) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint text NOT NULL UNIQUE,
    p256dh text NOT NULL,
    auth text NOT NULL,
    user_agent text,
    created_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_push_subs_user ON push_subscriptions(user_id);

CREATE TABLE IF NOT EXISTS push_deliveries (
    id text PRIMARY KEY DEFAULT gen_random_uuid()::text,
    notification_id char(36) NOT NULL REFERENCES notifications(id) ON DELETE CASCADE,
    subscription_id text NOT NULL REFERENCES push_subscriptions(id) ON DELETE CASCADE,
    status varchar(32) NOT NULL DEFAULT 'pending',
    attempts integer NOT NULL DEFAULT 0,
    max_attempts integer NOT NULL DEFAULT 3,
    run_at timestamptz NOT NULL DEFAULT now(),
    locked_at timestamptz,
    locked_by varchar(64),
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_push_deliveries_pending ON push_deliveries(status, run_at) WHERE status IN ('pending', 'failed');
CREATE INDEX IF NOT EXISTS idx_push_deliveries_sub ON push_deliveries(subscription_id);

-- Trigger fn: setiap ada baris baru di notifications, buat entri antrean
-- untuk setiap perangkat terdaftar milik pengguna tersebut.
CREATE OR REPLACE FUNCTION fn_enqueue_push_deliveries()
RETURNS trigger AS $$
BEGIN
    -- Masukkan antrean push untuk seluruh langganan aktif pengguna ini.
    INSERT INTO push_deliveries (id, notification_id, subscription_id, status, attempts, max_attempts, run_at, created_at, updated_at)
    SELECT
        gen_random_uuid()::text,
        NEW.id,
        s.id,
        'pending',
        0,
        3,
        now(),
        now(),
        now()
    FROM push_subscriptions s
    WHERE s.user_id = NEW.user_id;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_enqueue_push_deliveries ON notifications;
CREATE TRIGGER trg_enqueue_push_deliveries
    AFTER INSERT ON notifications
    FOR EACH ROW
    EXECUTE FUNCTION fn_enqueue_push_deliveries();
