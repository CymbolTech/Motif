//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/admin"
	"github.com/motifengine/motifengine/internal/auth"
	"github.com/motifengine/motifengine/internal/events"
	"github.com/motifengine/motifengine/internal/leaderboards"
)

// keyed authenticates a REST handler with access keys, as the API server does.
func keyed(db *pgxpool.Pool, handler http.Handler) http.Handler {
	return auth.RequireKey(func(ctx context.Context, raw string) (auth.Key, error) { return auth.LookupKey(ctx, db, raw) }, handler)
}

func restCall(t *testing.T, server *httptest.Server, method, path, key, origin string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, _ := json.Marshal(body)
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(auth.HeaderAPIKey, key)
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var decoded map[string]any
	_ = json.NewDecoder(response.Body).Decode(&decoded)
	return response.StatusCode, decoded
}

func errorCode(body map[string]any) string {
	if failure, ok := body["error"].(map[string]any); ok {
		code, _ := failure["code"].(string)
		return code
	}
	return ""
}

// A workspace starts without keys, creates the ones it needs, and each key is
// held to its scopes and allowed origins until it is revoked or deleted.
func TestAccessKeysGuardTheRESTAPI(t *testing.T) {
	db := disposableDatabase(t)
	mux := http.NewServeMux()
	mux.Handle("/v1/admin/", admin.NewHandler(db.pool, "keys-admin-key"))
	mux.Handle("/v1/events", keyed(db.pool, events.NewHandler(db.pool)))
	mux.Handle("/v1/leaderboards", keyed(db.pool, leaderboards.NewHandler(db.pool)))
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	status, signup := adminJSON(t, server, http.MethodPost, "/v1/admin/auth/signup", "", map[string]any{
		"email": "owner@keys.example", "password": "a-long-integration-password", "tenant_id": "keys", "name": "Keys", "environment_id": "production", "environment_name": "Production",
	})
	if status != http.StatusCreated {
		t.Fatalf("signup = %d %v", status, signup)
	}
	if _, ok := signup["integration_key"]; ok {
		t.Fatal("sign-up must not hand out a key")
	}
	session := signup["token"].(string)
	if status, listed := adminJSON(t, server, http.MethodGet, "/v1/admin/access-keys?tenant_id=keys&environment_id=production", session, nil); status != http.StatusOK || len(listed["access_keys"].([]any)) != 0 {
		t.Fatalf("a new workspace's keys = %d %v, want none", status, listed)
	}

	create := func(name string, scopes []string, origins []string) (string, string) {
		t.Helper()
		status, body := adminJSON(t, server, http.MethodPost, "/v1/admin/access-keys", session, map[string]any{"tenant_id": "keys", "environment_id": "production", "name": name, "scopes": scopes, "allowed_origins": origins})
		if status != http.StatusCreated {
			t.Fatalf("create %s = %d %v", name, status, body)
		}
		return body["key"].(string), body["access_key"].(map[string]any)["id"].(string)
	}
	browserKey, browserID := create("Website", []string{"read"}, []string{"https://shop.example.com/"})
	backendKey, backendID := create("Backend", []string{"read", "write"}, nil)
	if status, body := adminJSON(t, server, http.MethodPost, "/v1/admin/access-keys", session, map[string]any{"tenant_id": "keys", "environment_id": "production", "name": "x", "scopes": []string{"write", "mcp"}}); status != http.StatusBadRequest {
		t.Fatalf("mcp without read = %d %v, want 400", status, body)
	}

	event := map[string]any{"event_id": "order-1", "event_type": "order.completed", "user_id": "u1", "payload": map[string]any{"total": 10}}
	checks := []struct {
		name, method, path, key, origin string
		body                            any
		status                          int
		code                            string
	}{
		{"read from the allowed origin", http.MethodGet, "/v1/leaderboards", browserKey, "https://shop.example.com", nil, http.StatusOK, ""},
		{"read from another origin", http.MethodGet, "/v1/leaderboards", browserKey, "https://evil.example", nil, http.StatusForbidden, "origin_not_allowed"},
		{"read from a server", http.MethodGet, "/v1/leaderboards", browserKey, "", nil, http.StatusOK, ""},
		{"write without the write scope", http.MethodPost, "/v1/events", browserKey, "https://shop.example.com", event, http.StatusForbidden, "insufficient_scope"},
		{"write with the write scope", http.MethodPost, "/v1/events", backendKey, "", event, http.StatusAccepted, ""},
		{"an open key from any origin", http.MethodGet, "/v1/leaderboards", backendKey, "https://anywhere.example", nil, http.StatusOK, ""},
		{"an unknown key", http.MethodGet, "/v1/leaderboards", "motif_key_unknown", "", nil, http.StatusUnauthorized, "invalid_api_key"},
	}
	for _, check := range checks {
		status, body := restCall(t, server, check.method, check.path, check.key, check.origin, check.body)
		if status != check.status || (check.code != "" && errorCode(body) != check.code) {
			t.Fatalf("%s = %d %v, want %d %s", check.name, status, body, check.status, check.code)
		}
	}

	if status, body := adminJSON(t, server, http.MethodPatch, "/v1/admin/access-keys/"+browserID, session, map[string]any{"tenant_id": "keys", "scopes": []string{"read", "write"}, "allowed_origins": []string{}}); status != http.StatusOK || len(body["allowed_origins"].([]any)) != 0 {
		t.Fatalf("edit website key = %d %v", status, body)
	}
	if status, body := restCall(t, server, http.MethodGet, "/v1/leaderboards", browserKey, "https://evil.example", nil); status != http.StatusOK {
		t.Fatalf("a key opened to every origin = %d %v", status, body)
	}

	if status, body := adminJSON(t, server, http.MethodPost, "/v1/admin/access-keys/"+backendID+"/revoke?tenant_id=keys", session, nil); status != http.StatusOK || body["revoked_at"] == nil {
		t.Fatalf("revoke = %d %v", status, body)
	}
	if status, _ := restCall(t, server, http.MethodGet, "/v1/leaderboards", backendKey, "", nil); status != http.StatusUnauthorized {
		t.Fatalf("a revoked key = %d, want 401", status)
	}
	if status, _ := adminJSON(t, server, http.MethodDelete, "/v1/admin/access-keys/"+browserID+"?tenant_id=keys", session, nil); status != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204", status)
	}
	if status, _ := restCall(t, server, http.MethodGet, "/v1/leaderboards", browserKey, "", nil); status != http.StatusUnauthorized {
		t.Fatalf("a deleted key = %d, want 401", status)
	}
	if _, listed := adminJSON(t, server, http.MethodGet, "/v1/admin/access-keys?tenant_id=keys", session, nil); len(listed["access_keys"].([]any)) != 1 {
		t.Fatalf("keys after revoke and delete = %v, want the revoked key only", listed)
	}

	var actions []string
	rows, err := db.pool.Query(context.Background(), `SELECT action FROM audit_log WHERE resource_type = 'access_key' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			t.Fatal(err)
		}
		actions = append(actions, action)
	}
	if len(actions) != 5 || actions[4] != "access_key.deleted" {
		t.Fatalf("audit = %v, want two creates, an update, a revoke and a delete", actions)
	}
}
