package mcpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ActionDraft: data yang dikirim tool tulis saat menyiapkan aksi. Belum
// dieksekusi; menunggu tombol "Setujui" di bot Persetujuan.
type ActionDraft struct {
	Tool               string
	Summary            string // Ringkasan dalam bahasa Indonesia untuk kartu Telegram.
	RequiredPermission string // Kosong = semua admin boleh menyetujui.
	Params             any    // Payload yang disimpan dan dipakai saat eksekusi.
	// TTLSeconds: usia maksimum aksi sebelum otomatis expired. Default 15 menit
	// (dikonfigurasi di Propose).
	TTLSeconds int
}

// Action: baris agent_actions yang sudah dipersist.
type Action struct {
	ID                 string
	Code               string
	ClientID           string
	Tool               string
	Params             json.RawMessage
	Summary            string
	RequiredPermission string
	Status             string
	ApprovedBy         *string
	TelegramChatID     *int64
	TelegramMessageID  *int64
	Result             json.RawMessage
	ErrorMsg           string
	CreatedAt          time.Time
	DecidedAt          *time.Time
	ExecutedAt         *time.Time
	ExpiresAt          time.Time
}

// Propose: simpan draft ke agent_actions dengan status 'proposed'. Caller
// lalu mengirim kartu Telegram dan mencatat telegram_message_id lewat
// BindTelegramMessage. Kode pendek dikembalikan untuk ditempel di callback
// data (Telegram membatasi 64 byte, UUID terlalu panjang).
func (s *Server) Propose(ctx context.Context, client *AgentClient, draft ActionDraft) (*Action, error) {
	if !s.MCPConfig.Snapshot().WritesEnabled {
		return nil, errors.New("tool tulis dinonaktifkan di /admin/konfigurasi/mcp")
	}
	ttl := draft.TTLSeconds
	if ttl <= 0 {
		ttl = 15 * 60
	}
	paramsJSON, err := json.Marshal(draft.Params)
	if err != nil {
		return nil, fmt.Errorf("encode params: %w", err)
	}
	code, err := generateActionCode()
	if err != nil {
		return nil, err
	}
	var id string
	err = s.Pool.QueryRow(ctx, `
		INSERT INTO agent_actions (code, client_id, tool, params, summary, required_permission, expires_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6, NOW() + make_interval(secs => $7))
		RETURNING id`,
		code, client.ID, draft.Tool, string(paramsJSON), draft.Summary, draft.RequiredPermission, ttl,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert agent_actions: %w", err)
	}
	return &Action{
		ID:                 id,
		Code:               code,
		ClientID:           client.ID,
		Tool:               draft.Tool,
		Params:             paramsJSON,
		Summary:            draft.Summary,
		RequiredPermission: draft.RequiredPermission,
		Status:             "proposed",
		ExpiresAt:          time.Now().Add(time.Duration(ttl) * time.Second),
	}, nil
}

// BindTelegramMessage: catat message_id kartu Persetujuan setelah terkirim,
// supaya bot nanti bisa menyunting pesan ketika admin menekan tombol.
func (s *Server) BindTelegramMessage(ctx context.Context, actionID string, chatID, messageID int64) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE agent_actions SET telegram_chat_id=$1, telegram_message_id=$2 WHERE id=$3`,
		chatID, messageID, actionID)
	return err
}

// LookupByCode: cari aksi berdasarkan kode pendek (dipakai saat handler
// callback Telegram).
func (s *Server) LookupByCode(ctx context.Context, code string) (*Action, error) {
	row := s.Pool.QueryRow(ctx, `
		SELECT id, code, COALESCE(client_id::text, ''), tool, params::text, summary, required_permission,
		       status, approved_by::text, telegram_chat_id, telegram_message_id,
		       COALESCE(result::text, ''), COALESCE(error, ''),
		       created_at, decided_at, executed_at, expires_at
		FROM agent_actions WHERE code = $1 LIMIT 1`, code)
	var a Action
	var paramsStr, resultStr, approvedBy string
	err := row.Scan(&a.ID, &a.Code, &a.ClientID, &a.Tool, &paramsStr, &a.Summary, &a.RequiredPermission,
		&a.Status, &approvedBy, &a.TelegramChatID, &a.TelegramMessageID,
		&resultStr, &a.ErrorMsg, &a.CreatedAt, &a.DecidedAt, &a.ExecutedAt, &a.ExpiresAt)
	if err != nil {
		return nil, err
	}
	a.Params = json.RawMessage(paramsStr)
	if resultStr != "" {
		a.Result = json.RawMessage(resultStr)
	}
	if approvedBy != "" {
		a.ApprovedBy = &approvedBy
	}
	return &a, nil
}

// ClaimForExecution: compare-and-set status → 'executing'. Hanya satu
// panggilan yang akan berhasil pindah dari 'proposed'; sisanya menerima
// errAlreadyDecided. Ini kunci eksekusi tepat sekali walaupun tombol
// Telegram ditekan berulang atau beberapa admin menekan bersamaan.
func (s *Server) ClaimForExecution(ctx context.Context, actionID, approverUserID string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE agent_actions
		SET status='executing', approved_by=$2, decided_at=NOW()
		WHERE id=$1 AND status='proposed' AND expires_at > NOW()`,
		actionID, approverUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAlreadyDecided
	}
	return nil
}

