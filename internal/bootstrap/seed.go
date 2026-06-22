package bootstrap

import (
	"context"
	"os"

	"github.com/ddag/ddag/internal/auth"
	"github.com/ddag/ddag/internal/logging"
	"github.com/ddag/ddag/internal/rbac"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedCore inserts the permission catalog, the seven system roles with their
// permission sets, default scopes, and a super-admin user. It is idempotent:
// existing rows are left intact (only missing pieces are added). The super-admin
// password comes from DDAG_SUPERADMIN_PASSWORD (default for dev only).
func SeedCore(ctx context.Context, pool *pgxpool.Pool, log *logging.Logger) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Permissions.
	for _, p := range rbac.AllPermissions {
		if _, err := tx.Exec(ctx,
			`INSERT INTO permissions (code, description) VALUES ($1,$2)
			 ON CONFLICT (code) DO UPDATE SET description=EXCLUDED.description`,
			p.Code, p.Description); err != nil {
			return err
		}
	}

	// Roles + role_permissions.
	for role, perms := range rbac.DefaultRolePermissions {
		if _, err := tx.Exec(ctx,
			`INSERT INTO roles (name, description, is_system) VALUES ($1,$2,true)
			 ON CONFLICT (name) DO UPDATE SET description=EXCLUDED.description, is_system=true`,
			role, rbac.RoleDescriptions[role]); err != nil {
			return err
		}
		// Reset this role's permissions to the canonical set.
		if _, err := tx.Exec(ctx,
			`DELETE FROM role_permissions WHERE role_id=(SELECT id FROM roles WHERE name=$1)`, role); err != nil {
			return err
		}
		for _, code := range perms {
			if _, err := tx.Exec(ctx, `
				INSERT INTO role_permissions (role_id, permission_id)
				SELECT r.id, p.id FROM roles r, permissions p WHERE r.name=$1 AND p.code=$2
				ON CONFLICT DO NOTHING`, role, code); err != nil {
				return err
			}
		}
	}

	// Default OAuth2 scopes.
	scopes := [][2]string{
		{"brim.site.read", "Read BRIM site data"},
		{"brim.wo.read", "Read BRIM work orders"},
		{"customer.profile.read", "Read customer profiles"},
		{"inventory.stock.read", "Read inventory stock"},
	}
	for _, sc := range scopes {
		if _, err := tx.Exec(ctx,
			`INSERT INTO scopes (scope_code, description) VALUES ($1,$2) ON CONFLICT (scope_code) DO NOTHING`,
			sc[0], sc[1]); err != nil {
			return err
		}
	}

	// Super-admin user.
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username='superadmin')`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		password := os.Getenv("DDAG_SUPERADMIN_PASSWORD")
		if password == "" {
			password = "Admin#12345"
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			return err
		}
		var userID string
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (name, email, username, password_hash, status)
			VALUES ('Super Admin','admin@ddag.local','superadmin',$1,'active') RETURNING id`,
			hash).Scan(&userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_roles (user_id, role_id)
			SELECT $1, id FROM roles WHERE name=$2`, userID, rbac.RoleSuperAdmin); err != nil {
			return err
		}
		log.Info("seeded_superadmin", "username", "superadmin", "email", "admin@ddag.local")
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	log.Info("core_seed_complete")
	return nil
}
