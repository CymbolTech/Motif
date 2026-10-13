package admin

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/adminauth"
	"github.com/motifengine/motifengine/internal/audit"
	"github.com/motifengine/motifengine/internal/auth"
	"github.com/motifengine/motifengine/internal/entitlements"
	"github.com/motifengine/motifengine/internal/points"
	"github.com/motifengine/motifengine/internal/progression"
	"github.com/motifengine/motifengine/internal/rewards"
	appruntime "github.com/motifengine/motifengine/internal/runtime"
)

type Handler struct {
	db              *pgxpool.Pool
	apiKey          string
	limits          *entitlements.Store
	authStore       *adminauth.Store
	providerConfigs *rewards.ConfigStore
}

func NewHandler(db *pgxpool.Pool, apiKey string, stores ...*adminauth.Store) http.Handler {
	var authStore *adminauth.Store
	if len(stores) > 0 {
		authStore = stores[0]
	}
	if authStore == nil {
		authStore = adminauth.NewStore(db)
	}
	return NewHandlerWithProviderConfig(db, apiKey, authStore, rewards.NewConfigStore(db, ""))
}

func NewHandlerWithProviderConfig(db *pgxpool.Pool, apiKey string, authStore *adminauth.Store, providerConfigs *rewards.ConfigStore) http.Handler {
	if authStore == nil {
		authStore = adminauth.NewStore(db)
	}
	if providerConfigs == nil {
		providerConfigs = rewards.NewConfigStore(db, "")
	}
	return &Handler{db: db, apiKey: apiKey, limits: entitlements.NewStore(db), authStore: authStore, providerConfigs: providerConfigs}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/v1/admin/auth/login" && r.Method == http.MethodPost {
		h.login(w, r)
		return
	}
	if r.URL.Path == "/v1/admin/auth/logout" && r.Method == http.MethodPost {
		h.logout(w, r)
		return
	}
	principal, err := adminauth.FromRequest(r.Context(), h.authStore, r, h.apiKey)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_admin_session", "a valid admin session or bootstrap key is required")
		return
	}
	if !h.allowedRoute(r, principal) {
		writeError(w, http.StatusForbidden, "admin_forbidden", "this administrator is not allowed to access the requested scope")
		return
	}
	r = r.WithContext(adminauth.WithPrincipal(r.Context(), principal))
	if r.URL.Path == "/v1/admin/auth/me" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, principalResponse(principal))
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/admin-users" {
		h.listAdminUsers(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/admin-users" {
		h.createAdminUser(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/tenants" {
		h.listTenants(w, r)
		return
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/admin/tenants/") && strings.HasSuffix(r.URL.Path, "/status") {
		h.updateTenantStatus(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/tenants" {
		h.createTenant(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/credentials" {
		h.listCredentials(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/credentials" {
		h.createCredential(w, r)
		return
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/admin/credentials/") && strings.HasSuffix(r.URL.Path, "/rotate") {
		h.rotateCredential(w, r)
		return
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/admin/credentials/") && strings.HasSuffix(r.URL.Path, "/revoke") {
		h.revokeCredential(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/audit-log" {
		h.listAuditLog(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/plans" {
		h.listPlans(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/plans" {
		h.createPlan(w, r)
		return
	}
	if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/admin/plans/") {
		h.updatePlan(w, r)
		return
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/admin/plans/") && strings.HasSuffix(r.URL.Path, "/archive") {
		h.archivePlan(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/tenant-plans" {
		h.listTenantPlans(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/tenant-plans" {
		h.assignTenantPlan(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/event-types" {
		h.createEventType(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/event-types" {
		h.listEventTypes(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/admin/event-types/") && strings.HasSuffix(r.URL.Path, "/versions") {
		h.listEventTypeVersions(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/events" {
		h.listEvents(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/diagnostics" {
		h.diagnostics(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/usage" {
		h.usage(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/rules" {
		h.listRules(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/admin/rules/") && strings.HasSuffix(r.URL.Path, "/versions") {
		h.listRuleVersions(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/rules" {
		h.createRule(w, r)
		return
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/admin/rules/") && strings.HasSuffix(r.URL.Path, "/versions") {
		h.createRuleVersion(w, r)
		return
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/admin/rules/") && strings.Contains(r.URL.Path, "/versions/") && strings.HasSuffix(r.URL.Path, "/publish") {
		h.publishRuleVersion(w, r)
		return
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/admin/rules/") && strings.HasSuffix(r.URL.Path, "/publish") {
		h.publishRule(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/rule-executions" {
		h.listRuleExecutions(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/currencies" {
		h.createCurrency(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/currencies" {
		h.listCurrencies(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/missions" {
		h.createMission(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/missions" {
		h.listMissions(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/rewards" {
		h.createReward(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/rewards" {
		h.listRewards(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/achievements" {
		h.createAchievement(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/achievements" {
		h.listAchievements(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/levels" {
		h.createLevel(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/levels" {
		h.listLevels(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/redemptions" {
		h.listRedemptions(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/admin/redemptions/") && strings.HasSuffix(r.URL.Path, "/attempts") {
		h.listRedemptionAttempts(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/reward-providers" {
		h.listRewardProviders(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/reward-providers" {
		h.upsertRewardProvider(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/action-executions" {
		h.listRuleExecutions(w, r)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "route not found")
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type adminPrincipalResponse struct {
	ID        string   `json:"id"`
	Email     string   `json:"email"`
	Role      string   `json:"role"`
	TenantIDs []string `json:"tenant_ids,omitempty"`
}

func principalResponse(principal adminauth.Principal) adminPrincipalResponse {
	return adminPrincipalResponse{ID: principal.ID, Email: principal.Email, Role: principal.Role, TenantIDs: principal.TenantIDs}
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	principal, token, err := h.authStore.Authenticate(r.Context(), request.Email, request.Password, r.UserAgent(), remoteIP(r))
	if errors.Is(err, adminauth.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "email or password is incorrect")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_login_failed", "admin login could not be completed")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{ActorType: "platform_admin", ActorID: principal.ID, Action: "admin.login", ResourceType: "admin_session", ResourceID: principal.ID})
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expires_in": int(adminauth.SessionTTL.Seconds()), "admin": principalResponse(principal)})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		rawToken := strings.TrimSpace(header[len("Bearer "):])
		principal, err := h.authStore.Lookup(r.Context(), rawToken)
		_ = h.authStore.Revoke(r.Context(), rawToken)
		if err == nil {
			_ = audit.Record(r.Context(), h.db, audit.Entry{ActorType: "platform_admin", ActorID: principal.ID, Action: "admin.logout", ResourceType: "admin_session", ResourceID: principal.ID})
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

type adminUserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	TenantIDs []string  `json:"tenant_ids,omitempty"`
}

func (h *Handler) createAdminUser(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Email     string   `json:"email"`
		Password  string   `json:"password"`
		Role      string   `json:"role"`
		TenantIDs []string `json:"tenant_ids"`
	}
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.Email = strings.ToLower(strings.TrimSpace(request.Email))
	request.Role = strings.TrimSpace(request.Role)
	if request.Email == "" || !strings.Contains(request.Email, "@") || len(request.Email) > 254 || len(request.Password) < 12 {
		writeError(w, http.StatusBadRequest, "invalid_admin_user", "email is required and password must contain at least 12 characters")
		return
	}
	if request.Role == "" {
		request.Role = adminauth.RoleTenantAdmin
	}
	for index := range request.TenantIDs {
		request.TenantIDs[index] = strings.TrimSpace(request.TenantIDs[index])
	}
	user, err := h.authStore.CreateUser(r.Context(), request.Email, request.Password, request.Role, request.TenantIDs)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "admin_user_exists", "an administrator with this email already exists")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_admin_user", err.Error())
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{ActorType: "platform_admin", ActorID: user.ID, Action: "admin_user.created", ResourceType: "admin_user", ResourceID: user.ID, Metadata: auditMetadata(map[string]any{"role": user.Role, "tenant_ids": user.TenantIDs})})
	writeJSON(w, http.StatusCreated, adminUserResponse{ID: user.ID, Email: user.Email, Role: user.Role, Active: user.Active, CreatedAt: user.CreatedAt, TenantIDs: user.TenantIDs})
}

func (h *Handler) listAdminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.authStore.ListUsers(r.Context(), strings.TrimSpace(r.URL.Query().Get("tenant_id")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_user_lookup_failed", "administrators could not be read")
		return
	}
	result := make([]adminUserResponse, 0, len(users))
	for _, user := range users {
		result = append(result, adminUserResponse{ID: user.ID, Email: user.Email, Role: user.Role, Active: user.Active, CreatedAt: user.CreatedAt, TenantIDs: user.TenantIDs})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) allowedRoute(r *http.Request, principal adminauth.Principal) bool {
	if principal.IsSuperAdmin() {
		return true
	}
	path := r.URL.Path
	forbidden := []string{"/v1/admin/tenants", "/v1/admin/credentials", "/v1/admin/plans", "/v1/admin/tenant-plans", "/v1/admin/admin-users"}
	for _, prefix := range forbidden {
		if prefix == "/v1/admin/tenants" && path == prefix && r.Method == http.MethodGet {
			continue
		}
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return false
		}
	}
	if path == "/v1/admin/auth/me" && r.Method == http.MethodGet {
		return true
	}
	if r.Method != http.MethodGet {
		allowedCreate := map[string]bool{
			"/v1/admin/event-types":      true,
			"/v1/admin/rules":            true,
			"/v1/admin/currencies":       true,
			"/v1/admin/missions":         true,
			"/v1/admin/rewards":          true,
			"/v1/admin/reward-providers": true,
			"/v1/admin/achievements":     true,
			"/v1/admin/levels":           true,
		}
		if !allowedCreate[path] {
			return false
		}
		return requestBodyTenant(r, principal)
	}
	if path == "/v1/admin/tenants" {
		return true
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	return tenantID != "" && principal.AllowsTenant(tenantID)
}

func requestBodyTenant(r *http.Request, principal adminauth.Principal) bool {
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	var payload struct {
		TenantID string `json:"tenant_id"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return false
	}
	return principal.AllowsTenant(strings.TrimSpace(payload.TenantID))
}

func remoteIP(r *http.Request) string {
	value := r.RemoteAddr
	if index := strings.LastIndex(value, ":"); index > -1 {
		return value[:index]
	}
	return value
}

func (h *Handler) enforceResourceLimitTx(ctx context.Context, tx pgx.Tx, tenantID, environmentID, key, resource string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, tenantID+":"+key); err != nil {
		return fmt.Errorf("resource lock: %w", err)
	}
	query := map[string]string{
		"rules":      `SELECT count(*) FROM rules WHERE tenant_id = $1 AND environment_id = $2`,
		"currencies": `SELECT count(*) FROM currencies WHERE tenant_id = $1 AND environment_id = $2`,
		"rewards":    `SELECT count(*) FROM rewards WHERE tenant_id = $1 AND environment_id = $2`,
	}[resource]
	if query == "" {
		return fmt.Errorf("unsupported resource limit %s", resource)
	}
	var current int64
	if err := tx.QueryRow(ctx, query, tenantID, environmentID).Scan(&current); err != nil {
		return fmt.Errorf("resource count: %w", err)
	}
	return h.limits.CheckWith(ctx, tx, tenantID, key, current, 1)
}

func (h *Handler) writePlanLimitError(w http.ResponseWriter, err error) {
	var limitErr *entitlements.LimitError
	if errors.As(err, &limitErr) {
		writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{
			"code": "plan_limit_exceeded", "message": limitErr.Error(), "entitlement": limitErr.Key,
			"limit": limitErr.Limit, "current": limitErr.Current,
		}})
		return
	}
	writeError(w, http.StatusInternalServerError, "plan_lookup_failed", "tenant plan could not be evaluated")
}

type tenantResponse struct {
	TenantID        string `json:"tenant_id"`
	Name            string `json:"name"`
	EnvironmentID   string `json:"environment_id"`
	EnvironmentName string `json:"environment_name"`
	Status          string `json:"status"`
	IntegrationKey  string `json:"integration_api_key,omitempty"`
}

type credentialResponse struct {
	ID             int64      `json:"id"`
	TenantID       string     `json:"tenant_id"`
	EnvironmentID  string     `json:"environment_id"`
	Label          string     `json:"label"`
	CreatedAt      time.Time  `json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	Active         bool       `json:"active"`
	IntegrationKey string     `json:"integration_api_key,omitempty"`
}

type eventTypeResponse struct {
	TenantID      string          `json:"tenant_id"`
	EnvironmentID string          `json:"environment_id"`
	Name          string          `json:"name"`
	Schema        json.RawMessage `json:"schema"`
	Version       int             `json:"version"`
	Active        bool            `json:"active,omitempty"`
	CreatedAt     time.Time       `json:"created_at,omitempty"`
}

type eventAdminResponse struct {
	EventID         int64      `json:"event_id"`
	TenantID        string     `json:"tenant_id"`
	EnvironmentID   string     `json:"environment_id"`
	ExternalEventID string     `json:"external_event_id"`
	EventType       string     `json:"event_type"`
	Status          string     `json:"status"`
	ReceivedAt      time.Time  `json:"received_at"`
	ProcessedAt     *time.Time `json:"processed_at,omitempty"`
}

type ruleResponse struct {
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	EnvironmentID string          `json:"environment_id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	EventType     string          `json:"event_type"`
	Conditions    json.RawMessage `json:"conditions"`
	Action        json.RawMessage `json:"action"`
	Status        string          `json:"status"`
	Version       int             `json:"version"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	PublishedAt   *time.Time      `json:"published_at,omitempty"`
	StartsAt      *time.Time      `json:"starts_at,omitempty"`
	EndsAt        *time.Time      `json:"ends_at,omitempty"`
}

type ruleExecutionResponse struct {
	ID            int64      `json:"id"`
	RuleID        string     `json:"rule_id"`
	RuleVersionID string     `json:"rule_version_id"`
	RuleName      string     `json:"rule_name"`
	EventID       int64      `json:"event_id"`
	EventType     string     `json:"event_type"`
	ActionType    string     `json:"action_type"`
	Status        string     `json:"status"`
	Attempts      int        `json:"attempts"`
	LastError     string     `json:"last_error,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type ruleVersionResponse struct {
	ID          string          `json:"id"`
	RuleID      string          `json:"rule_id"`
	Version     int             `json:"version"`
	EventType   string          `json:"event_type"`
	Conditions  json.RawMessage `json:"conditions"`
	Action      json.RawMessage `json:"action"`
	Status      string          `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	PublishedAt *time.Time      `json:"published_at,omitempty"`
	StartsAt    *time.Time      `json:"starts_at,omitempty"`
	EndsAt      *time.Time      `json:"ends_at,omitempty"`
}

type diagnosticsResponse struct {
	EventCounts             map[string]int64 `json:"event_counts"`
	PendingOutbox           int64            `json:"pending_outbox"`
	FailedRuleExecutions    int64            `json:"failed_rule_executions"`
	RetryableRuleExecutions int64            `json:"retryable_rule_executions"`
	ActiveRules             int64            `json:"active_rules"`
	FailedRedemptions       int64            `json:"failed_redemptions"`
	GeneratedAt             time.Time        `json:"generated_at"`
}

type missionResponse struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	EnvironmentID string    `json:"environment_id"`
	Name          string    `json:"name"`
	EventType     string    `json:"event_type"`
	TargetCount   int       `json:"target_count"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
}

type redemptionAdminResponse struct {
	ID                     string     `json:"id"`
	RewardID               string     `json:"reward_id"`
	RewardName             string     `json:"reward_name"`
	TenantID               string     `json:"tenant_id"`
	EnvironmentID          string     `json:"environment_id"`
	ExternalUserID         string     `json:"external_user_id"`
	Status                 string     `json:"status"`
	IdempotencyKey         string     `json:"idempotency_key"`
	ProviderReference      string     `json:"provider_reference,omitempty"`
	FailureReason          string     `json:"failure_reason,omitempty"`
	AttemptCount           int        `json:"attempt_count"`
	ReconciliationAttempts int        `json:"reconciliation_attempts"`
	NextReconciliationAt   *time.Time `json:"next_reconciliation_at,omitempty"`
	LastProviderCheckError string     `json:"last_provider_check_error,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type rewardProviderConfigRequest struct {
	TenantID       string `json:"tenant_id"`
	EnvironmentID  string `json:"environment_id"`
	Provider       string `json:"provider"`
	DeliveryURL    string `json:"delivery_url"`
	StatusURL      string `json:"status_url"`
	Token          string `json:"token"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Active         *bool  `json:"active"`
}

type rewardProviderConfigResponse struct {
	TenantID        string `json:"tenant_id"`
	EnvironmentID   string `json:"environment_id"`
	Provider        string `json:"provider"`
	DeliveryURL     string `json:"delivery_url"`
	StatusURL       string `json:"status_url,omitempty"`
	TokenConfigured bool   `json:"token_configured"`
	TimeoutSeconds  int    `json:"timeout_seconds"`
	Active          bool   `json:"active"`
}

type redemptionAttemptAdminResponse struct {
	ID                int64      `json:"id"`
	RedemptionID      string     `json:"redemption_id"`
	Kind              string     `json:"kind"`
	Status            string     `json:"status"`
	ProviderState     string     `json:"provider_state,omitempty"`
	ProviderReference string     `json:"provider_reference,omitempty"`
	ErrorMessage      string     `json:"error_message,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
}

type planResponse struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	BillingInterval string           `json:"billing_interval"`
	PriceCents      int64            `json:"price_cents"`
	Entitlements    map[string]int64 `json:"entitlements"`
	Active          bool             `json:"active"`
	UpdatedAt       *time.Time       `json:"updated_at,omitempty"`
}

type tenantPlanResponse struct {
	TenantID     string           `json:"tenant_id"`
	PlanID       string           `json:"plan_id"`
	PlanName     string           `json:"plan_name"`
	Entitlements map[string]int64 `json:"entitlements"`
	Overrides    map[string]int64 `json:"overrides"`
	Effective    map[string]int64 `json:"effective"`
	AssignedAt   time.Time        `json:"assigned_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

func (h *Handler) createTenant(w http.ResponseWriter, r *http.Request) {
	var request TenantRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := ValidateTenantRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_tenant", err.Error())
		return
	}
	integrationKey, err := generateIntegrationKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "key_generation_failed", "integration key could not be generated")
		return
	}
	keyHash := auth.Digest(integrationKey)
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tenant_creation_failed", "tenant could not be created")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(r.Context(), `INSERT INTO tenants (id, name) VALUES ($1, $2)`, request.TenantID, request.Name); err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "tenant_exists", "tenant already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "tenant_creation_failed", "tenant could not be created")
		return
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO environments (tenant_id, id, name) VALUES ($1, $2, $3)`, request.TenantID, request.EnvironmentID, request.EnvironmentName); err != nil {
		writeError(w, http.StatusInternalServerError, "environment_creation_failed", "environment could not be created")
		return
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO integration_credentials (tenant_id, environment_id, key_hash, label, created_by) VALUES ($1, $2, $3, 'default', 'platform_admin')`, request.TenantID, request.EnvironmentID, keyHash); err != nil {
		writeError(w, http.StatusInternalServerError, "credential_creation_failed", "integration credential could not be created")
		return
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO tenant_plans (tenant_id, plan_id) VALUES ($1, 'starter')`, request.TenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "plan_assignment_failed", "starter plan could not be assigned")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "tenant_creation_failed", "tenant could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, ActorType: "platform_admin", Action: "tenant.created", ResourceType: "tenant", ResourceID: request.TenantID, Metadata: auditMetadata(map[string]any{"environment_id": request.EnvironmentID, "plan_id": "starter"})})
	writeJSON(w, http.StatusCreated, tenantResponse{TenantID: request.TenantID, Name: request.Name, EnvironmentID: request.EnvironmentID, EnvironmentName: request.EnvironmentName, Status: "active", IntegrationKey: integrationKey})
}

func (h *Handler) listTenants(w http.ResponseWriter, r *http.Request) {
	principal, _ := adminauth.PrincipalFromContext(r.Context())
	query := `
		SELECT t.id, t.name, e.id, e.name, t.status
		FROM tenants t JOIN environments e ON e.tenant_id = t.id
		ORDER BY t.id, e.id`
	args := []any{}
	if !principal.IsSuperAdmin() {
		query = `
			SELECT t.id, t.name, e.id, e.name, t.status
			FROM tenants t
			JOIN environments e ON e.tenant_id = t.id
			JOIN admin_memberships m ON m.tenant_id = t.id
			WHERE m.admin_user_id = $1
			ORDER BY t.id, e.id`
		args = append(args, principal.ID)
	}
	rows, err := h.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tenant_lookup_failed", "tenants could not be read")
		return
	}
	defer rows.Close()
	result := make([]tenantResponse, 0)
	for rows.Next() {
		var tenant tenantResponse
		if err := rows.Scan(&tenant.TenantID, &tenant.Name, &tenant.EnvironmentID, &tenant.EnvironmentName, &tenant.Status); err != nil {
			writeError(w, http.StatusInternalServerError, "tenant_lookup_failed", "tenants could not be read")
			return
		}
		result = append(result, tenant)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "tenant_lookup_failed", "tenants could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) updateTenantStatus(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/admin/tenants/"), "/status")
	if tenantID == "" || strings.Contains(tenantID, "/") {
		writeError(w, http.StatusNotFound, "tenant_not_found", "tenant was not found")
		return
	}
	var request struct {
		Status string `json:"status"`
	}
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 8*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.Status = strings.TrimSpace(request.Status)
	if request.Status != "active" && request.Status != "suspended" {
		writeError(w, http.StatusBadRequest, "invalid_tenant_status", "status must be active or suspended")
		return
	}
	command, err := h.db.Exec(r.Context(), `UPDATE tenants SET status = $2 WHERE id = $1`, tenantID, request.Status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tenant_update_failed", "tenant status could not be updated")
		return
	}
	if command.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "tenant_not_found", "tenant was not found")
		return
	}
	metadata, _ := json.Marshal(map[string]string{"status": request.Status})
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: tenantID, ActorType: "platform_admin", Action: "tenant.status_changed", ResourceType: "tenant", ResourceID: tenantID, Metadata: metadata})
	writeJSON(w, http.StatusOK, map[string]string{"tenant_id": tenantID, "status": request.Status})
}

func (h *Handler) createCredential(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID      string `json:"tenant_id"`
		EnvironmentID string `json:"environment_id"`
		Label         string `json:"label"`
	}
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 8*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.Label = strings.TrimSpace(request.Label)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) || request.Label == "" || len(request.Label) > 80 {
		writeError(w, http.StatusBadRequest, "invalid_credential", "tenant_id, environment_id, and a label are required")
		return
	}
	key, err := generateIntegrationKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "key_generation_failed", "integration key could not be generated")
		return
	}
	result := credentialResponse{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, Label: request.Label, IntegrationKey: key, Active: true}
	if err := h.db.QueryRow(r.Context(), `
		INSERT INTO integration_credentials (tenant_id, environment_id, key_hash, label, created_by)
		VALUES ($1, $2, $3, $4, 'platform_admin')
		RETURNING id, created_at`, request.TenantID, request.EnvironmentID, auth.Digest(key), request.Label).Scan(&result.ID, &result.CreatedAt); err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "credential_exists", "an active credential already exists for this environment")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "environment_not_found", "tenant environment was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "credential_creation_failed", "integration credential could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, ActorType: "platform_admin", Action: "credential.created", ResourceType: "credential", ResourceID: strconv.FormatInt(result.ID, 10), Metadata: auditMetadata(map[string]any{"label": request.Label})})
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) listCredentials(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	environmentID := r.URL.Query().Get("environment_id")
	rows, err := h.db.Query(r.Context(), `
		SELECT id, tenant_id, environment_id, label, created_at, last_used_at, revoked_at
		FROM integration_credentials
		WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2)
		ORDER BY created_at DESC, id DESC`, tenantID, environmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "credential_lookup_failed", "credentials could not be read")
		return
	}
	defer rows.Close()
	result := make([]credentialResponse, 0)
	for rows.Next() {
		var credential credentialResponse
		if err := rows.Scan(&credential.ID, &credential.TenantID, &credential.EnvironmentID, &credential.Label, &credential.CreatedAt, &credential.LastUsedAt, &credential.RevokedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "credential_lookup_failed", "credentials could not be read")
			return
		}
		credential.Active = credential.RevokedAt == nil
		result = append(result, credential)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "credential_lookup_failed", "credentials could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) rotateCredential(w http.ResponseWriter, r *http.Request) {
	id, ok := credentialIDFromPath(r.URL.Path, "/rotate")
	if !ok {
		writeError(w, http.StatusNotFound, "credential_not_found", "credential was not found")
		return
	}
	key, err := generateIntegrationKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "key_generation_failed", "integration key could not be generated")
		return
	}
	var result credentialResponse
	err = h.db.QueryRow(r.Context(), `
		UPDATE integration_credentials
		SET key_hash = $2, revoked_at = NULL, last_used_at = NULL
		WHERE id = $1
		RETURNING id, tenant_id, environment_id, label, created_at, last_used_at, revoked_at`, id, auth.Digest(key)).Scan(&result.ID, &result.TenantID, &result.EnvironmentID, &result.Label, &result.CreatedAt, &result.LastUsedAt, &result.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "credential_not_found", "credential was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "credential_rotation_failed", "credential could not be rotated")
		return
	}
	result.Active, result.IntegrationKey = true, key
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: result.TenantID, EnvironmentID: result.EnvironmentID, ActorType: "platform_admin", Action: "credential.rotated", ResourceType: "credential", ResourceID: strconv.FormatInt(id, 10)})
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) revokeCredential(w http.ResponseWriter, r *http.Request) {
	id, ok := credentialIDFromPath(r.URL.Path, "/revoke")
	if !ok {
		writeError(w, http.StatusNotFound, "credential_not_found", "credential was not found")
		return
	}
	var result credentialResponse
	err := h.db.QueryRow(r.Context(), `
		UPDATE integration_credentials SET revoked_at = COALESCE(revoked_at, now())
		WHERE id = $1
		RETURNING id, tenant_id, environment_id, label, created_at, last_used_at, revoked_at`, id).Scan(&result.ID, &result.TenantID, &result.EnvironmentID, &result.Label, &result.CreatedAt, &result.LastUsedAt, &result.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "credential_not_found", "credential was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "credential_revocation_failed", "credential could not be revoked")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: result.TenantID, EnvironmentID: result.EnvironmentID, ActorType: "platform_admin", Action: "credential.revoked", ResourceType: "credential", ResourceID: strconv.FormatInt(id, 10)})
	result.Active = false
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listAuditLog(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	entries, err := audit.List(r.Context(), h.db, r.URL.Query().Get("tenant_id"), r.URL.Query().Get("environment_id"), r.URL.Query().Get("action"), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "audit_lookup_failed", "audit log could not be read")
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (h *Handler) listPlans(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `SELECT id, name, description, billing_interval, price_cents, entitlements, active, updated_at FROM plans ORDER BY id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "plan_lookup_failed", "plans could not be read")
		return
	}
	defer rows.Close()
	result := make([]planResponse, 0)
	for rows.Next() {
		var plan planResponse
		var raw []byte
		if err := rows.Scan(&plan.ID, &plan.Name, &plan.Description, &plan.BillingInterval, &plan.PriceCents, &raw, &plan.Active, &plan.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "plan_lookup_failed", "plans could not be read")
			return
		}
		plan.Entitlements, err = entitlements.ParseLimits(raw)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "plan_lookup_failed", "plan entitlements are invalid")
			return
		}
		result = append(result, plan)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "plan_lookup_failed", "plans could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type planRequest struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Description     string           `json:"description"`
	BillingInterval string           `json:"billing_interval"`
	PriceCents      int64            `json:"price_cents"`
	Entitlements    map[string]int64 `json:"entitlements"`
}

func validatePlanRequest(request planRequest, requireID bool) (planRequest, error) {
	request.ID = strings.TrimSpace(request.ID)
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	request.BillingInterval = strings.ToLower(strings.TrimSpace(request.BillingInterval))
	if request.BillingInterval == "" {
		request.BillingInterval = "manual"
	}
	if requireID && !identifierPattern.MatchString(request.ID) {
		return planRequest{}, fmt.Errorf("id must be a valid identifier")
	}
	if request.Name == "" || len(request.Name) > 120 {
		return planRequest{}, fmt.Errorf("name must be between 1 and 120 characters")
	}
	if request.Description != "" && len(request.Description) > 500 {
		return planRequest{}, fmt.Errorf("description must be 500 characters or fewer")
	}
	if request.BillingInterval != "manual" && request.BillingInterval != "monthly" && request.BillingInterval != "yearly" {
		return planRequest{}, fmt.Errorf("billing_interval must be manual, monthly, or yearly")
	}
	if request.PriceCents < 0 {
		return planRequest{}, fmt.Errorf("price_cents must be non-negative")
	}
	if request.Entitlements == nil {
		request.Entitlements = map[string]int64{}
	}
	if err := entitlements.ValidateLimits(request.Entitlements); err != nil {
		return planRequest{}, err
	}
	return request, nil
}

func (h *Handler) createPlan(w http.ResponseWriter, r *http.Request) {
	var request planRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 32*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := validatePlanRequest(request, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_plan", err.Error())
		return
	}
	raw, _ := json.Marshal(request.Entitlements)
	var response planResponse
	err = h.db.QueryRow(r.Context(), `
		INSERT INTO plans (id, name, description, billing_interval, price_cents, entitlements)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		RETURNING id, name, description, billing_interval, price_cents, entitlements, active, updated_at`, request.ID, request.Name, request.Description, request.BillingInterval, request.PriceCents, raw).
		Scan(&response.ID, &response.Name, &response.Description, &response.BillingInterval, &response.PriceCents, &raw, &response.Active, &response.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "plan_exists", "plan already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "plan_creation_failed", "plan could not be created")
		return
	}
	response.Entitlements, _ = entitlements.ParseLimits(raw)
	_ = audit.Record(r.Context(), h.db, audit.Entry{ActorType: "platform_admin", Action: "plan.created", ResourceType: "plan", ResourceID: response.ID, Metadata: auditMetadata(map[string]any{"billing_interval": response.BillingInterval, "price_cents": response.PriceCents})})
	writeJSON(w, http.StatusCreated, response)
}

func planIDFromPath(path string, suffix string) (string, bool) {
	value := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/admin/plans/"), suffix)
	if value == "" || strings.Contains(value, "/") || !identifierPattern.MatchString(value) {
		return "", false
	}
	return value, true
}

func (h *Handler) updatePlan(w http.ResponseWriter, r *http.Request) {
	planID, ok := planIDFromPath(r.URL.Path, "")
	if !ok {
		writeError(w, http.StatusNotFound, "plan_not_found", "plan was not found")
		return
	}
	var request planRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 32*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.ID = planID
	request, err := validatePlanRequest(request, true)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_plan", err.Error())
		return
	}
	raw, _ := json.Marshal(request.Entitlements)
	var response planResponse
	err = h.db.QueryRow(r.Context(), `
		UPDATE plans SET name = $2, description = $3, billing_interval = $4, price_cents = $5, entitlements = $6::jsonb, updated_at = now()
		WHERE id = $1
		RETURNING id, name, description, billing_interval, price_cents, entitlements, active, updated_at`, planID, request.Name, request.Description, request.BillingInterval, request.PriceCents, raw).
		Scan(&response.ID, &response.Name, &response.Description, &response.BillingInterval, &response.PriceCents, &raw, &response.Active, &response.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "plan_not_found", "plan was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "plan_update_failed", "plan could not be updated")
		return
	}
	response.Entitlements, _ = entitlements.ParseLimits(raw)
	for _, tenantID := range h.planTenantIDs(r.Context(), planID) {
		h.limits.Invalidate(tenantID)
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{ActorType: "platform_admin", Action: "plan.updated", ResourceType: "plan", ResourceID: planID})
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) archivePlan(w http.ResponseWriter, r *http.Request) {
	planID, ok := planIDFromPath(r.URL.Path, "/archive")
	if !ok {
		writeError(w, http.StatusNotFound, "plan_not_found", "plan was not found")
		return
	}
	var active bool
	err := h.db.QueryRow(r.Context(), `UPDATE plans SET active = false, updated_at = now() WHERE id = $1 RETURNING active`, planID).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "plan_not_found", "plan was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "plan_archive_failed", "plan could not be archived")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{ActorType: "platform_admin", Action: "plan.archived", ResourceType: "plan", ResourceID: planID})
	writeJSON(w, http.StatusOK, map[string]any{"id": planID, "active": active})
}

func (h *Handler) planTenantIDs(ctx context.Context, planID string) []string {
	rows, err := h.db.Query(ctx, `SELECT tenant_id FROM tenant_plans WHERE plan_id = $1`, planID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

type usageResponse struct {
	TenantID    string           `json:"tenant_id"`
	Usage       map[string]int64 `json:"usage"`
	Limits      map[string]int64 `json:"limits"`
	GeneratedAt time.Time        `json:"generated_at"`
}

func (h *Handler) usage(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	if !identifierPattern.MatchString(tenantID) {
		writeError(w, http.StatusBadRequest, "invalid_tenant", "tenant_id is required")
		return
	}
	var usage usageResponse
	usage.TenantID = tenantID
	usage.Usage = map[string]int64{}
	queries := map[string]string{
		"events":     `SELECT count(*) FROM events WHERE tenant_id = $1`,
		"rules":      `SELECT count(*) FROM rules WHERE tenant_id = $1`,
		"currencies": `SELECT count(*) FROM currencies WHERE tenant_id = $1`,
		"rewards":    `SELECT count(*) FROM rewards WHERE tenant_id = $1`,
	}
	for key, query := range queries {
		var count int64
		if err := h.db.QueryRow(r.Context(), query, tenantID).Scan(&count); err != nil {
			writeError(w, http.StatusInternalServerError, "usage_lookup_failed", "tenant usage could not be read")
			return
		}
		usage.Usage[key] = count
	}
	var maxPayload int64
	if err := h.db.QueryRow(r.Context(), `SELECT coalesce(max(octet_length(payload::text)), 0) FROM events WHERE tenant_id = $1`, tenantID).Scan(&maxPayload); err != nil {
		writeError(w, http.StatusInternalServerError, "usage_lookup_failed", "tenant usage could not be read")
		return
	}
	usage.Usage["event_payload_bytes"] = maxPayload
	plan, err := h.limits.Load(r.Context(), tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, "tenant_plan_not_found", "tenant plan could not be read")
		return
	}
	usage.Limits = plan.Entitlements
	usage.GeneratedAt = time.Now().UTC()
	writeJSON(w, http.StatusOK, usage)
}

func (h *Handler) listTenantPlans(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	rows, err := h.db.Query(r.Context(), `
		SELECT tp.tenant_id, tp.plan_id, p.name, p.entitlements, tp.overrides, tp.assigned_at, tp.updated_at
		FROM tenant_plans tp JOIN plans p ON p.id = tp.plan_id
		WHERE ($1 = '' OR tp.tenant_id = $1) ORDER BY tp.tenant_id`, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tenant_plan_lookup_failed", "tenant plans could not be read")
		return
	}
	defer rows.Close()
	result := make([]tenantPlanResponse, 0)
	for rows.Next() {
		var item tenantPlanResponse
		var planRaw, overrideRaw []byte
		if err := rows.Scan(&item.TenantID, &item.PlanID, &item.PlanName, &planRaw, &overrideRaw, &item.AssignedAt, &item.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "tenant_plan_lookup_failed", "tenant plans could not be read")
			return
		}
		var parseErr error
		item.Entitlements, parseErr = entitlements.ParseLimits(planRaw)
		if parseErr == nil {
			item.Overrides, parseErr = entitlements.ParseLimits(overrideRaw)
		}
		if parseErr != nil {
			writeError(w, http.StatusInternalServerError, "tenant_plan_lookup_failed", "tenant plan entitlements are invalid")
			return
		}
		item.Effective = make(map[string]int64, len(item.Entitlements)+len(item.Overrides))
		for key, value := range item.Entitlements {
			item.Effective[key] = value
		}
		for key, value := range item.Overrides {
			item.Effective[key] = value
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "tenant_plan_lookup_failed", "tenant plans could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) assignTenantPlan(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID  string          `json:"tenant_id"`
		PlanID    string          `json:"plan_id"`
		Overrides json.RawMessage `json:"overrides"`
	}
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.PlanID = strings.TrimSpace(request.PlanID)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.PlanID) {
		writeError(w, http.StatusBadRequest, "invalid_tenant_plan", "tenant_id and plan_id must be valid identifiers")
		return
	}
	if len(request.Overrides) == 0 {
		request.Overrides = json.RawMessage(`{}`)
	}
	overrides, err := entitlements.ParseLimits(request.Overrides)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_tenant_plan", "overrides must contain non-negative integer limits")
		return
	}
	var planActive bool
	if err := h.db.QueryRow(r.Context(), `SELECT active FROM plans WHERE id = $1`, request.PlanID).Scan(&planActive); errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "plan_not_found", "plan was not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "tenant_plan_update_failed", "plan could not be read")
		return
	} else if !planActive {
		writeError(w, http.StatusConflict, "plan_inactive", "an archived plan cannot be assigned")
		return
	}
	if _, err := h.db.Exec(r.Context(), `
		INSERT INTO tenant_plans (tenant_id, plan_id, overrides, updated_at)
		VALUES ($1, $2, $3::jsonb, now())
		ON CONFLICT (tenant_id) DO UPDATE SET plan_id = EXCLUDED.plan_id, overrides = EXCLUDED.overrides, updated_at = now()`, request.TenantID, request.PlanID, request.Overrides); err != nil {
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "tenant_or_plan_not_found", "tenant or plan was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "tenant_plan_update_failed", "tenant plan could not be assigned")
		return
	}
	h.limits.Invalidate(request.TenantID)
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: request.TenantID, ActorType: "platform_admin", Action: "tenant_plan.assigned", ResourceType: "tenant_plan", ResourceID: request.TenantID, Metadata: auditMetadata(map[string]any{"plan_id": request.PlanID, "overrides": overrides})})
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": request.TenantID, "plan_id": request.PlanID, "overrides": overrides})
}

func credentialIDFromPath(path, suffix string) (int64, bool) {
	value := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/admin/credentials/"), suffix)
	if value == "" || strings.Contains(value, "/") {
		return 0, false
	}
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id > 0
}

func (h *Handler) createEventType(w http.ResponseWriter, r *http.Request) {
	var request EventTypeRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 64*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := ValidateEventTypeRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_event_type", err.Error())
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "event_type_creation_failed", "event type could not be created")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var version int
	err = tx.QueryRow(r.Context(), `SELECT current_version FROM event_types WHERE tenant_id = $1 AND environment_id = $2 AND name = $3 FOR UPDATE`, request.TenantID, request.EnvironmentID, request.Name).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		version = 1
		_, err = tx.Exec(r.Context(), `
			INSERT INTO event_types (tenant_id, environment_id, name, schema, current_version)
			VALUES ($1, $2, $3, $4::jsonb, $5)`, request.TenantID, request.EnvironmentID, request.Name, request.Schema, version)
	} else if err == nil {
		version++
		_, err = tx.Exec(r.Context(), `UPDATE event_types SET schema = $4::jsonb, current_version = $5, updated_at = now() WHERE tenant_id = $1 AND environment_id = $2 AND name = $3`, request.TenantID, request.EnvironmentID, request.Name, request.Schema, version)
	}
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO event_type_versions (tenant_id, environment_id, name, version, schema) VALUES ($1, $2, $3, $4, $5::jsonb)`, request.TenantID, request.EnvironmentID, request.Name, version, request.Schema)
	}
	if err == nil {
		err = tx.Commit(r.Context())
	}
	if err != nil {
		switch {
		case isForeignKeyViolation(err):
			writeError(w, http.StatusNotFound, "environment_not_found", "tenant environment was not found")
		default:
			writeError(w, http.StatusInternalServerError, "event_type_creation_failed", "event type could not be created")
		}
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, ActorType: "platform_admin", Action: "event_type.version_created", ResourceType: "event_type", ResourceID: request.Name, Metadata: auditMetadata(map[string]any{"version": version})})
	writeJSON(w, http.StatusCreated, eventTypeResponse{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, Name: request.Name, Schema: request.Schema, Version: version})
}

func (h *Handler) listEventTypes(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	environmentID := r.URL.Query().Get("environment_id")
	rows, err := h.db.Query(r.Context(), `
		SELECT tenant_id, environment_id, name, schema, current_version, active, created_at
		FROM event_types
		WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2)
		ORDER BY created_at DESC, name`, tenantID, environmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "event_type_lookup_failed", "event types could not be read")
		return
	}
	defer rows.Close()
	result := make([]eventTypeResponse, 0)
	for rows.Next() {
		var eventType eventTypeResponse
		if err := rows.Scan(&eventType.TenantID, &eventType.EnvironmentID, &eventType.Name, &eventType.Schema, &eventType.Version, &eventType.Active, &eventType.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "event_type_lookup_failed", "event types could not be read")
			return
		}
		result = append(result, eventType)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "event_type_lookup_failed", "event types could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listEventTypeVersions(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/admin/event-types/"), "/versions")
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	environmentID := strings.TrimSpace(r.URL.Query().Get("environment_id"))
	if name == "" || strings.Contains(name, "/") || tenantID == "" || environmentID == "" {
		writeError(w, http.StatusBadRequest, "invalid_event_type", "event type name, tenant_id, and environment_id are required")
		return
	}
	rows, err := h.db.Query(r.Context(), `
		SELECT tenant_id, environment_id, name, schema, version, created_at
		FROM event_type_versions
		WHERE tenant_id = $1 AND environment_id = $2 AND name = $3
		ORDER BY version DESC`, tenantID, environmentID, name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "event_type_version_lookup_failed", "event type versions could not be read")
		return
	}
	defer rows.Close()
	result := make([]eventTypeResponse, 0)
	for rows.Next() {
		var eventType eventTypeResponse
		if err := rows.Scan(&eventType.TenantID, &eventType.EnvironmentID, &eventType.Name, &eventType.Schema, &eventType.Version, &eventType.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "event_type_version_lookup_failed", "event type versions could not be read")
			return
		}
		eventType.Active = true
		result = append(result, eventType)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "event_type_version_lookup_failed", "event type versions could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	offset, err := parseListOffset(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_offset", "offset must be between 0 and 100000")
		return
	}
	tenantID := r.URL.Query().Get("tenant_id")
	environmentID := r.URL.Query().Get("environment_id")
	status := r.URL.Query().Get("status")
	if status != "" && status != "accepted" && status != "processing" && status != "processed" && status != "failed" {
		writeError(w, http.StatusBadRequest, "invalid_status", "status must be accepted, processing, processed, or failed")
		return
	}
	rows, err := h.db.Query(r.Context(), `
		SELECT id, tenant_id, environment_id, external_event_id, event_type, status, received_at, processed_at
		FROM events
		WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2) AND ($3 = '' OR status = $3)
		ORDER BY received_at DESC
		LIMIT $4 OFFSET $5`, tenantID, environmentID, status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "event_lookup_failed", "events could not be read")
		return
	}
	defer rows.Close()
	result := make([]eventAdminResponse, 0, limit)
	for rows.Next() {
		var event eventAdminResponse
		if err := rows.Scan(&event.EventID, &event.TenantID, &event.EnvironmentID, &event.ExternalEventID, &event.EventType, &event.Status, &event.ReceivedAt, &event.ProcessedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "event_lookup_failed", "events could not be read")
			return
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "event_lookup_failed", "events could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listRuleVersions(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseRuleVersionCollectionPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "rule_not_found", "rule was not found")
		return
	}
	rows, err := h.db.Query(r.Context(), `
		SELECT id, rule_id, version, event_type, conditions, action, status, created_at, published_at, starts_at, ends_at
		FROM rule_versions
		WHERE rule_id = $1
		ORDER BY version DESC`, ruleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rule_version_lookup_failed", "rule versions could not be read")
		return
	}
	defer rows.Close()
	result := make([]ruleVersionResponse, 0)
	for rows.Next() {
		var version ruleVersionResponse
		if err := rows.Scan(&version.ID, &version.RuleID, &version.Version, &version.EventType, &version.Conditions, &version.Action, &version.Status, &version.CreatedAt, &version.PublishedAt, &version.StartsAt, &version.EndsAt); err != nil {
			writeError(w, http.StatusInternalServerError, "rule_version_lookup_failed", "rule versions could not be read")
			return
		}
		result = append(result, version)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "rule_version_lookup_failed", "rule versions could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) diagnostics(w http.ResponseWriter, r *http.Request) {
	result := diagnosticsResponse{EventCounts: map[string]int64{}, GeneratedAt: time.Now().UTC()}
	tenantID := r.URL.Query().Get("tenant_id")
	environmentID := r.URL.Query().Get("environment_id")
	rows, err := h.db.Query(r.Context(), `SELECT status, count(*) FROM events WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2) GROUP BY status`, tenantID, environmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "diagnostics_lookup_failed", "diagnostics could not be read")
		return
	}
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "diagnostics_lookup_failed", "diagnostics could not be read")
			return
		}
		result.EventCounts[status] = count
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		writeError(w, http.StatusInternalServerError, "diagnostics_lookup_failed", "diagnostics could not be read")
		return
	}
	rows.Close()
	queries := []struct {
		target *int64
		query  string
	}{
		{&result.PendingOutbox, `SELECT count(*) FROM event_outbox o JOIN events e ON e.id = o.event_id WHERE o.published_at IS NULL AND ($1 = '' OR e.tenant_id = $1) AND ($2 = '' OR e.environment_id = $2)`},
		{&result.FailedRuleExecutions, `SELECT count(*) FROM rule_executions x JOIN events e ON e.id = x.event_id WHERE x.status = 'failed' AND ($1 = '' OR e.tenant_id = $1) AND ($2 = '' OR e.environment_id = $2)`},
		{&result.RetryableRuleExecutions, `SELECT count(*) FROM rule_executions x JOIN events e ON e.id = x.event_id WHERE x.status = 'failed' AND x.attempts < 3 AND ($1 = '' OR e.tenant_id = $1) AND ($2 = '' OR e.environment_id = $2)`},
		{&result.ActiveRules, `SELECT count(*) FROM rules WHERE status = 'published' AND ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2)`},
		{&result.FailedRedemptions, `SELECT count(*) FROM redemptions WHERE status = 'failed' AND ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2)`},
	}
	for _, item := range queries {
		if err := h.db.QueryRow(r.Context(), item.query, tenantID, environmentID).Scan(item.target); err != nil {
			writeError(w, http.StatusInternalServerError, "diagnostics_lookup_failed", "diagnostics could not be read")
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	var request RuleRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 64*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := ValidateRuleRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_rule", err.Error())
		return
	}
	ruleID, err := generateRuleID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rule_creation_failed", "rule could not be created")
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rule_creation_failed", "rule could not be created")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if err := h.enforceResourceLimitTx(r.Context(), tx, request.TenantID, request.EnvironmentID, "max_rules", "rules"); err != nil {
		h.writePlanLimitError(w, err)
		return
	}
	var rule ruleResponse
	err = tx.QueryRow(r.Context(), `
		INSERT INTO rules (id, tenant_id, environment_id, name, description, event_type, conditions, action, starts_at, ends_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8::jsonb, $9, $10)
		RETURNING id, tenant_id, environment_id, name, description, event_type, conditions, action, status, version, created_at, updated_at, published_at, starts_at, ends_at`, ruleID, request.TenantID, request.EnvironmentID, request.Name, request.Description, request.EventType, request.Conditions, request.Action, request.StartsAt, request.EndsAt).
		Scan(&rule.ID, &rule.TenantID, &rule.EnvironmentID, &rule.Name, &rule.Description, &rule.EventType, &rule.Conditions, &rule.Action, &rule.Status, &rule.Version, &rule.CreatedAt, &rule.UpdatedAt, &rule.PublishedAt, &rule.StartsAt, &rule.EndsAt)
	if err == nil {
		_, err = tx.Exec(r.Context(), `
			INSERT INTO rule_versions (id, rule_id, version, event_type, conditions, action, starts_at, ends_at, status, created_at, published_at)
			VALUES ($1, $2, 1, $3, $4::jsonb, $5::jsonb, $6, $7, 'draft', $8, NULL)`, ruleID+":v1", ruleID, request.EventType, request.Conditions, request.Action, request.StartsAt, request.EndsAt, rule.CreatedAt)
	}
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "rule_exists", "a rule with this name already exists in the environment")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "environment_not_found", "tenant environment was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "rule_creation_failed", "rule could not be created")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "rule_creation_failed", "rule could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: rule.TenantID, EnvironmentID: rule.EnvironmentID, ActorType: "platform_admin", Action: "rule.created", ResourceType: "rule", ResourceID: rule.ID, Metadata: auditMetadata(map[string]any{"version": rule.Version})})
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) createRuleVersion(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseRuleVersionCollectionPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "rule_not_found", "rule was not found")
		return
	}
	var request struct {
		EventType  string          `json:"event_type"`
		Conditions json.RawMessage `json:"conditions"`
		Action     json.RawMessage `json:"action"`
		StartsAt   *time.Time      `json:"starts_at,omitempty"`
		EndsAt     *time.Time      `json:"ends_at,omitempty"`
	}
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 64*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if request.EventType == "" {
		writeError(w, http.StatusBadRequest, "invalid_rule", "event_type is required")
		return
	}
	normalized, err := ValidateRuleRequest(RuleRequest{TenantID: "valid", EnvironmentID: "valid", Name: "version", EventType: request.EventType, Conditions: request.Conditions, Action: request.Action, StartsAt: request.StartsAt, EndsAt: request.EndsAt})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_rule", err.Error())
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rule_version_creation_failed", "rule version could not be created")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var version int
	var tenantID, environmentID string
	if err := tx.QueryRow(r.Context(), `SELECT coalesce(max(v.version), 0) + 1, r.tenant_id, r.environment_id FROM rules r LEFT JOIN rule_versions v ON v.rule_id = r.id WHERE r.id = $1 GROUP BY r.id, r.tenant_id, r.environment_id`, ruleID).Scan(&version, &tenantID, &environmentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "rule_not_found", "rule was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "rule_version_creation_failed", "rule version could not be created")
		return
	}
	versionID := fmt.Sprintf("%s:v%d", ruleID, version)
	if _, err := tx.Exec(r.Context(), `INSERT INTO rule_versions (id, rule_id, version, event_type, conditions, action, starts_at, ends_at) VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8)`, versionID, ruleID, version, normalized.EventType, normalized.Conditions, normalized.Action, normalized.StartsAt, normalized.EndsAt); err != nil {
		writeError(w, http.StatusInternalServerError, "rule_version_creation_failed", "rule version could not be created")
		return
	}
	var rule ruleResponse
	if err := tx.QueryRow(r.Context(), `UPDATE rules SET event_type = $2, conditions = $3::jsonb, action = $4::jsonb, starts_at = $5, ends_at = $6, version = $7, status = 'draft', published_at = NULL, updated_at = now() WHERE id = $1 RETURNING id, tenant_id, environment_id, name, description, event_type, conditions, action, status, version, created_at, updated_at, published_at, starts_at, ends_at`, ruleID, normalized.EventType, normalized.Conditions, normalized.Action, normalized.StartsAt, normalized.EndsAt, version).Scan(&rule.ID, &rule.TenantID, &rule.EnvironmentID, &rule.Name, &rule.Description, &rule.EventType, &rule.Conditions, &rule.Action, &rule.Status, &rule.Version, &rule.CreatedAt, &rule.UpdatedAt, &rule.PublishedAt, &rule.StartsAt, &rule.EndsAt); err != nil {
		writeError(w, http.StatusInternalServerError, "rule_version_creation_failed", "rule version could not be created")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "rule_version_creation_failed", "rule version could not be created")
		return
	}
	_ = tenantID
	_ = environmentID
	writeJSON(w, http.StatusCreated, rule)
}

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	environmentID := r.URL.Query().Get("environment_id")
	rows, err := h.db.Query(r.Context(), `
		SELECT id, tenant_id, environment_id, name, description, event_type, conditions, action, status, version, created_at, updated_at, published_at, starts_at, ends_at
		FROM rules
		WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2)
		ORDER BY updated_at DESC, name`, tenantID, environmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rule_lookup_failed", "rules could not be read")
		return
	}
	defer rows.Close()
	result := make([]ruleResponse, 0)
	for rows.Next() {
		var rule ruleResponse
		if err := rows.Scan(&rule.ID, &rule.TenantID, &rule.EnvironmentID, &rule.Name, &rule.Description, &rule.EventType, &rule.Conditions, &rule.Action, &rule.Status, &rule.Version, &rule.CreatedAt, &rule.UpdatedAt, &rule.PublishedAt, &rule.StartsAt, &rule.EndsAt); err != nil {
			writeError(w, http.StatusInternalServerError, "rule_lookup_failed", "rules could not be read")
			return
		}
		result = append(result, rule)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "rule_lookup_failed", "rules could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) publishRule(w http.ResponseWriter, r *http.Request) {
	ruleID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/admin/rules/"), "/publish")
	ruleID = strings.TrimSuffix(ruleID, "/")
	if ruleID == "" || strings.Contains(ruleID, "/") {
		writeError(w, http.StatusNotFound, "rule_not_found", "rule was not found")
		return
	}
	var version int
	if err := h.db.QueryRow(r.Context(), `SELECT version FROM rules WHERE id = $1 AND status <> 'archived'`, ruleID).Scan(&version); errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "rule_not_found", "rule was not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "rule_publish_failed", "rule could not be published")
		return
	}
	rule, err := h.publishRuleVersionTx(r.Context(), ruleID, version)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "rule_not_found", "rule was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rule_publish_failed", "rule could not be published")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: rule.TenantID, EnvironmentID: rule.EnvironmentID, ActorType: "platform_admin", Action: "rule.published", ResourceType: "rule", ResourceID: rule.ID, Metadata: auditMetadata(map[string]any{"version": rule.Version})})
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handler) publishRuleVersion(w http.ResponseWriter, r *http.Request) {
	ruleID, version, ok := parseRuleVersionPublishPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "rule_version_not_found", "rule version was not found")
		return
	}
	rule, err := h.publishRuleVersionTx(r.Context(), ruleID, version)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "rule_version_not_found", "rule version was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rule_publish_failed", "rule version could not be published")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: rule.TenantID, EnvironmentID: rule.EnvironmentID, ActorType: "platform_admin", Action: "rule.published", ResourceType: "rule", ResourceID: rule.ID, Metadata: auditMetadata(map[string]any{"version": rule.Version})})
	writeJSON(w, http.StatusOK, rule)
}

