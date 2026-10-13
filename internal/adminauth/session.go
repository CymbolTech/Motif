package adminauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RoleSuperAdmin  = "super_admin"
	RoleTenantAdmin = "tenant_admin"
	SessionTTL      = 12 * time.Hour
)

var (
	ErrInvalidCredentials = errors.New("invalid admin credentials")
	ErrInvalidSession     = errors.New("invalid admin session")
	ErrForbidden          = errors.New("admin permission denied")
)

type Principal struct {
	ID        string
	Email     string
	Role      string
	TenantIDs []string
	Source    string
}

type User struct {
	ID        string
	Email     string
	Role      string
	Active    bool
	CreatedAt time.Time
	TenantIDs []string
}

type contextKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, contextKey{}, principal)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(contextKey{}).(Principal)
	return principal, ok
}

func (p Principal) IsSuperAdmin() bool { return p.Role == RoleSuperAdmin }

func (p Principal) AllowsTenant(tenantID string) bool {
	if p.IsSuperAdmin() {
		return true
	}
	for _, allowed := range p.TenantIDs {
		if allowed == tenantID {
			return true
		}
	}
	return false
}

type Store struct {
	db  *pgxpool.Pool
	ttl time.Duration
}

func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db, ttl: SessionTTL} }

func (s *Store) Authenticate(ctx context.Context, email, password string, userAgent, ipAddress string) (Principal, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	var id, hash, role string
	var active bool
	err := s.db.QueryRow(ctx, `SELECT id, password_hash, role, active FROM admin_users WHERE email = $1`, email).Scan(&id, &hash, &role, &active)
	if errors.Is(err, pgx.ErrNoRows) || !active || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return Principal{}, "", ErrInvalidCredentials
	}
	principal, err := s.principal(ctx, id, email, role)
	if err != nil {
		return Principal{}, "", err
	}
	rawToken, err := randomToken(32)
	if err != nil {
		return Principal{}, "", err
	}
	idToken, err := randomToken(18)
	if err != nil {
		return Principal{}, "", err
	}
	expires := time.Now().UTC().Add(s.ttl)
	if _, err := s.db.Exec(ctx, `INSERT INTO admin_sessions (id, admin_user_id, token_hash, expires_at, user_agent, ip_address) VALUES ($1, $2, $3, $4, $5, $6)`, idToken, id, digest(rawToken), expires, truncate(userAgent, 300), truncate(ipAddress, 128)); err != nil {
		return Principal{}, "", err
	}
	return principal, rawToken, nil
}

func (s *Store) Lookup(ctx context.Context, rawToken string) (Principal, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return Principal{}, ErrInvalidSession
	}
	var sessionID, userID, email, role string
	var expires time.Time
	err := s.db.QueryRow(ctx, `
		SELECT s.id, u.id, u.email, u.role, s.expires_at
		FROM admin_sessions s JOIN admin_users u ON u.id = s.admin_user_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now() AND u.active = true`, digest(rawToken)).
		Scan(&sessionID, &userID, &email, &role, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrInvalidSession
	}
	if err != nil {
		return Principal{}, err
	}
	if _, err := s.db.Exec(ctx, `UPDATE admin_sessions SET last_used_at = now() WHERE id = $1`, sessionID); err != nil {
		return Principal{}, err
	}
	principal, err := s.principal(ctx, userID, email, role)
	if err != nil {
		return Principal{}, err
	}
	_ = expires
	return principal, nil
}

func (s *Store) Revoke(ctx context.Context, rawToken string) error {
	if strings.TrimSpace(rawToken) == "" {
		return nil
	}
	_, err := s.db.Exec(ctx, `UPDATE admin_sessions SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, digest(rawToken))
	return err
}

func (s *Store) EnsureSuperAdmin(ctx context.Context, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return fmt.Errorf("admin email and password are required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO admin_users (id, email, password_hash, role)
		VALUES ('bootstrap-super-admin', $1, $2, $3)
		ON CONFLICT (email) DO UPDATE SET role = EXCLUDED.role, active = true, updated_at = now()`, email, string(hash), RoleSuperAdmin)
	return err
}

