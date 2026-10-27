package admin

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/motifengine/motifengine/internal/adminauth"
)

func keyPrincipal(scopes ...string) adminauth.Principal {
	return adminauth.Principal{ID: "key_1", Role: adminauth.RoleTenantAdmin, TenantIDs: []string{"acme"}, Source: adminauth.SourceAccessKey, TokenID: "key_1", Scopes: scopes, EnvironmentIDs: []string{"sandbox"}}
}

func keyRequest(method, target, body string) *http.Request {
	parsed, _ := url.Parse(target)
	request := &http.Request{Method: method, URL: parsed, Header: http.Header{}}
	if body != "" {
		request.Body = io.NopCloser(bytes.NewBufferString(body))
	}
	return request
}

func TestAccessKeyScopesGateTheAdminAPI(t *testing.T) {
	const scoped = `{"tenant_id":"acme","environment_id":"sandbox","name":"r","event_type":"order.completed"}`
	publish := "/v1/admin/rules/rule_1/versions/2/publish?tenant_id=acme&environment_id=sandbox"
	every := []string{"read", "write", "edit", "delete", "mcp"}
	cases := []struct {
		name    string
		scopes  []string
		method  string
		target  string
		body    string
		allowed bool
	}{
		{"without mcp a key cannot reach the admin API", []string{"read", "write", "edit", "delete"}, http.MethodGet, "/v1/admin/rules?tenant_id=acme&environment_id=sandbox", "", false},
		{"read lists rules", []string{"read", "mcp"}, http.MethodGet, "/v1/admin/rules?tenant_id=acme&environment_id=sandbox", "", true},
		{"read checks a rule", []string{"read", "mcp"}, http.MethodPost, "/v1/admin/rules/check", scoped, true},
		{"read cannot create a rule", []string{"read", "mcp"}, http.MethodPost, "/v1/admin/rules", scoped, false},
		{"write creates a rule", []string{"read", "write", "mcp"}, http.MethodPost, "/v1/admin/rules", scoped, true},
		{"write saves a rule draft", []string{"read", "write", "mcp"}, http.MethodPost, "/v1/admin/rules/rule_1/versions?tenant_id=acme&environment_id=sandbox", "", true},
		{"write cannot publish", []string{"read", "write", "mcp"}, http.MethodPost, publish, "", false},
		{"edit publishes", []string{"read", "edit", "mcp"}, http.MethodPost, publish, "", true},
		{"write cannot change a currency", []string{"read", "write", "mcp"}, http.MethodPatch, "/v1/admin/currencies/coins", `{"tenant_id":"acme","environment_id":"sandbox"}`, false},
		{"edit changes a currency", []string{"read", "edit", "mcp"}, http.MethodPatch, "/v1/admin/currencies/coins", `{"tenant_id":"acme","environment_id":"sandbox"}`, true},
		{"edit deactivates a catalog item", []string{"read", "edit", "mcp"}, http.MethodPost, "/v1/admin/catalog-status", `{"tenant_id":"acme","environment_id":"sandbox"}`, true},
		{"write starts a round", []string{"read", "write", "mcp"}, http.MethodPost, "/v1/admin/leaderboards/lb_1/rounds", `{"tenant_id":"acme","environment_id":"sandbox"}`, true},
		{"write cannot end a round", []string{"read", "write", "mcp"}, http.MethodPost, "/v1/admin/leaderboards/lb_1/rounds/r_1/end", `{"tenant_id":"acme","environment_id":"sandbox"}`, false},
		{"edit ends a round", []string{"read", "edit", "mcp"}, http.MethodPost, "/v1/admin/leaderboards/lb_1/rounds/r_1/end", `{"tenant_id":"acme","environment_id":"sandbox"}`, true},
		{"edit cannot delete", []string{"read", "write", "edit", "mcp"}, http.MethodDelete, "/v1/admin/leaderboards/lb_1?tenant_id=acme&environment_id=sandbox", "", false},
		{"delete deletes a leaderboard", []string{"read", "delete", "mcp"}, http.MethodDelete, "/v1/admin/leaderboards/lb_1?tenant_id=acme&environment_id=sandbox", "", true},
		{"write sends a test event", []string{"read", "write", "mcp"}, http.MethodPost, "/v1/admin/events/test", scoped, true},
		{"keys cannot award points", every, http.MethodPost, "/v1/admin/points/transactions", scoped, false},
		{"keys cannot convert points", every, http.MethodPost, "/v1/admin/points/conversions", scoped, false},
		{"keys cannot redeem", every, http.MethodPost, "/v1/admin/redemptions/test", scoped, false},
		{"keys cannot configure providers", every, http.MethodPost, "/v1/admin/reward-providers", scoped, false},
		{"keys cannot create environments", every, http.MethodPost, "/v1/admin/environments", `{"tenant_id":"acme","id":"prod","name":"Prod"}`, false},
		{"keys cannot manage keys", every, http.MethodGet, "/v1/admin/access-keys?tenant_id=acme", "", false},
		{"keys stay inside their tenant", every, http.MethodGet, "/v1/admin/rules?tenant_id=globex&environment_id=sandbox", "", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := keyRequest(testCase.method, testCase.target, testCase.body)
			if got := (&Handler{}).allowedRoute(request, keyPrincipal(testCase.scopes...)); got != testCase.allowed {
				t.Fatalf("allowedRoute() = %t, want %t", got, testCase.allowed)
			}
		})
	}
}

