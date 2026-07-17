package store

import (
	"context"
	"time"

	"github.com/ddag/ddag/internal/models"
	"github.com/google/uuid"
)

// ---- Users ----

const userCols = `id, name, email, username, password_hash, status, tenant,
	failed_login_count, locked_until, last_login_at, created_by, created_at, updated_at`

// GetUserByID returns a user with roles populated.
func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var u models.User
	if err := s.get(ctx, &u, `SELECT `+userCols+` FROM users WHERE id=$1`, id); err != nil {
		return nil, err
	}
	roles, err := s.userRoleNames(ctx, id)
	if err != nil {
		return nil, err
	}
	u.Roles = roles
	return &u, nil
}

// GetUserByLogin finds a user by username or email (for login).
func (s *Store) GetUserByLogin(ctx context.Context, login string) (*models.User, error) {
	var u models.User
	if err := s.get(ctx, &u,
		`SELECT `+userCols+` FROM users WHERE username=$1 OR email=$1`, login); err != nil {
		return nil, err
	}
	roles, err := s.userRoleNames(ctx, u.ID)
	if err != nil {
		return nil, err
	}
	u.Roles = roles
	return &u, nil
}

func (s *Store) userRoleNames(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var names []string
	err := s.selectRows(ctx, &names,
		`SELECT r.name FROM roles r JOIN user_roles ur ON ur.role_id=r.id WHERE ur.user_id=$1 ORDER BY r.name`,
		userID)
	if names == nil {
		names = []string{}
	}
	return names, err
}

// ListUsers returns a page of users plus the total count.
func (s *Store) ListUsers(ctx context.Context, p ListParams) ([]models.User, int64, error) {
	p.Normalize()
	var total int64
	if err := s.get(ctx, &total,
		`SELECT count(*) FROM users WHERE ($1='' OR name ILIKE '%'||$1||'%' OR email ILIKE '%'||$1||'%' OR username ILIKE '%'||$1||'%')`,
		p.Search); err != nil {
		return nil, 0, err
	}
	var users []models.User
	if err := s.selectRows(ctx, &users,
		`SELECT `+userCols+` FROM users
		 WHERE ($1='' OR name ILIKE '%'||$1||'%' OR email ILIKE '%'||$1||'%' OR username ILIKE '%'||$1||'%')
		 ORDER BY `+p.OrderBy(map[string]string{"name": "name", "email": "email", "username": "username", "status": "status", "last_login_at": "last_login_at", "created_at": "created_at"}, "created_at")+` LIMIT $2 OFFSET $3`,
		p.Search, p.Limit, p.Offset()); err != nil {
		return nil, 0, err
	}
	for i := range users {
		r, err := s.userRoleNames(ctx, users[i].ID)
		if err != nil {
			return nil, 0, err
		}
		users[i].Roles = r
	}
	return users, total, nil
}

// CreateUser inserts a new user and returns its id.
func (s *Store) CreateUser(ctx context.Context, u *models.User) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (name, email, username, password_hash, status, tenant, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`,
		u.Name, u.Email, u.Username, u.PasswordHash, u.Status, u.Tenant, u.CreatedBy).Scan(&id)
	return id, err
}

// UpdateUser updates editable user fields.
func (s *Store) UpdateUser(ctx context.Context, id uuid.UUID, name, email string, tenant *string, status string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE users SET name=$2, email=$3, tenant=$4, status=$5 WHERE id=$1`,
		id, name, email, tenant, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetUserPassword sets a new password hash and clears lockout state.
func (s *Store) SetUserPassword(ctx context.Context, id uuid.UUID, hash string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET password_hash=$2, failed_login_count=0, locked_until=NULL WHERE id=$1`, id, hash)
	return err
}

// RecordLoginSuccess resets failure counters and stamps last_login_at.
func (s *Store) RecordLoginSuccess(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET last_login_at=now(), failed_login_count=0, locked_until=NULL WHERE id=$1`, id)
	return err
}

