-- 166 — Invalidate active sessions and password reset tokens at deploy.
-- Explicit security reset requested for this release; existing secrets remain unchanged.

BEGIN;

UPDATE public.user_sessions
SET revoked = TRUE
WHERE revoked = FALSE;

UPDATE public.password_reset_tokens
SET consumed_at = NOW()
WHERE consumed_at IS NULL;

COMMIT;