// MarkExecuted: dipanggil setelah API memberi respons sukses.
func (s *Server) MarkExecuted(ctx context.Context, actionID string, result any) error {
	buf, _ := json.Marshal(result)
	_, err := s.Pool.Exec(ctx,
		`UPDATE agent_actions SET status='executed', executed_at=NOW(), result=$2::jsonb WHERE id=$1`,
		actionID, string(buf))
	return err
}

// MarkFailed: dipanggil saat pemanggilan API mengembalikan error.
func (s *Server) MarkFailed(ctx context.Context, actionID string, errMsg string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE agent_actions SET status='failed', executed_at=NOW(), error=$2 WHERE id=$1`,
		actionID, errMsg)
	return err
}

// Reject: dipanggil saat admin menekan "Tolak". Tidak ada compare-and-set
// yang rumit; kita menolak hanya bila status masih proposed supaya tombol
// yang ditekan setelah aksi dieksekusi tidak menimpa hasil.
func (s *Server) Reject(ctx context.Context, actionID, approverUserID, reason string) error {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE agent_actions
		SET status='rejected', approved_by=$2, decided_at=NOW(), error=$3
		WHERE id=$1 AND status='proposed'`,
		actionID, approverUserID, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAlreadyDecided
	}
	return nil
}

// ErrAlreadyDecided: aksi sudah tidak berstatus 'proposed' lagi (sudah
// disetujui, ditolak, dieksekusi, atau kedaluwarsa).
var ErrAlreadyDecided = errors.New("aksi sudah diputuskan")

// ---------- helper ----------

// generateActionCode: 8 byte acak → 16 hex char. Ruang 2^64 cukup supaya
// tabrakan praktis nol untuk antrian aksi yang jarang.
func generateActionCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// AdminForTelegramUser: cari admin berdasarkan Telegram user ID. Dipakai saat
// bot menangani callback_query: from.id → platform_admins.user_id.
func (s *Server) AdminForTelegramUser(ctx context.Context, tgUserID int64) (adminID, email, role string, perms map[string]bool, err error) {
	var permsRaw []byte
	err = s.Pool.QueryRow(ctx, `
		SELECT pa.user_id::text, u.email, pa.role, COALESCE(pa.permissions, '{}'::jsonb)
		FROM platform_admins pa JOIN users u ON u.id = pa.user_id
		WHERE pa.telegram_user_id = $1 AND u.is_active = TRUE AND u.approved_at IS NOT NULL
		LIMIT 1`, tgUserID,
	).Scan(&adminID, &email, &role, &permsRaw)
	if err != nil {
		return "", "", "", nil, err
	}
	perms = map[string]bool{}
	if len(permsRaw) > 0 {
		_ = json.Unmarshal(permsRaw, &perms)
	}
	return
}

// CanApprove: cek apakah admin boleh menyetujui aksi berdasar required
// permission. Super admin selalu boleh.
func CanApprove(role string, perms map[string]bool, required string) bool {
	if role == "super_admin" {
		return true
	}
	if required == "" {
		// Aksi tanpa persyaratan khusus boleh disetujui admin biasa; ini bukan
		// jalur umum (kebanyakan tool tulis WAJIB mengisi permission), tapi
		// menjaga simetri dengan adminGuard("") di handler.
		return true
	}
	return perms[required]
}