func (h *Handler) publishRuleVersionTx(ctx context.Context, ruleID string, version int) (ruleResponse, error) {
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return ruleResponse{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE rule_versions SET status = 'archived' WHERE rule_id = $1 AND status = 'published'`, ruleID); err != nil {
		return ruleResponse{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE rule_versions SET status = 'published', published_at = now() WHERE rule_id = $1 AND version = $2 AND status <> 'archived'`, ruleID, version); err != nil {
		return ruleResponse{}, err
	}
	var rule ruleResponse
	err = tx.QueryRow(ctx, `
		UPDATE rules r
		SET status = 'published', version = v.version, event_type = v.event_type, conditions = v.conditions, action = v.action, starts_at = v.starts_at, ends_at = v.ends_at, published_at = now(), updated_at = now()
		FROM rule_versions v
		WHERE r.id = $1 AND v.rule_id = r.id AND v.version = $2 AND v.status = 'published'
		RETURNING r.id, r.tenant_id, r.environment_id, r.name, r.description, r.event_type, r.conditions, r.action, r.status, r.version, r.created_at, r.updated_at, r.published_at, r.starts_at, r.ends_at`, ruleID, version).
		Scan(&rule.ID, &rule.TenantID, &rule.EnvironmentID, &rule.Name, &rule.Description, &rule.EventType, &rule.Conditions, &rule.Action, &rule.Status, &rule.Version, &rule.CreatedAt, &rule.UpdatedAt, &rule.PublishedAt, &rule.StartsAt, &rule.EndsAt)
	if err != nil {
		return ruleResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ruleResponse{}, err
	}
	return rule, nil
}