func (s *Store) CreateUser(ctx context.Context, email, password, role string, tenantIDs []string) (User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	role = strings.TrimSpace(role)
	if email == "" || password == "" || (role != RoleSuperAdmin && role != RoleTenantAdmin) {
		return User{}, ErrInvalidCredentials
	}
	if role == RoleTenantAdmin && len(tenantIDs) == 0 {
		return User{}, fmt.Errorf("tenant admin requires at least one tenant membership")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	id, err := randomToken(12)
	if err != nil {
		return User{}, err
	}
	id = "admin_" + id
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var user User
	err = tx.QueryRow(ctx, `INSERT INTO admin_users (id, email, password_hash, role) VALUES ($1, $2, $3, $4) RETURNING id, email, role, active, created_at`, id, email, string(hash), role).
		Scan(&user.ID, &user.Email, &user.Role, &user.Active, &user.CreatedAt)
	if err != nil {
		return User{}, err
	}
	for _, tenantID := range tenantIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO admin_memberships (admin_user_id, tenant_id) VALUES ($1, $2)`, user.ID, strings.TrimSpace(tenantID)); err != nil {
			return User{}, err
		}
		user.TenantIDs = append(user.TenantIDs, strings.TrimSpace(tenantID))
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Store) ListUsers(ctx context.Context, tenantID string) ([]User, error) {
	rows, err := s.db.Query(ctx, `
		SELECT DISTINCT u.id, u.email, u.role, u.active, u.created_at
		FROM admin_users u LEFT JOIN admin_memberships m ON m.admin_user_id = u.id
		WHERE ($1 = '' OR m.tenant_id = $1) ORDER BY u.created_at DESC`, strings.TrimSpace(tenantID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Email, &user.Role, &user.Active, &user.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for index := range users {
		rows, err := s.db.Query(ctx, `SELECT tenant_id FROM admin_memberships WHERE admin_user_id = $1 ORDER BY tenant_id`, users[index].ID)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var tenantID string
			if err := rows.Scan(&tenantID); err != nil {
				rows.Close()
				return nil, err
			}
			users[index].TenantIDs = append(users[index].TenantIDs, tenantID)
		}
		rows.Close()
	}
	return users, nil
}

func (s *Store) principal(ctx context.Context, id, email, role string) (Principal, error) {
	principal := Principal{ID: id, Email: email, Role: role, Source: "session"}
	if role == RoleSuperAdmin {
		return principal, nil
	}
	rows, err := s.db.Query(ctx, `SELECT tenant_id FROM admin_memberships WHERE admin_user_id = $1 ORDER BY tenant_id`, id)
	if err != nil {
		return Principal{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var tenantID string
		if err := rows.Scan(&tenantID); err != nil {
			return Principal{}, err
		}
		principal.TenantIDs = append(principal.TenantIDs, tenantID)
	}
	return principal, rows.Err()
}

func FromRequest(ctx context.Context, store *Store, r *http.Request, bootstrapKey string) (Principal, error) {
	if header := strings.TrimSpace(r.Header.Get("Authorization")); strings.HasPrefix(strings.ToLower(header), "bearer ") {
		principal, err := store.Lookup(ctx, strings.TrimSpace(header[len("Bearer "):]))
		if err == nil {
			return principal, nil
		}
	}
	provided := []byte(r.Header.Get("X-Motif-Admin-Key"))
	expected := []byte(bootstrapKey)
	if len(provided) > 0 && len(provided) == len(expected) {
		match := byte(0)
		for i := range provided {
			match |= provided[i] ^ expected[i]
		}
		if match == 0 {
			return Principal{ID: "bootstrap-key", Email: "bootstrap", Role: RoleSuperAdmin, Source: "bootstrap_key"}, nil
		}
	}
	return Principal{}, ErrInvalidSession
}

func digest(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[:max]
}
