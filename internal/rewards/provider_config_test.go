package rewards

import "testing"

func TestSealSecretRoundTrip(t *testing.T) {
	sealed, err := sealSecret("local-encryption-key", "provider-token")
	if err != nil {
		t.Fatalf("sealSecret() error = %v", err)
	}
	if string(sealed) == "provider-token" {
		t.Fatal("sealSecret() returned plaintext")
	}
	opened, err := openSecret("local-encryption-key", sealed)
	if err != nil {
		t.Fatalf("openSecret() error = %v", err)
	}
	if opened != "provider-token" {
		t.Fatalf("openSecret() = %q, want provider-token", opened)
	}
}

func TestNormalizeScopeProviderConfigRejectsHTTPWithoutURL(t *testing.T) {
	_, err := normalizeScopeProviderConfig(ScopeProviderConfig{Provider: "http", TimeoutSeconds: 8})
	if err == nil {
		t.Fatal("normalizeScopeProviderConfig() error = nil, want missing URL error")
	}
}
