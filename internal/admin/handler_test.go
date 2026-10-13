package admin

import (
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
