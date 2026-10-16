package audit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type recordingExecutor struct {
	query string
	args  []any
}

func (r *recordingExecutor) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	r.query, r.args = query, args
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func TestRecordDefaultsMetadataToAnObject(t *testing.T) {
	recorder := &recordingExecutor{}
	if err := Record(context.Background(), recorder, Entry{ActorType: "system", Action: "test", ResourceType: "fixture"}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	metadata, ok := recorder.args[len(recorder.args)-1].(json.RawMessage)
	if !ok || string(metadata) != `{}` {
		t.Fatalf("Record() metadata = %q, want {}", metadata)
	}
}

func TestRecordAttributesPlatformWritesToTheContextActor(t *testing.T) {
	recorder := &recordingExecutor{}
	ctx := WithActor(context.Background(), Actor{Type: "access_token", ID: "tok_1", Client: "mcp"})
	entry := Entry{ActorType: "platform_admin", Action: "rule.created", ResourceType: "rule", Metadata: json.RawMessage(`{"version":2}`)}
	if err := Record(ctx, recorder, entry); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if recorder.args[2] != "access_token" || recorder.args[3] != "tok_1" {
		t.Fatalf("Record() actor = %v/%v, want access_token/tok_1", recorder.args[2], recorder.args[3])
	}
	var metadata map[string]any
	if err := json.Unmarshal(recorder.args[len(recorder.args)-1].(json.RawMessage), &metadata); err != nil {
		t.Fatalf("metadata is not JSON: %v", err)
	}
	if metadata["client"] != "mcp" || metadata["version"] != float64(2) {
		t.Fatalf("metadata = %v, want client and original fields", metadata)
	}
}

func TestRecordKeepsExplicitActors(t *testing.T) {
	recorder := &recordingExecutor{}
	ctx := WithActor(context.Background(), Actor{Type: "access_token", ID: "tok_1"})
	if err := Record(ctx, recorder, Entry{ActorType: "tenant_admin", ActorID: "admin_1", Action: "admin.login", ResourceType: "admin_session"}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if recorder.args[2] != "tenant_admin" || recorder.args[3] != "admin_1" {
		t.Fatalf("Record() actor = %v/%v, want tenant_admin/admin_1", recorder.args[2], recorder.args[3])
	}
}
