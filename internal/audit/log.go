package audit

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Entry struct {
	ID            int64           `json:"id"`
	TenantID      string          `json:"tenant_id,omitempty"`
	EnvironmentID string          `json:"environment_id,omitempty"`
	ActorType     string          `json:"actor_type"`
	ActorID       string          `json:"actor_id,omitempty"`
	Action        string          `json:"action"`
	ResourceType  string          `json:"resource_type"`
	ResourceID    string          `json:"resource_id,omitempty"`
	Metadata      json.RawMessage `json:"metadata"`
	CreatedAt     string          `json:"created_at"`
}

type Executor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func Record(ctx context.Context, db Executor, entry Entry) error {
	metadata := entry.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	_, err := db.Exec(ctx, `
		INSERT INTO audit_log (tenant_id, environment_id, actor_type, actor_id, action, resource_type, resource_id, metadata)
		VALUES (NULLIF($1, ''), NULLIF($2, ''), $3, NULLIF($4, ''), $5, $6, NULLIF($7, ''), $8::jsonb)`,
		entry.TenantID, entry.EnvironmentID, entry.ActorType, entry.ActorID, entry.Action, entry.ResourceType, entry.ResourceID, metadata)
	return err
}

func List(ctx context.Context, db *pgxpool.Pool, tenantID, environmentID, action string, limit int) ([]Entry, error) {
	rows, err := db.Query(ctx, `
		SELECT id, COALESCE(tenant_id, ''), COALESCE(environment_id, ''), actor_type, COALESCE(actor_id, ''), action, resource_type, COALESCE(resource_id, ''), metadata, created_at::text
		FROM audit_log
		WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2) AND ($3 = '' OR action = $3)
		ORDER BY created_at DESC, id DESC LIMIT $4`, tenantID, environmentID, action, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := make([]Entry, 0, limit)
	for rows.Next() {
		var entry Entry
		if err := rows.Scan(&entry.ID, &entry.TenantID, &entry.EnvironmentID, &entry.ActorType, &entry.ActorID, &entry.Action, &entry.ResourceType, &entry.ResourceID, &entry.Metadata, &entry.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

var _ Executor = (*pgxpool.Pool)(nil)
var _ Executor = (pgx.Tx)(nil)
