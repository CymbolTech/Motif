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

func tokenPrincipal(scopes []string, environments ...string) adminauth.Principal {
	return adminauth.Principal{ID: "admin_1", Role: adminauth.RoleTenantAdmin, TenantIDs: []string{"acme"}, Source: adminauth.SourceAccessToken, TokenID: "tok_1", Scopes: scopes, EnvironmentIDs: environments}
}

func tokenRequest(method, target, body string) *http.Request {
	parsed, _ := url.Parse(target)
	request := &http.Request{Method: method, URL: parsed, Header: http.Header{}}
	if body != "" {
		request.Body = io.NopCloser(bytes.NewBufferString(body))
	}
	return request
}

func TestAccessTokenScopesGateReadsWritesAndPublishing(t *testing.T) {
	const scoped = `{"tenant_id":"acme","environment_id":"sandbox","name":"r","event_type":"order.completed"}`
	publish := "/v1/admin/rules/rule_1/versions/2/publish?tenant_id=acme&environment_id=sandbox"
	cases := []struct {
		name    string
		scopes  []string
		method  string
		target  string
		body    string
		allowed bool
	}{
		{"read lists rules", []string{"read"}, http.MethodGet, "/v1/admin/rules?tenant_id=acme&environment_id=sandbox", "", true},
		{"read checks a rule", []string{"read"}, http.MethodPost, "/v1/admin/rules/check", scoped, true},
		{"read cannot create a rule", []string{"read"}, http.MethodPost, "/v1/admin/rules", scoped, false},
		{"write creates a rule", []string{"read", "write"}, http.MethodPost, "/v1/admin/rules", scoped, true},
		{"write cannot publish", []string{"read", "write"}, http.MethodPost, publish, "", false},
		{"publish publishes", []string{"read", "publish"}, http.MethodPost, publish, "", true},
		{"write sends a test event", []string{"read", "write"}, http.MethodPost, "/v1/admin/events/test", scoped, true},
		{"write cannot award points", []string{"read", "write", "publish"}, http.MethodPost, "/v1/admin/points/transactions", scoped, false},
		{"write cannot convert points", []string{"read", "write", "publish"}, http.MethodPost, "/v1/admin/points/conversions", scoped, false},
		{"write cannot redeem", []string{"read", "write", "publish"}, http.MethodPost, "/v1/admin/redemptions/test", scoped, false},
		{"write cannot configure providers", []string{"read", "write", "publish"}, http.MethodPost, "/v1/admin/reward-providers", scoped, false},
		{"write cannot create environments", []string{"read", "write", "publish"}, http.MethodPost, "/v1/admin/environments", `{"tenant_id":"acme","id":"prod","name":"Prod"}`, false},
		{"tokens cannot manage tokens", []string{"read", "write", "publish"}, http.MethodGet, "/v1/admin/access-tokens?tenant_id=acme", "", false},
		{"tokens cannot read credentials", []string{"read", "write", "publish"}, http.MethodGet, "/v1/admin/credentials?tenant_id=acme", "", false},
		{"tokens stay inside their tenant", []string{"read"}, http.MethodGet, "/v1/admin/rules?tenant_id=globex&environment_id=sandbox", "", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := tokenRequest(testCase.method, testCase.target, testCase.body)
			if got := (&Handler{}).allowedRoute(request, tokenPrincipal(testCase.scopes)); got != testCase.allowed {
				t.Fatalf("allowedRoute() = %t, want %t", got, testCase.allowed)
			}
		})
	}
}

func TestAccessTokenEnvironmentAllowList(t *testing.T) {
	principal := tokenPrincipal([]string{"read", "write"}, "sandbox")
	cases := []struct {
		name    string
		method  string
		target  string
		body    string
		allowed bool
	}{
		{"allowed environment read", http.MethodGet, "/v1/admin/events?tenant_id=acme&environment_id=sandbox", "", true},
		{"other environment read", http.MethodGet, "/v1/admin/events?tenant_id=acme&environment_id=production", "", false},
		{"read without an environment", http.MethodGet, "/v1/admin/events?tenant_id=acme", "", false},
		{"tenant-level read", http.MethodGet, "/v1/admin/environments?tenant_id=acme", "", true},
		{"allowed environment write", http.MethodPost, "/v1/admin/event-types", `{"tenant_id":"acme","environment_id":"sandbox","name":"order.completed"}`, true},
		{"other environment write", http.MethodPost, "/v1/admin/event-types", `{"tenant_id":"acme","environment_id":"production","name":"order.completed"}`, false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := tokenRequest(testCase.method, testCase.target, testCase.body)
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

func TestSessionsKeepEveryScope(t *testing.T) {
	session := adminauth.Principal{ID: "admin_1", Role: adminauth.RoleTenantAdmin, TenantIDs: []string{"acme"}, Source: "session"}
	request := tokenRequest(http.MethodPost, "/v1/admin/rules/rule_1/versions/2/publish?tenant_id=acme&environment_id=sandbox", "")
	if !(&Handler{}).allowedRoute(request, session) {
		t.Fatal("a signed-in tenant administrator should still publish rules")
	}
	tokens := tokenRequest(http.MethodPost, "/v1/admin/access-tokens", `{"tenant_id":"acme","name":"ci"}`)
	if !(&Handler{}).allowedRoute(tokens, session) {
		t.Fatal("a signed-in tenant administrator should manage access tokens for their workspace")
	}
}

func TestValidateAccessTokenRequest(t *testing.T) {
	request, err := ValidateAccessTokenRequest(AccessTokenRequest{TenantID: "acme", Name: " Claude Code ", Scopes: []string{"publish", "read", "write", "read"}, EnvironmentIDs: []string{"sandbox", "sandbox"}})
	if err != nil {
		t.Fatalf("ValidateAccessTokenRequest() error = %v", err)
	}
	if strings.Join(request.Scopes, ",") != "read,write,publish" || len(request.EnvironmentIDs) != 1 || request.Name != "Claude Code" {
		t.Fatalf("ValidateAccessTokenRequest() = %+v", request)
	}
	for _, invalid := range []AccessTokenRequest{
		{TenantID: "acme", Name: "x", Scopes: []string{"write"}},
		{TenantID: "acme", Name: "x", Scopes: []string{"read", "admin"}},
		{TenantID: "acme", Name: "", Scopes: []string{"read"}},
		{TenantID: "acme", Name: "x", Scopes: []string{"read"}, ExpiresInDays: 400},
		{TenantID: "acme", Name: "x", Scopes: []string{"read"}, EnvironmentIDs: []string{"Bad Env"}},
	} {
		if _, err := ValidateAccessTokenRequest(invalid); err == nil {
			t.Fatalf("ValidateAccessTokenRequest(%+v) should fail", invalid)
		}
	}
}

func TestClientLabelKeepsShortSafeValues(t *testing.T) {
	request := tokenRequest(http.MethodGet, "/", "")
	request.Header.Set("X-Motif-Client", "MCP")
	if clientLabel(request) != "mcp" {
		t.Fatalf("clientLabel() = %q, want mcp", clientLabel(request))
	}
	request.Header.Set("X-Motif-Client", "mcp; drop table")
	if clientLabel(request) != "" {
		t.Fatalf("clientLabel() = %q, want empty for unsafe input", clientLabel(request))
	}
}