func parseRuleVersionCollectionPath(path string) (string, bool) {
	value := strings.TrimPrefix(path, "/v1/admin/rules/")
	value = strings.TrimSuffix(value, "/versions")
	return value, value != "" && !strings.Contains(value, "/")
}

func parseRuleVersionPublishPath(path string) (string, int, bool) {
	value := strings.TrimPrefix(path, "/v1/admin/rules/")
	parts := strings.Split(strings.TrimSuffix(value, "/publish"), "/versions/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.Contains(parts[0], "/") {
		return "", 0, false
	}
	version, err := strconv.Atoi(parts[1])
	return parts[0], version, err == nil && version > 0
}

func parseListOffset(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("offset")
	if raw == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(raw)
	if err != nil || offset < 0 || offset > 100000 {
		return 0, errors.New("invalid offset")
	}
	return offset, nil
}

func (h *Handler) listRuleExecutions(w http.ResponseWriter, r *http.Request) {
	limit := 8
	if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	offset, err := parseListOffset(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_offset", "offset must be between 0 and 100000")
		return
	}
	statusFilter := r.URL.Query().Get("status")
	if statusFilter != "" && statusFilter != "pending" && statusFilter != "recorded" && statusFilter != "failed" {
		writeError(w, http.StatusBadRequest, "invalid_status", "status must be pending, recorded, or failed")
		return
	}
	tenantID := r.URL.Query().Get("tenant_id")
	environmentID := r.URL.Query().Get("environment_id")
	rows, err := h.db.Query(r.Context(), `
		SELECT x.id, x.rule_id, x.rule_version_id, r.name, x.event_id, e.event_type, x.action_type, x.status, x.attempts, COALESCE(x.last_error, ''), x.started_at, x.completed_at, x.created_at
		FROM rule_executions x
		JOIN rules r ON r.id = x.rule_id
		JOIN events e ON e.id = x.event_id
		WHERE ($1 = '' OR x.status = $1)
		  AND ($2 = '' OR e.tenant_id = $2)
		  AND ($3 = '' OR e.environment_id = $3)
		ORDER BY x.created_at DESC
		LIMIT $4 OFFSET $5`, statusFilter, tenantID, environmentID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rule_execution_lookup_failed", "rule executions could not be read")
		return
	}
	defer rows.Close()
	result := make([]ruleExecutionResponse, 0, limit)
	for rows.Next() {
		var execution ruleExecutionResponse
		if err := rows.Scan(&execution.ID, &execution.RuleID, &execution.RuleVersionID, &execution.RuleName, &execution.EventID, &execution.EventType, &execution.ActionType, &execution.Status, &execution.Attempts, &execution.LastError, &execution.StartedAt, &execution.CompletedAt, &execution.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "rule_execution_lookup_failed", "rule executions could not be read")
			return
		}
		result = append(result, execution)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "rule_execution_lookup_failed", "rule executions could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createCurrency(w http.ResponseWriter, r *http.Request) {
	var request points.CurrencyRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 32*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := points.ValidateCurrencyRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_currency", err.Error())
		return
	}
	currencyID, err := generateCurrencyID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "currency_creation_failed", "currency could not be created")
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "currency_creation_failed", "currency could not be created")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if err := h.enforceResourceLimitTx(r.Context(), tx, request.TenantID, request.EnvironmentID, "max_currencies", "currencies"); err != nil {
		h.writePlanLimitError(w, err)
		return
	}
	var currency points.Currency
	err = tx.QueryRow(r.Context(), `
		INSERT INTO currencies (id, tenant_id, environment_id, code, name, unit_scale)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, tenant_id, environment_id, code, name, unit_scale, policy_version, active, created_at, updated_at`, currencyID, request.TenantID, request.EnvironmentID, request.Code, request.Name, request.UnitScale).
		Scan(&currency.ID, &currency.TenantID, &currency.EnvironmentID, &currency.Code, &currency.Name, &currency.UnitScale, &currency.PolicyVersion, &currency.Active, &currency.CreatedAt, &currency.UpdatedAt)
	if err == nil {
		_, err = tx.Exec(r.Context(), `INSERT INTO currency_policy_versions (currency_id, version, policy) VALUES ($1, 1, $2::jsonb)`, currencyID, request.Policy)
		currency.Policy = request.Policy
	}
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "currency_exists", "a currency with this code already exists in the environment")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "environment_not_found", "tenant environment was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "currency_creation_failed", "currency could not be created")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "currency_creation_failed", "currency could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: currency.TenantID, EnvironmentID: currency.EnvironmentID, ActorType: "platform_admin", Action: "currency.created", ResourceType: "currency", ResourceID: currency.ID, Metadata: auditMetadata(map[string]any{"code": currency.Code})})
	writeJSON(w, http.StatusCreated, currency)
}

