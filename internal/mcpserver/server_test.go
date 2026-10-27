package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// fakeAPI answers the admin routes the server needs and records what it received.
type fakeAPI struct {
	mu           sync.Mutex
	scopes       []string
	environments []string
	requests     []recordedRequest
}

type recordedRequest struct {
	Method string
	Path   string
	Query  string
	Body   map[string]any
	Client string
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") != "Bearer motif_at_valid" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"code":"invalid_admin_session","message":"a valid admin session is required"}}`)
		return
	}
	var body map[string]any
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	f.mu.Lock()
	f.requests = append(f.requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: body, Client: r.Header.Get(ClientHeader)})
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	write := func(value any) { _ = json.NewEncoder(w).Encode(value) }
	switch r.URL.Path {
	case "/v1/admin/auth/me":
		write(map[string]any{"id": "admin_1", "email": "owner@acme.example", "role": "tenant_admin", "tenant_ids": []string{"acme"}, "source": "access_token", "token_id": "tok_1", "scopes": f.scopes, "environment_ids": f.environments})
	case "/v1/admin/tenants":
		write([]map[string]any{{"tenant_id": "acme", "name": "Acme Rewards", "environment_id": "sandbox", "environment_name": "Sandbox", "status": "active"}})
	case "/v1/admin/environments":
		write([]map[string]any{{"tenant_id": "acme", "environment_id": "sandbox", "name": "Sandbox"}, {"tenant_id": "acme", "environment_id": "production", "name": "Production"}})
	case "/v1/admin/usage":
		write(map[string]any{"tenant_id": "acme", "rules": 1})
	case "/v1/admin/diagnostics":
		write(map[string]any{"tenant_id": "acme", "events": 3})
	case "/v1/admin/event-types":
		write([]map[string]any{{"tenant_id": "acme", "name": "order.completed", "schema": map[string]any{"type": "object", "properties": map[string]any{"order_id": map[string]any{"type": "string"}, "total": map[string]any{"type": "number", "minimum": 0}, "payment_status": map[string]any{"type": "string", "enum": []string{"paid", "pending"}}}}}})
	case "/v1/admin/leaderboards/weekly/standings":
		write(map[string]any{"round": map[string]any{"id": "round_7", "status": "active"}, "entries": []any{}})
	case "/v1/admin/leaderboards/weekly/rounds":
		write([]map[string]any{{"id": "round_8", "status": "scheduled"}, {"id": "round_7", "status": "active"}})
	case "/v1/admin/rules/check":
		write(map[string]any{"valid": true, "errors": []string{}, "sample": map[string]any{"matched": false, "reason": "conditions.all[1]: $.total (99) >= 120 is false"}})
	default:
		write(map[string]any{"tenant_id": "acme", "ok": true})
	}
}

func (f *fakeAPI) last(path string) recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	for index := len(f.requests) - 1; index >= 0; index-- {
		if f.requests[index].Path == path {
			return f.requests[index]
		}
	}
	return recordedRequest{}
}

type bearer struct {
	token string
	next  http.RoundTripper
}

func (b bearer) RoundTrip(request *http.Request) (*http.Response, error) {
	request = request.Clone(request.Context())
	request.Header.Set("Authorization", "Bearer "+b.token)
	return b.next.RoundTrip(request)
}

func connect(t *testing.T, api *fakeAPI, token string) (*mcp.ClientSession, error) {
	t.Helper()
	apiServer := httptest.NewServer(api)
	t.Cleanup(apiServer.Close)
	mcpServer := httptest.NewServer(NewHTTPHandler(Options{APIURL: apiServer.URL, ConsoleURL: "https://console.example", PublicAPIURL: "https://console.example/api", Version: "test"}))
	t.Cleanup(mcpServer.Close)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	transport := &mcp.StreamableClientTransport{Endpoint: mcpServer.URL, HTTPClient: &http.Client{Transport: bearer{token: token, next: http.DefaultTransport}}, MaxRetries: -1}
	session, err := client.Connect(context.Background(), transport, nil)
	if err == nil {
		t.Cleanup(func() { _ = session.Close() })
	}
	return session, err
}

func toolNames(t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()
	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	names := []string{}
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func call(t *testing.T, session *mcp.ClientSession, name string, arguments map[string]any) (*mcp.CallToolResult, string) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: arguments})
	if err != nil {
		t.Fatalf("CallTool(%s) error = %v", name, err)
	}
	var text strings.Builder
	for _, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			text.WriteString(textContent.Text)
		}
	}
	return result, text.String()
}

func TestToolsFollowTheTokenScopes(t *testing.T) {
	readOnly, err := connect(t, &fakeAPI{scopes: []string{"read"}}, "motif_at_valid")
	if err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	names := toolNames(t, readOnly)
	for _, want := range []string{"get_workspace", "read_docs", "check_rule", "get_event_trace", "get_integration_snippet"} {
		if !slices.Contains(names, want) {
			t.Fatalf("read-only tools %v are missing %s", names, want)
		}
	}
	for _, hidden := range []string{"save_rule_draft", "send_test_event", "publish_rule", "create_currency"} {
		if slices.Contains(names, hidden) {
			t.Fatalf("read-only token should not see %s", hidden)
		}
	}

	writer, err := connect(t, &fakeAPI{scopes: []string{"read", "write"}}, "motif_at_valid")
	if err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	names = toolNames(t, writer)
	if !slices.Contains(names, "save_rule_draft") || slices.Contains(names, "publish_rule") {
		t.Fatalf("write token tools = %v, want drafts without publishing", names)
	}

	publisher, err := connect(t, &fakeAPI{scopes: []string{"read", "write", "publish"}}, "motif_at_valid")
	if err != nil {
		t.Fatalf("connect() error = %v", err)
	}
	if !slices.Contains(toolNames(t, publisher), "publish_rule") {
		t.Fatal("publish token should see publish_rule")
	}
}

