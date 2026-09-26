-- 087 — Password reset tokens (hashed, 1h TTL). Additive.

BEGIN;

CREATE TABLE IF NOT EXISTS public.password_reset_tokens (
  id character(36) PRIMARY KEY,
  user_id character(36) NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
  token_hash character(64) NOT NULL UNIQUE,
  expires_at timestamp with time zone NOT NULL,
  consumed_at timestamp with time zone,
  created_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_password_reset_tokens_user
  ON public.password_reset_tokens (user_id, expires_at DESC);

COMMIT;
