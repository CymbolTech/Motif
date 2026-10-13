package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidCredential = errors.New("invalid credential")
	ErrTenantSuspended   = errors.New("tenant is suspended")
)

type Scope struct {
	TenantID      string
	EnvironmentID string
}

func Digest(value string) []byte {
	digest := sha256.Sum256([]byte(value))
	return digest[:]
}

// LookupIntegrationCredential resolves the tenant and environment from a
// hashed credential. Suspended tenants behave like revoked credentials so a
// credential cannot be used to read or write data after suspension.
func LookupIntegrationCredential(ctx context.Context, db *pgxpool.Pool, apiKey string) (Scope, error) {
	var scope Scope
	var credentialID int64
	err := db.QueryRow(ctx, `
		SELECT c.id, c.tenant_id, c.environment_id
		FROM integration_credentials c
		JOIN tenants t ON t.id = c.tenant_id
		WHERE c.key_hash = $1 AND c.revoked_at IS NULL AND t.status = 'active'`, Digest(apiKey)).
		Scan(&credentialID, &scope.TenantID, &scope.EnvironmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Scope{}, ErrInvalidCredential
	}
	if err != nil {
		return Scope{}, err
	}
	_, _ = db.Exec(ctx, `UPDATE integration_credentials SET last_used_at = $2 WHERE id = $1`, credentialID, time.Now().UTC())
	return scope, nil
}

func EnsureActiveTenant(ctx context.Context, db *pgxpool.Pool, tenantID, environmentID string) error {
	var status string
	err := db.QueryRow(ctx, `SELECT t.status FROM tenants t JOIN environments e ON e.tenant_id = t.id AND e.id = $2 WHERE t.id = $1`, tenantID, environmentID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrInvalidCredential
	}
	if err != nil {
		return err
	}
	if status != "active" {
		return ErrTenantSuspended
	}
	return nil
}