func (h *Handler) listCurrencies(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	environmentID := r.URL.Query().Get("environment_id")
	rows, err := h.db.Query(r.Context(), `
		SELECT c.id, c.tenant_id, c.environment_id, c.code, c.name, c.unit_scale, c.policy_version, p.policy, c.active, c.created_at, c.updated_at
		FROM currencies c
		JOIN currency_policy_versions p ON p.currency_id = c.id AND p.version = c.policy_version
		WHERE ($1 = '' OR c.tenant_id = $1) AND ($2 = '' OR c.environment_id = $2)
		ORDER BY c.tenant_id, c.environment_id, c.code`, tenantID, environmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "currency_lookup_failed", "currencies could not be read")
		return
	}
	defer rows.Close()
	result := make([]points.Currency, 0)
	for rows.Next() {
		var currency points.Currency
		if err := rows.Scan(&currency.ID, &currency.TenantID, &currency.EnvironmentID, &currency.Code, &currency.Name, &currency.UnitScale, &currency.PolicyVersion, &currency.Policy, &currency.Active, &currency.CreatedAt, &currency.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "currency_lookup_failed", "currencies could not be read")
			return
		}
		result = append(result, currency)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "currency_lookup_failed", "currencies could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createMission(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID      string `json:"tenant_id"`
		EnvironmentID string `json:"environment_id"`
		Name          string `json:"name"`
		EventType     string `json:"event_type"`
		TargetCount   int    `json:"target_count"`
	}
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.Name = strings.TrimSpace(request.Name)
	request.EventType = strings.TrimSpace(request.EventType)
	if request.TenantID == "" || request.EnvironmentID == "" || request.Name == "" || request.EventType == "" || request.TargetCount < 1 || request.TargetCount > 100000 {
		writeError(w, http.StatusBadRequest, "invalid_mission", "tenant, environment, name, event_type, and target_count are required")
		return
	}
	missionID, err := generateMissionID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "mission_creation_failed", "mission could not be created")
		return
	}
	var mission missionResponse
	err = h.db.QueryRow(r.Context(), `INSERT INTO missions (id, tenant_id, environment_id, name, event_type, target_count) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, tenant_id, environment_id, name, event_type, target_count, active, created_at`, missionID, request.TenantID, request.EnvironmentID, request.Name, request.EventType, request.TargetCount).Scan(&mission.ID, &mission.TenantID, &mission.EnvironmentID, &mission.Name, &mission.EventType, &mission.TargetCount, &mission.Active, &mission.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "mission_exists", "a mission with this name already exists in the environment")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "environment_not_found", "tenant environment was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "mission_creation_failed", "mission could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: mission.TenantID, EnvironmentID: mission.EnvironmentID, ActorType: "platform_admin", Action: "mission.created", ResourceType: "mission", ResourceID: mission.ID})
	writeJSON(w, http.StatusCreated, mission)
}

