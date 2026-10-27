package bootstrap

import (
	"context"
	"crypto/sha256"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/adminauth"
)

// EnsureAccessKey creates the operator's bootstrap tenant and an access key
// that reads and writes through the REST API, without persisting the raw
// secret. Workspaces created by sign-up get no key; their administrators
// create keys in the console.
func EnsureAccessKey(ctx context.Context, db *pgxpool.Pool, tenantID, environmentID, apiKey string) error {
	keyHash := sha256.Sum256([]byte(apiKey))
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		INSERT INTO tenants (id, name) VALUES ($1, $1)
		ON CONFLICT (id) DO NOTHING`, tenantID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO environments (tenant_id, id, name) VALUES ($1, $2, $2)
		ON CONFLICT (tenant_id, id) DO NOTHING`, tenantID, environmentID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tenant_plans (tenant_id, plan_id) VALUES ($1, 'starter')
		ON CONFLICT (tenant_id) DO NOTHING`, tenantID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO access_keys (id, tenant_id, environment_id, name, key_hash, scopes)
		VALUES ('key_bootstrap_' || substr(encode($3, 'hex'), 1, 12), $1, $2, 'Bootstrap key', $3, ARRAY['read', 'write'])
		ON CONFLICT DO NOTHING`, tenantID, environmentID, keyHash[:]); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// EnsureSuperAdmin creates or reactivates the configured local bootstrap
// administrator. The password is stored as a bcrypt hash and is never
// persisted or returned in plaintext.
func EnsureSuperAdmin(ctx context.Context, db *pgxpool.Pool, email, password string) error {
	return adminauth.NewStore(db).EnsureSuperAdmin(ctx, email, password)
}
