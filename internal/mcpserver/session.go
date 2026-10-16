package mcpserver

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
)

const (
	scopeRead    = "read"
	scopeWrite   = "write"
	scopePublish = "publish"
)

type Environment struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Session is what one access token may do: its workspace, environments and
// scopes, plus a client that calls the API with that token.
type Session struct {
	Client       *Client
	TenantID     string
	Workspace    string
	Status       string
	Email        string
	TokenID      string
	Scopes       []string
	Environments []Environment
	ConsoleURL   string
	PublicAPIURL string
}

func (s *Session) Can(scope string) bool { return slices.Contains(s.Scopes, scope) }

// Environment resolves the environment a tool call targets. With exactly one
// usable environment it is the default; otherwise the caller must choose.
func (s *Session) Environment(requested string) (string, error) {
	requested = strings.TrimSpace(requested)
	ids := make([]string, 0, len(s.Environments))
	for _, environment := range s.Environments {
		ids = append(ids, environment.ID)
	}
	if requested == "" {
		if len(ids) == 1 {
			return ids[0], nil
		}
		return "", fmt.Errorf("choose an environment: one of %s", strings.Join(ids, ", "))
	}
	if !slices.Contains(ids, requested) {
		return "", fmt.Errorf("environment %q is not available to this token; use one of %s", requested, strings.Join(ids, ", "))
	}
	return requested, nil
}

// Scope returns the query parameters that select the workspace and environment.
func (s *Session) Scope(environmentID string) url.Values {
	return url.Values{"tenant_id": {s.TenantID}, "environment_id": {environmentID}}
}

// Body adds the workspace and environment to a request body.
func (s *Session) Body(environmentID string, fields map[string]any) map[string]any {
	body := map[string]any{"tenant_id": s.TenantID, "environment_id": environmentID}
	for key, value := range fields {
		body[key] = value
	}
	return body
}

// Resolver turns a bearer token into a Session by asking the API who it is.
// Results are cached briefly so a burst of tool calls costs one lookup.
type Resolver struct {
	APIURL       string
	ConsoleURL   string
	PublicAPIURL string
	HTTPClient   *http.Client
	TTL          time.Duration

	mu    sync.Mutex
	cache map[[32]byte]cachedSession
}

type cachedSession struct {
	session *Session
	expires time.Time
}

const maxCachedSessions = 1024

var errNotAccessToken = errors.New("use a workspace access token (motif_at_…) created under Access tokens in the console")

type principal struct {
	ID             string   `json:"id"`
	Email          string   `json:"email"`
	TenantIDs      []string `json:"tenant_ids"`
	Source         string   `json:"source"`
	TokenID        string   `json:"token_id"`
	Scopes         []string `json:"scopes"`
	EnvironmentIDs []string `json:"environment_ids"`
}

type tenantRow struct {
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
}

type environmentRow struct {
	ID   string `json:"environment_id"`
	Name string `json:"name"`
}

func (r *Resolver) Resolve(ctx context.Context, token string) (*Session, error) {
	token = strings.TrimSpace(token)
	if !strings.HasPrefix(token, "motif_at_") {
		return nil, fmt.Errorf("%w: %w", auth.ErrInvalidToken, errNotAccessToken)
	}
	key := sha256.Sum256([]byte(token))
	now := time.Now()
	r.mu.Lock()
	if cached, ok := r.cache[key]; ok && now.Before(cached.expires) {
		r.mu.Unlock()
		return cached.session, nil
	}
	r.mu.Unlock()

	client := NewClient(r.APIURL, token, r.HTTPClient)
	var who principal
	if err := client.Do(ctx, http.MethodGet, "/v1/admin/auth/me", nil, nil, &who); err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized {
			return nil, fmt.Errorf("%w: the token is invalid, expired or revoked", auth.ErrInvalidToken)
		}
		return nil, err
	}
	if who.Source != "access_token" || len(who.TenantIDs) != 1 {
		return nil, fmt.Errorf("%w: %w", auth.ErrInvalidToken, errNotAccessToken)
	}
	session := &Session{
		Client: client, TenantID: who.TenantIDs[0], Email: who.Email, TokenID: who.TokenID, Scopes: who.Scopes,
		ConsoleURL: strings.TrimRight(r.ConsoleURL, "/"), PublicAPIURL: strings.TrimRight(r.PublicAPIURL, "/"),
	}
	var tenants []tenantRow
	if err := client.Do(ctx, http.MethodGet, "/v1/admin/tenants", nil, nil, &tenants); err != nil {
		return nil, err
	}
	for _, tenant := range tenants {
		if tenant.TenantID == session.TenantID {
			session.Workspace, session.Status = tenant.Name, tenant.Status
		}
	}
	var environments []environmentRow
	if err := client.Do(ctx, http.MethodGet, "/v1/admin/environments", url.Values{"tenant_id": {session.TenantID}}, nil, &environments); err != nil {
		return nil, err
	}
	for _, environment := range environments {
		if len(who.EnvironmentIDs) == 0 || slices.Contains(who.EnvironmentIDs, environment.ID) {
			session.Environments = append(session.Environments, Environment{ID: environment.ID, Name: environment.Name})
		}
	}

	r.mu.Lock()
	if r.cache == nil || len(r.cache) >= maxCachedSessions {
		r.cache = map[[32]byte]cachedSession{}
	}
	r.cache[key] = cachedSession{session: session, expires: now.Add(r.ttl())}
	r.mu.Unlock()
	return session, nil
}

// Verify adapts Resolve to the SDK's bearer-token middleware.
func (r *Resolver) Verify(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	session, err := r.Resolve(ctx, token)
	if err != nil {
		return nil, err
	}
	return &auth.TokenInfo{
		Scopes:     session.Scopes,
		UserID:     session.TokenID,
		Expiration: time.Now().Add(r.ttl()),
		Extra:      map[string]any{sessionKey: session},
	}, nil
}

func (r *Resolver) ttl() time.Duration {
	if r.TTL > 0 {
		return r.TTL
	}
	return time.Minute
}

const sessionKey = "motif_session"

func sessionFromTokenInfo(info *auth.TokenInfo) *Session {
	if info == nil {
		return nil
	}
	session, _ := info.Extra[sessionKey].(*Session)
	return session
}
