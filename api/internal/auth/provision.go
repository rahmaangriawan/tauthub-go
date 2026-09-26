package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Port lib/auth/provision.ts — provisioning akun baru dipakai bersama oleh
// register email/password dan Google Sign-In. Semua query di dalam transaksi
// pemanggil (tx dari WithTx). Urutan & kolom persis.

var ErrPublicIDExhausted = errors.New("Could not allocate a unique partner public id")

func uniquePartnerPublicID(ctx context.Context, tx pgx.Tx) (string, error) {
	for i := 0; i < 60; i++ {
		candidate := BuildShortPublicId()
		var x int
		err := tx.QueryRow(ctx, "SELECT 1 FROM partners WHERE public_id = $1 LIMIT 1", candidate).Scan(&x)
		if errors.Is(err, pgx.ErrNoRows) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", ErrPublicIDExhausted
}

func uniqueBrandPublicID(ctx context.Context, tx pgx.Tx) (string, error) {
	for i := 0; i < 60; i++ {
		candidate := BuildShortPublicId()
		var x int
		err := tx.QueryRow(ctx, "SELECT 1 FROM brands WHERE public_id = $1 LIMIT 1", candidate).Scan(&x)
		if errors.Is(err, pgx.ErrNoRows) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", ErrPublicIDExhausted
}

// ProvisionPartner: baris partners aktif + audit trail.
func ProvisionPartner(ctx context.Context, tx pgx.Tx, userID, displayName, partnerCategory string) error {
	if partnerCategory == "" {
		partnerCategory = "lainnya"
	}
	partnerID := uuid.NewString()
	publicID, err := uniquePartnerPublicID(ctx, tx)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO partners (id, user_id, name, public_id, category, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, 'active', NOW(), NOW())",
		partnerID, userID, displayName, publicID, partnerCategory); err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		"INSERT INTO audit_logs (id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, 'partner.registered', 'partner', $3)",
		uuid.NewString(), userID, partnerID)
	return err
}

// ProvisionClient: brand default + ownership + profil turunan + audit trail.
func ProvisionClient(ctx context.Context, tx pgx.Tx, userID, displayName string) error {
	brandID := uuid.NewString()
	publicID, err := uniqueBrandPublicID(ctx, tx)
	if err != nil {
		return err
	}
	slug := Slugify(displayName)
	if slug == "" {
		slug = "brand"
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO brands (id, name, slug, public_id, ownership_type, client_name, industry, primary_language, status, created_by) VALUES ($1, $2, $3, $4, 'client', $5, NULL, 'id-ID', 'active', $6)",
		brandID, displayName, slug, publicID, displayName, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO brand_memberships (id, brand_id, user_id, role) VALUES ($1, $2, $3, 'owner')",
		uuid.NewString(), brandID, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "INSERT INTO brand_profiles (brand_id) VALUES ($1)", brandID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO voice_profiles (brand_id, personality, preferred_terms, banned_terms) VALUES ($1, '[]', '[]', '[]')",
		brandID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		"INSERT INTO editorial_rule_sets (brand_id, required_rules, forbidden_rules) VALUES ($1, '[]', '[]')",
		brandID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		"INSERT INTO audit_logs (id, brand_id, actor_user_id, action, target_type, target_id) VALUES ($1, $2, $3, 'brand.created', 'brand', $4)",
		uuid.NewString(), brandID, userID, brandID)
	return err
}

// HealRoleProvisioning: self-heal akun lama yang kosong row turunan — idempoten.
func HealRoleProvisioning(ctx context.Context, tx pgx.Tx, role, userID, displayName string) error {
	switch role {
	case "partner":
		var x int
		err := tx.QueryRow(ctx,
			"SELECT 1 FROM partners WHERE user_id = $1 AND status = 'active' LIMIT 1", userID).Scan(&x)
		if err != nil { // tidak ada → buat
			return ProvisionPartner(ctx, tx, userID, displayName, "")
		}
		return nil
	case "client":
		var x int
		err := tx.QueryRow(ctx,
			"SELECT 1 FROM brand_memberships WHERE user_id = $1 LIMIT 1", userID).Scan(&x)
		if err != nil {
			return ProvisionClient(ctx, tx, userID, displayName)
		}
		return nil
	default:
		return nil
	}
}
