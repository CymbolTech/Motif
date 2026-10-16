package adminauth

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	// AccessTokenPrefix marks long-lived workspace tokens so they never reach
	// the session lookup and are easy to recognise in secret scanners.
	AccessTokenPrefix = "motif_at_"
	SourceAccessToken = "access_token"

	ScopeRead    = "read"
	ScopeWrite   = "write"
	ScopePublish = "publish"
)

var (
	ErrInvalidAccessToken  = errors.New("invalid access token")
	ErrAccessTokenNotFound = errors.New("access token not found")
)

// AccessToken is the stored, non-secret view of a workspace access token.
type AccessToken struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	Name           string     `json:"name"`
	Hint           string     `json:"hint"`
	Scopes         []string   `json:"scopes"`
	EnvironmentIDs []string   `json:"environment_ids"`
	CreatedBy      string     `json:"created_by"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type NewAccessToken struct {
	TenantID       string
	CreatedBy      string
	Name           string
	Scopes         []string
	EnvironmentIDs []string
	ExpiresAt      *time.Time
}

func (p Principal) IsAccessToken() bool { return p.Source == SourceAccessToken }

// HasScope reports whether the principal may use a scope. Sessions and the
// bootstrap key carry every scope; access tokens carry only what was granted.
func (p Principal) HasScope(scope string) bool {
	if !p.IsAccessToken() {
		return true
	}
	return slices.Contains(p.Scopes, scope)
}

// AllowsEnvironment reports whether the principal may act in an environment.
// An access token without an environment list may use every environment of its tenant.
func (p Principal) AllowsEnvironment(environmentID string) bool {
	if !p.IsAccessToken() || len(p.EnvironmentIDs) == 0 {
		return true
	}
	return slices.Contains(p.EnvironmentIDs, environmentID)
}

func (s *Store) CreateAccessToken(ctx context.Context, input NewAccessToken) (AccessToken, string, error) {
	secret, err := randomToken(32)
	if err != nil {
		return AccessToken{}, "", err
	}
	id, err := randomToken(12)
	if err != nil {
		return AccessToken{}, "", err
	}
	raw := AccessTokenPrefix + secret
	hint := AccessTokenPrefix + secret[:4] + "…" + secret[len(secret)-4:]
	environments := input.EnvironmentIDs
	if environments == nil {
		environments = []string{}
	}
	token := AccessToken{}
	err = s.db.QueryRow(ctx, `
		INSERT INTO access_tokens (id, tenant_id, created_by, name, token_hash, token_hint, scopes, environment_ids, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, tenant_id, name, token_hint, scopes, environment_ids, created_by, expires_at, last_used_at, revoked_at, created_at`,
		"tok_"+id, input.TenantID, input.CreatedBy, input.Name, digest(raw), hint, input.Scopes, environments, input.ExpiresAt).
		Scan(&token.ID, &token.TenantID, &token.Name, &token.Hint, &token.Scopes, &token.EnvironmentIDs, &token.CreatedBy, &token.ExpiresAt, &token.LastUsedAt, &token.RevokedAt, &token.CreatedAt)
	if err != nil {
		return AccessToken{}, "", err
	}
	return token, raw, nil
}

func (s *Store) ListAccessTokens(ctx context.Context, tenantID string) ([]AccessToken, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, tenant_id, name, token_hint, scopes, environment_ids, created_by, expires_at, last_used_at, revoked_at, created_at
		FROM access_tokens WHERE tenant_id = $1 ORDER BY created_at DESC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tokens := make([]AccessToken, 0)
	for rows.Next() {
		var token AccessToken
		if err := rows.Scan(&token.ID, &token.TenantID, &token.Name, &token.Hint, &token.Scopes, &token.EnvironmentIDs, &token.CreatedBy, &token.ExpiresAt, &token.LastUsedAt, &token.RevokedAt, &token.CreatedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

func (s *Store) RevokeAccessToken(ctx context.Context, tenantID, id string) (AccessToken, error) {
	var token AccessToken
	err := s.db.QueryRow(ctx, `
		UPDATE access_tokens SET revoked_at = COALESCE(revoked_at, now())
		WHERE tenant_id = $1 AND id = $2
		RETURNING id, tenant_id, name, token_hint, scopes, environment_ids, created_by, expires_at, last_used_at, revoked_at, created_at`, tenantID, id).
		Scan(&token.ID, &token.TenantID, &token.Name, &token.Hint, &token.Scopes, &token.EnvironmentIDs, &token.CreatedBy, &token.ExpiresAt, &token.LastUsedAt, &token.RevokedAt, &token.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AccessToken{}, ErrAccessTokenNotFound
	}
	return token, err
}

// LookupAccessToken resolves a raw token to a principal confined to the
// token's tenant. The token stops working when it is revoked or expires, when
// its creator is deactivated or loses the membership, or when the tenant is suspended.
func (s *Store) LookupAccessToken(ctx context.Context, raw string) (Principal, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, AccessTokenPrefix) {
		return Principal{}, ErrInvalidAccessToken
	}
	var principal Principal
	var tenantID string
	err := s.db.QueryRow(ctx, `
		SELECT t.id, t.tenant_id, t.scopes, t.environment_ids, u.id, u.email
		FROM access_tokens t
		JOIN admin_users u ON u.id = t.created_by AND u.active = true
		JOIN tenants tn ON tn.id = t.tenant_id AND tn.status = 'active'
		WHERE t.token_hash = $1
		  AND t.revoked_at IS NULL
		  AND (t.expires_at IS NULL OR t.expires_at > now())
		  AND (u.role = 'super_admin' OR EXISTS (
		        SELECT 1 FROM admin_memberships m WHERE m.admin_user_id = u.id AND m.tenant_id = t.tenant_id))`, digest(raw)).
		Scan(&principal.TokenID, &tenantID, &principal.Scopes, &principal.EnvironmentIDs, &principal.ID, &principal.Email)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrInvalidAccessToken
	}
	if err != nil {
		return Principal{}, err
	}
	if _, err := s.db.Exec(ctx, `UPDATE access_tokens SET last_used_at = now() WHERE id = $1`, principal.TokenID); err != nil {
		return Principal{}, err
	}
	principal.Role = RoleTenantAdmin
	principal.TenantIDs = []string{tenantID}
	principal.Source = SourceAccessToken
	return principal, nil
}
