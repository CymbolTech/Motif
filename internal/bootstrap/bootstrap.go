package bootstrap

import (
	"context"
	"crypto/sha256"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/adminauth"
)

// EnsureIntegrationCredential creates the local development tenant and one
// credential without persisting the raw secret. Production provisioning will
// move to an authenticated platform-admin workflow in a later increment.
func EnsureIntegrationCredential(ctx context.Context, db *pgxpool.Pool, tenantID, environmentID, apiKey string) error {
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
		INSERT INTO integration_credentials (tenant_id, environment_id, key_hash)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id, environment_id) DO NOTHING`, tenantID, environmentID, keyHash[:]); err != nil {
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
