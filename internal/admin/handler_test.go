package admin

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/motifengine/motifengine/internal/adminauth"
)

func TestAllowedRouteAllowsTenantAdminToListMembershipTenants(t *testing.T) {
	request := &http.Request{Method: http.MethodGet, URL: &url.URL{Path: "/v1/admin/tenants"}}
	principal := adminauth.Principal{ID: "admin_tenant", Role: adminauth.RoleTenantAdmin, TenantIDs: []string{"tenant-a"}}

	if !(&Handler{}).allowedRoute(request, principal) {
		t.Fatal("tenant administrators should be allowed to load their membership-scoped tenant list")
	}
}

func TestAllowedRouteStillRejectsTenantAdminTenantWrites(t *testing.T) {
	request := &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/v1/admin/tenants"}}
	principal := adminauth.Principal{ID: "admin_tenant", Role: adminauth.RoleTenantAdmin, TenantIDs: []string{"tenant-a"}}

	if (&Handler{}).allowedRoute(request, principal) {
		t.Fatal("tenant administrators must not create tenants")
	}
}

func TestAllowedRouteKeepsPlatformControlsForOpsOnly(t *testing.T) {
	platformRoutes := []struct {
		name   string
		method string
		path   string
	}{
		{name: "tenant creation", method: http.MethodPost, path: "/v1/admin/tenants"},
		{name: "plans", method: http.MethodGet, path: "/v1/admin/plans"},
		{name: "credentials", method: http.MethodGet, path: "/v1/admin/credentials?tenant_id=tenant-a"},
		{name: "administrator identities", method: http.MethodGet, path: "/v1/admin/admin-users"},
	}
	tenantAdmin := adminauth.Principal{ID: "admin_tenant", Role: adminauth.RoleTenantAdmin, TenantIDs: []string{"tenant-a"}}
	superAdmin := adminauth.Principal{ID: "admin_ops", Role: adminauth.RoleSuperAdmin}
	for _, route := range platformRoutes {
		t.Run(route.name, func(t *testing.T) {
			parsed, err := url.Parse(route.path)
			if err != nil {
				t.Fatal(err)
			}
			request := &http.Request{Method: route.method, URL: parsed}
			if (&Handler{}).allowedRoute(request, tenantAdmin) {
				t.Fatalf("tenant administrators must not access platform route %s", route.path)
			}
			if !(&Handler{}).allowedRoute(request, superAdmin) {
				t.Fatalf("Super Admins should retain access for the future Ops service: %s", route.path)
			}
		})
	}
}

func TestAllowedRouteScopesEnvironmentCreationToTenantMembership(t *testing.T) {
	principal := adminauth.Principal{ID: "admin_tenant", Role: adminauth.RoleTenantAdmin, TenantIDs: []string{"tenant-a"}}
	payload, _ := json.Marshal(map[string]string{"tenant_id": "tenant-a", "environment_id": "prod", "name": "Production"})
	request := &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/v1/admin/environments"}, Body: io.NopCloser(bytes.NewReader(payload))}
	if !(&Handler{}).allowedRoute(request, principal) {
		t.Fatal("tenant administrators should be able to add environments to their own tenant")
	}

	payload, _ = json.Marshal(map[string]string{"tenant_id": "tenant-b", "environment_id": "prod", "name": "Production"})
	request.Body = io.NopCloser(bytes.NewReader(payload))
	if (&Handler{}).allowedRoute(request, principal) {
		t.Fatal("tenant administrators must not add environments to another tenant")
	}
}

func TestAllowedRouteScopesMeasurementRoutesToTenantMembership(t *testing.T) {
	principal := adminauth.Principal{ID: "admin_tenant", Role: adminauth.RoleTenantAdmin, TenantIDs: []string{"tenant-a"}}

	request := &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/v1/admin/analytics/funnel"}, Body: io.NopCloser(bytes.NewReader([]byte(`{"tenant_id":"tenant-a","environment_id":"prod","steps":[]}`)))}
	if !(&Handler{}).allowedRoute(request, principal) {
		t.Fatal("tenant administrators should be able to run scoped funnel reports")
	}

	request = &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/v1/admin/metrics/repeat_buyers/archive", RawQuery: "tenant_id=tenant-a&environment_id=prod"}}
	if !(&Handler{}).allowedRoute(request, principal) {
		t.Fatal("tenant administrators should be able to archive metrics in their tenant")
	}

	request = &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/v1/admin/metrics/repeat_buyers/archive", RawQuery: "tenant_id=tenant-b&environment_id=prod"}}
	if (&Handler{}).allowedRoute(request, principal) {
		t.Fatal("tenant administrators must not archive metrics in another tenant")
	}

	request = &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/v1/admin/analytics/experiment"}, Body: io.NopCloser(bytes.NewReader([]byte(`{"tenant_id":"tenant-a","environment_id":"prod","id":"checkout","conversion_event_type":"order.completed","variants":[{"id":"control","weight":50},{"id":"variant","weight":50}]}`)))}
	if !(&Handler{}).allowedRoute(request, principal) {
		t.Fatal("tenant administrators should be able to run scoped experiment reports")
	}
}

func TestAllowedRouteScopesRuleVersionActionsToTenantMembership(t *testing.T) {
	principal := adminauth.Principal{ID: "admin_tenant", Role: adminauth.RoleTenantAdmin, TenantIDs: []string{"tenant-a"}}

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "create version", method: http.MethodPost, path: "/v1/admin/rules/rule-1/versions"},
		{name: "publish rule", method: http.MethodPost, path: "/v1/admin/rules/rule-1/publish"},
		{name: "publish version", method: http.MethodPost, path: "/v1/admin/rules/rule-1/versions/2/publish"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := &http.Request{Method: test.method, URL: &url.URL{Path: test.path, RawQuery: "tenant_id=tenant-a&environment_id=prod"}}
			if !(&Handler{}).allowedRoute(request, principal) {
				t.Fatalf("tenant administrators should be allowed to manage rules in their tenant: %s", test.path)
			}
		})
	}
}

func TestAllowedRouteRejectsRuleVersionActionsOutsideTenantMembership(t *testing.T) {
	principal := adminauth.Principal{ID: "admin_tenant", Role: adminauth.RoleTenantAdmin, TenantIDs: []string{"tenant-a"}}
	request := &http.Request{Method: http.MethodPost, URL: &url.URL{Path: "/v1/admin/rules/rule-1/publish", RawQuery: "tenant_id=tenant-b&environment_id=prod"}}
	if (&Handler{}).allowedRoute(request, principal) {
		t.Fatal("tenant administrators must not manage rules in another tenant")
	}
}
