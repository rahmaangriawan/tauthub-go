package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Port lib/auth/reset-tokens.ts — token reset password: raw 32 byte hex,
// disimpan sebagai sha256 hex, sekali pakai, konsumsi token lama saat buat baru.

type ResetTokenResult struct {
	Valid   bool
	UserID  string
	TokenID string
}

func GenerateResetToken() (raw, hash string) {
	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	raw = hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(sum[:])
}

// CreateResetToken: FOR UPDATE user, konsumsi token lama, insert token baru.
func CreateResetToken(ctx context.Context, pool *pgxpool.Pool, userID string, expiresInMinutes int) (string, error) {
	if expiresInMinutes <= 0 {
		return "", fmt.Errorf("Reset token lifetime must be a positive number of minutes")
	}
	raw, hash := GenerateResetToken()
	id := uuid.NewString()
	expiresAt := time.Now().Add(time.Duration(expiresInMinutes) * time.Minute)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, "SELECT id FROM users WHERE id = $1 FOR UPDATE", userID); err != nil {
		_ = tx.Rollback(ctx)
		return "", err
	}
	if _, err := tx.Exec(ctx,
		"UPDATE password_reset_tokens SET consumed_at = NOW() WHERE user_id = $1 AND consumed_at IS NULL", userID); err != nil {
		_ = tx.Rollback(ctx)
		return "", err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)",
		id, userID, hash, expiresAt); err != nil {
		_ = tx.Rollback(ctx)
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return raw, nil
}

// VerifyResetToken: hash → cari token aktif.
func VerifyResetToken(ctx context.Context, pool *pgxpool.Pool, rawToken string) ResetTokenResult {
	sum := sha256.Sum256([]byte(rawToken))
	hash := hex.EncodeToString(sum[:])
	var id, userID string
	err := pool.QueryRow(ctx,
		"SELECT id, user_id FROM password_reset_tokens WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > NOW() LIMIT 1",
		hash).Scan(&id, &userID)
	if err != nil {
		return ResetTokenResult{Valid: false}
	}
	return ResetTokenResult{Valid: true, UserID: userID, TokenID: id}
}

// ClaimResetToken: konsumsi atomik — rowCount harus 1.
func ClaimResetToken(ctx context.Context, tx pgx.Tx, tokenID string) bool {
	ct, err := tx.Exec(ctx,
		"UPDATE password_reset_tokens SET consumed_at = NOW() WHERE id = $1 AND consumed_at IS NULL AND expires_at > NOW()",
		tokenID)
	return err == nil && ct.RowsAffected() == 1
}
