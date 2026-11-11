// Package mcpconfig memuat dan menyimpan konfigurasi MCP admin dari tabel
// mcp_config. Pola sama dengan platform_wa_config: secret tersegel memakai
// CMS_ENCRYPTION_SECRET (fallback APP_SECRET), dibaca saat boot, dan
// di-reload bila ada pg_notify('mcp_config_changed').
//
// Dipakai dari dua binary:
//   - cmd/api — untuk mengisi Authenticator.ServiceKeyProvider.
//   - cmd/mcp — untuk server MCP + bot Telegram + listener.
//
// Keduanya memegang *Store yang sama bentuknya; proses yang berbeda
// masing-masing menjalankan ListenForChanges-nya sendiri.
package mcpconfig

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rahmaangriawan/tauthub-go/internal/crypto"
)

// Snapshot: salinan nilai konfigurasi yang aman dibaca bersama (immutable
// setelah dibuat).
type Snapshot struct {
	ServiceKey         string
	ServiceKeyHint     string
	TelegramBotToken   string
	TelegramBotHint    string
	TelegramChatIDs    []int64
	WritesEnabled      bool
	MaxCreditAdjustTC  int64
	UpdatedAt          time.Time
}

// Store: wadah snapshot dengan RWMutex. Metode baca cepat (lock baca),
// metode tulis mengganti pointer snapshot.
type Store struct {
	pool          *pgxpool.Pool
	appSecret     string
	cmsSecret     string
	mu            sync.RWMutex
	snap          Snapshot
	logger        *slog.Logger
	disabledWrite bool // true untuk Store readonly (cmd/mcp tidak butuh tulis)
}

// New: buat store dan muat snapshot awal dari DB.
func New(ctx context.Context, pool *pgxpool.Pool, appSecret, cmsSecret string) (*Store, error) {
	s := &Store{
		pool:      pool,
		appSecret: appSecret,
		cmsSecret: cmsSecret,
		logger:    slog.Default().With("mod", "mcpconfig"),
	}
	if err := s.Reload(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// Snapshot: ambil snapshot aktif (salin).
func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
}

// ServiceKey: getter cepat untuk Authenticator.ServiceKeyProvider.
func (s *Store) ServiceKey() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap.ServiceKey
}

// Reload: baca ulang dari DB. Aman dipanggil dari goroutine mana pun.
func (s *Store) Reload(ctx context.Context) error {
	row := s.pool.QueryRow(ctx, `
		SELECT COALESCE(sealed_service_key, ''), COALESCE(service_key_hint, ''),
		       COALESCE(sealed_telegram_bot_token, ''), COALESCE(telegram_bot_hint, ''),
		       COALESCE(telegram_chat_ids, '{}'::bigint[]),
		       writes_enabled, max_credit_adjust_tc, updated_at
		FROM mcp_config WHERE id = 'default' LIMIT 1`)
	var sealedKey, keyHint, sealedTok, tokHint string
	var chatIDs []int64
	var writes bool
	var maxAdj int64
	var updatedAt time.Time
	if err := row.Scan(&sealedKey, &keyHint, &sealedTok, &tokHint, &chatIDs, &writes, &maxAdj, &updatedAt); err != nil {
		// Baris belum ada: diamkan dengan default. Migrasi 312 menyisipkan
		// 'default', tapi bila DB masih terlampau lama, berfungsi tanpa MCP.
		s.mu.Lock()
		s.snap = Snapshot{WritesEnabled: true, MaxCreditAdjustTC: 50_000_000}
		s.mu.Unlock()
		return nil
	}
	svc, err := s.unsealMaybe(sealedKey)
	if err != nil {
		s.logger.Warn("unseal service key gagal", "err", err)
	}
	tok, err := s.unsealMaybe(sealedTok)
	if err != nil {
		s.logger.Warn("unseal telegram token gagal", "err", err)
	}
	snap := Snapshot{
		ServiceKey:        svc,
		ServiceKeyHint:    keyHint,
		TelegramBotToken:  tok,
		TelegramBotHint:   tokHint,
		TelegramChatIDs:   chatIDs,
		WritesEnabled:     writes,
		MaxCreditAdjustTC: maxAdj,
		UpdatedAt:         updatedAt,
	}
	s.mu.Lock()
	s.snap = snap
	s.mu.Unlock()
	return nil
}

func (s *Store) unsealMaybe(sealed string) (string, error) {
	if strings.TrimSpace(sealed) == "" {
		return "", nil
	}
	return crypto.UnsealCompat(sealed, s.cmsSecret, s.appSecret)
}

// Seal: enkripsi nilai baru dipakai handler PUT sebelum simpan.
func (s *Store) Seal(plain string) (string, error) {
	secret := s.cmsSecret
	if secret == "" {
		secret = s.appSecret
	}
	return crypto.Seal(plain, secret)
}

// ListenForChanges: subscribe pg_notify('mcp_config_changed') dan Reload
// setiap kali tiba. Reconnect otomatis. Dipanggil cmd/api dan cmd/mcp di
// goroutine terpisah.
func (s *Store) ListenForChanges(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := s.listenOnce(ctx); err != nil && ctx.Err() == nil {
			s.logger.Warn("listen gagal, reconnect 5s", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (s *Store) listenOnce(ctx context.Context) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "LISTEN mcp_config_changed"); err != nil {
		return err
	}
	for {
		notif, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return err
		}
		_ = notif
		if err := s.Reload(ctx); err != nil {
			s.logger.Warn("reload gagal", "err", err)
		} else {
			s.logger.Info("konfigurasi MCP di-reload")
		}
	}
}
