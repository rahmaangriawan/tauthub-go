-- 116 -- Tabel sesi login untuk revocability access token (cek sid di auth/http.ts).
-- Idempoten: DB yang dibuat dari schema.pg.sql penuh sudah punya tabel ini.
CREATE TABLE IF NOT EXISTS public.user_sessions (
  id          character(36) NOT NULL,
  user_id     character(36) NOT NULL,
  refresh_jti character(36) NOT NULL,
  expires_at  timestamp(6) with time zone NOT NULL,
  revoked     boolean DEFAULT false NOT NULL,
  created_at  timestamp(6) with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS user_sessions_pkey ON public.user_sessions (id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_sessions_refresh_jti ON public.user_sessions (refresh_jti);
CREATE INDEX IF NOT EXISTS ix_user_sessions_user ON public.user_sessions (user_id);
