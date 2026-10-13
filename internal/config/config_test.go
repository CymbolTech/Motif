package config

import "testing"

func TestLoadUsesConfiguredValues(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example/db")
	t.Setenv("NATS_URL", "nats://example:4222")
	t.Setenv("API_ADDR", ":9090")
	t.Setenv("WORKER_HEALTH_ADDR", ":9091")
	t.Setenv("MOTIF_CORS_ORIGINS", " http://localhost:3000,https://console.example ")
	t.Setenv("MOTIF_RATE_LIMIT_RPS", "12.5")
	t.Setenv("MOTIF_RATE_LIMIT_BURST", "25")

	c, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if c.DatabaseURL != "postgres://example/db" || c.NATSURL != "nats://example:4222" {
		t.Fatalf("unexpected dependency configuration: %+v", c)
	}
	if c.APIAddr != ":9090" || c.WorkerHealthAddr != ":9091" {
		t.Fatalf("unexpected addresses: %+v", c)
	}
	if c.RateLimitRPS != 12.5 || c.RateLimitBurst != 25 || len(c.CORSOrigins) != 2 {
		t.Fatalf("unexpected API protection configuration: %+v", c)
	}
}

func TestLoadRejectsMissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing DATABASE_URL error")
	}
}

func TestLoadRequiresHTTPProviderURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example/db")
	t.Setenv("MOTIF_REWARD_PROVIDER", "http")
	t.Setenv("MOTIF_REWARD_PROVIDER_URL", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want missing HTTP provider URL error")
	}
}

func TestLoadRejectsDefaultSecretsInProduction(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example/db")
	t.Setenv("MOTIF_RUNTIME_ENV", "production")
	t.Setenv("MOTIF_CORS_ORIGINS", "https://console.example")
	t.Setenv("MOTIF_REWARD_PROVIDER_ENCRYPTION_KEY", "production-key")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want default secret rejection")
	}
}

func TestLoadRequiresProductionEncryptionKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example/db")
	t.Setenv("MOTIF_RUNTIME_ENV", "production")
	t.Setenv("MOTIF_BOOTSTRAP_API_KEY", "bootstrap-production")
	t.Setenv("MOTIF_ADMIN_API_KEY", "admin-production")
	t.Setenv("MOTIF_ADMIN_PASSWORD", "password-production")
	t.Setenv("MOTIF_CORS_ORIGINS", "https://console.example")
	t.Setenv("MOTIF_REWARD_PROVIDER_ENCRYPTION_KEY", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want encryption key requirement")
	}
}
