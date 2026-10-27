package auth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDigestIsStableAndDoesNotExposePlaintext(t *testing.T) {
	first := Digest("motif_test_secret")
	second := Digest("motif_test_secret")
	if !bytes.Equal(first, second) {
		t.Fatal("Digest returned different values for the same secret")
	}
	if bytes.Contains(first, []byte("motif_test_secret")) {
		t.Fatal("Digest contains the plaintext secret")
	}
}

func TestValidateKeySpec(t *testing.T) {
	spec, err := ValidateKeySpec(KeySpec{
		TenantID: "acme", EnvironmentID: "production", Name: " Backend ",
		Scopes:         []string{"mcp", "write", "read", "write"},
		AllowedOrigins: []string{"https://Shop.Example.com/", "https://shop.example.com", "http://localhost:3000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != "Backend" || strings.Join(spec.Scopes, ",") != "read,write,mcp" {
		t.Fatalf("spec = %+v; want trimmed name and scopes in display order", spec)
	}
	if strings.Join(spec.AllowedOrigins, " ") != "https://shop.example.com http://localhost:3000" {
		t.Fatalf("origins = %v; want normalized and deduplicated", spec.AllowedOrigins)
	}
	valid := KeySpec{TenantID: "acme", EnvironmentID: "production", Name: "x", Scopes: []string{"read"}}
	for name, mutate := range map[string]func(*KeySpec){
		"no scopes":         func(s *KeySpec) { s.Scopes = nil },
		"unknown scope":     func(s *KeySpec) { s.Scopes = []string{"admin"} },
		"publish is gone":   func(s *KeySpec) { s.Scopes = []string{"publish"} },
		"mcp without read":  func(s *KeySpec) { s.Scopes = []string{"write", "mcp"} },
		"no name":           func(s *KeySpec) { s.Name = " " },
		"no environment":    func(s *KeySpec) { s.EnvironmentID = "" },
		"origin with path":  func(s *KeySpec) { s.AllowedOrigins = []string{"https://shop.example.com/qr"} },
		"origin no scheme":  func(s *KeySpec) { s.AllowedOrigins = []string{"shop.example.com"} },
		"wildcard origin":   func(s *KeySpec) { s.AllowedOrigins = []string{"*"} },
		"expiry too long":   func(s *KeySpec) { s.ExpiresInDays = 400 },
		"negative lifetime": func(s *KeySpec) { s.ExpiresInDays = -1 },
	} {
		spec := valid
		mutate(&spec)
		if _, err := ValidateKeySpec(spec); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := ValidateKeySpec(KeySpec{TenantID: "acme", EnvironmentID: "production", Name: "ingest", Scopes: []string{"write"}}); err != nil {
		t.Fatalf("a write-only key is valid: %v", err)
	}
}

func TestAllowsOrigin(t *testing.T) {
	open := Key{}
	if !open.AllowsOrigin("https://anything.example") {
		t.Fatal("a key without allowed origins accepts every origin")
	}
	bound := Key{AllowedOrigins: []string{"https://shop.example.com"}}
	if !bound.AllowsOrigin("https://SHOP.example.com") {
		t.Fatal("origins compare case-insensitively")
	}
	for _, origin := range []string{"https://evil.example", "http://shop.example.com", "https://shop.example.com:8443", "null"} {
		if bound.AllowsOrigin(origin) {
			t.Errorf("AllowsOrigin(%q) = true", origin)
		}
	}
}

func TestIsIntegrationPath(t *testing.T) {
	for _, path := range []string{"/v1/events", "/v1/events/evt_1", "/v1/users/u1/points", "/v1/points/conversions", "/v1/leaderboards", "/v1/leaderboards/weekly", "/v1/rewards/r1/redemptions", "/v1/redemptions/rd_1"} {
		if !IsIntegrationPath(path) {
			t.Errorf("IsIntegrationPath(%q) = false", path)
		}
	}
	for _, path := range []string{"/v1/admin/events", "/v1/eventsx", "/health/ready", "/v1/points"} {
		if IsIntegrationPath(path) {
			t.Errorf("IsIntegrationPath(%q) = true", path)
		}
	}
}

func TestRequireKey(t *testing.T) {
	keys := map[string]Key{
		"reader":  {ID: "key_r", TenantID: "acme", EnvironmentID: "production", Scopes: []string{ScopeRead}},
		"writer":  {ID: "key_w", TenantID: "acme", EnvironmentID: "production", Scopes: []string{ScopeRead, ScopeWrite}},
		"browser": {ID: "key_b", TenantID: "acme", EnvironmentID: "production", Scopes: []string{ScopeRead}, AllowedOrigins: []string{"https://shop.example.com"}},
	}
	lookup := func(_ context.Context, raw string) (Key, error) {
		if key, ok := keys[raw]; ok {
			return key, nil
		}
		return Key{}, ErrInvalidCredential
	}
	var seen Key
	handler := RequireKey(lookup, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = KeyFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	cases := []struct {
		name, method, key, origin string
		status                    int
		code                      string
	}{
		{name: "missing key", method: http.MethodGet, status: http.StatusUnauthorized, code: "missing_api_key"},
		{name: "unknown key", method: http.MethodGet, key: "nope", status: http.StatusUnauthorized, code: "invalid_api_key"},
		{name: "read with read", method: http.MethodGet, key: "reader", status: http.StatusOK},
		{name: "write needs write", method: http.MethodPost, key: "reader", status: http.StatusForbidden, code: "insufficient_scope"},
		{name: "write with write", method: http.MethodPost, key: "writer", status: http.StatusOK},
		{name: "delete needs delete", method: http.MethodDelete, key: "writer", status: http.StatusForbidden, code: "insufficient_scope"},
		{name: "any origin on an open key", method: http.MethodGet, key: "writer", origin: "https://other.example", status: http.StatusOK},
		{name: "allowed origin", method: http.MethodGet, key: "browser", origin: "https://shop.example.com", status: http.StatusOK},
		{name: "other origin", method: http.MethodGet, key: "browser", origin: "https://other.example", status: http.StatusForbidden, code: "origin_not_allowed"},
		{name: "server call to a bound key", method: http.MethodGet, key: "browser", status: http.StatusOK},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			seen = Key{}
			request := httptest.NewRequest(testCase.method, "/v1/leaderboards", nil)
			if testCase.key != "" {
				request.Header.Set(HeaderAPIKey, testCase.key)
			}
			if testCase.origin != "" {
				request.Header.Set("Origin", testCase.origin)
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)
			if recorder.Code != testCase.status {
				t.Fatalf("status = %d; want %d (%s)", recorder.Code, testCase.status, recorder.Body.String())
			}
			if testCase.code != "" && !strings.Contains(recorder.Body.String(), `"`+testCase.code+`"`) {
				t.Fatalf("body = %s; want code %s", recorder.Body.String(), testCase.code)
			}
			if testCase.status == http.StatusOK && seen.ID != keys[testCase.key].ID {
				t.Fatalf("handler saw key %q; want %q", seen.ID, keys[testCase.key].ID)
			}
			if testCase.origin != "" && recorder.Header().Get("Access-Control-Allow-Origin") != testCase.origin {
				t.Fatal("responses to a browser carry its origin so the caller can read errors")
			}
		})
	}
}

func TestRequireKeyAnswersPreflightWithoutAKey(t *testing.T) {
	called := false
	handler := RequireKey(func(context.Context, string) (Key, error) {
		called = true
		return Key{}, ErrInvalidCredential
	}, http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodOptions, "/v1/events", nil)
	request.Header.Set("Origin", "https://shop.example.com")
	request.Header.Set("Access-Control-Request-Headers", "x-motif-api-key, content-type")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent || called {
		t.Fatalf("preflight status = %d, lookup called = %t", recorder.Code, called)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "https://shop.example.com" || !strings.Contains(recorder.Header().Get("Access-Control-Allow-Headers"), "X-Motif-API-Key") {
		t.Fatalf("preflight headers = %v", recorder.Header())
	}
}

func TestRequestKeyRejectsUnauthenticatedHandlers(t *testing.T) {
	recorder := httptest.NewRecorder()
	if _, ok := RequestKey(recorder, httptest.NewRequest(http.MethodGet, "/v1/events/evt", nil)); ok || recorder.Code != http.StatusUnauthorized {
		t.Fatalf("RequestKey without RequireKey = %t, %d", ok, recorder.Code)
	}
}
