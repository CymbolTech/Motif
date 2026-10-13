package rewards

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrProviderConfigNotFound = errors.New("reward provider configuration was not found")
	ErrInvalidProviderConfig  = errors.New("invalid reward provider configuration")
)

type ScopeProviderConfig struct {
	TenantID        string
	EnvironmentID   string
	Provider        string
	URL             string
	StatusURL       string
	Token           string
	TokenConfigured bool
	TimeoutSeconds  int
	Active          bool
}

func normalizeScopeProviderConfig(config ScopeProviderConfig) (ScopeProviderConfig, error) {
	config.TenantID = strings.TrimSpace(config.TenantID)
	config.EnvironmentID = strings.TrimSpace(config.EnvironmentID)
	config.Provider = strings.ToLower(strings.TrimSpace(config.Provider))
	config.URL = strings.TrimSpace(config.URL)
	config.StatusURL = strings.TrimSpace(config.StatusURL)
	if !identifierPattern.MatchString(config.TenantID) || !identifierPattern.MatchString(config.EnvironmentID) {
		return ScopeProviderConfig{}, fmt.Errorf("%w: tenant_id and environment_id must be valid identifiers", ErrInvalidProviderConfig)
	}
	if config.Provider == "" {
		config.Provider = "mock"
	}
	if config.Provider != "mock" && config.Provider != "http" {
		return ScopeProviderConfig{}, fmt.Errorf("%w: provider must be mock or http", ErrInvalidProviderConfig)
	}
	if config.Provider == "http" && config.URL == "" {
		return ScopeProviderConfig{}, fmt.Errorf("%w: url is required for an http provider", ErrInvalidProviderConfig)
	}
	if config.Provider == "http" {
		if err := validateProviderEndpoint(config.URL); err != nil {
			return ScopeProviderConfig{}, fmt.Errorf("%w: delivery_url: %v", ErrInvalidProviderConfig, err)
		}
		if config.StatusURL != "" {
			if err := validateProviderEndpoint(config.StatusURL); err != nil {
				return ScopeProviderConfig{}, fmt.Errorf("%w: status_url: %v", ErrInvalidProviderConfig, err)
			}
		}
	}
	if config.TimeoutSeconds == 0 {
		config.TimeoutSeconds = 8
	}
	if config.TimeoutSeconds < 1 || config.TimeoutSeconds > 60 {
		return ScopeProviderConfig{}, fmt.Errorf("%w: timeout_seconds must be between 1 and 60", ErrInvalidProviderConfig)
	}
	return config, nil
}

func validateProviderEndpoint(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("must be an absolute http or https URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("must use http or https")
	}
	if parsed.User != nil {
		return fmt.Errorf("must not include user credentials")
	}
	return nil
}

type ConfigStore struct {
	db            *pgxpool.Pool
	encryptionKey string
}

func NewConfigStore(db *pgxpool.Pool, encryptionKey string) *ConfigStore {
	return &ConfigStore{db: db, encryptionKey: encryptionKey}
}

