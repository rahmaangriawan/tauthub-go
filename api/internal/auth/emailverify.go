package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Verifikasi email SEBELUM akun dibuat (pola resettokens.go): kode 6 digit
// disimpan sebagai sha256 hex berpepper, sekali pakai, 5 percobaan. Setelah
// kode benar, baris yang sama memegang tiket acak yang harus dibawa ke
// /auth/register; tiket dihanguskan di dalam transaksi pembuatan akun.

const (
	EmailCodeTTL        = 10 * time.Minute
	EmailCodeResendWait = 60 * time.Second
	EmailTicketTTL      = 15 * time.Minute
	EmailCodeMaxAttempt = 5
	EmailCodePurpose    = "register"
)

func GenerateEmailCode() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		// rand gagal hampir mustahil; jangan pernah mengembalikan kode tebakan.
		panic("crypto/rand gagal: " + err.Error())
	}
	s := n.String()
	return strings.Repeat("0", 6-len(s)) + s
}

func hashEmailSecret(secret, email, value string) string {
	sum := sha256.Sum256([]byte(secret + "|" + strings.ToLower(email) + "|" + value))
	return hex.EncodeToString(sum[:])
}

// LastCodeIssuedAt: waktu kode terakhir untuk email ini (cooldown kirim ulang).
func LastCodeIssuedAt(ctx context.Context, pool *pgxpool.Pool, email string) (time.Time, bool) {
	var at time.Time
	err := pool.QueryRow(ctx,
		"SELECT created_at FROM email_verification_codes WHERE email = $1 AND purpose = $2 ORDER BY created_at DESC LIMIT 1",
		strings.ToLower(email), EmailCodePurpose).Scan(&at)
	return at, err == nil
}

// CreateEmailCode: hanguskan kode lama email itu, buat kode baru. Return kode mentah.
func CreateEmailCode(ctx context.Context, pool *pgxpool.Pool, secret, email, ip string) (string, error) {
	email = strings.ToLower(email)
	code := GenerateEmailCode()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx,
		"UPDATE email_verification_codes SET consumed_at = NOW() WHERE email = $1 AND purpose = $2 AND consumed_at IS NULL",
		email, EmailCodePurpose); err != nil {
		return "", err
	}
	var ipPtr *string
	if ip != "" {
		ipPtr = &ip
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO email_verification_codes (id, email, purpose, code_hash, expires_at, request_ip) VALUES ($1, $2, $3, $4, $5, $6)",
		uuid.NewString(), email, EmailCodePurpose, hashEmailSecret(secret, email, code), time.Now().Add(EmailCodeTTL), ipPtr); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return code, nil
}

// VerifyEmailCode: cocokkan kode dengan baris aktif terbaru. Salah → attempts+1
// (≥5 hangus). Benar → verified_at + tiket. Pesan ke pengguna sengaja seragam.
func VerifyEmailCode(ctx context.Context, pool *pgxpool.Pool, secret, email, code string) (ticket string, ok bool) {
	email = strings.ToLower(email)
	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", false
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id, storedHash string
	var attempts int
	err = tx.QueryRow(ctx, `
		SELECT id, code_hash, attempts FROM email_verification_codes
		WHERE email = $1 AND purpose = $2 AND consumed_at IS NULL AND verified_at IS NULL AND expires_at > NOW()
		ORDER BY created_at DESC LIMIT 1 FOR UPDATE`, email, EmailCodePurpose).Scan(&id, &storedHash, &attempts)
	if err != nil {
		return "", false
	}
	if storedHash != hashEmailSecret(secret, email, code) {
		attempts++
		if attempts >= EmailCodeMaxAttempt {
			_, _ = tx.Exec(ctx, "UPDATE email_verification_codes SET attempts = $2, consumed_at = NOW() WHERE id = $1", id, attempts)
		} else {
			_, _ = tx.Exec(ctx, "UPDATE email_verification_codes SET attempts = $2 WHERE id = $1", id, attempts)
		}
		_ = tx.Commit(ctx)
		return "", false
	}

	buf := make([]byte, 32)
	_, _ = rand.Read(buf)
	ticket = hex.EncodeToString(buf)
	if _, err := tx.Exec(ctx,
		"UPDATE email_verification_codes SET verified_at = NOW(), ticket_hash = $2 WHERE id = $1",
		id, hashEmailSecret(secret, email, ticket)); err != nil {
		return "", false
	}
	if err := tx.Commit(ctx); err != nil {
		return "", false
	}
	return ticket, true
}

// ClaimEmailTicket: hanguskan tiket di dalam tx Register — RowsAffected harus 1.
func ClaimEmailTicket(ctx context.Context, tx pgx.Tx, secret, email, ticket string) bool {
	if ticket == "" {
		return false
	}
	email = strings.ToLower(email)
	ct, err := tx.Exec(ctx, `
		UPDATE email_verification_codes SET consumed_at = NOW()
		WHERE email = $1 AND purpose = $2 AND ticket_hash = $3 AND consumed_at IS NULL
		  AND verified_at IS NOT NULL AND verified_at > NOW() - make_interval(secs => $4)`,
		email, EmailCodePurpose, hashEmailSecret(secret, email, ticket), EmailTicketTTL.Seconds())
	return err == nil && ct.RowsAffected() == 1
}

// IsRegistrationEmailBlocked: email masuk daftar blokir (OTP tidak dijawab 48 jam).
func IsRegistrationEmailBlocked(ctx context.Context, pool *pgxpool.Pool, email string) bool {
	var one int
	err := pool.QueryRow(ctx,
		"SELECT 1 FROM registration_email_blocks WHERE email = $1 AND unblocked_at IS NULL LIMIT 1",
		strings.ToLower(email)).Scan(&one)
	return err == nil
}