func (h *Handler) listMissions(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	environmentID := r.URL.Query().Get("environment_id")
	rows, err := h.db.Query(r.Context(), `SELECT id, tenant_id, environment_id, name, event_type, target_count, active, created_at FROM missions WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2) ORDER BY created_at, id`, tenantID, environmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "mission_lookup_failed", "missions could not be read")
		return
	}
	defer rows.Close()
	result := make([]missionResponse, 0)
	for rows.Next() {
		var mission missionResponse
		if err := rows.Scan(&mission.ID, &mission.TenantID, &mission.EnvironmentID, &mission.Name, &mission.EventType, &mission.TargetCount, &mission.Active, &mission.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "mission_lookup_failed", "missions could not be read")
			return
		}
		result = append(result, mission)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "mission_lookup_failed", "missions could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createReward(w http.ResponseWriter, r *http.Request) {
	var request rewards.RewardRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 64*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := rewards.ValidateRewardRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_reward", err.Error())
		return
	}
	rewardID, err := generateRewardID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reward_creation_failed", "reward could not be created")
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reward_creation_failed", "reward could not be created")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if err := h.enforceResourceLimitTx(r.Context(), tx, request.TenantID, request.EnvironmentID, "max_rewards", "rewards"); err != nil {
		h.writePlanLimitError(w, err)
		return
	}
	var costCurrencyID *string
	if request.CostCurrency != "" {
		var id string
		err = tx.QueryRow(r.Context(), `SELECT id FROM currencies WHERE tenant_id = $1 AND environment_id = $2 AND code = $3 AND active = true`, request.TenantID, request.EnvironmentID, request.CostCurrency).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "currency_not_found", "cost currency was not found in the tenant environment")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "reward_creation_failed", "reward could not be created")
			return
		}
		costCurrencyID = &id
	}
	var reward rewards.Reward
	err = tx.QueryRow(r.Context(), `
		INSERT INTO rewards (id, tenant_id, environment_id, name, reward_type, description, cost_currency_id, cost_amount, payload)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, 0), $9::jsonb)
		RETURNING id, tenant_id, environment_id, name, reward_type, description, COALESCE(cost_amount, 0), payload, active, created_at, updated_at`, rewardID, request.TenantID, request.EnvironmentID, request.Name, request.RewardType, request.Description, costCurrencyID, request.CostAmount, request.Payload).
		Scan(&reward.ID, &reward.TenantID, &reward.EnvironmentID, &reward.Name, &reward.RewardType, &reward.Description, &reward.CostAmount, &reward.Payload, &reward.Active, &reward.CreatedAt, &reward.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "reward_exists", "a reward with this name already exists in the environment")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "environment_not_found", "tenant environment was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "reward_creation_failed", "reward could not be created")
		return
	}
	reward.CostCurrency = request.CostCurrency
	if request.CostAmount == 0 {
		reward.CostAmount = 0
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "reward_creation_failed", "reward could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: reward.TenantID, EnvironmentID: reward.EnvironmentID, ActorType: "platform_admin", Action: "reward.created", ResourceType: "reward", ResourceID: reward.ID, Metadata: auditMetadata(map[string]any{"reward_type": reward.RewardType})})
	writeJSON(w, http.StatusCreated, reward)
}

