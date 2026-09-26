package brands

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/rahmaangriawan/tauthub-go/internal/auth"
	"github.com/rahmaangriawan/tauthub-go/internal/httpx"
)

// Port v1/brands/{brandId}/members + [membershipId] — role guard & pesan persis.

func (d *Deps) MembersList(w http.ResponseWriter, r *http.Request) {
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
	members := []map[string]any{}
	mRows, err := d.Pool.Query(r.Context(), `
		SELECT m.id, m.user_id, m.role, m.created_at, u.email, u.display_name
		FROM brand_memberships m JOIN users u ON u.id = m.user_id
		WHERE m.brand_id = $1 ORDER BY m.created_at, u.display_name`, row.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	defer mRows.Close()
	for mRows.Next() {
		var id, userID, role string
		var createdAt time.Time
		var email, displayName string
		if mRows.Scan(&id, &userID, &role, &createdAt, &email, &displayName) == nil {
			members = append(members, map[string]any{
				"id": id, "user_id": userID, "role": role, "created_at": isoMilli(createdAt),
				"email": email, "display_name": displayName,
			})
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(members)
}

func (d *Deps) MembersCreate(w http.ResponseWriter, r *http.Request) {
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
	var body struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	email := lowerStr(body.Email)
	validRole := map[string]bool{"admin": true, "strategist": true, "writer": true, "reviewer": true, "viewer": true}
	if !validRole[body.Role] {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid enum value. Expected 'admin' | 'strategist' | 'writer' | 'reviewer' | 'viewer'")
		return
	}
	_ = email

	var userID, userEmail, displayName string
	err = d.Pool.QueryRow(r.Context(),
		"SELECT id, email, display_name FROM users WHERE email = $1 LIMIT 1", email).
		Scan(&userID, &userEmail, &displayName)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Registered user not found")
		return
	}

	var membershipID string
	txErr := d.WithTxMember(r.Context(), row.ID, user.ID, userID, body.Role, &membershipID)
	if txErr != nil {
		writeError(w, txErr)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": membershipID, "user_id": userID, "email": userEmail,
		"display_name": displayName, "role": body.Role, "created_at": isoMilli(time.Now()),
	})
}

// WithTxMember: INSERT membership + audit dalam satu transaksi (cache invalidasi
// setelah commit).
func (d *Deps) WithTxMember(ctx context.Context, brandID, actorID, targetUserID, role string, out *string) error {
	tx, err := d.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	id := uuid.NewString()
	if _, err := tx.Exec(ctx,
		"INSERT INTO brand_memberships (id, brand_id, user_id, role) VALUES ($1, $2, $3, $4)",
		id, brandID, targetUserID, role); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'membership.created', 'membership', $4)",
		uuid.NewString(), brandID, actorID, id); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	InvalidateBrandCache(brandID)
	*out = id
	return nil
}

func (d *Deps) MemberUpdate(w http.ResponseWriter, r *http.Request) {
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
	membershipID := chiURLParam(r, "membershipId")
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid Input")
		return
	}
	validRole := map[string]bool{"admin": true, "strategist": true, "writer": true, "reviewer": true, "viewer": true}
	if !validRole[body.Role] {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid enum value. Expected 'admin' | 'strategist' | 'writer' | 'reviewer' | 'viewer'")
		return
	}

	var mUserID, mRole string
	var mCreatedAt time.Time
	var mEmail, mDisplayName string
	err = d.Pool.QueryRow(r.Context(), `
		SELECT m.user_id, m.role, u.email, u.display_name, m.created_at
		FROM brand_memberships m JOIN users u ON u.id = m.user_id
		WHERE m.id = $1 AND m.brand_id = $2`, membershipID, row.ID).
		Scan(&mUserID, &mRole, &mEmail, &mDisplayName, &mCreatedAt)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Member not found")
		return
	}
	if mUserID == user.ID {
		httpx.Error(w, http.StatusBadRequest, "You cannot change your own role")
		return
	}
	if mRole == "owner" {
		httpx.Error(w, http.StatusBadRequest, "Ownership transfer requires a dedicated flow")
		return
	}
	var actorRole string
	if err := d.Pool.QueryRow(r.Context(),
		"SELECT role FROM brand_memberships WHERE brand_id = $1 AND user_id = $2", row.ID, user.ID).Scan(&actorRole); err == nil {
		if actorRole == "admin" && (mRole == "admin" || body.Role == "admin") {
			httpx.Error(w, http.StatusForbidden, "Only the owner can manage admins")
			return
		}
	}
	if _, err := d.Pool.Exec(r.Context(),
		"UPDATE brand_memberships SET role = $1 WHERE id = $2 AND brand_id = $3", body.Role, membershipID, row.ID); err != nil {
		writeError(w, err)
		return
	}
	InvalidateBrandCache(row.ID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": membershipID, "user_id": mUserID, "role": body.Role, "created_at": isoMilli(mCreatedAt),
		"email": mEmail, "display_name": mDisplayName,
	})
}

func (d *Deps) MemberDelete(w http.ResponseWriter, r *http.Request) {
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
	membershipID := chiURLParam(r, "membershipId")
	var mUserID, mRole string
	err = d.Pool.QueryRow(r.Context(),
		"SELECT user_id, role FROM brand_memberships WHERE id = $1 AND brand_id = $2",
		membershipID, row.ID).Scan(&mUserID, &mRole)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "Member not found")
		return
	}
	if mUserID == user.ID {
		httpx.Error(w, http.StatusBadRequest, "You cannot remove your own membership")
		return
	}
	if mRole == "owner" {
		httpx.Error(w, http.StatusBadRequest, "Brand owner cannot be removed")
		return
	}
	if _, err := d.Pool.Exec(r.Context(),
		"DELETE FROM brand_memberships WHERE id = $1 AND brand_id = $2", membershipID, row.ID); err != nil {
		writeError(w, err)
		return
	}
	InvalidateBrandCache(row.ID)
	auth.WriteAuditLog(r.Context(), d.Pool, auth.AuditLogInput{
		ActorUserID: &user.ID, BrandID: &row.ID, Action: "brand.member.removed",
		TargetType: strPtr("brand_membership"), TargetID: &membershipID,
	})
	w.WriteHeader(http.StatusNoContent)
}

func lowerStr(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}
