package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/motifengine/motifengine/internal/adminauth"
	"github.com/motifengine/motifengine/internal/audit"
	appruntime "github.com/motifengine/motifengine/internal/runtime"
)

const maxAccessTokenDays = 365

type AccessTokenRequest struct {
	TenantID       string   `json:"tenant_id"`
	Name           string   `json:"name"`
	Scopes         []string `json:"scopes"`
	EnvironmentIDs []string `json:"environment_ids"`
	ExpiresInDays  int      `json:"expires_in_days"`
}

// ValidateAccessTokenRequest normalizes a token request. Every token can read;
// write and publish are granted separately so publishing a rule stays an
// explicit decision.
func ValidateAccessTokenRequest(request AccessTokenRequest) (AccessTokenRequest, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.Name = strings.TrimSpace(request.Name)
	if !identifierPattern.MatchString(request.TenantID) {
		return request, errors.New("tenant_id is required")
	}
	if request.Name == "" || len(request.Name) > 80 {
		return request, errors.New("name must be between 1 and 80 characters")
	}
	granted := map[string]bool{}
	for _, scope := range request.Scopes {
		scope = strings.TrimSpace(scope)
		if scope != adminauth.ScopeRead && scope != adminauth.ScopeWrite && scope != adminauth.ScopePublish {
			return request, fmt.Errorf("unsupported scope %q; use read, write or publish", scope)
		}
		granted[scope] = true
	}
	if !granted[adminauth.ScopeRead] {
		return request, errors.New("scopes must include read")
	}
	request.Scopes = request.Scopes[:0]
	for _, scope := range []string{adminauth.ScopeRead, adminauth.ScopeWrite, adminauth.ScopePublish} {
		if granted[scope] {
			request.Scopes = append(request.Scopes, scope)
		}
	}
	environments := make([]string, 0, len(request.EnvironmentIDs))
	for _, environmentID := range request.EnvironmentIDs {
		environmentID = strings.TrimSpace(environmentID)
		if !identifierPattern.MatchString(environmentID) {
			return request, fmt.Errorf("environment %q is not a valid identifier", environmentID)
		}
		if !slices.Contains(environments, environmentID) {
			environments = append(environments, environmentID)
		}
	}
	request.EnvironmentIDs = environments
	if request.ExpiresInDays < 0 || request.ExpiresInDays > maxAccessTokenDays {
		return request, fmt.Errorf("expires_in_days must be between 0 (no expiry) and %d", maxAccessTokenDays)
	}
	return request, nil
}

