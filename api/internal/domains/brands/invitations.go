package brands

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	emailqueue "github.com/rahmaangriawan/tauthub-go/internal/email"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Undangan anggota brand lewat email. Token acak 32 byte; yang disimpan hanya
// hash-nya. Undangan hanya bisa diterima oleh akun dengan email yang sama dengan
// yang diundang, jadi tautan yang bocor tidak memberi akses ke orang lain.

const invitationTTL = 7 * 24 * time.Hour

func hashInviteToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func newInviteToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func maskEmail(email string) string {
	at := strings.Index(email, "@")
	if at <= 1 {
		return email
	}
	return email[:1] + strings.Repeat("*", at-1) + email[at:]
}

// inviteByEmail: dipanggil MembersCreate saat email belum terdaftar. Undangan
// terbuka sebelumnya untuk email yang sama dicabut dulu, jadi mengundang ulang
// sama dengan mengirim ulang dan tautan lama mati.
func (d *Deps) inviteByEmail(w http.ResponseWriter, r *http.Request, brand BrandRow, actorID, email, role string) {
	ctx := r.Context()
	token, err := newInviteToken()
	if err != nil {
		writeError(w, err)
		return
	}
	expires := time.Now().Add(invitationTTL)
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		writeError(w, err)
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx,
		"UPDATE brand_invitations SET revoked_at = NOW() WHERE brand_id = $1 AND lower(email) = $2 AND accepted_at IS NULL AND revoked_at IS NULL",
		brand.ID, email); err != nil {
		writeError(w, err)
		return
	}
	id := uuid.NewString()
	if _, err := tx.Exec(ctx,
		"INSERT INTO brand_invitations (id, brand_id, email, role, token_hash, invited_by, expires_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		id, brand.ID, email, role, hashInviteToken(token), actorID, expires); err != nil {
		writeError(w, err)
		return
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'invitation.created', 'invitation', $4)",
		uuid.NewString(), brand.ID, actorID, id); err != nil {
		writeError(w, err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, err)
		return
	}

	var inviter string
	_ = d.Pool.QueryRow(ctx, "SELECT COALESCE(NULLIF(display_name, ''), email) FROM users WHERE id = $1", actorID).Scan(&inviter)
	base := strings.TrimRight(d.PublicAppURL, "/")
	if base == "" {
		base = "https://app.tauthub.com"
	}
	emailSent := true
	if _, _, sendErr := emailqueue.SendTransactionalEmail(ctx, d.Pool, emailqueue.SendOptions{
		Template: "brand-invitation", To: email, Event: "brand.invitation",
		Data: map[string]string{
			"link": base + "/undangan/" + token, "brand": brand.Name, "role": role, "inviter": inviter,
		},
	}); sendErr != nil {
		emailSent = false
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"invited": true, "id": id, "email": email, "role": role,
		"expires_at": isoMilli(expires), "email_sent": emailSent,
	})
}

