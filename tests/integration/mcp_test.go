//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/motifengine/motifengine/internal/admin"
	"github.com/motifengine/motifengine/internal/mcpserver"
	"github.com/motifengine/motifengine/internal/points"
	"github.com/motifengine/motifengine/internal/processing"
	"github.com/nats-io/nats.go"
)

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(request)
}

func adminJSON(t *testing.T, server *httptest.Server, method, path, bearer string, body any) (int, map[string]any) {
	t.Helper()
	encoded, _ := json.Marshal(body)
	request, err := http.NewRequest(method, server.URL+path, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	payload, _ := io.ReadAll(response.Body)
	var decoded map[string]any
	_ = json.Unmarshal(payload, &decoded)
	return response.StatusCode, decoded
}

func startIsolatedWorker(t *testing.T, db testDatabase) {
	t.Helper()
	natsURL := os.Getenv("MOTIF_INTEGRATION_NATS_URL")
	parsed, err := url.Parse(natsURL)
	if natsURL == "" || err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1") {
		t.Fatal("MOTIF_INTEGRATION_NATS_URL must point to an isolated local JetStream server")
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("connect to isolated NATS: %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.StreamInfo("MOTIF_EVENTS"); err == nil {
		t.Fatal("NATS stream already exists; refusing to use a shared development server")
	} else if !errors.Is(err, nats.ErrStreamNotFound) {
		t.Fatalf("inspect NATS stream: %v", err)
	}
	t.Cleanup(func() {
		if err := js.DeleteStream("MOTIF_EVENTS"); err != nil && !errors.Is(err, nats.ErrStreamNotFound) {
			t.Errorf("delete isolated NATS stream: %v", err)
		}
	})
	workerCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	if err := processing.New(db.pool, js, slog.New(slog.NewTextHandler(io.Discard, nil))).Start(workerCtx); err != nil {
		t.Fatalf("start worker: %v", err)
	}
}

func connectMCP(t *testing.T, mcpURL, token string) (*mcp.ClientSession, error) {
	t.Helper()
	client := mcp.NewClient(&mcp.Implementation{Name: "integration", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: mcpURL, HTTPClient: &http.Client{Transport: bearerTransport{token: token}}, MaxRetries: -1}, nil)
	if err == nil {
		t.Cleanup(func() { _ = session.Close() })
	}
	return session, err
}

func callTool(t *testing.T, session *mcp.ClientSession, name string, arguments map[string]any) (map[string]any, string, bool) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	var text strings.Builder
	for _, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			text.WriteString(textContent.Text)
		}
	}
	structured, _ := result.StructuredContent.(map[string]any)
	if structured == nil {
		_ = json.Unmarshal([]byte(text.String()), &structured)
	}
	return structured, text.String(), result.IsError
}