func (s *ConfigStore) Upsert(ctx context.Context, config ScopeProviderConfig) (ScopeProviderConfig, error) {
	if s == nil || s.db == nil {
		return ScopeProviderConfig{}, fmt.Errorf("provider config store is not configured")
	}
	normalized, err := normalizeScopeProviderConfig(config)
	if err != nil {
		return ScopeProviderConfig{}, err
	}
	var sealed []byte
	if normalized.Token != "" {
		sealed, err = sealSecret(s.encryptionKey, normalized.Token)
		if err != nil {
			return ScopeProviderConfig{}, err
		}
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO reward_provider_configs
		    (tenant_id, environment_id, provider, delivery_url, status_url, token_ciphertext, timeout_seconds, active, updated_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $7, $8, now())
		ON CONFLICT (tenant_id, environment_id) DO UPDATE SET
		    provider = EXCLUDED.provider,
		    delivery_url = EXCLUDED.delivery_url,
		    status_url = EXCLUDED.status_url,
		    token_ciphertext = CASE WHEN EXCLUDED.provider = 'mock' THEN NULL ELSE COALESCE(EXCLUDED.token_ciphertext, reward_provider_configs.token_ciphertext) END,
		    timeout_seconds = EXCLUDED.timeout_seconds,
		    active = EXCLUDED.active,
		    updated_at = now()`, normalized.TenantID, normalized.EnvironmentID, normalized.Provider, normalized.URL, normalized.StatusURL, sealed, normalized.TimeoutSeconds, normalized.Active)
	if err != nil {
		return ScopeProviderConfig{}, err
	}
	// Do not retain the plaintext token in the value returned to callers.
	var configured bool
	if err := s.db.QueryRow(ctx, `SELECT token_ciphertext IS NOT NULL FROM reward_provider_configs WHERE tenant_id = $1 AND environment_id = $2`, normalized.TenantID, normalized.EnvironmentID).Scan(&configured); err != nil {
		return ScopeProviderConfig{}, err
	}
	normalized.Token = ""
	normalized.TokenConfigured = configured
	return normalized, nil
}

func (s *ConfigStore) Get(ctx context.Context, tenantID, environmentID string) (ScopeProviderConfig, error) {
	if s == nil || s.db == nil {
		return ScopeProviderConfig{}, ErrProviderConfigNotFound
	}
	var config ScopeProviderConfig
	var sealed []byte
	err := s.db.QueryRow(ctx, `
		SELECT tenant_id, environment_id, provider, delivery_url, COALESCE(status_url, ''), token_ciphertext, timeout_seconds, active
		FROM reward_provider_configs WHERE tenant_id = $1 AND environment_id = $2`, tenantID, environmentID).
		Scan(&config.TenantID, &config.EnvironmentID, &config.Provider, &config.URL, &config.StatusURL, &sealed, &config.TimeoutSeconds, &config.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return ScopeProviderConfig{}, ErrProviderConfigNotFound
	}
	if err != nil {
		return ScopeProviderConfig{}, err
	}
	if len(sealed) > 0 {
		config.TokenConfigured = true
		config.Token, err = openSecret(s.encryptionKey, sealed)
		if err != nil {
			return ScopeProviderConfig{}, fmt.Errorf("decrypt provider token: %w", err)
		}
	}
	return config, nil
}

func (s *ConfigStore) List(ctx context.Context, tenantID, environmentID string) ([]ScopeProviderConfig, error) {
	if s == nil || s.db == nil {
		return nil, ErrProviderConfigNotFound
	}
	rows, err := s.db.Query(ctx, `
		SELECT tenant_id, environment_id, provider, delivery_url, COALESCE(status_url, ''), token_ciphertext, timeout_seconds, active
		FROM reward_provider_configs
		WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2)
		ORDER BY tenant_id, environment_id`, tenantID, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	configs := make([]ScopeProviderConfig, 0)
	for rows.Next() {
		var config ScopeProviderConfig
		var sealed []byte
		if err := rows.Scan(&config.TenantID, &config.EnvironmentID, &config.Provider, &config.URL, &config.StatusURL, &sealed, &config.TimeoutSeconds, &config.Active); err != nil {
			return nil, err
		}
		config.TokenConfigured = len(sealed) > 0
		configs = append(configs, config)
	}
	return configs, rows.Err()
}

type ProviderResolver interface {
	Resolve(ctx context.Context, tenantID, environmentID string) (Provider, error)
}

type StaticProviderResolver struct{ Provider Provider }

func (r StaticProviderResolver) Resolve(_ context.Context, _, _ string) (Provider, error) {
	if r.Provider == nil {
		return nil, fmt.Errorf("reward provider is not configured")
	}
	return r.Provider, nil
}

type ScopeProviderResolver struct {
	Store    *ConfigStore
	Fallback Provider
}

func (r ScopeProviderResolver) Resolve(ctx context.Context, tenantID, environmentID string) (Provider, error) {
	if r.Store != nil {
		config, err := r.Store.Get(ctx, tenantID, environmentID)
		if err == nil {
			if !config.Active {
				return nil, fmt.Errorf("reward provider is disabled for tenant scope")
			}
			return NewProvider(ProviderConfig{Kind: config.Provider, URL: config.URL, StatusURL: config.StatusURL, Token: config.Token, Timeout: time.Duration(config.TimeoutSeconds) * time.Second}), nil
		}
		if !errors.Is(err, ErrProviderConfigNotFound) {
			return nil, err
		}
	}
	if r.Fallback == nil {
		return nil, fmt.Errorf("reward provider is not configured")
	}
	return r.Fallback, nil
}

func sealSecret(key, value string) ([]byte, error) {
	if strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("MOTIF_REWARD_PROVIDER_ENCRYPTION_KEY is required to store provider tokens")
	}
	block, err := aes.NewCipher(secretKey(key))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nonce, nonce, []byte(value), nil)
	encoded := make([]byte, base64.RawStdEncoding.EncodedLen(len(sealed)))
	base64.RawStdEncoding.Encode(encoded, sealed)
	return encoded, nil
}

func openSecret(key string, sealed []byte) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("MOTIF_REWARD_PROVIDER_ENCRYPTION_KEY is required to read provider tokens")
	}
	decoded := make([]byte, base64.RawStdEncoding.DecodedLen(len(sealed)))
	n, err := base64.RawStdEncoding.Decode(decoded, sealed)
	if err != nil {
		return "", err
	}
	decoded = decoded[:n]
	block, err := aes.NewCipher(secretKey(key))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(decoded) < gcm.NonceSize() {
		return "", fmt.Errorf("provider token ciphertext is truncated")
	}
	plain, err := gcm.Open(nil, decoded[:gcm.NonceSize()], decoded[gcm.NonceSize():], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func secretKey(value string) []byte {
	sum := sha256.Sum256([]byte(value))
	return sum[:]
}

var _ ProviderResolver = ScopeProviderResolver{}
var _ ProviderResolver = StaticProviderResolver{}
