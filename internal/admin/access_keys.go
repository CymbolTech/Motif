package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/motifengine/motifengine/internal/adminauth"
	"github.com/motifengine/motifengine/internal/audit"
	"github.com/motifengine/motifengine/internal/auth"
	appruntime "github.com/motifengine/motifengine/internal/runtime"
)

// accessKeyUpdate changes a key's name, scopes or allowed origins. Omitted
// fields keep their value; "allowed_origins": [] opens the key to every origin.
type accessKeyUpdate struct {
	TenantID       string    `json:"tenant_id"`
	Name           *string   `json:"name"`
	Scopes         *[]string `json:"scopes"`
	AllowedOrigins *[]string `json:"allowed_origins"`
}

func (h *Handler) serveAccessKeys(w http.ResponseWriter, r *http.Request) {
	id, action, ok := parseAccessKeyPath(r.URL.Path)
	switch {
	case !ok:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	case id == "" && r.Method == http.MethodGet:
		h.listAccessKeys(w, r)
	case id == "" && r.Method == http.MethodPost:
		h.createAccessKey(w, r)
	case id != "" && action == "" && r.Method == http.MethodPatch:
		h.updateAccessKey(w, r, id)
	case id != "" && action == "" && r.Method == http.MethodDelete:
		h.removeAccessKey(w, r, id, false)
	case id != "" && action == "revoke" && r.Method == http.MethodPost:
		h.removeAccessKey(w, r, id, true)
	default:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

// parseAccessKeyPath splits /v1/admin/access-keys[/{id}[/revoke]].
func parseAccessKeyPath(path string) (id, action string, ok bool) {
	if path == "/v1/admin/access-keys" {
		return "", "", true
	}
	rest, found := strings.CutPrefix(path, "/v1/admin/access-keys/")
	if !found {
		return "", "", false
	}
	parts := strings.Split(rest, "/")
	switch {
	case len(parts) == 1 && parts[0] != "":
		return parts[0], "", true
	case len(parts) == 2 && parts[0] != "" && parts[1] == "revoke":
		return parts[0], parts[1], true
	}
	return "", "", false
}

func isAccessKeyPath(path string) bool {
	_, _, ok := parseAccessKeyPath(path)
	return ok
}

func (h *Handler) listAccessKeys(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	if !identifierPattern.MatchString(tenantID) {
		writeError(w, http.StatusBadRequest, "invalid_tenant", "tenant_id is required")
		return
	}
	keys, err := auth.ListKeys(r.Context(), h.db, tenantID, strings.TrimSpace(r.URL.Query().Get("environment_id")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "access_key_lookup_failed", "access keys could not be read")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"access_keys": keys})
}

func (h *Handler) createAccessKey(w http.ResponseWriter, r *http.Request) {
	principal, _ := adminauth.PrincipalFromContext(r.Context())
	var spec auth.KeySpec
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &spec); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	spec, err := auth.ValidateKeySpec(spec)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_access_key", err.Error())
		return
	}
	createdBy := ""
	if principal.Source == "session" {
		createdBy = principal.ID
	}
	key, raw, err := auth.CreateKey(r.Context(), h.db, spec, createdBy)
	if err != nil {
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "environment_not_found", "the environment does not belong to this workspace")
			return
		}
		writeError(w, http.StatusInternalServerError, "access_key_creation_failed", "access key could not be created")
		return
	}
	h.auditAccessKey(r, principal, "access_key.created", key)
	writeJSON(w, http.StatusCreated, map[string]any{"key": raw, "access_key": key})
}

func (h *Handler) updateAccessKey(w http.ResponseWriter, r *http.Request, id string) {
	principal, _ := adminauth.PrincipalFromContext(r.Context())
	var update accessKeyUpdate
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &update); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	tenantID := strings.TrimSpace(update.TenantID)
	current, err := auth.FindKey(r.Context(), h.db, tenantID, id)
	if errors.Is(err, auth.ErrKeyNotFound) {
		writeError(w, http.StatusNotFound, "access_key_not_found", "access key was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "access_key_update_failed", "access key could not be updated")
		return
	}
	spec := auth.KeySpec{TenantID: current.TenantID, EnvironmentID: current.EnvironmentID, Name: current.Name, Scopes: current.Scopes, AllowedOrigins: current.AllowedOrigins}
	if update.Name != nil {
		spec.Name = *update.Name
	}
	if update.Scopes != nil {
		spec.Scopes = *update.Scopes
	}
	if update.AllowedOrigins != nil {
		spec.AllowedOrigins = *update.AllowedOrigins
	}
	spec, err = auth.ValidateKeySpec(spec)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_access_key", err.Error())
		return
	}
	key, err := auth.UpdateKey(r.Context(), h.db, tenantID, id, spec)
	if errors.Is(err, auth.ErrKeyNotFound) {
		writeError(w, http.StatusNotFound, "access_key_not_found", "access key was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "access_key_update_failed", "access key could not be updated")
		return
	}
	h.auditAccessKey(r, principal, "access_key.updated", key)
	writeJSON(w, http.StatusOK, key)
}

