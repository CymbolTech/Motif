package admin

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/motifengine/motifengine/internal/adminauth"
)

func TestParseLeaderboardPath(t *testing.T) {
	for path, want := range map[string]leaderboardRoute{
		"/v1/admin/leaderboards":                            {},
		"/v1/admin/leaderboards/lb_1":                       {boardID: "lb_1"},
		"/v1/admin/leaderboards/lb_1/standings":             {boardID: "lb_1", resource: "standings"},
		"/v1/admin/leaderboards/lb_1/rounds":                {boardID: "lb_1", resource: "rounds"},
		"/v1/admin/leaderboards/lb_1/rounds/round_1":        {boardID: "lb_1", resource: "rounds", roundID: "round_1"},
		"/v1/admin/leaderboards/lb_1/rounds/round_1/end":    {boardID: "lb_1", resource: "rounds", roundID: "round_1", action: "end"},
		"/v1/admin/leaderboards/lb_1/rounds/round_1/cancel": {boardID: "lb_1", resource: "rounds", roundID: "round_1", action: "cancel"},
	} {
		route, ok := parseLeaderboardPath(path)
		if !ok || route != want {
			t.Errorf("parseLeaderboardPath(%q) = %+v, %v; want %+v", path, route, ok, want)
		}
	}
	for _, path := range []string{"/v1/admin/leaderboards/", "/v1/admin/leaderboards/lb_1/rounds/round_1/delete", "/v1/admin/leaderboards/lb_1//rounds"} {
		if _, ok := parseLeaderboardPath(path); ok {
			t.Errorf("parseLeaderboardPath(%q) accepted an unknown route", path)
		}
	}
}

func TestAllowedRouteScopesLeaderboardWritesToTenantMembership(t *testing.T) {
	principal := adminauth.Principal{ID: "admin_tenant", Role: adminauth.RoleTenantAdmin, TenantIDs: []string{"tenant-a"}}
	routes := []struct{ method, path string }{
		{http.MethodPost, "/v1/admin/leaderboards"},
		{http.MethodPatch, "/v1/admin/leaderboards/lb_1"},
		{http.MethodPost, "/v1/admin/leaderboards/lb_1/rounds"},
		{http.MethodPatch, "/v1/admin/leaderboards/lb_1/rounds/round_1"},
		{http.MethodPost, "/v1/admin/leaderboards/lb_1/rounds/round_1/end"},
		{http.MethodPost, "/v1/admin/leaderboards/lb_1/rounds/round_1/cancel"},
	}
	for _, route := range routes {
		for tenant, want := range map[string]bool{"tenant-a": true, "tenant-b": false} {
			body := `{"tenant_id":"` + tenant + `","environment_id":"prod"}`
			request := &http.Request{Method: route.method, URL: &url.URL{Path: route.path}, Body: io.NopCloser(bytes.NewReader([]byte(body)))}
			if got := (&Handler{}).allowedRoute(request, principal); got != want {
				t.Errorf("%s %s for %s: allowed = %v; want %v", route.method, route.path, tenant, got, want)
			}
		}
	}
	for tenant, want := range map[string]bool{"tenant-a": true, "tenant-b": false} {
		remove := &http.Request{Method: http.MethodDelete, URL: &url.URL{Path: "/v1/admin/leaderboards/lb_1", RawQuery: "tenant_id=" + tenant + "&environment_id=prod"}}
		if got := (&Handler{}).allowedRoute(remove, principal); got != want {
			t.Errorf("DELETE leaderboard for %s: allowed = %v; want %v", tenant, got, want)
		}
	}
	read := &http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/v1/admin/leaderboards/lb_1/standings", RawQuery: "tenant_id=tenant-b&environment_id=prod"}}
	if (&Handler{}).allowedRoute(read, principal) {
		t.Error("tenant administrators must not read another tenant's standings")
	}
}