func TestAccessKeyStaysInItsEnvironment(t *testing.T) {
	principal := keyPrincipal("read", "write", "mcp")
	cases := []struct {
		name    string
		method  string
		target  string
		body    string
		allowed bool
	}{
		{"its environment read", http.MethodGet, "/v1/admin/events?tenant_id=acme&environment_id=sandbox", "", true},
		{"other environment read", http.MethodGet, "/v1/admin/events?tenant_id=acme&environment_id=production", "", false},
		{"read without an environment", http.MethodGet, "/v1/admin/events?tenant_id=acme", "", false},
		{"tenant-level read", http.MethodGet, "/v1/admin/environments?tenant_id=acme", "", true},
		{"its environment write", http.MethodPost, "/v1/admin/event-types", `{"tenant_id":"acme","environment_id":"sandbox","name":"order.completed"}`, true},
		{"other environment write", http.MethodPost, "/v1/admin/event-types", `{"tenant_id":"acme","environment_id":"production","name":"order.completed"}`, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := keyRequest(testCase.method, testCase.target, testCase.body)
			if got := (&Handler{}).allowedRoute(request, principal); got != testCase.allowed {
				t.Fatalf("allowedRoute() = %t, want %t", got, testCase.allowed)
			}
			if testCase.body != "" {
				restored, _ := io.ReadAll(request.Body)
				if !strings.Contains(string(restored), `"tenant_id"`) {
					t.Fatal("the request body must be restored for the handler")
				}
			}
		})
	}
}

func TestSessionsManageAccessKeys(t *testing.T) {
	session := adminauth.Principal{ID: "admin_1", Role: adminauth.RoleTenantAdmin, TenantIDs: []string{"acme"}, Source: "session"}
	cases := []struct {
		name    string
		method  string
		target  string
		body    string
		allowed bool
	}{
		{"publish a rule", http.MethodPost, "/v1/admin/rules/rule_1/versions/2/publish?tenant_id=acme&environment_id=sandbox", "", true},
		{"list keys", http.MethodGet, "/v1/admin/access-keys?tenant_id=acme&environment_id=sandbox", "", true},
		{"create a key", http.MethodPost, "/v1/admin/access-keys", `{"tenant_id":"acme","environment_id":"sandbox","name":"backend"}`, true},
		{"create a key elsewhere", http.MethodPost, "/v1/admin/access-keys", `{"tenant_id":"globex","environment_id":"sandbox","name":"backend"}`, false},
		{"edit a key", http.MethodPatch, "/v1/admin/access-keys/key_1", `{"tenant_id":"acme","name":"renamed"}`, true},
		{"edit a key elsewhere", http.MethodPatch, "/v1/admin/access-keys/key_1", `{"tenant_id":"globex","name":"renamed"}`, false},
		{"revoke a key", http.MethodPost, "/v1/admin/access-keys/key_1/revoke?tenant_id=acme", "", true},
		{"delete a key", http.MethodDelete, "/v1/admin/access-keys/key_1?tenant_id=acme", "", true},
		{"delete a key elsewhere", http.MethodDelete, "/v1/admin/access-keys/key_1?tenant_id=globex", "", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := (&Handler{}).allowedRoute(keyRequest(testCase.method, testCase.target, testCase.body), session); got != testCase.allowed {
				t.Fatalf("allowedRoute() = %t, want %t", got, testCase.allowed)
			}
		})
	}
}

func TestParseAccessKeyPath(t *testing.T) {
	for path, want := range map[string][2]string{
		"/v1/admin/access-keys":              {"", ""},
		"/v1/admin/access-keys/key_1":        {"key_1", ""},
		"/v1/admin/access-keys/key_1/revoke": {"key_1", "revoke"},
	} {
		id, action, ok := parseAccessKeyPath(path)
		if !ok || id != want[0] || action != want[1] {
			t.Errorf("parseAccessKeyPath(%q) = %q, %q, %t", path, id, action, ok)
		}
	}
	for _, path := range []string{"/v1/admin/access-keys/", "/v1/admin/access-keys/key_1/rotate", "/v1/admin/access-keysx"} {
		if _, _, ok := parseAccessKeyPath(path); ok {
			t.Errorf("parseAccessKeyPath(%q) accepted an unknown route", path)
		}
	}
}

func TestClientLabelKeepsShortSafeValues(t *testing.T) {
	request := keyRequest(http.MethodGet, "/", "")
	request.Header.Set("X-Motif-Client", "MCP")
	if clientLabel(request) != "mcp" {
		t.Fatalf("clientLabel() = %q, want mcp", clientLabel(request))
	}
	request.Header.Set("X-Motif-Client", "mcp; drop table")
	if clientLabel(request) != "" {
		t.Fatalf("clientLabel() = %q, want empty for unsafe input", clientLabel(request))
	}
}