func (h *Handler) listRewards(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	environmentID := r.URL.Query().Get("environment_id")
	rows, err := h.db.Query(r.Context(), `
		SELECT r.id, r.tenant_id, r.environment_id, r.name, r.reward_type, r.description,
		       COALESCE(c.code, ''), COALESCE(r.cost_amount, 0), r.payload, r.active, r.created_at, r.updated_at
		FROM rewards r LEFT JOIN currencies c ON c.id = r.cost_currency_id
		WHERE ($1 = '' OR r.tenant_id = $1) AND ($2 = '' OR r.environment_id = $2)
		ORDER BY r.created_at DESC, r.name`, tenantID, environmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reward_lookup_failed", "rewards could not be read")
		return
	}
	defer rows.Close()
	result := make([]rewards.Reward, 0)
	for rows.Next() {
		var reward rewards.Reward
		if err := rows.Scan(&reward.ID, &reward.TenantID, &reward.EnvironmentID, &reward.Name, &reward.RewardType, &reward.Description, &reward.CostCurrency, &reward.CostAmount, &reward.Payload, &reward.Active, &reward.CreatedAt, &reward.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "reward_lookup_failed", "rewards could not be read")
			return
		}
		result = append(result, reward)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "reward_lookup_failed", "rewards could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createAchievement(w http.ResponseWriter, r *http.Request) {
	var request progression.AchievementRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 32*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := progression.ValidateAchievementRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_achievement", err.Error())
		return
	}
	achievementID, err := generateAchievementID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "achievement_creation_failed", "achievement could not be created")
		return
	}
	var achievement progression.Achievement
	err = h.db.QueryRow(r.Context(), `
		INSERT INTO achievements (id, tenant_id, environment_id, name, description, event_type, required_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tenant_id, environment_id, name, description, event_type, required_count, active, created_at`, achievementID, request.TenantID, request.EnvironmentID, request.Name, request.Description, request.EventType, request.RequiredCount).
		Scan(&achievement.ID, &achievement.TenantID, &achievement.EnvironmentID, &achievement.Name, &achievement.Description, &achievement.EventType, &achievement.RequiredCount, &achievement.Active, &achievement.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "achievement_exists", "an achievement with this name already exists in the environment")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "environment_not_found", "tenant environment was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "achievement_creation_failed", "achievement could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: achievement.TenantID, EnvironmentID: achievement.EnvironmentID, ActorType: "platform_admin", Action: "achievement.created", ResourceType: "achievement", ResourceID: achievement.ID})
	writeJSON(w, http.StatusCreated, achievement)
}

