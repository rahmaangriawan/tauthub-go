-- Contoh query fase 0 (dipakai fase 1 auth). Konvensi: placeholder $1..$n langsung
-- (JANGAN port sistem konversi `?`->`$n` dari app lama — REWRITE/01_BACKEND_GO.md §4).

-- name: GetUserByEmail :one
SELECT id, email, password_hash, display_name, role, is_active, approved_at,
       failed_login_count, locked_until, two_factor_enabled
FROM users
WHERE email = $1;

-- name: GetUserSessionByRefreshJTI :one
SELECT id, user_id, refresh_jti, expires_at, revoked, created_at
FROM user_sessions
WHERE refresh_jti = $1;
