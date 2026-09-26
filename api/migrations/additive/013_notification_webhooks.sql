-- 013_notification_webhooks.sql
-- Webhook endpoint milik user (klien / mitra) untuk menerima callback event
-- publikasi selain notifikasi in-app dan email. Additive.
-- =====================================================================

CREATE TABLE IF NOT EXISTS public.notification_webhooks (
    id character(36) NOT NULL,
    user_id character(36) NOT NULL,
    url character varying(500) NOT NULL,
    secret character varying(200),
    active boolean DEFAULT true NOT NULL,
    created_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT notification_webhooks_pkey PRIMARY KEY (id),
    CONSTRAINT fk_notification_webhooks_user FOREIGN KEY (user_id) REFERENCES public.users (id),
    CONSTRAINT ck_notification_webhooks_url CHECK (((url)::text ~* '^https?://'::text))
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_notification_webhook_user_url
    ON public.notification_webhooks (user_id, url);

CREATE INDEX IF NOT EXISTS ix_notification_webhooks_active
    ON public.notification_webhooks (active);