// RecordLoginFailure increments the failure counter and locks the account when
// the threshold is reached.
func (s *Store) RecordLoginFailure(ctx context.Context, id uuid.UUID, maxFailures int, lockFor time.Duration) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE users SET
			failed_login_count = failed_login_count + 1,
			locked_until = CASE WHEN failed_login_count + 1 >= $2 THEN now() + $3::interval ELSE locked_until END
		WHERE id=$1`,
		id, maxFailures, lockFor.String())
	return err
}

// SetUserRoles replaces a user's role assignments (by role names).
func (s *Store) SetUserRoles(ctx context.Context, userID uuid.UUID, roleNames []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM user_roles WHERE user_id=$1`, userID); err != nil {
		return err
	}
	for _, name := range roleNames {
		if _, err := tx.Exec(ctx,
			`INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name=$2
			 ON CONFLICT DO NOTHING`, userID, name); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ---- Permissions for a user (effective set) ----

// UserPermissionCodes returns the union of permission codes across a user's roles.
func (s *Store) UserPermissionCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var codes []string
	err := s.selectRows(ctx, &codes, `
		SELECT DISTINCT p.code
		FROM permissions p
		JOIN role_permissions rp ON rp.permission_id=p.id
		JOIN user_roles ur ON ur.role_id=rp.role_id
		WHERE ur.user_id=$1`, userID)
	if codes == nil {
		codes = []string{}
	}
	return codes, err
}

// ---- Roles & Permissions ----

// ListRoles returns every role for metadata consumers.
func (s *Store) ListRoles(ctx context.Context) ([]models.Role, error) {
	roles, _, err := s.ListRolesPage(ctx, ListParams{Page: 1, Limit: 200})
	return roles, err
}

// ListRolesPage returns a searchable, sortable role page plus its total count.
func (s *Store) ListRolesPage(ctx context.Context, p ListParams) ([]models.Role, int64, error) {
	p.Normalize()
	var total int64
	if err := s.get(ctx, &total, roleListCountSQL(), p.Search); err != nil {
		return nil, 0, err
	}
	var roles []models.Role
	if err := s.selectRows(ctx, &roles, roleListSQL(p), p.Search, p.Limit, p.Offset()); err != nil {
		return nil, 0, err
	}
	for i := range roles {
		var perms []string
		if err := s.selectRows(ctx, &perms,
			`SELECT p.code FROM permissions p JOIN role_permissions rp ON rp.permission_id=p.id
			 WHERE rp.role_id=$1 ORDER BY p.code`, roles[i].ID); err != nil {
			return nil, 0, err
		}
		if perms == nil {
			perms = []string{}
		}
		roles[i].Permissions = perms
	}
	return roles, total, nil
}

func (s *Store) GetRoleByName(ctx context.Context, name string) (*models.Role, error) {
	var r models.Role
	err := s.get(ctx, &r,
		`SELECT id, name, description, is_system, created_at, updated_at FROM roles WHERE name=$1`, name)
	return &r, err
}

func (s *Store) CreateRole(ctx context.Context, name, description string) (uuid.UUID, error) {
	var id uuid.UUID
	err := s.pool.QueryRow(ctx,
		`INSERT INTO roles (name, description) VALUES ($1,$2) RETURNING id`, name, description).Scan(&id)
	return id, err
}

func (s *Store) UpdateRoleDescription(ctx context.Context, id uuid.UUID, description string) error {
	_, err := s.pool.Exec(ctx, `UPDATE roles SET description=$2 WHERE id=$1`, id, description)
	return err
}

func (s *Store) DeleteRole(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM roles WHERE id=$1 AND is_system=false`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListPermissions(ctx context.Context) ([]models.Permission, error) {
	var perms []models.Permission
	err := s.selectRows(ctx, &perms,
		`SELECT id, code, description, created_at, updated_at FROM permissions ORDER BY code`)
	return perms, err
}

// SetRolePermissions replaces a role's permissions (by permission codes).
func (s *Store) SetRolePermissions(ctx context.Context, roleID uuid.UUID, codes []string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, roleID); err != nil {
		return err
	}
	for _, code := range codes {
		if _, err := tx.Exec(ctx,
			`INSERT INTO role_permissions (role_id, permission_id) SELECT $1, id FROM permissions WHERE code=$2
			 ON CONFLICT DO NOTHING`, roleID, code); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// ---- Settings ----

func (s *Store) ListSettings(ctx context.Context) ([]models.Setting, error) {
	var out []models.Setting
	err := s.selectRows(ctx, &out, `SELECT key, value, category, value_type, scope, description, default_value, restart_required, updated_by, updated_at FROM settings ORDER BY category, key`)
	return out, err
}

func (s *Store) GetSetting(ctx context.Context, key string) (*models.Setting, error) {
	var st models.Setting
	err := s.get(ctx, &st, `SELECT key, value, category, value_type, scope, description, default_value, restart_required, updated_by, updated_at FROM settings WHERE key=$1`, key)
	return &st, err
}

func (s *Store) UpsertSetting(ctx context.Context, key string, value []byte, by *uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO settings (key, value, updated_by, updated_at) VALUES ($1,$2,$3,now())
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value, updated_by=EXCLUDED.updated_by, updated_at=now()`,
		key, value, by)
	return err
}
