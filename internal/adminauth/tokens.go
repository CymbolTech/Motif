package adminauth

import (
	"context"
	"errors"
	"slices"

	"github.com/motifengine/motifengine/internal/auth"
)

// SourceAccessKey marks principals authenticated with an access key that
// carries the MCP scope.
const SourceAccessKey = "access_key"

var ErrInvalidAccessKey = errors.New("invalid access key")

func (p Principal) IsAccessKey() bool { return p.Source == SourceAccessKey }

// HasScope reports whether the principal may use a scope. Sessions and the
// bootstrap key carry every scope; access keys carry only what was granted.
func (p Principal) HasScope(scope string) bool {
	if !p.IsAccessKey() {
		return true
	}
	return slices.Contains(p.Scopes, scope)
}

// AllowsEnvironment reports whether the principal may act in an environment.
// An access key belongs to exactly one environment.
func (p Principal) AllowsEnvironment(environmentID string) bool {
	if !p.IsAccessKey() || len(p.EnvironmentIDs) == 0 {
		return true
	}
	return slices.Contains(p.EnvironmentIDs, environmentID)
}

// LookupAccessKey resolves a raw access key to a principal confined to the
// key's workspace and environment. Keys belong to the workspace rather than
// to whoever created them, so they keep working when that administrator
// leaves; revoking or deleting the key stops it.
func (s *Store) LookupAccessKey(ctx context.Context, raw string) (Principal, error) {
	key, err := auth.LookupKey(ctx, s.db, raw)
	if errors.Is(err, auth.ErrInvalidCredential) {
		return Principal{}, ErrInvalidAccessKey
	}
	if err != nil {
		return Principal{}, err
	}
	return Principal{
		ID: key.ID, Email: key.CreatedByEmail, Role: RoleTenantAdmin, TenantIDs: []string{key.TenantID}, Source: SourceAccessKey,
		TokenID: key.ID, Scopes: key.Scopes, EnvironmentIDs: []string{key.EnvironmentID},
	}, nil
}
