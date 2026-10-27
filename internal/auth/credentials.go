package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// KeyPrefix marks access keys so they are easy to recognise in logs and
	// secret scanners. Keys migrated from MCP access tokens keep the older
	// LegacyTokenPrefix; keys migrated from integration credentials have none.
	KeyPrefix         = "motif_key_"
	LegacyTokenPrefix = "motif_at_"

	ScopeRead   = "read"
	ScopeWrite  = "write"
	ScopeEdit   = "edit"
	ScopeDelete = "delete"
	// ScopeMCP lets the key configure the workspace through the MCP server,
	// which calls the admin API. Without it a key only reaches the REST API.
	ScopeMCP = "mcp"

	maxKeyDays       = 365
	maxAllowedOrigin = 20
)

// Scopes lists every scope in display order.
var Scopes = []string{ScopeRead, ScopeWrite, ScopeEdit, ScopeDelete, ScopeMCP}

var (
	ErrInvalidCredential = errors.New("invalid credential")
	ErrKeyNotFound       = errors.New("access key was not found")

	identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
)

type Scope struct {
	TenantID      string
	EnvironmentID string
}

// Key is the stored, non-secret view of an access key.
type Key struct {
	ID             string     `json:"id"`
	TenantID       string     `json:"tenant_id"`
	EnvironmentID  string     `json:"environment_id"`
	Name           string     `json:"name"`
	Hint           string     `json:"hint"`
	Scopes         []string   `json:"scopes"`
	AllowedOrigins []string   `json:"allowed_origins"`
	CreatedBy      *string    `json:"created_by"`
	CreatedByEmail string     `json:"created_by_email,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	RevokedAt      *time.Time `json:"revoked_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (k Key) Has(scope string) bool { return slices.Contains(k.Scopes, scope) }

func (k Key) Scope() Scope { return Scope{TenantID: k.TenantID, EnvironmentID: k.EnvironmentID} }

// AllowsOrigin reports whether a browser on origin may use the key. A key
// without allowed origins accepts every origin.
func (k Key) AllowsOrigin(origin string) bool {
	if len(k.AllowedOrigins) == 0 {
		return true
	}
	normalized, err := NormalizeOrigin(origin)
	return err == nil && slices.Contains(k.AllowedOrigins, normalized)
}

// LooksLikeKey reports whether a bearer value is an access key rather than an
// administrator session token.
func LooksLikeKey(raw string) bool {
	return strings.HasPrefix(raw, KeyPrefix) || strings.HasPrefix(raw, LegacyTokenPrefix)
}

func Digest(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	return digest[:]
}

// KeySpec is what an administrator chooses for a key. The environment is
// fixed when the key is created.
type KeySpec struct {
	TenantID       string   `json:"tenant_id"`
	EnvironmentID  string   `json:"environment_id"`
	Name           string   `json:"name"`
	Scopes         []string `json:"scopes"`
	AllowedOrigins []string `json:"allowed_origins"`
	ExpiresInDays  int      `json:"expires_in_days"`
}

// ValidateKeySpec normalizes a key request: scopes in display order, origins
// as scheme://host[:port]. MCP needs read, since the MCP server reads the
// workspace before it does anything else.
func ValidateKeySpec(spec KeySpec) (KeySpec, error) {
	spec.TenantID = strings.TrimSpace(spec.TenantID)
	spec.EnvironmentID = strings.TrimSpace(spec.EnvironmentID)
	spec.Name = strings.TrimSpace(spec.Name)
	if !identifierPattern.MatchString(spec.TenantID) || !identifierPattern.MatchString(spec.EnvironmentID) {
		return spec, errors.New("tenant_id and environment_id are required")
	}
	if spec.Name == "" || len(spec.Name) > 80 {
		return spec, errors.New("name must be between 1 and 80 characters")
	}
	granted := map[string]bool{}
	for _, scope := range spec.Scopes {
		scope = strings.TrimSpace(scope)
		if !slices.Contains(Scopes, scope) {
			return spec, fmt.Errorf("unsupported scope %q; use read, write, edit, delete or mcp", scope)
		}
		granted[scope] = true
	}
	if len(granted) == 0 {
		return spec, errors.New("choose at least one scope")
	}
	if granted[ScopeMCP] && !granted[ScopeRead] {
		return spec, errors.New("the mcp scope needs read")
	}
	spec.Scopes = make([]string, 0, len(granted))
	for _, scope := range Scopes {
		if granted[scope] {
			spec.Scopes = append(spec.Scopes, scope)
		}
	}
	if len(spec.AllowedOrigins) > maxAllowedOrigin {
		return spec, fmt.Errorf("allowed_origins can list at most %d origins", maxAllowedOrigin)
	}
	origins := make([]string, 0, len(spec.AllowedOrigins))
	for _, value := range spec.AllowedOrigins {
		origin, err := NormalizeOrigin(value)
		if err != nil {
			return spec, err
		}
		if !slices.Contains(origins, origin) {
			origins = append(origins, origin)
		}
	}
	spec.AllowedOrigins = origins
	if spec.ExpiresInDays < 0 || spec.ExpiresInDays > maxKeyDays {
		return spec, fmt.Errorf("expires_in_days must be between 0 (no expiry) and %d", maxKeyDays)
	}
	return spec, nil
}

// NormalizeOrigin turns "https://Shop.example.com/" into
// "https://shop.example.com", the form browsers send in the Origin header.
func NormalizeOrigin(value string) (string, error) {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil ||
		(parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%q is not an origin; use the form https://shop.example.com", value)
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host), nil
}

const keyColumns = `k.id, k.tenant_id, k.environment_id, k.name, k.key_hint, k.scopes, k.allowed_origins, k.created_by, COALESCE(u.email, ''), k.expires_at, k.last_used_at, k.revoked_at, k.created_at, k.updated_at`

func scanKey(row pgx.Row) (Key, error) {
	var key Key
	err := row.Scan(&key.ID, &key.TenantID, &key.EnvironmentID, &key.Name, &key.Hint, &key.Scopes, &key.AllowedOrigins, &key.CreatedBy, &key.CreatedByEmail, &key.ExpiresAt, &key.LastUsedAt, &key.RevokedAt, &key.CreatedAt, &key.UpdatedAt)
	return key, err
}

// CreateKey stores a validated key and returns the secret, which is never
// stored and cannot be shown again. createdBy is empty for keys that no
// signed-in administrator created.
func CreateKey(ctx context.Context, db *pgxpool.Pool, spec KeySpec, createdBy string) (Key, string, error) {
	secret, err := randomToken(32)
	if err != nil {
		return Key{}, "", err
	}
	id, err := randomToken(12)
	if err != nil {
		return Key{}, "", err
	}
	raw := KeyPrefix + secret
	hint := KeyPrefix + secret[:4] + "…" + secret[len(secret)-4:]
	var expiresAt *time.Time
	if spec.ExpiresInDays > 0 {
		expiry := time.Now().UTC().Add(time.Duration(spec.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &expiry
	}
	var creator *string
	if createdBy != "" {
		creator = &createdBy
	}
	key, err := scanKey(db.QueryRow(ctx, `
		WITH k AS (
			INSERT INTO access_keys (id, tenant_id, environment_id, name, key_hash, key_hint, scopes, allowed_origins, created_by, expires_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING *
		)
		SELECT `+keyColumns+` FROM k LEFT JOIN admin_users u ON u.id = k.created_by`,
		"key_"+id, spec.TenantID, spec.EnvironmentID, spec.Name, Digest(raw), hint, spec.Scopes, spec.AllowedOrigins, creator, expiresAt))
	if err != nil {
		return Key{}, "", err
	}
	return key, raw, nil
}

func ListKeys(ctx context.Context, db *pgxpool.Pool, tenantID, environmentID string) ([]Key, error) {
	rows, err := db.Query(ctx, `
		SELECT `+keyColumns+`
		FROM access_keys k LEFT JOIN admin_users u ON u.id = k.created_by
		WHERE k.tenant_id = $1 AND ($2 = '' OR k.environment_id = $2)
		ORDER BY k.created_at DESC, k.id`, tenantID, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := make([]Key, 0)
	for rows.Next() {
		key, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func FindKey(ctx context.Context, db *pgxpool.Pool, tenantID, id string) (Key, error) {
	key, err := scanKey(db.QueryRow(ctx, `
		SELECT `+keyColumns+`
		FROM access_keys k LEFT JOIN admin_users u ON u.id = k.created_by
		WHERE k.tenant_id = $1 AND k.id = $2`, tenantID, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Key{}, ErrKeyNotFound
	}
	return key, err
}

// UpdateKey changes a key's name, scopes and allowed origins. The secret, the
// environment and the expiry stay as they are.
func UpdateKey(ctx context.Context, db *pgxpool.Pool, tenantID, id string, spec KeySpec) (Key, error) {
	return keyChange(ctx, db, `
		UPDATE access_keys SET name = $3, scopes = $4, allowed_origins = $5, updated_at = now()
		WHERE tenant_id = $1 AND id = $2 RETURNING *`, tenantID, id, spec.Name, spec.Scopes, spec.AllowedOrigins)
}

// RevokeKey stops a key from working and keeps it listed.
func RevokeKey(ctx context.Context, db *pgxpool.Pool, tenantID, id string) (Key, error) {
	return keyChange(ctx, db, `
		UPDATE access_keys SET revoked_at = COALESCE(revoked_at, now()), updated_at = now()
		WHERE tenant_id = $1 AND id = $2 RETURNING *`, tenantID, id)
}

// DeleteKey removes a key; it stops working at once.
func DeleteKey(ctx context.Context, db *pgxpool.Pool, tenantID, id string) (Key, error) {
	return keyChange(ctx, db, `DELETE FROM access_keys WHERE tenant_id = $1 AND id = $2 RETURNING *`, tenantID, id)
}

func keyChange(ctx context.Context, db *pgxpool.Pool, statement string, args ...any) (Key, error) {
	key, err := scanKey(db.QueryRow(ctx, `WITH k AS (`+statement+`) SELECT `+keyColumns+` FROM k LEFT JOIN admin_users u ON u.id = k.created_by`, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return Key{}, ErrKeyNotFound
	}
	return key, err
}

// LookupKey resolves a raw key. Revoked and expired keys, and keys of
// suspended tenants, are rejected like unknown keys.
func LookupKey(ctx context.Context, db *pgxpool.Pool, raw string) (Key, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Key{}, ErrInvalidCredential
	}
	key, err := scanKey(db.QueryRow(ctx, `
		SELECT `+keyColumns+`
		FROM access_keys k
		JOIN tenants t ON t.id = k.tenant_id AND t.status = 'active'
		LEFT JOIN admin_users u ON u.id = k.created_by
		WHERE k.key_hash = $1 AND k.revoked_at IS NULL AND (k.expires_at IS NULL OR k.expires_at > now())`, Digest(raw)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Key{}, ErrInvalidCredential
	}
	if err != nil {
		return Key{}, err
	}
	// Record use at most once a minute so busy keys do not write on every request.
	if _, err := db.Exec(ctx, `UPDATE access_keys SET last_used_at = now() WHERE id = $1 AND (last_used_at IS NULL OR last_used_at < now() - interval '1 minute')`, key.ID); err != nil {
		return Key{}, err
	}
	return key, nil
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