func (h *Handler) listAchievements(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	environmentID := r.URL.Query().Get("environment_id")
	rows, err := h.db.Query(r.Context(), `
		SELECT id, tenant_id, environment_id, name, description, event_type, required_count, active, created_at
		FROM achievements
		WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2)
		ORDER BY created_at DESC, name`, tenantID, environmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "achievement_lookup_failed", "achievements could not be read")
		return
	}
	defer rows.Close()
	result := make([]progression.Achievement, 0)
	for rows.Next() {
		var achievement progression.Achievement
		if err := rows.Scan(&achievement.ID, &achievement.TenantID, &achievement.EnvironmentID, &achievement.Name, &achievement.Description, &achievement.EventType, &achievement.RequiredCount, &achievement.Active, &achievement.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "achievement_lookup_failed", "achievements could not be read")
			return
		}
		result = append(result, achievement)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "achievement_lookup_failed", "achievements could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createLevel(w http.ResponseWriter, r *http.Request) {
	var request progression.LevelRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := progression.ValidateLevelRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_level", err.Error())
		return
	}
	levelID, err := generateLevelID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "level_creation_failed", "level could not be created")
		return
	}
	active := true
	if request.Active != nil {
		active = *request.Active
	}
	var level progression.Level
	err = h.db.QueryRow(r.Context(), `
		INSERT INTO level_definitions (id, tenant_id, environment_id, level, name, xp_threshold, active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, tenant_id, environment_id, level, name, xp_threshold, active, created_at`, levelID, request.TenantID, request.EnvironmentID, request.Level, request.Name, request.XPThreshold, active).
		Scan(&level.ID, &level.TenantID, &level.EnvironmentID, &level.Level, &level.Name, &level.XPThreshold, &level.Active, &level.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "level_exists", "level or xp threshold already exists in the environment")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "environment_not_found", "tenant environment was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "level_creation_failed", "level could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: level.TenantID, EnvironmentID: level.EnvironmentID, ActorType: "platform_admin", Action: "level.created", ResourceType: "level", ResourceID: level.ID, Metadata: auditMetadata(map[string]any{"level": level.Level})})
	writeJSON(w, http.StatusCreated, level)
}