func (h *Handler) listAccessTokens(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	if !identifierPattern.MatchString(tenantID) {
		writeError(w, http.StatusBadRequest, "invalid_tenant", "tenant_id is required")
		return
	}
	tokens, err := h.authStore.ListAccessTokens(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "access_token_lookup_failed", "access tokens could not be read")
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (h *Handler) createAccessToken(w http.ResponseWriter, r *http.Request) {
	principal, _ := adminauth.PrincipalFromContext(r.Context())
	if principal.Source != "session" {
		writeError(w, http.StatusForbidden, "session_required", "access tokens are created from a signed-in administrator session")
		return
	}
	var request AccessTokenRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 8*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := ValidateAccessTokenRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_access_token", err.Error())
		return
	}
	if len(request.EnvironmentIDs) > 0 {
		var found int
		if err := h.db.QueryRow(r.Context(), `SELECT count(*) FROM environments WHERE tenant_id = $1 AND id = ANY($2)`, request.TenantID, request.EnvironmentIDs).Scan(&found); err != nil {
			writeError(w, http.StatusInternalServerError, "access_token_creation_failed", "access token could not be created")
			return
		}
		if found != len(request.EnvironmentIDs) {
			writeError(w, http.StatusNotFound, "environment_not_found", "every environment must belong to the workspace")
			return
		}
	}
	var expiresAt *time.Time
	if request.ExpiresInDays > 0 {
		expiry := time.Now().UTC().Add(time.Duration(request.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &expiry
	}
	token, raw, err := h.authStore.CreateAccessToken(r.Context(), adminauth.NewAccessToken{
		TenantID: request.TenantID, CreatedBy: principal.ID, Name: request.Name,
		Scopes: request.Scopes, EnvironmentIDs: request.EnvironmentIDs, ExpiresAt: expiresAt,
	})
	if err != nil {
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "tenant_not_found", "tenant was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "access_token_creation_failed", "access token could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: token.TenantID, ActorType: auditActorType(principal), ActorID: principal.ID, Action: "access_token.created", ResourceType: "access_token", ResourceID: token.ID, Metadata: auditMetadata(map[string]any{"scopes": token.Scopes, "environment_ids": token.EnvironmentIDs})})
	writeJSON(w, http.StatusCreated, map[string]any{"token": raw, "access_token": token})
}

func (h *Handler) revokeAccessToken(w http.ResponseWriter, r *http.Request) {
	principal, _ := adminauth.PrincipalFromContext(r.Context())
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/admin/access-tokens/"), "/revoke")
	if !identifierPattern.MatchString(tenantID) || id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusBadRequest, "invalid_access_token", "tenant_id and a token id are required")
		return
	}
	token, err := h.authStore.RevokeAccessToken(r.Context(), tenantID, id)
	if errors.Is(err, adminauth.ErrAccessTokenNotFound) {
		writeError(w, http.StatusNotFound, "access_token_not_found", "access token was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "access_token_revoke_failed", "access token could not be revoked")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: token.TenantID, ActorType: auditActorType(principal), ActorID: principal.ID, Action: "access_token.revoked", ResourceType: "access_token", ResourceID: token.ID})
	writeJSON(w, http.StatusOK, token)
}

func isAccessTokenRevokePath(path string) bool {
	if !strings.HasPrefix(path, "/v1/admin/access-tokens/") || !strings.HasSuffix(path, "/revoke") {
		return false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/admin/access-tokens/"), "/revoke")
	return id != "" && !strings.Contains(id, "/")
}

// Routes an access token can never reach, whatever its scopes: credentials and
// provider secrets, value-moving operations, token management and platform
// administration.
var accessTokenDeniedPrefixes = []string{
	"/v1/admin/access-tokens",
	"/v1/admin/credentials",
	"/v1/admin/reward-providers",
	"/v1/admin/points",
	"/v1/admin/redemptions/test",
	"/v1/admin/redemptions/retry",
	"/v1/admin/admin-users",
	"/v1/admin/plans",
	"/v1/admin/tenant-plans",
}

// Non-GET routes that compute an answer without changing state.
var readOnlyPostPaths = map[string]bool{
	"/v1/admin/rules/check":          true,
	"/v1/admin/analytics/funnel":     true,
	"/v1/admin/analytics/retention":  true,
	"/v1/admin/analytics/segment":    true,
	"/v1/admin/analytics/experiment": true,
}

// Tenant-level reads that do not select an environment.
var tenantLevelReadPaths = map[string]bool{
	"/v1/admin/auth/me":      true,
	"/v1/admin/tenants":      true,
	"/v1/admin/environments": true,
	"/v1/admin/usage":        true,
}

// accessTokenAllows applies a token's scopes and environment allow-list on top
// of the tenant membership checks every administrator gets.
func accessTokenAllows(r *http.Request, principal adminauth.Principal) bool {
	path := r.URL.Path
	for _, prefix := range accessTokenDeniedPrefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return false
		}
	}
	if r.Method == http.MethodPost && path == "/v1/admin/environments" {
		return false
	}
	if !principal.HasScope(requiredScope(r)) {
		return false
	}
	if len(principal.EnvironmentIDs) == 0 || (r.Method == http.MethodGet && tenantLevelReadPaths[path]) {
		return true
	}
	environmentID, ok := requestEnvironment(r)
	return ok && principal.AllowsEnvironment(environmentID)
}

func requiredScope(r *http.Request) string {
	switch {
	case r.Method == http.MethodGet || readOnlyPostPaths[r.URL.Path]:
		return adminauth.ScopeRead
	case isScopedRuleAction(r.URL.Path) && strings.HasSuffix(r.URL.Path, "/publish"):
		return adminauth.ScopePublish
	default:
		return adminauth.ScopeWrite
	}
}

// requestEnvironment reads the environment a request targets: the query string
// for reads and rule actions, the JSON body for every other write. The body is
// restored for the handler.
func requestEnvironment(r *http.Request) (string, bool) {
	if r.Method == http.MethodGet || isScopedRuleAction(r.URL.Path) {
		environmentID := strings.TrimSpace(r.URL.Query().Get("environment_id"))
		return environmentID, environmentID != ""
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		return "", false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var payload struct {
		EnvironmentID string `json:"environment_id"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return "", false
	}
	environmentID := strings.TrimSpace(payload.EnvironmentID)
	return environmentID, environmentID != ""
}

// clientLabel keeps a short, safe label from X-Motif-Client for audit metadata.
func clientLabel(r *http.Request) string {
	value := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Motif-Client")))
	if len(value) > 32 {
		value = value[:32]
	}
	for _, character := range value {
		if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.') {
			return ""
		}
	}
	return value
}
