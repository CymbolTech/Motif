package adminauth

import "testing"

func TestPrincipalAllowsTenant(t *testing.T) {
	principal := Principal{Role: RoleTenantAdmin, TenantIDs: []string{"tenant-a"}}
	if !principal.AllowsTenant("tenant-a") || principal.AllowsTenant("tenant-b") {
		t.Fatalf("tenant scope was not enforced: %+v", principal)
	}
	if !(Principal{Role: RoleSuperAdmin}).AllowsTenant("any-tenant") {
		t.Fatal("super admin should allow every tenant")
	}
}

func TestTruncateBoundsMetadata(t *testing.T) {
	if got := truncate("abcdef", 3); got != "abc" {
		t.Fatalf("truncate() = %q", got)
	}
}
