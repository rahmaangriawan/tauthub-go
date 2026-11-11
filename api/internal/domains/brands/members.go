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
	seen := map[string]bool{}
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
			seen[userID] = true
			members = append(members, map[string]any{
				"id": id, "user_id": userID, "role": role, "created_at": isoMilli(createdAt),
				"email": email, "display_name": displayName, "via": "membership",
			})
		}
	}

	// Anggota workspace yang tidak punya baris membership tetap bisa membuka
	// brand ini lewat fallback workspace, jadi mereka harus terlihat di daftar.
	// Tanpa ini admin mengira brand hanya dipakai orang yang tertulis di
	// brand_memberships dan salah menilai siapa yang punya akses.
	if row.WorkspaceID != nil && *row.WorkspaceID != "" {
		wRows, err := d.Pool.Query(r.Context(), `
			SELECT wm.user_id, wm.role, wm.created_at, u.email, u.display_name
			FROM workspace_members wm JOIN users u ON u.id = wm.user_id
			WHERE wm.workspace_id = $1 ORDER BY wm.created_at, u.display_name`, *row.WorkspaceID)
		if err != nil {
			writeError(w, err)
			return
		}
		defer wRows.Close()
		for wRows.Next() {
			var userID, role string
			var createdAt time.Time
			var email, displayName string
			if wRows.Scan(&userID, &role, &createdAt, &email, &displayName) == nil && !seen[userID] {
				seen[userID] = true
				// id sengaja kosong: baris ini bukan membership brand, jadi
				// tidak bisa diubah/dihapus lewat endpoint members brand. Peran
				// ditampilkan apa adanya dari workspace, bukan peran karangan.
				members = append(members, map[string]any{
					"id": "", "user_id": userID, "role": role, "created_at": isoMilli(createdAt),
					"email": email, "display_name": displayName, "via": "workspace",
				})
			}
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
	validRole := map[string]bool{"admin": true, "strategist": true, "writer": true, "finance": true, "reviewer": true, "viewer": true}
	if !validRole[body.Role] {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid enum value. Expected 'admin' | 'strategist' | 'writer' | 'finance' | 'reviewer' | 'viewer'")
		return
	}

	// Aktor non-owner (admin) tidak boleh memberi peran admin.
	if row.Role != "owner" && body.Role == "admin" {
		httpx.Error(w, http.StatusForbidden, "Only the owner can manage admins")
		return
	}

	var userID, userEmail, displayName string
	err = d.Pool.QueryRow(r.Context(),
		"SELECT id, email, display_name FROM users WHERE email = $1 LIMIT 1", email).
		Scan(&userID, &userEmail, &displayName)
	if err != nil {
		// Email belum terdaftar: kirim undangan. Peran terpasang pada undangan
		// dan baru berlaku saat penerima masuk memakai email yang sama.
		d.inviteByEmail(w, r, row, user.ID, email, body.Role)
		return
	}

	// Paritas dengan MemberUpdate/MemberDelete: aktor non-owner (admin) tidak
	// boleh memberi peran admin dan tidak boleh menimpa peran efektif owner/admin
	// milik target (peran dari workspace) dengan baris eksplisit berperan rendah.
	if row.Role != "owner" {
		if body.Role == "admin" {
			httpx.Error(w, http.StatusForbidden, "Only the owner can manage admins")
			return
		}
		if _, effRole, effErr := EffectiveRole(r.Context(), d.Pool, row.ID, userID); effErr == nil &&
			(effRole == "owner" || effRole == "admin") {
			httpx.Error(w, http.StatusForbidden, "Only the owner can manage admins")
			return
		}
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
	validRole := map[string]bool{"admin": true, "strategist": true, "writer": true, "finance": true, "reviewer": true, "viewer": true}
	if !validRole[body.Role] {
		httpx.Error(w, http.StatusUnprocessableEntity, "Invalid enum value. Expected 'admin' | 'strategist' | 'writer' | 'finance' | 'reviewer' | 'viewer'")
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
	// Peran target dan aktor diambil dari jalur efektif. Query lama membaca
	// brand_memberships langsung lalu gagal senyap kalau aktor tidak punya
	// baris di sana: admin workspace jadi bisa mengubah admin brand. Fail-open
	// di jalur otorisasi sama dengan eskalasi hak.
	targetRole := mRole
	if _, effRole, effErr := EffectiveRole(r.Context(), d.Pool, row.ID, mUserID); effErr == nil {
		targetRole = effRole
	}
	if targetRole == "owner" {
		httpx.Error(w, http.StatusBadRequest, "Ownership transfer requires a dedicated flow")
		return
	}
	if row.Role == "admin" && (targetRole == "admin" || body.Role == "admin") {
		httpx.Error(w, http.StatusForbidden, "Only the owner can manage admins")
		return
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
	// Pemilik bisa datang dari workspace (owner workspace) tanpa baris
	// membership berperan owner. Cek peran efektif supaya pemilik tidak bisa
	// dilepas lewat pintu belakang ini.
	targetRole := mRole
	if _, effRole, effErr := EffectiveRole(r.Context(), d.Pool, row.ID, mUserID); effErr == nil {
		targetRole = effRole
	}
	if targetRole == "owner" {
		httpx.Error(w, http.StatusBadRequest, "Brand owner cannot be removed")
		return
	}
	// Paritas dengan MemberUpdate: hanya owner yang boleh menyentuh admin.
	// Jalur hapus sebelumnya tidak punya penjaga ini, sehingga admin (termasuk
	// yang masuk lewat workspace) bisa mengeluarkan admin lain padahal jalur
	// ubah peran melarangnya. Jalur yang lebih longgar menentukan hasil
	// akhirnya, jadi penjaganya harus sama.
	if row.Role == "admin" && targetRole == "admin" {
		httpx.Error(w, http.StatusForbidden, "Only the owner can manage admins")
		return
	}
	// Menghapus baris membership hanya mencabut izin EKSPLISIT; kalau user masih
	// anggota workspace, aksesnya turun ke peran workspace (bukan hilang).
	// Untuk mencabut penuh, hapus dari workspace lewat endpoint workspace.
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