// removeAccessKey revokes a key, which keeps it listed, or deletes it.
// Either way the key stops working at once.
func (h *Handler) removeAccessKey(w http.ResponseWriter, r *http.Request, id string, revoke bool) {
	principal, _ := adminauth.PrincipalFromContext(r.Context())
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	if !identifierPattern.MatchString(tenantID) {
		writeError(w, http.StatusBadRequest, "invalid_tenant", "tenant_id is required")
		return
	}
	change, action := auth.DeleteKey, "access_key.deleted"
	if revoke {
		change, action = auth.RevokeKey, "access_key.revoked"
	}
	key, err := change(r.Context(), h.db, tenantID, id)
	if errors.Is(err, auth.ErrKeyNotFound) {
		writeError(w, http.StatusNotFound, "access_key_not_found", "access key was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "access_key_change_failed", "access key could not be changed")
		return
	}
	h.auditAccessKey(r, principal, action, key)
	if revoke {
		writeJSON(w, http.StatusOK, key)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) auditAccessKey(r *http.Request, principal adminauth.Principal, action string, key auth.Key) {
	_ = audit.Record(r.Context(), h.db, audit.Entry{
		TenantID: key.TenantID, EnvironmentID: key.EnvironmentID, ActorType: auditActorType(principal), ActorID: principal.ID,
		Action: action, ResourceType: "access_key", ResourceID: key.ID,
		Metadata: auditMetadata(map[string]any{"name": key.Name, "scopes": key.Scopes, "allowed_origins": key.AllowedOrigins}),
	})
}

// Routes an access key can never reach, whatever its scopes: key management,
// provider secrets, value-moving operations and platform administration.
var accessKeyDeniedPrefixes = []string{
	"/v1/admin/access-keys",
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

// accessKeyAllows applies a key's scopes and environment on top of the tenant
// checks every administrator gets. Only keys with the MCP scope reach the
// admin API at all.
func accessKeyAllows(r *http.Request, principal adminauth.Principal) bool {
	if !principal.HasScope(auth.ScopeMCP) {
		return false
	}
	path := r.URL.Path
	for _, prefix := range accessKeyDeniedPrefixes {
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
	if r.Method == http.MethodGet && tenantLevelReadPaths[path] {
		return true
	}
	environmentID, ok := requestEnvironment(r)
	return ok && principal.AllowsEnvironment(environmentID)
}

// requiredScope maps an admin request to a key scope: reads need read,
// creating something needs write, changing what exists (including publishing
// a rule and ending a round) needs edit, and deleting needs delete.
func requiredScope(r *http.Request) string {
	switch {
	case r.Method == http.MethodGet || readOnlyPostPaths[r.URL.Path]:
		return auth.ScopeRead
	case r.Method == http.MethodDelete:
		return auth.ScopeDelete
	case r.Method == http.MethodPost && createsResource(r.URL.Path):
		return auth.ScopeWrite
	default:
		return auth.ScopeEdit
	}
}

// createsResource reports POST routes that add something new: a catalog item,
// a rule or rule draft, a leaderboard round, or a test event.
func createsResource(path string) bool {
	switch path {
	case "/v1/admin/event-types", "/v1/admin/events/test", "/v1/admin/rules", "/v1/admin/currencies", "/v1/admin/leaderboards",
		"/v1/admin/missions", "/v1/admin/goals", "/v1/admin/campaigns", "/v1/admin/metrics", "/v1/admin/rewards",
		"/v1/admin/achievements", "/v1/admin/levels":
		return true
	}
	if strings.HasPrefix(path, "/v1/admin/rules/") && strings.HasSuffix(path, "/versions") {
		return true
	}
	route, ok := parseLeaderboardPath(path)
	return ok && route.boardID != "" && route.resource == "rounds" && route.roundID == ""
}

// requestEnvironment reads the environment a request targets: the query string
// for reads, deletes and rule actions, the JSON body for every other write. The body is
// restored for the handler.
func requestEnvironment(r *http.Request) (string, bool) {
	if r.Method == http.MethodGet || r.Method == http.MethodDelete || isScopedRuleAction(r.URL.Path) {
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
