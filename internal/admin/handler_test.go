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
