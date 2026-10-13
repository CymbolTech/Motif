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