func (h *Handler) listLevels(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	environmentID := r.URL.Query().Get("environment_id")
	rows, err := h.db.Query(r.Context(), `
		SELECT id, tenant_id, environment_id, level, name, xp_threshold, active, created_at
		FROM level_definitions
		WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2)
		ORDER BY tenant_id, environment_id, level`, tenantID, environmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "level_lookup_failed", "levels could not be read")
		return
	}
	defer rows.Close()
	result := make([]progression.Level, 0)
	for rows.Next() {
		var level progression.Level
		if err := rows.Scan(&level.ID, &level.TenantID, &level.EnvironmentID, &level.Level, &level.Name, &level.XPThreshold, &level.Active, &level.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "level_lookup_failed", "levels could not be read")
			return
		}
		result = append(result, level)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "level_lookup_failed", "levels could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listRedemptions(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	tenantID := r.URL.Query().Get("tenant_id")
	environmentID := r.URL.Query().Get("environment_id")
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if strings.HasPrefix(strings.ToLower(status), "all") {
		status = ""
	}
	if status != "" && status != "requested" && status != "reserved" && status != "delivering" && status != "fulfilled" && status != "failed" && status != "cancelled" {
		writeError(w, http.StatusBadRequest, "invalid_status", "status is invalid")
		return
	}
	rows, err := h.db.Query(r.Context(), `
		SELECT d.id, d.reward_id, r.name, d.tenant_id, d.environment_id, d.external_user_id, d.status,
		       d.idempotency_key, COALESCE(d.provider_reference, ''), COALESCE(d.failure_reason, ''),
		       (SELECT count(*) FROM reward_delivery_attempts a WHERE a.redemption_id = d.id),
		       d.reconciliation_attempts, d.next_reconciliation_at, COALESCE(d.last_provider_check_error, ''),
		       d.created_at, d.updated_at
		FROM redemptions d JOIN rewards r ON r.id = d.reward_id
		WHERE ($1 = '' OR d.tenant_id = $1) AND ($2 = '' OR d.environment_id = $2) AND ($3 = '' OR d.status = $3)
		ORDER BY d.created_at DESC, d.id DESC LIMIT $4`, tenantID, environmentID, status, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "redemption_lookup_failed", "redemptions could not be read")
		return
	}
	defer rows.Close()
	result := make([]redemptionAdminResponse, 0, limit)
	for rows.Next() {
		var redemption redemptionAdminResponse
		if err := rows.Scan(&redemption.ID, &redemption.RewardID, &redemption.RewardName, &redemption.TenantID, &redemption.EnvironmentID, &redemption.ExternalUserID, &redemption.Status, &redemption.IdempotencyKey, &redemption.ProviderReference, &redemption.FailureReason, &redemption.AttemptCount, &redemption.ReconciliationAttempts, &redemption.NextReconciliationAt, &redemption.LastProviderCheckError, &redemption.CreatedAt, &redemption.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "redemption_lookup_failed", "redemptions could not be read")
			return
		}
		result = append(result, redemption)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "redemption_lookup_failed", "redemptions could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listRedemptionAttempts(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/admin/redemptions/"), "/attempts")
	id = strings.TrimSuffix(id, "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "redemption_not_found", "redemption was not found")
		return
	}
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	environmentID := strings.TrimSpace(r.URL.Query().Get("environment_id"))
	rows, err := h.db.Query(r.Context(), `
		SELECT a.id, a.redemption_id, a.kind, a.status, COALESCE(a.provider_state, ''),
		       COALESCE(a.provider_reference, ''), COALESCE(a.error_message, ''), a.created_at, a.completed_at
		FROM reward_delivery_attempts a
		JOIN redemptions d ON d.id = a.redemption_id
		WHERE a.redemption_id = $1 AND ($2 = '' OR d.tenant_id = $2) AND ($3 = '' OR d.environment_id = $3)
		ORDER BY a.created_at ASC, a.id ASC`, id, tenantID, environmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "redemption_attempt_lookup_failed", "redemption attempts could not be read")
		return
	}
	defer rows.Close()
	result := make([]redemptionAttemptAdminResponse, 0)
	for rows.Next() {
		var attempt redemptionAttemptAdminResponse
		if err := rows.Scan(&attempt.ID, &attempt.RedemptionID, &attempt.Kind, &attempt.Status, &attempt.ProviderState, &attempt.ProviderReference, &attempt.ErrorMessage, &attempt.CreatedAt, &attempt.CompletedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "redemption_attempt_lookup_failed", "redemption attempts could not be read")
			return
		}
		result = append(result, attempt)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "redemption_attempt_lookup_failed", "redemption attempts could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listRewardProviders(w http.ResponseWriter, r *http.Request) {
	configs, err := h.providerConfigs.List(r.Context(), strings.TrimSpace(r.URL.Query().Get("tenant_id")), strings.TrimSpace(r.URL.Query().Get("environment_id")))
	if errors.Is(err, rewards.ErrProviderConfigNotFound) {
		writeJSON(w, http.StatusOK, []rewardProviderConfigResponse{})
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reward_provider_lookup_failed", "reward provider configurations could not be read")
		return
	}
	result := make([]rewardProviderConfigResponse, 0, len(configs))
	for _, config := range configs {
		result = append(result, rewardProviderConfigResponse{
			TenantID: config.TenantID, EnvironmentID: config.EnvironmentID, Provider: config.Provider,
			DeliveryURL: config.URL, StatusURL: config.StatusURL, TokenConfigured: config.TokenConfigured,
			TimeoutSeconds: config.TimeoutSeconds, Active: config.Active,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) upsertRewardProvider(w http.ResponseWriter, r *http.Request) {
	var request rewardProviderConfigRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	active := true
	if request.Active != nil {
		active = *request.Active
	}
	config, err := h.providerConfigs.Upsert(r.Context(), rewards.ScopeProviderConfig{
		TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, Provider: request.Provider,
		URL: request.DeliveryURL, StatusURL: request.StatusURL, Token: request.Token,
		TimeoutSeconds: request.TimeoutSeconds, Active: active,
	})
	if err != nil {
		if errors.Is(err, rewards.ErrInvalidProviderConfig) || strings.Contains(err.Error(), "encryption key") {
			writeError(w, http.StatusBadRequest, "invalid_reward_provider", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "reward_provider_update_failed", "reward provider configuration could not be saved")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{
		TenantID: config.TenantID, EnvironmentID: config.EnvironmentID, ActorType: "platform_admin",
		Action: "reward_provider_config.updated", ResourceType: "reward_provider_config",
		ResourceID: config.TenantID + ":" + config.EnvironmentID,
		Metadata:   auditMetadata(map[string]any{"provider": config.Provider, "token_configured": config.TokenConfigured, "active": config.Active}),
	})
	writeJSON(w, http.StatusOK, rewardProviderConfigResponse{
		TenantID: config.TenantID, EnvironmentID: config.EnvironmentID, Provider: config.Provider,
		DeliveryURL: config.URL, StatusURL: config.StatusURL, TokenConfigured: config.TokenConfigured,
		TimeoutSeconds: config.TimeoutSeconds, Active: config.Active,
	})
}

func generateIntegrationKey() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "motif_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func generateRuleID() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "rule_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func generateCurrencyID() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "currency_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func generateMissionID() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "mission_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func generateRewardID() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "reward_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func generateAchievementID() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "achievement_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func generateLevelID() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return "level_" + base64.RawURLEncoding.EncodeToString(value), nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func auditMetadata(value map[string]any) json.RawMessage {
	encoded, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}