func TestInvalidTokensCannotConnect(t *testing.T) {
	if _, err := connect(t, &fakeAPI{scopes: []string{"read"}}, "motif_at_revoked"); err == nil {
		t.Fatal("a token the API rejects must not connect")
	}
	if _, err := connect(t, &fakeAPI{scopes: []string{"read"}}, "an-admin-session-token"); err == nil {
		t.Fatal("only workspace access tokens may connect")
	}
}

func TestWorkspaceOutputHidesWorkspaceIdentifiers(t *testing.T) {
	session, err := connect(t, &fakeAPI{scopes: []string{"read"}, environments: []string{"sandbox"}}, "motif_at_valid")
	if err != nil {
		t.Fatal(err)
	}
	result, text := call(t, session, "get_workspace", nil)
	if result.IsError || !strings.Contains(text, "Acme Rewards") {
		t.Fatalf("get_workspace = %s", text)
	}
	if strings.Contains(text, "tenant") || strings.Contains(text, "acme\"") {
		t.Fatalf("get_workspace leaked workspace identifiers: %s", text)
	}
	if strings.Contains(text, "production") {
		t.Fatalf("an environment-restricted token should only list its environments: %s", text)
	}
}

func TestEnvironmentMustBeChosenWhenSeveralAreAvailable(t *testing.T) {
	session, err := connect(t, &fakeAPI{scopes: []string{"read"}}, "motif_at_valid")
	if err != nil {
		t.Fatal(err)
	}
	result, text := call(t, session, "list_event_types", nil)
	if !result.IsError || !strings.Contains(text, "sandbox") || !strings.Contains(text, "production") {
		t.Fatalf("list_event_types without environment = %s, want an error naming the choices", text)
	}
	result, _ = call(t, session, "list_event_types", map[string]any{"environment": "staging"})
	if !result.IsError {
		t.Fatal("an environment outside the token must be rejected before calling the API")
	}
}

func TestCheckRuleSendsTheScopedRuleAndSample(t *testing.T) {
	api := &fakeAPI{scopes: []string{"read"}, environments: []string{"sandbox"}}
	session, err := connect(t, api, "motif_at_valid")
	if err != nil {
		t.Fatal(err)
	}
	result, text := call(t, session, "check_rule", map[string]any{
		"event_type":     "order.completed",
		"conditions":     map[string]any{"all": []any{map[string]any{"eq": []any{"$.payment_status", "paid"}}, map[string]any{"gte": []any{"$.total", 120}}}},
		"actions":        []any{map[string]any{"type": "award_points", "currency": "loyalty", "amount": 50}},
		"sample_payload": map[string]any{"payment_status": "paid", "total": 99},
	})
	if result.IsError || !strings.Contains(text, "conditions.all[1]") {
		t.Fatalf("check_rule = %s", text)
	}
	sent := api.last("/v1/admin/rules/check")
	if sent.Body["tenant_id"] != "acme" || sent.Body["environment_id"] != "sandbox" || sent.Body["sample_payload"] == nil || sent.Client != "mcp" {
		t.Fatalf("check_rule sent %+v", sent)
	}
}

func TestAPIErrorsCarryRecoveryHints(t *testing.T) {
	err := (&APIError{Status: http.StatusForbidden, Code: "admin_forbidden", Message: "not allowed"}).Error()
	if !strings.Contains(err, "permission") {
		t.Fatalf("APIError = %q, want a permission hint", err)
	}
}

func TestDocsAreServedAsToolAndResources(t *testing.T) {
	session, err := connect(t, &fakeAPI{scopes: []string{"read"}, environments: []string{"sandbox"}}, "motif_at_valid")
	if err != nil {
		t.Fatal(err)
	}
	_, text := call(t, session, "read_docs", map[string]any{"topic": "rule-language"})
	if !strings.Contains(text, "# Rule language") {
		t.Fatalf("read_docs = %.80s", text)
	}
	resource, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "motif://docs/quickstart"})
	if err != nil || len(resource.Contents) != 1 || !strings.Contains(resource.Contents[0].Text, "check_rule") {
		t.Fatalf("ReadResource(quickstart) = %v, %v", resource, err)
	}
	for _, topic := range docTopics {
		text, err := docText(topic.Name)
		if err != nil || strings.Contains(strings.ToLower(text), "tenant") {
			t.Fatalf("doc %s must exist and use workspace language", topic.Name)
		}
	}
}

func TestPromptsAreAvailable(t *testing.T) {
	session, err := connect(t, &fakeAPI{scopes: []string{"read"}, environments: []string{"sandbox"}}, "motif_at_valid")
	if err != nil {
		t.Fatal(err)
	}
	prompt, err := session.GetPrompt(context.Background(), &mcp.GetPromptParams{Name: "integrate_app", Arguments: map[string]string{"goal": "points for paid orders"}})
	if err != nil || len(prompt.Messages) != 1 {
		t.Fatalf("GetPrompt(integrate_app) = %v, %v", prompt, err)
	}
	text := prompt.Messages[0].Content.(*mcp.TextContent).Text
	if !strings.Contains(text, "points for paid orders") || !strings.Contains(text, "check_rule") {
		t.Fatalf("integrate_app prompt = %.120s", text)
	}
}
