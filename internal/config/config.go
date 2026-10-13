package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL                  string
	NATSURL                      string
	APIAddr                      string
	WorkerHealthAddr             string
	BootstrapAPIKey              string
	BootstrapTenant              string
	BootstrapEnv                 string
	AdminAPIKey                  string
	AdminEmail                   string
	AdminPassword                string
	CORSOrigins                  []string
	RateLimitRPS                 float64
	RateLimitBurst               int
	MetricsEnabled               bool
	RewardProvider               string
	RewardProviderURL            string
	RewardProviderStatusURL      string
	RewardProviderToken          string
	RewardProviderEncryptionKey  string
	RewardProviderTimeoutSeconds int
}

func Load() (Config, error) {
	c := Config{
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		NATSURL:          valueOr("NATS_URL", "nats://localhost:4222"),
		APIAddr:          valueOr("API_ADDR", "127.0.0.1:8080"),
		WorkerHealthAddr: valueOr("WORKER_HEALTH_ADDR", "127.0.0.1:8081"),
		BootstrapAPIKey:  valueOr("MOTIF_BOOTSTRAP_API_KEY", "dev_only_change_me"),
		BootstrapTenant:  valueOr("MOTIF_BOOTSTRAP_TENANT", "demo-tenant"),
		BootstrapEnv:     valueOr("MOTIF_BOOTSTRAP_ENVIRONMENT", "sandbox"),
		AdminAPIKey:      valueOr("MOTIF_ADMIN_API_KEY", "admin_dev_only_change_me"),
		AdminEmail:       valueOr("MOTIF_ADMIN_EMAIL", "admin@motif.local"),
		AdminPassword:    valueOr("MOTIF_ADMIN_PASSWORD", "admin_dev_only_change_me"),
		CORSOrigins:      splitCSV(os.Getenv("MOTIF_CORS_ORIGINS")),
	}
	var parseErr error
	if c.RateLimitRPS, parseErr = strconv.ParseFloat(valueOr("MOTIF_RATE_LIMIT_RPS", "30"), 64); parseErr != nil || c.RateLimitRPS <= 0 {
		return Config{}, fmt.Errorf("MOTIF_RATE_LIMIT_RPS must be a positive number")
	}
	if c.RateLimitBurst, parseErr = strconv.Atoi(valueOr("MOTIF_RATE_LIMIT_BURST", "60")); parseErr != nil || c.RateLimitBurst < 1 {
		return Config{}, fmt.Errorf("MOTIF_RATE_LIMIT_BURST must be a positive integer")
	}
	c.MetricsEnabled = strings.EqualFold(valueOr("MOTIF_METRICS_ENABLED", "true"), "true")
	c.RewardProvider = strings.ToLower(strings.TrimSpace(valueOr("MOTIF_REWARD_PROVIDER", "mock")))
	if c.RewardProvider != "mock" && c.RewardProvider != "http" {
		return Config{}, fmt.Errorf("MOTIF_REWARD_PROVIDER must be mock or http")
	}
	c.RewardProviderURL = strings.TrimSpace(os.Getenv("MOTIF_REWARD_PROVIDER_URL"))
	c.RewardProviderStatusURL = strings.TrimSpace(os.Getenv("MOTIF_REWARD_PROVIDER_STATUS_URL"))
	c.RewardProviderToken = os.Getenv("MOTIF_REWARD_PROVIDER_TOKEN")
	c.RewardProviderEncryptionKey = os.Getenv("MOTIF_REWARD_PROVIDER_ENCRYPTION_KEY")
	if c.RewardProvider == "http" && c.RewardProviderURL == "" {
		return Config{}, fmt.Errorf("MOTIF_REWARD_PROVIDER_URL is required when MOTIF_REWARD_PROVIDER=http")
	}
	if c.RewardProviderTimeoutSeconds, parseErr = strconv.Atoi(valueOr("MOTIF_REWARD_PROVIDER_TIMEOUT_SECONDS", "8")); parseErr != nil || c.RewardProviderTimeoutSeconds < 1 || c.RewardProviderTimeoutSeconds > 60 {
		return Config{}, fmt.Errorf("MOTIF_REWARD_PROVIDER_TIMEOUT_SECONDS must be between 1 and 60")
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func valueOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