func listToolNames(t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func TestMCPConfiguresTestsAndExplainsAWorkspace(t *testing.T) {
	db := disposableDatabase(t)
	startIsolatedWorker(t, db)
	ctx := context.Background()

	mux := http.NewServeMux()
	mux.Handle("/v1/admin/", admin.NewHandler(db.pool, "mcp-integration-admin-key"))
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)
	mcpServer := httptest.NewServer(mcpserver.NewHTTPHandler(mcpserver.Options{APIURL: api.URL, ConsoleURL: "https://console.example", PublicAPIURL: "https://console.example/api", Version: "integration"}))
	t.Cleanup(mcpServer.Close)

	status, signup := adminJSON(t, api, http.MethodPost, "/v1/admin/auth/signup", "", map[string]any{
		"email": "owner@mcp.example", "password": "a-long-integration-password", "tenant_id": "mcp-workspace", "name": "MCP Workspace",
		"environment_id": "sandbox", "environment_name": "Sandbox",
	})
	if status != http.StatusCreated {
		t.Fatalf("signup = %d %v", status, signup)
	}
	session := signup["token"].(string)
	if status, body := adminJSON(t, api, http.MethodPost, "/v1/admin/environments", session, map[string]any{"tenant_id": "mcp-workspace", "environment_id": "production", "name": "Production"}); status != http.StatusCreated {
		t.Fatalf("create production environment = %d %v", status, body)
	}
	createToken := func(name string, scopes []string, environments []string) (string, string) {
		t.Helper()
		status, body := adminJSON(t, api, http.MethodPost, "/v1/admin/access-tokens", session, map[string]any{"tenant_id": "mcp-workspace", "name": name, "scopes": scopes, "environment_ids": environments, "expires_in_days": 30})
		if status != http.StatusCreated {
			t.Fatalf("create %s token = %d %v", name, status, body)
		}
		return body["token"].(string), body["access_token"].(map[string]any)["id"].(string)
	}
	builderToken, builderID := createToken("builder", []string{"read", "write", "publish"}, []string{"sandbox"})
	writerToken, _ := createToken("writer", []string{"read", "write"}, nil)
	readerToken, readerID := createToken("reader", []string{"read"}, nil)

	if status, _ := adminJSON(t, api, http.MethodPost, "/v1/admin/access-tokens", builderToken, map[string]any{"tenant_id": "mcp-workspace", "name": "minted", "scopes": []string{"read"}}); status != http.StatusForbidden {
		t.Fatalf("an access token minting another token = %d, want 403", status)
	}

	builder, err := connectMCP(t, mcpServer.URL, builderToken)
	if err != nil {
		t.Fatalf("connect builder: %v", err)
	}
	workspace, text, failed := callTool(t, builder, "get_workspace", nil)
	if failed || !strings.Contains(text, "MCP Workspace") || strings.Contains(text, "mcp-workspace") {
		t.Fatalf("get_workspace = %s", text)
	}
	if environments := workspace["environments"].([]any); len(environments) != 1 {
		t.Fatalf("builder token environments = %v, want sandbox only", environments)
	}
	if _, text, failed := callTool(t, builder, "list_event_types", map[string]any{"environment": "production"}); !failed || !strings.Contains(text, "not available") {
		t.Fatalf("builder reaching production = %s, want a refusal", text)
	}

	steps := []struct {
		tool      string
		arguments map[string]any
	}{
		{"register_event_type", map[string]any{"name": "order.completed", "schema": map[string]any{"type": "object", "required": []any{"order_id", "total", "payment_status"}, "properties": map[string]any{"order_id": map[string]any{"type": "string"}, "total": map[string]any{"type": "number"}, "payment_status": map[string]any{"type": "string"}}}}},
		{"create_currency", map[string]any{"code": "loyalty", "name": "Loyalty points"}},
	}
	for _, step := range steps {
		if _, text, failed := callTool(t, builder, step.tool, step.arguments); failed {
			t.Fatalf("%s failed: %s", step.tool, text)
		}
	}
	rule := map[string]any{
		"event_type": "order.completed",
		"conditions": map[string]any{"all": []any{map[string]any{"eq": []any{"$.payment_status", "paid"}}, map[string]any{"gte": []any{"$.total", 120}}}},
		"actions":    []any{map[string]any{"type": "award_points", "currency": "loyalty", "amount": 50}},
	}
	check := func(sample map[string]any) map[string]any {
		t.Helper()
		arguments := map[string]any{"sample_payload": sample}
		for key, value := range rule {
			arguments[key] = value
		}
		result, text, failed := callTool(t, builder, "check_rule", arguments)
		if failed || result["valid"] != true {
			t.Fatalf("check_rule = %s", text)
		}
		return result["sample"].(map[string]any)
	}
	if sample := check(map[string]any{"order_id": "o-1", "total": 129.9, "payment_status": "paid"}); sample["matched"] != true {
		t.Fatalf("a large paid order should match: %v", sample)
	}
	if sample := check(map[string]any{"order_id": "o-2", "total": 99, "payment_status": "paid"}); sample["matched"] != false || !strings.Contains(sample["reason"].(string), "conditions.all[1]") {
		t.Fatalf("a small order should be explained as all[1]: %v", sample)
	}

	arguments := map[string]any{"name": "Large paid orders"}
	for key, value := range rule {
		arguments[key] = value
	}
	saved, text, failed := callTool(t, builder, "save_rule_draft", arguments)
	if failed {
		t.Fatalf("save_rule_draft = %s", text)
	}
	draft := saved["draft"].(map[string]any)
	ruleID := draft["id"].(string)
	if _, text, failed := callTool(t, builder, "publish_rule", map[string]any{"rule_id": ruleID, "version": 1}); failed {
		t.Fatalf("publish_rule = %s", text)
	}

	sendOrder := func(eventID string) map[string]any {
		t.Helper()
		result, text, failed := callTool(t, builder, "send_test_event", map[string]any{"event_type": "order.completed", "user_id": "test-user-1", "event_id": eventID, "payload": map[string]any{"order_id": "o-1", "total": 129.9, "payment_status": "paid"}})
		if failed {
			t.Fatalf("send_test_event = %s", text)
		}
		return result["trace"].(map[string]any)
	}
	trace := sendOrder("test_order_1")
	if trace["event"].(map[string]any)["status"] != "processed" || len(trace["point_entries"].([]any)) != 1 {
		t.Fatalf("trace after one order = %v", trace)
	}
	sendOrder("test_order_1")
	state, text, failed := callTool(t, builder, "get_user_state", map[string]any{"user_id": "test-user-1"})
	if failed || !strings.Contains(text, `"loyalty"`) {
		t.Fatalf("get_user_state = %s", text)
	}
	var awards int
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM point_ledger_entries WHERE entry_type = 'award'`).Scan(&awards); err != nil || awards != 1 {
		t.Fatalf("resending the same event awarded %d times (err %v), want once; state %v", awards, err, state)
	}

	var actorType, actorID, client string
	if err := db.pool.QueryRow(ctx, `SELECT actor_type, actor_id, metadata->>'client' FROM audit_log WHERE action = 'rule.created' ORDER BY id DESC LIMIT 1`).Scan(&actorType, &actorID, &client); err != nil {
		t.Fatal(err)
	}
	if actorType != "access_token" || actorID != builderID || client != "mcp" {
		t.Fatalf("rule.created audit = %s/%s/%s, want access_token/%s/mcp", actorType, actorID, client, builderID)
	}

	writer, err := connectMCP(t, mcpServer.URL, writerToken)
	if err != nil {
		t.Fatal(err)
	}
	if names := listToolNames(t, writer); slices.Contains(names, "publish_rule") || !slices.Contains(names, "save_rule_draft") {
		t.Fatalf("writer tools = %v", names)
	}
	if status, _ := adminJSON(t, api, http.MethodPost, "/v1/admin/rules/"+ruleID+"/versions/1/publish?tenant_id=mcp-workspace&environment_id=sandbox", writerToken, nil); status != http.StatusForbidden {
		t.Fatalf("writer publishing through the API = %d, want 403", status)
	}

	reader, err := connectMCP(t, mcpServer.URL, readerToken)
	if err != nil {
		t.Fatal(err)
	}
	if names := listToolNames(t, reader); slices.Contains(names, "save_rule_draft") || slices.Contains(names, "send_test_event") {
		t.Fatalf("reader tools = %v", names)
	}
	if status, body := adminJSON(t, api, http.MethodPost, "/v1/admin/access-tokens/"+readerID+"/revoke?tenant_id=mcp-workspace", session, nil); status != http.StatusOK {
		t.Fatalf("revoke reader = %d %v", status, body)
	}
	if status, _ := adminJSON(t, api, http.MethodGet, "/v1/admin/auth/me", readerToken, nil); status != http.StatusUnauthorized {
		t.Fatalf("revoked token = %d, want 401", status)
	}
}

// An MCP client runs the café flow: a manual weekly board rotated by hand,
// read back by key, and the ranks visible in a user's state.
func TestMCPRunsALeaderboard(t *testing.T) {
	db := disposableDatabase(t)
	ctx := context.Background()
	mux := http.NewServeMux()
	mux.Handle("/v1/admin/", admin.NewHandler(db.pool, "mcp-integration-admin-key"))
	api := httptest.NewServer(mux)
	t.Cleanup(api.Close)
	mcpServer := httptest.NewServer(mcpserver.NewHTTPHandler(mcpserver.Options{APIURL: api.URL, ConsoleURL: "https://console.example", PublicAPIURL: "https://console.example/api", Version: "integration"}))
	t.Cleanup(mcpServer.Close)

	status, signup := adminJSON(t, api, http.MethodPost, "/v1/admin/auth/signup", "", map[string]any{
		"email": "owner@cafe.example", "password": "a-long-integration-password", "tenant_id": "cafe", "name": "Cafe", "environment_id": "sandbox", "environment_name": "Sandbox",
	})
	if status != http.StatusCreated {
		t.Fatalf("signup = %d %v", status, signup)
	}
	createToken := func(scopes []string) string {
		t.Helper()
		status, body := adminJSON(t, api, http.MethodPost, "/v1/admin/access-tokens", signup["token"].(string), map[string]any{"tenant_id": "cafe", "name": strings.Join(scopes, "-"), "scopes": scopes, "expires_in_days": 30})
		if status != http.StatusCreated {
			t.Fatalf("create token = %d %v", status, body)
		}
		return body["token"].(string)
	}
	builder, err := connectMCP(t, mcpServer.URL, createToken([]string{"read", "write"}))
	if err != nil {
		t.Fatal(err)
	}
	mustCall := func(tool string, arguments map[string]any) map[string]any {
		t.Helper()
		result, text, failed := callTool(t, builder, tool, arguments)
		if failed {
			t.Fatalf("%s failed: %s", tool, text)
		}
		return result
	}
	mustCall("create_currency", map[string]any{"code": "coins", "name": "Coins"})
	mustCall("create_leaderboard", map[string]any{"key": "weekly", "name": "Weekly", "rank": "points", "currency": "coins", "rounds": "manual"})
	mustCall("create_leaderboard", map[string]any{"key": "monthly", "name": "Monthly", "rank": "points", "currency": "coins", "rounds": "recurring", "every": "month", "timezone": "Asia/Tehran"})
	if _, text, failed := callTool(t, builder, "end_leaderboard_round", map[string]any{"leaderboard": "weekly"}); !failed || !strings.Contains(text, "start_leaderboard_round") {
		t.Fatalf("ending with no round running = %s, want a hint to start one", text)
	}
	mustCall("start_leaderboard_round", map[string]any{"leaderboard": "weekly", "name": "Week 44", "ends_at": time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339)})

	ledger := points.NewLedger(db.pool)
	scope := points.Scope{TenantID: "cafe", EnvironmentID: "sandbox"}
	for index, award := range []struct {
		user   string
		amount int64
	}{{"sara", 40}, {"ali", 90}} {
		if _, err := ledger.Award(ctx, scope, award.user, "coins", award.amount, fmt.Sprintf("cafe-%d", index), nil); err != nil {
			t.Fatal(err)
		}
	}

	board := mustCall("get_leaderboard", map[string]any{"leaderboard": "weekly"})
	standings := board["standings"].(map[string]any)
	entries := standings["entries"].([]any)
	if standings["round"].(map[string]any)["name"] != "Week 44" || len(entries) != 2 || entries[0].(map[string]any)["user_id"] != "ali" {
		t.Fatalf("get_leaderboard = %v", standings)
	}
	if monthly := mustCall("get_leaderboard", map[string]any{"leaderboard": "monthly"})["standings"].(map[string]any); len(monthly["entries"].([]any)) != 2 {
		t.Fatalf("the recurring board should have counted both awards: %v", monthly)
	}
	_, text, _ := callTool(t, builder, "get_user_state", map[string]any{"user_id": "sara"})
	if !strings.Contains(text, `"rank":2`) {
		t.Fatalf("get_user_state should include sara's rank 2: %s", text)
	}

	mustCall("update_leaderboard_round", map[string]any{"leaderboard": "weekly", "name": "Week 44 (long)", "open_ended": true})
	if round := mustCall("get_leaderboard", map[string]any{"leaderboard": "weekly"})["standings"].(map[string]any)["round"].(map[string]any); round["ends_at"] != nil || round["name"] != "Week 44 (long)" {
		t.Fatalf("round after update = %v", round)
	}
	if ended := mustCall("end_leaderboard_round", map[string]any{"leaderboard": "weekly"}); ended["action"] != "ended" {
		t.Fatalf("end_leaderboard_round = %v", ended)
	}
	if round := mustCall("get_leaderboard", map[string]any{"leaderboard": "weekly"})["standings"].(map[string]any)["round"].(map[string]any); round["status"] != "ended" {
		t.Fatalf("the board should show the ended round's final standings: %v", round)
	}

	catalog := mustCall("list_catalog", map[string]any{"kind": "leaderboards"})
	var monthlyID string
	for _, item := range catalog["items"].([]any) {
		if board := item.(map[string]any); board["key"] == "monthly" {
			monthlyID = board["id"].(string)
		}
	}
	mustCall("update_catalog_item", map[string]any{"kind": "leaderboard", "id": monthlyID, "active": false})
	var active bool
	if err := db.pool.QueryRow(ctx, `SELECT active FROM leaderboards WHERE id = $1`, monthlyID).Scan(&active); err != nil || active {
		t.Fatalf("monthly board active = %v (err %v), want deactivated", active, err)
	}

	reader, err := connectMCP(t, mcpServer.URL, createToken([]string{"read"}))
	if err != nil {
		t.Fatal(err)
	}
	if names := listToolNames(t, reader); !slices.Contains(names, "get_leaderboard") || slices.Contains(names, "start_leaderboard_round") {
		t.Fatalf("reader tools = %v", names)
	}
}