// InvitationsList: GET /v1/brands/{brandId}/invitations (admin) — undangan yang
// masih terbuka.
func (d *Deps) InvitationsList(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		writeError(w, err)
		return
	}
	rows, err := d.Pool.Query(r.Context(), `
		SELECT id, email, role, expires_at, created_at
		FROM brand_invitations
		WHERE brand_id = $1 AND accepted_at IS NULL AND revoked_at IS NULL
		ORDER BY created_at DESC`, row.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	now := time.Now()
	for rows.Next() {
		var id, email, role string
		var expires, created time.Time
		if rows.Scan(&id, &email, &role, &expires, &created) != nil {
			continue
		}
		out = append(out, map[string]any{
			"id": id, "email": email, "role": role, "expires_at": isoMilli(expires),
			"created_at": isoMilli(created), "expired": expires.Before(now),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

// InvitationRevoke: DELETE /v1/brands/{brandId}/invitations/{invitationId}.
func (d *Deps) InvitationRevoke(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	row, err := BrandAccess(r.Context(), d.Pool, chiURLParam(r, "brandId"), user.ID, "admin")
	if err != nil {
		writeError(w, err)
		return
	}
	tag, err := d.Pool.Exec(r.Context(),
		"UPDATE brand_invitations SET revoked_at = NOW() WHERE id = $1 AND brand_id = $2 AND accepted_at IS NULL AND revoked_at IS NULL",
		chiURLParam(r, "invitationId"), row.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	if tag.RowsAffected() == 0 {
		httpx.Error(w, http.StatusNotFound, "Undangan tidak ditemukan.")
		return
	}
	_, _ = d.Pool.Exec(r.Context(),
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'invitation.revoked', 'invitation', $4)",
		uuid.NewString(), row.ID, user.ID, chiURLParam(r, "invitationId"))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
}

type invitationRow struct {
	ID, BrandID, BrandName, Email, Role, InvitedBy string
	ExpiresAt                                      time.Time
	Accepted, Revoked                              bool
}

func (d *Deps) loadInvitation(r *http.Request, token string) (*invitationRow, error) {
	var inv invitationRow
	var acceptedAt, revokedAt *time.Time
	err := d.Pool.QueryRow(r.Context(), `
		SELECT i.id, i.brand_id, b.name, i.email, i.role, i.invited_by, i.expires_at, i.accepted_at, i.revoked_at
		FROM brand_invitations i JOIN brands b ON b.id = i.brand_id
		WHERE i.token_hash = $1 AND b.deleted_at IS NULL`, hashInviteToken(token)).
		Scan(&inv.ID, &inv.BrandID, &inv.BrandName, &inv.Email, &inv.Role, &inv.InvitedBy, &inv.ExpiresAt, &acceptedAt, &revokedAt)
	if err != nil {
		return nil, nil
	}
	inv.Accepted = acceptedAt != nil
	inv.Revoked = revokedAt != nil
	return &inv, nil
}

// InvitationPreview: GET /v1/invitations/{token} (publik). Hanya nama brand,
// peran, email tersamar, dan status; tidak ada data lain.
func (d *Deps) InvitationPreview(w http.ResponseWriter, r *http.Request) {
	inv, _ := d.loadInvitation(r, chiURLParam(r, "token"))
	if inv == nil {
		httpx.Error(w, http.StatusNotFound, "Undangan tidak ditemukan.")
		return
	}
	status := "open"
	switch {
	case inv.Accepted:
		status = "accepted"
	case inv.Revoked:
		status = "revoked"
	case inv.ExpiresAt.Before(time.Now()):
		status = "expired"
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"brand_name": inv.BrandName, "role": inv.Role, "email_hint": maskEmail(inv.Email), "status": status,
	})
}

// InvitationAccept: POST /v1/invitations/{token}/accept (login). Email akun harus
// sama dengan email undangan.
func (d *Deps) InvitationAccept(w http.ResponseWriter, r *http.Request) {
	user := httpx.AuthUserFromCtx(r)
	if user == nil {
		httpx.Error(w, http.StatusUnauthorized, "Invalid or expired access token")
		return
	}
	inv, _ := d.loadInvitation(r, chiURLParam(r, "token"))
	if inv == nil || inv.Revoked {
		httpx.Error(w, http.StatusNotFound, "Undangan tidak ditemukan atau sudah dicabut.")
		return
	}
	if inv.Accepted {
		httpx.Error(w, http.StatusConflict, "Undangan ini sudah dipakai.")
		return
	}
	if inv.ExpiresAt.Before(time.Now()) {
		httpx.Error(w, http.StatusGone, "Undangan sudah kedaluwarsa. Minta admin mengirim ulang.")
		return
	}
	if !strings.EqualFold(strings.TrimSpace(user.Email), inv.Email) {
		httpx.Error(w, http.StatusForbidden, "Undangan ini ditujukan untuk email lain. Masuk memakai email yang diundang.")
		return
	}
	var exists bool
	_ = d.Pool.QueryRow(r.Context(),
		"SELECT EXISTS (SELECT 1 FROM brand_memberships WHERE brand_id = $1 AND user_id = $2)", inv.BrandID, user.ID).Scan(&exists)
	if !exists {
		var membershipID string
		if err := d.WithTxMember(r.Context(), inv.BrandID, inv.InvitedBy, user.ID, inv.Role, &membershipID); err != nil {
			writeError(w, err)
			return
		}
	}
	_, _ = d.Pool.Exec(r.Context(),
		"UPDATE brand_invitations SET accepted_at = NOW(), accepted_by = $2 WHERE id = $1", inv.ID, user.ID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "brand_id": inv.BrandID, "role": inv.Role})
}
