package admin

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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
	"github.com/motifengine/motifengine/internal/entitlements"
	eventdomain "github.com/motifengine/motifengine/internal/events"
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
	rewardOps       *rewards.Handler
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
	rewardOps, _ := rewards.NewHandlerWithResolver(db, rewards.ScopeProviderResolver{Store: providerConfigs, Fallback: rewards.NewProvider(rewards.ProviderConfig{Kind: "mock"})}).(*rewards.Handler)
	return &Handler{db: db, apiKey: apiKey, limits: entitlements.NewStore(db), authStore: authStore, providerConfigs: providerConfigs, rewardOps: rewardOps}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/v1/admin/auth/login" && r.Method == http.MethodPost {
		h.login(w, r)
		return
	}
	if r.URL.Path == "/v1/admin/auth/signup" && r.Method == http.MethodPost {
		h.signup(w, r)
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
	if principal.IsAccessKey() {
		r = r.WithContext(audit.WithActor(r.Context(), audit.Actor{Type: adminauth.SourceAccessKey, ID: principal.TokenID, Client: clientLabel(r)}))
	}
	if r.URL.Path == "/v1/admin/auth/me" && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, principalResponse(principal))
		return
	}
	if isAccessKeyPath(r.URL.Path) {
		h.serveAccessKeys(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/rules/check" {
		h.checkRule(w, r)
		return
	}
	if r.Method == http.MethodGet && isEventTracePath(r.URL.Path) {
		h.eventTrace(w, r)
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
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/environments" {
		h.listEnvironments(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/environments" {
		h.createEnvironment(w, r)
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
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/events/test" {
		h.testEvent(w, r)
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
	if r.URL.Path == "/v1/admin/leaderboards" || strings.HasPrefix(r.URL.Path, "/v1/admin/leaderboards/") {
		h.serveLeaderboards(w, r)
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
	if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/admin/currencies/") {
		h.updateCatalogResource(w, r, "currency")
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
	if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/admin/missions/") {
		h.updateCatalogResource(w, r, "mission")
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/goals" {
		h.createGoal(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/goals" {
		h.listGoals(w, r)
		return
	}
	if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/admin/goals/") {
		h.updateEngagementResource(w, r, "goal")
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/campaigns" {
		h.createCampaign(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/campaigns" {
		h.listCampaigns(w, r)
		return
	}
	if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/admin/campaigns/") {
		h.updateEngagementResource(w, r, "campaign")
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/metrics" {
		h.createMetric(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/metrics" {
		h.listMetrics(w, r)
		return
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/admin/metrics/") && strings.HasSuffix(r.URL.Path, "/archive") {
		h.archiveMetric(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/analytics" {
		h.analytics(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/analytics/query" {
		h.reportQuery(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/reports/dashboard" {
		h.reportDashboard(w, r)
		return
	}
	if r.Method == http.MethodPut && r.URL.Path == "/v1/admin/reports/dashboard" {
		h.updateReportDashboard(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/analytics/funnel" {
		h.funnelAnalytics(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/analytics/retention" {
		h.retentionAnalytics(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/analytics/segment" {
		h.segmentAnalytics(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/analytics/experiment" {
		h.experimentAnalytics(w, r)
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
	if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/admin/rewards/") {
		h.updateCatalogResource(w, r, "reward")
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
	if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/admin/achievements/") {
		h.updateCatalogResource(w, r, "achievement")
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
	if r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/v1/admin/levels/") {
		h.updateCatalogResource(w, r, "level")
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/redemptions" {
		h.listRedemptions(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/user-profile" {
		h.userProfile(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/user-leaderboards" {
		h.userLeaderboards(w, r)
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/v1/admin/user-transactions" {
		h.userTransactions(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/points/transactions" {
		h.createPointTransaction(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/points/conversions" {
		h.createPointConversion(w, r)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/admin/redemptions/") && strings.HasSuffix(r.URL.Path, "/attempts") {
		h.listRedemptionAttempts(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/redemptions/test" {
		h.testRedemption(w, r)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/redemptions/retry" {
		h.retryAdminRedemption(w, r)
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
	if r.Method == http.MethodPost && r.URL.Path == "/v1/admin/catalog-status" {
		h.updateCatalogStatus(w, r)
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
	ID             string   `json:"id"`
	Email          string   `json:"email"`
	Role           string   `json:"role"`
	TenantIDs      []string `json:"tenant_ids,omitempty"`
	Source         string   `json:"source,omitempty"`
	TokenID        string   `json:"token_id,omitempty"`
	Scopes         []string `json:"scopes,omitempty"`
	EnvironmentIDs []string `json:"environment_ids,omitempty"`
}

func principalResponse(principal adminauth.Principal) adminPrincipalResponse {
	response := adminPrincipalResponse{ID: principal.ID, Email: principal.Email, Role: principal.Role, TenantIDs: principal.TenantIDs, Source: principal.Source}
	if principal.IsAccessKey() {
		response.TokenID, response.Scopes, response.EnvironmentIDs = principal.TokenID, principal.Scopes, principal.EnvironmentIDs
	}
	return response
}

func auditActorType(principal adminauth.Principal) string {
	if principal.IsSuperAdmin() {
		return "super_admin"
	}
	return "tenant_admin"
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
	_ = audit.Record(r.Context(), h.db, audit.Entry{ActorType: auditActorType(principal), ActorID: principal.ID, Action: "admin.login", ResourceType: "admin_session", ResourceID: principal.ID})
	writeJSON(w, http.StatusOK, map[string]any{"token": token, "expires_in": int(adminauth.SessionTTL.Seconds()), "admin": principalResponse(principal)})
}

// signup provisions the first tenant workspace and its owner in one public,
// self-serve flow. Platform administrators remain separate and are created by
// bootstrap configuration for the future Ops service.
func (h *Handler) signup(w http.ResponseWriter, r *http.Request) {
	var request TenantSignupRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	validated, err := ValidateTenantSignupRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_signup", err.Error())
		return
	}
	var emailExists bool
	if err := h.db.QueryRow(r.Context(), `SELECT EXISTS (SELECT 1 FROM admin_users WHERE email = $1)`, validated.Email).Scan(&emailExists); err != nil {
		writeError(w, http.StatusInternalServerError, "signup_failed", "account could not be created")
		return
	}
	if emailExists {
		writeError(w, http.StatusConflict, "admin_user_exists", "an account with this email already exists")
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "signup_failed", "workspace could not be created")
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(r.Context(), `INSERT INTO tenants (id, name) VALUES ($1, $2)`, validated.TenantID, validated.Name); err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "tenant_exists", "this workspace id is already in use")
			return
		}
		writeError(w, http.StatusInternalServerError, "signup_failed", "workspace could not be created")
		return
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO environments (tenant_id, id, name) VALUES ($1, $2, $3)`, validated.TenantID, validated.EnvironmentID, validated.EnvironmentName); err != nil {
		writeError(w, http.StatusInternalServerError, "signup_failed", "workspace environment could not be created")
		return
	}
	if _, err := tx.Exec(r.Context(), `INSERT INTO tenant_plans (tenant_id, plan_id) VALUES ($1, 'starter')`, validated.TenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "signup_failed", "workspace plan could not be assigned")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "signup_failed", "workspace could not be created")
		return
	}
	user, err := h.authStore.CreateUser(r.Context(), validated.Email, validated.Password, adminauth.RoleTenantAdmin, []string{validated.TenantID})
	if err != nil {
		_, _ = h.db.Exec(r.Context(), `DELETE FROM tenants WHERE id = $1`, validated.TenantID)
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "admin_user_exists", "an account with this email already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "signup_failed", "account could not be created")
		return
	}
	principal, token, err := h.authStore.Authenticate(r.Context(), validated.Email, validated.Password, r.UserAgent(), remoteIP(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "signup_failed", "account session could not be started")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: validated.TenantID, EnvironmentID: validated.EnvironmentID, ActorType: "tenant_admin", ActorID: user.ID, Action: "tenant.signup", ResourceType: "tenant", ResourceID: validated.TenantID})
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":      token,
		"expires_in": int(adminauth.SessionTTL.Seconds()),
		"admin":      principalResponse(principal),
		"tenant":     tenantResponse{TenantID: validated.TenantID, Name: validated.Name, EnvironmentID: validated.EnvironmentID, EnvironmentName: validated.EnvironmentName, Status: "active"},
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(header), "bearer ") {
		rawToken := strings.TrimSpace(header[len("Bearer "):])
		principal, err := h.authStore.Lookup(r.Context(), rawToken)
		_ = h.authStore.Revoke(r.Context(), rawToken)
		if err == nil {
			_ = audit.Record(r.Context(), h.db, audit.Entry{ActorType: auditActorType(principal), ActorID: principal.ID, Action: "admin.logout", ResourceType: "admin_session", ResourceID: principal.ID})
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
	if principal.IsAccessKey() && !accessKeyAllows(r, principal) {
		return false
	}
	path := r.URL.Path
	forbidden := []string{"/v1/admin/tenants", "/v1/admin/plans", "/v1/admin/tenant-plans", "/v1/admin/admin-users"}
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
	if strings.HasPrefix(path, "/v1/admin/metrics/") {
		return r.Method == http.MethodPost && principal.AllowsTenant(strings.TrimSpace(r.URL.Query().Get("tenant_id")))
	}
	if r.Method != http.MethodGet {
		if r.Method == http.MethodPatch && isEditableAdminResourcePath(path) {
			return requestBodyTenant(r, principal)
		}
		// Rule version and publish actions do not carry a tenant body. They must
		// carry the selected scope in the query string so tenant administrators
		// can be authorized before the handler looks up the rule by id.
		if isScopedRuleAction(r.URL.Path) || (isAccessKeyPath(path) && (r.Method == http.MethodDelete || strings.HasSuffix(path, "/revoke"))) {
			return principal.AllowsTenant(strings.TrimSpace(r.URL.Query().Get("tenant_id")))
		}
		if r.Method == http.MethodPost && isLeaderboardWritePath(path) {
			return requestBodyTenant(r, principal)
		}
		if r.Method == http.MethodDelete && strings.HasPrefix(path, "/v1/admin/leaderboards/") {
			return principal.AllowsTenant(strings.TrimSpace(r.URL.Query().Get("tenant_id")))
		}
		allowedCreate := map[string]bool{
			"/v1/admin/access-keys":          true,
			"/v1/admin/rules/check":          true,
			"/v1/admin/event-types":          true,
			"/v1/admin/environments":         true,
			"/v1/admin/events/test":          true,
			"/v1/admin/redemptions/test":     true,
			"/v1/admin/redemptions/retry":    true,
			"/v1/admin/rules":                true,
			"/v1/admin/currencies":           true,
			"/v1/admin/leaderboards":         true,
			"/v1/admin/missions":             true,
			"/v1/admin/goals":                true,
			"/v1/admin/campaigns":            true,
			"/v1/admin/metrics":              true,
			"/v1/admin/analytics/funnel":     true,
			"/v1/admin/analytics/retention":  true,
			"/v1/admin/analytics/segment":    true,
			"/v1/admin/analytics/experiment": true,
			"/v1/admin/reports/dashboard":    true,
			"/v1/admin/rewards":              true,
			"/v1/admin/reward-providers":     true,
			"/v1/admin/achievements":         true,
			"/v1/admin/levels":               true,
			"/v1/admin/catalog-status":       true,
			"/v1/admin/points/transactions":  true,
			"/v1/admin/points/conversions":   true,
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

func isEditableAdminResourcePath(path string) bool {
	for _, resource := range []string{"currencies", "missions", "goals", "campaigns", "rewards", "achievements", "levels", "leaderboards", "access-keys"} {
		if strings.HasPrefix(path, "/v1/admin/"+resource+"/") {
			return true
		}
	}
	return false
}

func isScopedRuleAction(path string) bool {
	if strings.HasPrefix(path, "/v1/admin/rules/") && strings.HasSuffix(path, "/versions") {
		_, ok := parseRuleVersionCollectionPath(path)
		return ok
	}
	if strings.HasPrefix(path, "/v1/admin/rules/") && strings.HasSuffix(path, "/publish") {
		if _, ok := parseRuleVersionCollectionPath(strings.TrimSuffix(path, "/publish")); ok {
			return true
		}
		_, _, ok := parseRuleVersionPublishPath(path)
		return ok
	}
	return false
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
		"rules":        `SELECT count(*) FROM rules WHERE tenant_id = $1 AND environment_id = $2`,
		"currencies":   `SELECT count(*) FROM currencies WHERE tenant_id = $1 AND environment_id = $2`,
		"rewards":      `SELECT count(*) FROM rewards WHERE tenant_id = $1 AND environment_id = $2`,
		"leaderboards": `SELECT count(*) FROM leaderboards WHERE tenant_id = $1 AND environment_id = $2`,
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
}

type environmentResponse struct {
	TenantID  string    `json:"tenant_id"`
	ID        string    `json:"environment_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
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

type goalResponse struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	EnvironmentID string    `json:"environment_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	EventType     string    `json:"event_type"`
	TargetCount   int64     `json:"target_count"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
}

type campaignResponse struct {
	ID            string     `json:"id"`
	TenantID      string     `json:"tenant_id"`
	EnvironmentID string     `json:"environment_id"`
	GoalID        string     `json:"goal_id,omitempty"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	EventType     string     `json:"event_type"`
	StartsAt      *time.Time `json:"starts_at,omitempty"`
	EndsAt        *time.Time `json:"ends_at,omitempty"`
	Active        bool       `json:"active"`
	CreatedAt     time.Time  `json:"created_at"`
}

type metricResponse struct {
	TenantID      string    `json:"tenant_id"`
	EnvironmentID string    `json:"environment_id"`
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	EventType     string    `json:"event_type"`
	Aggregation   string    `json:"aggregation"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type funnelStepResponse struct {
	Name           string  `json:"name"`
	EventType      string  `json:"event_type"`
	Users          int64   `json:"users"`
	ConversionRate float64 `json:"conversion_rate"`
}

type funnelResponse struct {
	TenantID      string               `json:"tenant_id"`
	EnvironmentID string               `json:"environment_id"`
	From          *time.Time           `json:"from,omitempty"`
	To            *time.Time           `json:"to,omitempty"`
	Steps         []funnelStepResponse `json:"steps"`
	GeneratedAt   time.Time            `json:"generated_at"`
}

type retentionIntervalResponse struct {
	Days           int     `json:"days"`
	ReturningUsers int64   `json:"returning_users"`
	Rate           float64 `json:"rate"`
}

type retentionResponse struct {
	TenantID        string                      `json:"tenant_id"`
	EnvironmentID   string                      `json:"environment_id"`
	CohortEventType string                      `json:"cohort_event_type"`
	ReturnEventType string                      `json:"return_event_type"`
	From            *time.Time                  `json:"from,omitempty"`
	To              *time.Time                  `json:"to,omitempty"`
	CohortUsers     int64                       `json:"cohort_users"`
	Intervals       []retentionIntervalResponse `json:"intervals"`
	GeneratedAt     time.Time                   `json:"generated_at"`
}

type segmentResponse struct {
	TenantID      string     `json:"tenant_id"`
	EnvironmentID string     `json:"environment_id"`
	EventType     string     `json:"event_type"`
	MinEvents     int        `json:"min_events"`
	Users         int64      `json:"users"`
	Events        int64      `json:"events"`
	From          *time.Time `json:"from,omitempty"`
	To            *time.Time `json:"to,omitempty"`
	GeneratedAt   time.Time  `json:"generated_at"`
}

type experimentVariantResponse struct {
	ID             string  `json:"id"`
	Weight         int     `json:"weight"`
	AssignedUsers  int64   `json:"assigned_users"`
	ConvertedUsers int64   `json:"converted_users"`
	ConversionRate float64 `json:"conversion_rate"`
}

type experimentResponse struct {
	TenantID        string                      `json:"tenant_id"`
	EnvironmentID   string                      `json:"environment_id"`
	ID              string                      `json:"id"`
	ConversionEvent string                      `json:"conversion_event_type"`
	Variants        []experimentVariantResponse `json:"variants"`
	From            *time.Time                  `json:"from,omitempty"`
	To              *time.Time                  `json:"to,omitempty"`
	GeneratedAt     time.Time                   `json:"generated_at"`
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
	if _, err := tx.Exec(r.Context(), `INSERT INTO tenant_plans (tenant_id, plan_id) VALUES ($1, 'starter')`, request.TenantID); err != nil {
		writeError(w, http.StatusInternalServerError, "plan_assignment_failed", "starter plan could not be assigned")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "tenant_creation_failed", "tenant could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, ActorType: "platform_admin", Action: "tenant.created", ResourceType: "tenant", ResourceID: request.TenantID, Metadata: auditMetadata(map[string]any{"environment_id": request.EnvironmentID, "plan_id": "starter"})})
	writeJSON(w, http.StatusCreated, tenantResponse{TenantID: request.TenantID, Name: request.Name, EnvironmentID: request.EnvironmentID, EnvironmentName: request.EnvironmentName, Status: "active"})
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
			WHERE t.id = ANY($1)
			ORDER BY t.id, e.id`
		args = append(args, principal.TenantIDs)
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

func (h *Handler) listEnvironments(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	if !identifierPattern.MatchString(tenantID) {
		writeError(w, http.StatusBadRequest, "invalid_tenant", "tenant_id is required")
		return
	}
	rows, err := h.db.Query(r.Context(), `SELECT tenant_id, id, name, created_at FROM environments WHERE tenant_id = $1 ORDER BY id`, tenantID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "environment_lookup_failed", "environments could not be read")
		return
	}
	defer rows.Close()
	result := make([]environmentResponse, 0)
	for rows.Next() {
		var environment environmentResponse
		if err := rows.Scan(&environment.TenantID, &environment.ID, &environment.Name, &environment.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "environment_lookup_failed", "environments could not be read")
			return
		}
		result = append(result, environment)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "environment_lookup_failed", "environments could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createEnvironment(w http.ResponseWriter, r *http.Request) {
	var request EnvironmentRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 8*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := ValidateEnvironmentRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_environment", err.Error())
		return
	}
	if _, err := h.db.Exec(r.Context(), `INSERT INTO environments (tenant_id, id, name) VALUES ($1, $2, $3)`, request.TenantID, request.ID, request.Name); err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "environment_exists", "environment already exists")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "tenant_not_found", "tenant was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "environment_creation_failed", "environment could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: request.TenantID, EnvironmentID: request.ID, ActorType: "platform_admin", Action: "environment.created", ResourceType: "environment", ResourceID: request.ID})
	writeJSON(w, http.StatusCreated, environmentResponse{TenantID: request.TenantID, ID: request.ID, Name: request.Name, CreatedAt: time.Now().UTC()})
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

// testEvent is the admin-console equivalent of the public integration event
// route. It keeps the selected tenant/environment boundary server-authorized
// while avoiding the need to expose or persist a raw integration credential in
// the browser just to exercise the receipt path.
func (h *Handler) testEvent(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID        string          `json:"tenant_id"`
		EnvironmentID   string          `json:"environment_id"`
		ExternalEventID string          `json:"event_id"`
		EventType       string          `json:"event_type"`
		UserID          string          `json:"user_id"`
		Payload         json.RawMessage `json:"payload"`
	}
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 80*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) {
		writeError(w, http.StatusBadRequest, "invalid_scope", "tenant_id and environment_id are required")
		return
	}
	validated, err := eventdomain.Validate(eventdomain.Request{
		ExternalEventID: request.ExternalEventID,
		EventType:       request.EventType,
		UserID:          request.UserID,
		Payload:         request.Payload,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_event", err.Error())
		return
	}

	if status := h.activeTenantStatus(r.Context(), request.TenantID); status != "active" {
		if status == "" {
			writeError(w, http.StatusNotFound, "tenant_not_found", "tenant was not found")
		} else {
			writeError(w, http.StatusConflict, "tenant_suspended", "tenant is suspended")
		}
		return
	}
	if err := h.limits.Check(r.Context(), request.TenantID, "max_event_payload_bytes", int64(len(validated.CanonicalPayload)), 0); err != nil {
		if errors.As(err, new(*entitlements.LimitError)) {
			writeError(w, http.StatusRequestEntityTooLarge, "plan_limit_exceeded", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "plan_lookup_failed", "tenant plan could not be evaluated")
		return
	}

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "event_not_accepted", "event could not be accepted")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	var tenantStatus string
	if err := tx.QueryRow(r.Context(), `SELECT status FROM tenants WHERE id = $1`, request.TenantID).Scan(&tenantStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "tenant_not_found", "tenant was not found")
		} else {
			writeError(w, http.StatusInternalServerError, "event_not_accepted", "event could not be accepted")
		}
		return
	}
	if tenantStatus != "active" {
		writeError(w, http.StatusConflict, "tenant_suspended", "tenant is suspended")
		return
	}
	var schema json.RawMessage
	schemaVersion := 1
	schemaErr := tx.QueryRow(r.Context(), `
		SELECT schema, current_version
		FROM event_types
		WHERE tenant_id = $1 AND environment_id = $2 AND name = $3 AND active = true`, request.TenantID, request.EnvironmentID, validated.EventType).Scan(&schema, &schemaVersion)
	if schemaErr == nil {
		if err := eventdomain.ValidateSchema(schema, validated.CanonicalPayload); err != nil {
			writeError(w, http.StatusUnprocessableEntity, "event_schema_mismatch", err.Error())
			return
		}
	} else if !errors.Is(schemaErr, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "event_not_accepted", "event schema could not be read")
		return
	}

	var eventID int64
	var eventStatus string
	var userID *string
	if validated.UserID != "" {
		value := validated.UserID
		userID = &value
	}
	err = tx.QueryRow(r.Context(), `
		INSERT INTO events (tenant_id, environment_id, external_event_id, event_type, occurred_at, external_user_id, payload, payload_hash, event_schema_version)
		VALUES ($1, $2, $3, $4, NULL, $5, $6::jsonb, decode($7, 'hex'), $8)
		ON CONFLICT (tenant_id, environment_id, external_event_id) DO NOTHING
		RETURNING id, status`, request.TenantID, request.EnvironmentID, validated.ExternalEventID, validated.EventType, userID, validated.CanonicalPayload, validated.PayloadHash, schemaVersion).Scan(&eventID, &eventStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		var existingHash []byte
		var existingSchemaVersion int
		if queryErr := tx.QueryRow(r.Context(), `SELECT id, status, payload_hash, event_schema_version FROM events WHERE tenant_id = $1 AND environment_id = $2 AND external_event_id = $3`, request.TenantID, request.EnvironmentID, validated.ExternalEventID).Scan(&eventID, &eventStatus, &existingHash, &existingSchemaVersion); queryErr != nil {
			writeError(w, http.StatusInternalServerError, "event_lookup_failed", "event could not be read")
			return
		}
		decodedHash, _ := hex.DecodeString(validated.PayloadHash)
		if !bytes.Equal(existingHash, decodedHash) {
			writeError(w, http.StatusConflict, "event_id_conflict", "event_id already exists with a different payload")
			return
		}
		schemaVersion = existingSchemaVersion
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "event_not_accepted", "event could not be accepted")
		return
	} else if _, err := tx.Exec(r.Context(), `INSERT INTO event_outbox (event_id) VALUES ($1)`, eventID); err != nil {
		writeError(w, http.StatusInternalServerError, "event_not_accepted", "event could not be queued")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "event_not_accepted", "event could not be accepted")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, ActorType: "platform_admin", Action: "event.test_accepted", ResourceType: "event", ResourceID: strconv.FormatInt(eventID, 10)})
	writeJSON(w, http.StatusAccepted, map[string]any{"event_id": eventID, "external_event_id": validated.ExternalEventID, "status": eventStatus, "schema_version": schemaVersion})
}

func (h *Handler) activeTenantStatus(ctx context.Context, tenantID string) string {
	var status string
	if err := h.db.QueryRow(ctx, `SELECT status FROM tenants WHERE id = $1`, tenantID).Scan(&status); err != nil {
		return ""
	}
	return status
}

func (h *Handler) listRuleVersions(w http.ResponseWriter, r *http.Request) {
	ruleID, ok := parseRuleVersionCollectionPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "rule_not_found", "rule was not found")
		return
	}
	requestedTenantID, requestedEnvironmentID := strings.TrimSpace(r.URL.Query().Get("tenant_id")), strings.TrimSpace(r.URL.Query().Get("environment_id"))
	if err := h.verifyRuleScope(r.Context(), ruleID, requestedTenantID, requestedEnvironmentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "rule_not_found", "rule was not found")
		} else {
			writeError(w, http.StatusBadRequest, "invalid_scope", "tenant_id and environment_id must be provided together")
		}
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
	requestedTenantID, requestedEnvironmentID := strings.TrimSpace(r.URL.Query().Get("tenant_id")), strings.TrimSpace(r.URL.Query().Get("environment_id"))
	if err := h.verifyRuleScope(r.Context(), ruleID, requestedTenantID, requestedEnvironmentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "rule_not_found", "rule was not found")
		} else {
			writeError(w, http.StatusBadRequest, "invalid_scope", "tenant_id and environment_id must be provided together")
		}
		return
	}
	var request struct {
		Name        *string         `json:"name,omitempty"`
		Description *string         `json:"description,omitempty"`
		EventType   string          `json:"event_type"`
		Conditions  json.RawMessage `json:"conditions"`
		Action      json.RawMessage `json:"action"`
		Actions     json.RawMessage `json:"actions"`
		StartsAt    *time.Time      `json:"starts_at,omitempty"`
		EndsAt      *time.Time      `json:"ends_at,omitempty"`
	}
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 64*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if request.EventType == "" {
		writeError(w, http.StatusBadRequest, "invalid_rule", "event_type is required")
		return
	}
	normalized, err := ValidateRuleRequest(RuleRequest{TenantID: "valid", EnvironmentID: "valid", Name: "version", EventType: request.EventType, Conditions: request.Conditions, Action: request.Action, Actions: request.Actions, StartsAt: request.StartsAt, EndsAt: request.EndsAt})
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
	var tenantID, environmentID, currentName, currentDescription string
	if err := tx.QueryRow(r.Context(), `SELECT coalesce(max(v.version), 0) + 1, r.tenant_id, r.environment_id, r.name, r.description FROM rules r LEFT JOIN rule_versions v ON v.rule_id = r.id WHERE r.id = $1 GROUP BY r.id, r.tenant_id, r.environment_id, r.name, r.description`, ruleID).Scan(&version, &tenantID, &environmentID, &currentName, &currentDescription); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "rule_not_found", "rule was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "rule_version_creation_failed", "rule version could not be created")
		return
	}
	updatedName, updatedDescription := currentName, currentDescription
	if request.Name != nil {
		updatedName = strings.TrimSpace(*request.Name)
		if updatedName == "" {
			writeError(w, http.StatusBadRequest, "invalid_rule", "name is required")
			return
		}
	}
	if request.Description != nil {
		updatedDescription = strings.TrimSpace(*request.Description)
	}
	if len(updatedName) > 120 || len(updatedDescription) > 500 {
		writeError(w, http.StatusBadRequest, "invalid_rule", "name must be 120 characters or fewer and description must be 500 characters or fewer")
		return
	}
	versionID := fmt.Sprintf("%s:v%d", ruleID, version)
	if _, err := tx.Exec(r.Context(), `INSERT INTO rule_versions (id, rule_id, version, event_type, conditions, action, starts_at, ends_at) VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7, $8)`, versionID, ruleID, version, normalized.EventType, normalized.Conditions, normalized.Action, normalized.StartsAt, normalized.EndsAt); err != nil {
		writeError(w, http.StatusInternalServerError, "rule_version_creation_failed", "rule version could not be created")
		return
	}
	var rule ruleResponse
	if err := tx.QueryRow(r.Context(), `UPDATE rules SET name = $2, description = $3, event_type = $4, conditions = $5::jsonb, action = $6::jsonb, starts_at = $7, ends_at = $8, version = $9, status = 'draft', published_at = NULL, updated_at = now() WHERE id = $1 RETURNING id, tenant_id, environment_id, name, description, event_type, conditions, action, status, version, created_at, updated_at, published_at, starts_at, ends_at`, ruleID, updatedName, updatedDescription, normalized.EventType, normalized.Conditions, normalized.Action, normalized.StartsAt, normalized.EndsAt, version).Scan(&rule.ID, &rule.TenantID, &rule.EnvironmentID, &rule.Name, &rule.Description, &rule.EventType, &rule.Conditions, &rule.Action, &rule.Status, &rule.Version, &rule.CreatedAt, &rule.UpdatedAt, &rule.PublishedAt, &rule.StartsAt, &rule.EndsAt); err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "rule_exists", "a rule with this name already exists in the environment")
			return
		}
		writeError(w, http.StatusInternalServerError, "rule_version_creation_failed", "rule version could not be created")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "rule_version_creation_failed", "rule version could not be created")
		return
	}
	_ = tenantID
	_ = environmentID
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: rule.TenantID, EnvironmentID: rule.EnvironmentID, ActorType: "platform_admin", Action: "rule.version_created", ResourceType: "rule", ResourceID: rule.ID, Metadata: auditMetadata(map[string]any{"version": rule.Version})})
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
	if err := h.verifyRuleScope(r.Context(), ruleID, strings.TrimSpace(r.URL.Query().Get("tenant_id")), strings.TrimSpace(r.URL.Query().Get("environment_id"))); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "rule_not_found", "rule was not found")
		} else {
			writeError(w, http.StatusBadRequest, "invalid_scope", "tenant_id and environment_id must be provided together")
		}
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
	if err := h.verifyRuleScope(r.Context(), ruleID, strings.TrimSpace(r.URL.Query().Get("tenant_id")), strings.TrimSpace(r.URL.Query().Get("environment_id"))); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "rule_version_not_found", "rule version was not found")
		} else {
			writeError(w, http.StatusBadRequest, "invalid_scope", "tenant_id and environment_id must be provided together")
		}
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

// verifyRuleScope keeps id-based rule actions inside the scope explicitly
// selected by a tenant administrator. Super Admin callers may omit the query
// parameters for backwards compatibility with the platform console.
func (h *Handler) verifyRuleScope(ctx context.Context, ruleID, tenantID, environmentID string) error {
	if tenantID == "" && environmentID == "" {
		return nil
	}
	if tenantID == "" || environmentID == "" {
		return errors.New("incomplete rule scope")
	}
	var exists bool
	if err := h.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM rules WHERE id = $1 AND tenant_id = $2 AND environment_id = $3)`, ruleID, tenantID, environmentID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}
	return nil
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

func (h *Handler) createGoal(w http.ResponseWriter, r *http.Request) {
	var request GoalRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := ValidateGoalRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_goal", err.Error())
		return
	}
	var goal goalResponse
	err = h.db.QueryRow(r.Context(), `INSERT INTO goals (id, tenant_id, environment_id, name, description, event_type, target_count) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id, tenant_id, environment_id, name, description, event_type, target_count, active, created_at`, request.ID, request.TenantID, request.EnvironmentID, request.Name, request.Description, request.EventType, request.TargetCount).Scan(&goal.ID, &goal.TenantID, &goal.EnvironmentID, &goal.Name, &goal.Description, &goal.EventType, &goal.TargetCount, &goal.Active, &goal.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "goal_exists", "goal already exists")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "environment_not_found", "tenant environment was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "goal_creation_failed", "goal could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: goal.TenantID, EnvironmentID: goal.EnvironmentID, ActorType: "platform_admin", Action: "goal.created", ResourceType: "goal", ResourceID: goal.ID})
	writeJSON(w, http.StatusCreated, goal)
}

func (h *Handler) listGoals(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `SELECT id, tenant_id, environment_id, name, description, event_type, target_count, active, created_at FROM goals WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2) ORDER BY created_at DESC, id`, r.URL.Query().Get("tenant_id"), r.URL.Query().Get("environment_id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "goal_lookup_failed", "goals could not be read")
		return
	}
	defer rows.Close()
	result := make([]goalResponse, 0)
	for rows.Next() {
		var goal goalResponse
		if err := rows.Scan(&goal.ID, &goal.TenantID, &goal.EnvironmentID, &goal.Name, &goal.Description, &goal.EventType, &goal.TargetCount, &goal.Active, &goal.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "goal_lookup_failed", "goals could not be read")
			return
		}
		result = append(result, goal)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "goal_lookup_failed", "goals could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) createCampaign(w http.ResponseWriter, r *http.Request) {
	var request CampaignRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := ValidateCampaignRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_campaign", err.Error())
		return
	}
	if request.GoalID != "" {
		var goalScope string
		if err := h.db.QueryRow(r.Context(), `SELECT tenant_id || ':' || environment_id FROM goals WHERE id = $1`, request.GoalID).Scan(&goalScope); errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "goal_not_found", "goal was not found")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "goal_lookup_failed", "goal could not be read")
			return
		} else if goalScope != request.TenantID+":"+request.EnvironmentID {
			writeError(w, http.StatusBadRequest, "goal_scope_mismatch", "goal must belong to the campaign scope")
			return
		}
	}
	var campaign campaignResponse
	err = h.db.QueryRow(r.Context(), `INSERT INTO campaigns (id, tenant_id, environment_id, goal_id, name, description, event_type, starts_at, ends_at) VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9) RETURNING id, tenant_id, environment_id, COALESCE(goal_id,''), name, description, event_type, starts_at, ends_at, active, created_at`, request.ID, request.TenantID, request.EnvironmentID, request.GoalID, request.Name, request.Description, request.EventType, request.StartsAt, request.EndsAt).Scan(&campaign.ID, &campaign.TenantID, &campaign.EnvironmentID, &campaign.GoalID, &campaign.Name, &campaign.Description, &campaign.EventType, &campaign.StartsAt, &campaign.EndsAt, &campaign.Active, &campaign.CreatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "campaign_exists", "campaign already exists")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "goal_or_environment_not_found", "goal or tenant environment was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "campaign_creation_failed", "campaign could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: campaign.TenantID, EnvironmentID: campaign.EnvironmentID, ActorType: "platform_admin", Action: "campaign.created", ResourceType: "campaign", ResourceID: campaign.ID})
	writeJSON(w, http.StatusCreated, campaign)
}

func (h *Handler) listCampaigns(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(r.Context(), `SELECT id, tenant_id, environment_id, COALESCE(goal_id,''), name, description, event_type, starts_at, ends_at, active, created_at FROM campaigns WHERE ($1 = '' OR tenant_id = $1) AND ($2 = '' OR environment_id = $2) ORDER BY created_at DESC, id`, r.URL.Query().Get("tenant_id"), r.URL.Query().Get("environment_id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "campaign_lookup_failed", "campaigns could not be read")
		return
	}
	defer rows.Close()
	result := make([]campaignResponse, 0)
	for rows.Next() {
		var campaign campaignResponse
		if err := rows.Scan(&campaign.ID, &campaign.TenantID, &campaign.EnvironmentID, &campaign.GoalID, &campaign.Name, &campaign.Description, &campaign.EventType, &campaign.StartsAt, &campaign.EndsAt, &campaign.Active, &campaign.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "campaign_lookup_failed", "campaigns could not be read")
			return
		}
		result = append(result, campaign)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "campaign_lookup_failed", "campaigns could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) analytics(w http.ResponseWriter, r *http.Request) {
	tenantID, environmentID := strings.TrimSpace(r.URL.Query().Get("tenant_id")), strings.TrimSpace(r.URL.Query().Get("environment_id"))
	if !identifierPattern.MatchString(tenantID) || !identifierPattern.MatchString(environmentID) {
		writeError(w, http.StatusBadRequest, "invalid_scope", "tenant_id and environment_id are required")
		return
	}
	rows, err := h.db.Query(r.Context(), `SELECT c.id, c.name, c.event_type, c.starts_at, c.ends_at, COALESCE(g.target_count, 0), COUNT(e.id), COUNT(DISTINCT e.external_user_id) FROM campaigns c LEFT JOIN goals g ON g.id = c.goal_id LEFT JOIN events e ON e.tenant_id = c.tenant_id AND e.environment_id = c.environment_id AND e.event_type = c.event_type AND (c.starts_at IS NULL OR COALESCE(e.occurred_at, e.received_at) >= c.starts_at) AND (c.ends_at IS NULL OR COALESCE(e.occurred_at, e.received_at) < c.ends_at) WHERE c.tenant_id = $1 AND c.environment_id = $2 GROUP BY c.id, c.name, c.event_type, c.starts_at, c.ends_at, g.target_count ORDER BY c.created_at DESC`, tenantID, environmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "analytics_lookup_failed", "campaign analytics could not be read")
		return
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, name, eventType string
		var startsAt, endsAt *time.Time
		var target, eventCount, userCount int64
		if err := rows.Scan(&id, &name, &eventType, &startsAt, &endsAt, &target, &eventCount, &userCount); err != nil {
			writeError(w, http.StatusInternalServerError, "analytics_lookup_failed", "campaign analytics could not be read")
			return
		}
		result = append(result, map[string]any{"campaign_id": id, "name": name, "event_type": eventType, "starts_at": startsAt, "ends_at": endsAt, "target_count": target, "event_count": eventCount, "unique_users": userCount})
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "analytics_lookup_failed", "campaign analytics could not be read")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "environment_id": environmentID, "campaigns": result, "generated_at": time.Now().UTC()})
}

func (h *Handler) createMetric(w http.ResponseWriter, r *http.Request) {
	var request MetricRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := ValidateMetricRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_metric", err.Error())
		return
	}
	var metric metricResponse
	err = h.db.QueryRow(r.Context(), `INSERT INTO metric_definitions (tenant_id, environment_id, id, name, description, event_type, aggregation) VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING tenant_id, environment_id, id, name, description, event_type, aggregation, active, created_at, updated_at`, request.TenantID, request.EnvironmentID, request.ID, request.Name, request.Description, request.EventType, request.Aggregation).Scan(&metric.TenantID, &metric.EnvironmentID, &metric.ID, &metric.Name, &metric.Description, &metric.EventType, &metric.Aggregation, &metric.Active, &metric.CreatedAt, &metric.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "metric_exists", "a metric with this id already exists in the environment")
			return
		}
		if isForeignKeyViolation(err) {
			writeError(w, http.StatusNotFound, "environment_not_found", "tenant environment was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "metric_creation_failed", "metric could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: metric.TenantID, EnvironmentID: metric.EnvironmentID, ActorType: "platform_admin", Action: "metric.created", ResourceType: "metric", ResourceID: metric.ID})
	writeJSON(w, http.StatusCreated, metric)
}

func (h *Handler) listMetrics(w http.ResponseWriter, r *http.Request) {
	tenantID, environmentID := strings.TrimSpace(r.URL.Query().Get("tenant_id")), strings.TrimSpace(r.URL.Query().Get("environment_id"))
	if err := validateAnalyticsScope(tenantID, environmentID, nil, nil); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_scope", err.Error())
		return
	}
	rows, err := h.db.Query(r.Context(), `SELECT tenant_id, environment_id, id, name, description, event_type, aggregation, active, created_at, updated_at FROM metric_definitions WHERE tenant_id = $1 AND environment_id = $2 ORDER BY created_at DESC, id`, tenantID, environmentID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "metric_lookup_failed", "metrics could not be read")
		return
	}
	defer rows.Close()
	result := make([]metricResponse, 0)
	for rows.Next() {
		var metric metricResponse
		if err := rows.Scan(&metric.TenantID, &metric.EnvironmentID, &metric.ID, &metric.Name, &metric.Description, &metric.EventType, &metric.Aggregation, &metric.Active, &metric.CreatedAt, &metric.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "metric_lookup_failed", "metrics could not be read")
			return
		}
		result = append(result, metric)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "metric_lookup_failed", "metrics could not be read")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) archiveMetric(w http.ResponseWriter, r *http.Request) {
	metricID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/admin/metrics/"), "/archive")
	tenantID, environmentID := strings.TrimSpace(r.URL.Query().Get("tenant_id")), strings.TrimSpace(r.URL.Query().Get("environment_id"))
	if !identifierPattern.MatchString(metricID) {
		writeError(w, http.StatusNotFound, "metric_not_found", "metric was not found")
		return
	}
	if err := validateAnalyticsScope(tenantID, environmentID, nil, nil); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_scope", err.Error())
		return
	}
	var active bool
	err := h.db.QueryRow(r.Context(), `UPDATE metric_definitions SET active = false, updated_at = now() WHERE tenant_id = $1 AND environment_id = $2 AND id = $3 RETURNING active`, tenantID, environmentID, metricID).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "metric_not_found", "metric was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "metric_update_failed", "metric could not be archived")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: tenantID, EnvironmentID: environmentID, ActorType: "platform_admin", Action: "metric.archived", ResourceType: "metric", ResourceID: metricID})
	writeJSON(w, http.StatusOK, map[string]any{"tenant_id": tenantID, "environment_id": environmentID, "id": metricID, "active": active})
}

type measurementEvent struct {
	UserID    string
	EventType string
	At        time.Time
}

func (h *Handler) queryMeasurementEvents(ctx context.Context, tenantID, environmentID string, from, to *time.Time) ([]measurementEvent, error) {
	query := `SELECT external_user_id, event_type, COALESCE(occurred_at, received_at) FROM events WHERE tenant_id = $1 AND environment_id = $2 AND external_user_id IS NOT NULL AND external_user_id <> ''`
	args := []any{tenantID, environmentID}
	if from != nil {
		query += fmt.Sprintf(" AND COALESCE(occurred_at, received_at) >= $%d", len(args)+1)
		args = append(args, *from)
	}
	if to != nil {
		query += fmt.Sprintf(" AND COALESCE(occurred_at, received_at) < $%d", len(args)+1)
		args = append(args, *to)
	}
	query += ` ORDER BY external_user_id, COALESCE(occurred_at, received_at), id`
	rows, err := h.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]measurementEvent, 0)
	for rows.Next() {
		var event measurementEvent
		if err := rows.Scan(&event.UserID, &event.EventType, &event.At); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (h *Handler) funnelAnalytics(w http.ResponseWriter, r *http.Request) {
	var request FunnelRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := ValidateFunnelRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_funnel", err.Error())
		return
	}
	events, err := h.queryMeasurementEvents(r.Context(), request.TenantID, request.EnvironmentID, request.From, request.To)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "funnel_lookup_failed", "funnel events could not be read")
		return
	}
	progress := make(map[string]int)
	counts := make([]int64, len(request.Steps))
	for _, event := range events {
		step := progress[event.UserID]
		if step >= len(request.Steps) || event.EventType != request.Steps[step].EventType {
			continue
		}
		counts[step]++
		progress[event.UserID] = step + 1
	}
	steps := make([]funnelStepResponse, len(request.Steps))
	base := float64(counts[0])
	for index, step := range request.Steps {
		rate := 0.0
		if base > 0 {
			rate = float64(counts[index]) / base
		}
		steps[index] = funnelStepResponse{Name: step.Name, EventType: step.EventType, Users: counts[index], ConversionRate: rate}
	}
	writeJSON(w, http.StatusOK, funnelResponse{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, From: request.From, To: request.To, Steps: steps, GeneratedAt: time.Now().UTC()})
}

func (h *Handler) retentionAnalytics(w http.ResponseWriter, r *http.Request) {
	var request RetentionRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := ValidateRetentionRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_retention", err.Error())
		return
	}
	events, err := h.queryMeasurementEvents(r.Context(), request.TenantID, request.EnvironmentID, request.From, request.To)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "retention_lookup_failed", "retention events could not be read")
		return
	}
	cohorts := make(map[string]time.Time)
	returned := make(map[string]map[int]bool)
	for _, event := range events {
		if event.EventType == request.CohortEventType {
			if _, exists := cohorts[event.UserID]; !exists {
				cohorts[event.UserID] = event.At.UTC()
				returned[event.UserID] = make(map[int]bool)
			}
		}
		cohortAt, exists := cohorts[event.UserID]
		if !exists || event.EventType != request.ReturnEventType || !event.At.After(cohortAt) {
			continue
		}
		for _, interval := range request.Intervals {
			if utcDayDifference(cohortAt, event.At) == interval {
				returned[event.UserID][interval] = true
			}
		}
	}
	cohortUsers := int64(len(cohorts))
	intervals := make([]retentionIntervalResponse, len(request.Intervals))
	for index, days := range request.Intervals {
		var returning int64
		for userID := range cohorts {
			if returned[userID][days] {
				returning++
			}
		}
		rate := 0.0
		if cohortUsers > 0 {
			rate = float64(returning) / float64(cohortUsers)
		}
		intervals[index] = retentionIntervalResponse{Days: days, ReturningUsers: returning, Rate: rate}
	}
	writeJSON(w, http.StatusOK, retentionResponse{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, CohortEventType: request.CohortEventType, ReturnEventType: request.ReturnEventType, From: request.From, To: request.To, CohortUsers: cohortUsers, Intervals: intervals, GeneratedAt: time.Now().UTC()})
}

func utcDayDifference(from, to time.Time) int {
	start := time.Date(from.UTC().Year(), from.UTC().Month(), from.UTC().Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(to.UTC().Year(), to.UTC().Month(), to.UTC().Day(), 0, 0, 0, 0, time.UTC)
	return int(end.Sub(start) / (24 * time.Hour))
}

func (h *Handler) segmentAnalytics(w http.ResponseWriter, r *http.Request) {
	var request SegmentRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := ValidateSegmentRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_segment", err.Error())
		return
	}
	events, err := h.queryMeasurementEvents(r.Context(), request.TenantID, request.EnvironmentID, request.From, request.To)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "segment_lookup_failed", "segment events could not be read")
		return
	}
	counts := make(map[string]int)
	var matchingEvents int64
	for _, event := range events {
		if event.EventType != request.EventType {
			continue
		}
		counts[event.UserID]++
		matchingEvents++
	}
	var users int64
	for _, count := range counts {
		if count >= request.MinEvents {
			users++
		}
	}
	writeJSON(w, http.StatusOK, segmentResponse{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, EventType: request.EventType, MinEvents: request.MinEvents, Users: users, Events: matchingEvents, From: request.From, To: request.To, GeneratedAt: time.Now().UTC()})
}

func (h *Handler) experimentAnalytics(w http.ResponseWriter, r *http.Request) {
	var request ExperimentRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request, err := ValidateExperimentRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_experiment", err.Error())
		return
	}
	events, err := h.queryMeasurementEvents(r.Context(), request.TenantID, request.EnvironmentID, request.From, request.To)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "experiment_lookup_failed", "experiment events could not be read")
		return
	}
	assigned := make(map[string]int)
	converted := make(map[string]bool)
	for _, event := range events {
		if _, exists := assigned[event.UserID]; !exists {
			assigned[event.UserID] = chooseExperimentVariant(request.TenantID, request.ID, event.UserID, request.Variants)
		}
		if event.EventType == request.ConversionEvent {
			converted[event.UserID] = true
		}
	}
	variants := make([]experimentVariantResponse, len(request.Variants))
	for index, variant := range request.Variants {
		var assignedUsers, convertedUsers int64
		for userID, variantIndex := range assigned {
			if variantIndex != index {
				continue
			}
			assignedUsers++
			if converted[userID] {
				convertedUsers++
			}
		}
		rate := 0.0
		if assignedUsers > 0 {
			rate = float64(convertedUsers) / float64(assignedUsers)
		}
		variants[index] = experimentVariantResponse{ID: variant.ID, Weight: variant.Weight, AssignedUsers: assignedUsers, ConvertedUsers: convertedUsers, ConversionRate: rate}
	}
	writeJSON(w, http.StatusOK, experimentResponse{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, ID: request.ID, ConversionEvent: request.ConversionEvent, Variants: variants, From: request.From, To: request.To, GeneratedAt: time.Now().UTC()})
}

func chooseExperimentVariant(tenantID, experimentID, userID string, variants []ExperimentVariantRequest) int {
	digest := sha256.Sum256([]byte(tenantID + ":" + experimentID + ":" + userID))
	point := int((uint16(digest[0])<<8 | uint16(digest[1])) % 100)
	cumulative := 0
	for index, variant := range variants {
		cumulative += variant.Weight
		if point < cumulative {
			return index
		}
	}
	return len(variants) - 1
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

func (h *Handler) userProfile(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	environmentID := strings.TrimSpace(r.URL.Query().Get("environment_id"))
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if !identifierPattern.MatchString(tenantID) || !identifierPattern.MatchString(environmentID) || userID == "" || len(userID) > 200 {
		writeError(w, http.StatusBadRequest, "invalid_user_scope", "tenant_id, environment_id, and user_id are required")
		return
	}
	snapshot, err := points.LoadUserSnapshot(r.Context(), h.db, points.Scope{TenantID: tenantID, EnvironmentID: environmentID}, userID, 12)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "user_profile_lookup_failed", "user profile could not be read")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (h *Handler) userTransactions(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	environmentID := strings.TrimSpace(r.URL.Query().Get("environment_id"))
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if !identifierPattern.MatchString(tenantID) || !identifierPattern.MatchString(environmentID) || userID == "" || len(userID) > 200 {
		writeError(w, http.StatusBadRequest, "invalid_user_scope", "tenant_id, environment_id, and user_id are required")
		return
	}
	limit := 12
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	entries, nextCursor, err := points.LoadUserTransactions(r.Context(), h.db, points.Scope{TenantID: tenantID, EnvironmentID: environmentID}, userID, limit, strings.TrimSpace(r.URL.Query().Get("cursor")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_cursor", "cursor is invalid")
		return
	}
	response := map[string]any{"user_id": userID, "transactions": entries}
	if nextCursor != "" {
		response["next_cursor"] = nextCursor
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) createPointTransaction(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID       string          `json:"tenant_id"`
		EnvironmentID  string          `json:"environment_id"`
		ExternalUserID string          `json:"external_user_id"`
		Currency       string          `json:"currency"`
		Amount         int64           `json:"amount"`
		Type           string          `json:"type"`
		IdempotencyKey string          `json:"idempotency_key"`
		Provenance     json.RawMessage `json:"provenance"`
	}
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 32*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.TenantID, request.EnvironmentID, request.ExternalUserID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.EnvironmentID), strings.TrimSpace(request.ExternalUserID)
	request.Currency, request.Type, request.IdempotencyKey = strings.TrimSpace(request.Currency), strings.TrimSpace(request.Type), strings.TrimSpace(request.IdempotencyKey)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) || request.ExternalUserID == "" || request.Currency == "" || request.Amount <= 0 || request.IdempotencyKey == "" || (request.Type != "award" && request.Type != "spend") {
		writeError(w, http.StatusBadRequest, "invalid_transaction", "tenant, environment, user, currency, positive amount, type, and idempotency_key are required")
		return
	}
	ledger := points.NewLedger(h.db)
	var entry points.LedgerEntry
	var err error
	if request.Type == "award" {
		entry, err = ledger.Award(r.Context(), points.Scope{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID}, request.ExternalUserID, request.Currency, request.Amount, request.IdempotencyKey, request.Provenance)
	} else {
		entry, err = ledger.Spend(r.Context(), points.Scope{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID}, request.ExternalUserID, request.Currency, request.Amount, request.IdempotencyKey, request.Provenance)
	}
	switch {
	case errors.Is(err, points.ErrCurrencyNotFound):
		writeError(w, http.StatusNotFound, "currency_not_found", "currency was not found")
	case errors.Is(err, points.ErrInsufficientBalance):
		writeError(w, http.StatusConflict, "insufficient_balance", "point balance is insufficient")
	case errors.Is(err, points.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", "idempotency key was already used with a different request")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "transaction_failed", "point transaction could not be recorded")
	default:
		_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, ActorType: "platform_admin", Action: "points." + request.Type, ResourceType: "point_ledger_entry", ResourceID: strconv.FormatInt(entry.ID, 10), Metadata: auditMetadata(map[string]any{"user_id": request.ExternalUserID, "currency": request.Currency, "amount": request.Amount})})
		writeJSON(w, http.StatusCreated, entry)
	}
}

func (h *Handler) createPointConversion(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID        string `json:"tenant_id"`
		EnvironmentID   string `json:"environment_id"`
		ExternalUserID  string `json:"external_user_id"`
		SourceCurrency  string `json:"source_currency"`
		TargetCurrency  string `json:"target_currency"`
		Amount          int64  `json:"amount"`
		RateNumerator   int64  `json:"rate_numerator"`
		RateDenominator int64  `json:"rate_denominator"`
		IdempotencyKey  string `json:"idempotency_key"`
	}
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.TenantID, request.EnvironmentID, request.ExternalUserID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.EnvironmentID), strings.TrimSpace(request.ExternalUserID)
	request.SourceCurrency, request.TargetCurrency, request.IdempotencyKey = strings.TrimSpace(request.SourceCurrency), strings.TrimSpace(request.TargetCurrency), strings.TrimSpace(request.IdempotencyKey)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) || request.ExternalUserID == "" || request.SourceCurrency == "" || request.TargetCurrency == "" || request.Amount <= 0 || request.RateNumerator <= 0 || request.RateDenominator <= 0 || request.IdempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "invalid_conversion", "tenant, environment, user, currencies, positive amount, positive rate, and idempotency_key are required")
		return
	}
	conversion, err := points.Convert(r.Context(), h.db, points.ConversionRequest{Scope: points.Scope{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID}, ExternalUserID: request.ExternalUserID, SourceCurrency: request.SourceCurrency, TargetCurrency: request.TargetCurrency, Amount: request.Amount, RateNumerator: request.RateNumerator, RateDenominator: request.RateDenominator, IdempotencyKey: request.IdempotencyKey})
	switch {
	case errors.Is(err, points.ErrCurrencyNotFound):
		writeError(w, http.StatusNotFound, "currency_not_found", "currency was not found")
	case errors.Is(err, points.ErrInsufficientBalance):
		writeError(w, http.StatusConflict, "insufficient_balance", "point balance is insufficient")
	case errors.Is(err, points.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", "idempotency key was already used with a different request")
	case err != nil:
		writeError(w, http.StatusBadRequest, "conversion_failed", err.Error())
	default:
		_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, ActorType: "platform_admin", Action: "points.converted", ResourceType: "point_conversion", ResourceID: conversion.ID})
		writeJSON(w, http.StatusCreated, conversion)
	}
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

type adminRedemptionRequest struct {
	TenantID       string `json:"tenant_id"`
	EnvironmentID  string `json:"environment_id"`
	RewardID       string `json:"reward_id"`
	RedemptionID   string `json:"redemption_id"`
	ExternalUserID string `json:"external_user_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (h *Handler) testRedemption(w http.ResponseWriter, r *http.Request) {
	var request adminRedemptionRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.RewardID = strings.TrimSpace(request.RewardID)
	request.ExternalUserID = strings.TrimSpace(request.ExternalUserID)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) || request.RewardID == "" || request.ExternalUserID == "" || request.IdempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "invalid_redemption", "tenant, environment, reward, external user, and idempotency key are required")
		return
	}
	redemption, err := h.rewardOps.RedeemForAdmin(r.Context(), points.Scope{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID}, request.RewardID, request.ExternalUserID, request.IdempotencyKey)
	h.writeAdminRedemptionResult(w, r, redemption, err, request.TenantID, request.EnvironmentID, "redemption.tested")
}

func (h *Handler) retryAdminRedemption(w http.ResponseWriter, r *http.Request) {
	var request adminRedemptionRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.RedemptionID = strings.TrimSpace(request.RedemptionID)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) || request.RedemptionID == "" {
		writeError(w, http.StatusBadRequest, "invalid_redemption", "tenant, environment, and redemption_id are required")
		return
	}
	redemption, err := h.rewardOps.RetryForAdmin(r.Context(), points.Scope{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID}, request.RedemptionID)
	h.writeAdminRedemptionResult(w, r, redemption, err, request.TenantID, request.EnvironmentID, "redemption.retried")
}

func (h *Handler) writeAdminRedemptionResult(w http.ResponseWriter, r *http.Request, redemption rewards.Redemption, err error, tenantID, environmentID, action string) {
	switch {
	case errors.Is(err, points.ErrInsufficientBalance):
		writeError(w, http.StatusConflict, "insufficient_balance", "point balance is insufficient")
	case errors.Is(err, points.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", "idempotency key was already used with a different request")
	case errors.Is(err, points.ErrCurrencyNotFound):
		writeError(w, http.StatusNotFound, "currency_not_found", "redemption currency was not found")
	case errors.Is(err, rewards.ErrRedemptionNotRetryable):
		writeError(w, http.StatusConflict, "redemption_not_retryable", "redemption cannot be retried in its current state")
	case errors.Is(err, rewards.ErrDeliveryUnknown):
		writeJSON(w, http.StatusAccepted, redemption)
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "redemption_not_found", "redemption or reward was not found")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "redemption_failed", "redemption could not be completed")
	default:
		_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: tenantID, EnvironmentID: environmentID, ActorType: "platform_admin", Action: action, ResourceType: "redemption", ResourceID: redemption.ID, Metadata: auditMetadata(map[string]any{"status": redemption.Status})})
		writeJSON(w, http.StatusOK, redemption)
	}
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

func (h *Handler) updateCatalogStatus(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID      string `json:"tenant_id"`
		EnvironmentID string `json:"environment_id"`
		Resource      string `json:"resource"`
		ID            string `json:"id"`
		Active        bool   `json:"active"`
	}
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 8*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.Resource = strings.TrimSpace(request.Resource)
	request.ID = strings.TrimSpace(request.ID)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) || request.ID == "" {
		writeError(w, http.StatusBadRequest, "invalid_catalog_resource", "tenant_id, environment_id, and id are required")
		return
	}
	queries := map[string]string{
		"currency":    `UPDATE currencies SET active = $4, updated_at = now() WHERE id = $1 AND tenant_id = $2 AND environment_id = $3`,
		"reward":      `UPDATE rewards SET active = $4, updated_at = now() WHERE id = $1 AND tenant_id = $2 AND environment_id = $3`,
		"mission":     `UPDATE missions SET active = $4 WHERE id = $1 AND tenant_id = $2 AND environment_id = $3`,
		"achievement": `UPDATE achievements SET active = $4 WHERE id = $1 AND tenant_id = $2 AND environment_id = $3`,
		"level":       `UPDATE level_definitions SET active = $4 WHERE id = $1 AND tenant_id = $2 AND environment_id = $3`,
		"leaderboard": `UPDATE leaderboards SET active = $4, updated_at = now() WHERE id = $1 AND tenant_id = $2 AND environment_id = $3`,
	}
	query, ok := queries[request.Resource]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_catalog_resource", "resource must be currency, reward, mission, achievement, level, or leaderboard")
		return
	}
	command, err := h.db.Exec(r.Context(), query, request.ID, request.TenantID, request.EnvironmentID, request.Active)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "catalog_status_update_failed", "catalog status could not be updated")
		return
	}
	if command.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "catalog_resource_not_found", "catalog resource was not found")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: request.TenantID, EnvironmentID: request.EnvironmentID, ActorType: "platform_admin", Action: "catalog.status_changed", ResourceType: request.Resource, ResourceID: request.ID, Metadata: auditMetadata(map[string]any{"active": request.Active})})
	writeJSON(w, http.StatusOK, map[string]any{"resource": request.Resource, "id": request.ID, "active": request.Active})
}

func catalogIDFromPath(path, resource string) (string, bool) {
	prefix := "/v1/admin/" + resource + "/"
	if !strings.HasPrefix(path, prefix) {
		return "", false
	}
	id := strings.TrimPrefix(path, prefix)
	if id == "" || strings.Contains(id, "/") || len(id) > 128 {
		return "", false
	}
	return id, true
}

func (h *Handler) updateCatalogResource(w http.ResponseWriter, r *http.Request, resource string) {
	id, ok := catalogIDFromPath(r.URL.Path, map[string]string{"currency": "currencies", "mission": "missions", "achievement": "achievements", "level": "levels", "reward": "rewards"}[resource])
	if !ok {
		writeError(w, http.StatusNotFound, "catalog_resource_not_found", "catalog resource was not found")
		return
	}
	switch resource {
	case "currency":
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
		tx, err := h.db.Begin(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "currency_update_failed", "currency could not be updated")
			return
		}
		defer func() { _ = tx.Rollback(r.Context()) }()
		var currency points.Currency
		err = tx.QueryRow(r.Context(), `UPDATE currencies SET name = $4, unit_scale = $5, updated_at = now() WHERE id = $1 AND tenant_id = $2 AND environment_id = $3 RETURNING id, tenant_id, environment_id, code, name, unit_scale, policy_version, active, created_at, updated_at`, id, request.TenantID, request.EnvironmentID, request.Name, request.UnitScale).
			Scan(&currency.ID, &currency.TenantID, &currency.EnvironmentID, &currency.Code, &currency.Name, &currency.UnitScale, &currency.PolicyVersion, &currency.Active, &currency.CreatedAt, &currency.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "currency_not_found", "currency was not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "currency_update_failed", "currency could not be updated")
			return
		}
		var nextVersion int
		if err = tx.QueryRow(r.Context(), `SELECT policy_version + 1 FROM currencies WHERE id = $1 FOR UPDATE`, id).Scan(&nextVersion); err != nil {
			writeError(w, http.StatusInternalServerError, "currency_update_failed", "currency policy could not be updated")
			return
		}
		if _, err = tx.Exec(r.Context(), `INSERT INTO currency_policy_versions (currency_id, version, policy) VALUES ($1, $2, $3::jsonb)`, id, nextVersion, request.Policy); err != nil {
			writeError(w, http.StatusInternalServerError, "currency_update_failed", "currency policy could not be updated")
			return
		}
		if err = tx.QueryRow(r.Context(), `UPDATE currencies SET policy_version = $2, updated_at = now() WHERE id = $1 RETURNING policy_version, updated_at`, id, nextVersion).Scan(&currency.PolicyVersion, &currency.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "currency_update_failed", "currency policy could not be updated")
			return
		}
		currency.Policy = request.Policy
		if err = tx.Commit(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "currency_update_failed", "currency could not be updated")
			return
		}
		_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: currency.TenantID, EnvironmentID: currency.EnvironmentID, ActorType: "platform_admin", Action: "currency.updated", ResourceType: "currency", ResourceID: currency.ID})
		writeJSON(w, http.StatusOK, currency)
	case "mission":
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
		request.TenantID, request.EnvironmentID, request.Name, request.EventType = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.EnvironmentID), strings.TrimSpace(request.Name), strings.TrimSpace(request.EventType)
		if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) || request.Name == "" || len(request.Name) > 120 || !eventTypePattern.MatchString(request.EventType) || request.TargetCount < 1 || request.TargetCount > 100000 {
			writeError(w, http.StatusBadRequest, "invalid_mission", "tenant, environment, name, event_type, and target_count are required and valid")
			return
		}
		var mission missionResponse
		err := h.db.QueryRow(r.Context(), `UPDATE missions SET name = $4, event_type = $5, target_count = $6 WHERE id = $1 AND tenant_id = $2 AND environment_id = $3 RETURNING id, tenant_id, environment_id, name, event_type, target_count, active, created_at`, id, request.TenantID, request.EnvironmentID, request.Name, request.EventType, request.TargetCount).Scan(&mission.ID, &mission.TenantID, &mission.EnvironmentID, &mission.Name, &mission.EventType, &mission.TargetCount, &mission.Active, &mission.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "mission_not_found", "mission was not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "mission_update_failed", "mission could not be updated")
			return
		}
		_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: mission.TenantID, EnvironmentID: mission.EnvironmentID, ActorType: "platform_admin", Action: "mission.updated", ResourceType: "mission", ResourceID: mission.ID})
		writeJSON(w, http.StatusOK, mission)
	case "achievement":
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
		var achievement progression.Achievement
		err = h.db.QueryRow(r.Context(), `UPDATE achievements SET name = $4, description = $5, event_type = $6, required_count = $7 WHERE id = $1 AND tenant_id = $2 AND environment_id = $3 RETURNING id, tenant_id, environment_id, name, description, event_type, required_count, active, created_at`, id, request.TenantID, request.EnvironmentID, request.Name, request.Description, request.EventType, request.RequiredCount).Scan(&achievement.ID, &achievement.TenantID, &achievement.EnvironmentID, &achievement.Name, &achievement.Description, &achievement.EventType, &achievement.RequiredCount, &achievement.Active, &achievement.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "achievement_not_found", "achievement was not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "achievement_update_failed", "achievement could not be updated")
			return
		}
		_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: achievement.TenantID, EnvironmentID: achievement.EnvironmentID, ActorType: "platform_admin", Action: "achievement.updated", ResourceType: "achievement", ResourceID: achievement.ID})
		writeJSON(w, http.StatusOK, achievement)
	case "level":
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
		var level progression.Level
		err = h.db.QueryRow(r.Context(), `UPDATE level_definitions SET name = $4, xp_threshold = $5 WHERE id = $1 AND tenant_id = $2 AND environment_id = $3 RETURNING id, tenant_id, environment_id, level, name, xp_threshold, active, created_at`, id, request.TenantID, request.EnvironmentID, request.Name, request.XPThreshold).Scan(&level.ID, &level.TenantID, &level.EnvironmentID, &level.Level, &level.Name, &level.XPThreshold, &level.Active, &level.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "level_not_found", "level was not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "level_update_failed", "level could not be updated")
			return
		}
		_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: level.TenantID, EnvironmentID: level.EnvironmentID, ActorType: "platform_admin", Action: "level.updated", ResourceType: "level", ResourceID: level.ID})
		writeJSON(w, http.StatusOK, level)
	case "reward":
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
		tx, err := h.db.Begin(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "reward_update_failed", "reward could not be updated")
			return
		}
		defer func() { _ = tx.Rollback(r.Context()) }()
		var costCurrencyID *string
		if request.CostCurrency != "" {
			var currencyID string
			if err = tx.QueryRow(r.Context(), `SELECT id FROM currencies WHERE tenant_id = $1 AND environment_id = $2 AND code = $3 AND active = true`, request.TenantID, request.EnvironmentID, request.CostCurrency).Scan(&currencyID); errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, "currency_not_found", "cost currency was not found in the tenant environment")
				return
			} else if err != nil {
				writeError(w, http.StatusInternalServerError, "reward_update_failed", "reward currency could not be read")
				return
			}
			costCurrencyID = &currencyID
		}
		var reward rewards.Reward
		err = tx.QueryRow(r.Context(), `UPDATE rewards SET name = $4, description = $5, cost_currency_id = $6, cost_amount = NULLIF($7, 0), payload = $8::jsonb, updated_at = now() WHERE id = $1 AND tenant_id = $2 AND environment_id = $3 RETURNING id, tenant_id, environment_id, name, reward_type, description, COALESCE((SELECT code FROM currencies WHERE id = cost_currency_id), ''), COALESCE(cost_amount, 0), payload, active, created_at, updated_at`, id, request.TenantID, request.EnvironmentID, request.Name, request.Description, costCurrencyID, request.CostAmount, request.Payload).Scan(&reward.ID, &reward.TenantID, &reward.EnvironmentID, &reward.Name, &reward.RewardType, &reward.Description, &reward.CostCurrency, &reward.CostAmount, &reward.Payload, &reward.Active, &reward.CreatedAt, &reward.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "reward_not_found", "reward was not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "reward_update_failed", "reward could not be updated")
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "reward_update_failed", "reward could not be updated")
			return
		}
		_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: reward.TenantID, EnvironmentID: reward.EnvironmentID, ActorType: "platform_admin", Action: "reward.updated", ResourceType: "reward", ResourceID: reward.ID})
		writeJSON(w, http.StatusOK, reward)
	}
}

func (h *Handler) updateEngagementResource(w http.ResponseWriter, r *http.Request, resource string) {
	id, ok := catalogIDFromPath(r.URL.Path, resource+"s")
	if !ok {
		writeError(w, http.StatusNotFound, resource+"_not_found", resource+" was not found")
		return
	}
	if resource == "goal" {
		var request GoalRequest
		if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
			return
		}
		request.ID = id
		var err error
		request, err = ValidateGoalRequest(request)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_goal", err.Error())
			return
		}
		var goal goalResponse
		err = h.db.QueryRow(r.Context(), `UPDATE goals SET name = $4, description = $5, event_type = $6, target_count = $7 WHERE id = $1 AND tenant_id = $2 AND environment_id = $3 RETURNING id, tenant_id, environment_id, name, description, event_type, target_count, active, created_at`, id, request.TenantID, request.EnvironmentID, request.Name, request.Description, request.EventType, request.TargetCount).Scan(&goal.ID, &goal.TenantID, &goal.EnvironmentID, &goal.Name, &goal.Description, &goal.EventType, &goal.TargetCount, &goal.Active, &goal.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "goal_not_found", "goal was not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "goal_update_failed", "goal could not be updated")
			return
		}
		_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: goal.TenantID, EnvironmentID: goal.EnvironmentID, ActorType: "platform_admin", Action: "goal.updated", ResourceType: "goal", ResourceID: goal.ID})
		writeJSON(w, http.StatusOK, goal)
		return
	}
	var request CampaignRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.ID = id
	var err error
	request, err = ValidateCampaignRequest(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_campaign", err.Error())
		return
	}
	if request.GoalID != "" {
		var scope string
		if err = h.db.QueryRow(r.Context(), `SELECT tenant_id || ':' || environment_id FROM goals WHERE id = $1`, request.GoalID).Scan(&scope); errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "goal_not_found", "goal was not found")
			return
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, "goal_lookup_failed", "goal could not be read")
			return
		} else if scope != request.TenantID+":"+request.EnvironmentID {
			writeError(w, http.StatusBadRequest, "goal_scope_mismatch", "goal must belong to the campaign scope")
			return
		}
	}
	var campaign campaignResponse
	err = h.db.QueryRow(r.Context(), `UPDATE campaigns SET goal_id = NULLIF($4,''), name = $5, description = $6, event_type = $7, starts_at = $8, ends_at = $9 WHERE id = $1 AND tenant_id = $2 AND environment_id = $3 RETURNING id, tenant_id, environment_id, COALESCE(goal_id,''), name, description, event_type, starts_at, ends_at, active, created_at`, id, request.TenantID, request.EnvironmentID, request.GoalID, request.Name, request.Description, request.EventType, request.StartsAt, request.EndsAt).Scan(&campaign.ID, &campaign.TenantID, &campaign.EnvironmentID, &campaign.GoalID, &campaign.Name, &campaign.Description, &campaign.EventType, &campaign.StartsAt, &campaign.EndsAt, &campaign.Active, &campaign.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "campaign_not_found", "campaign was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "campaign_update_failed", "campaign could not be updated")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: campaign.TenantID, EnvironmentID: campaign.EnvironmentID, ActorType: "platform_admin", Action: "campaign.updated", ResourceType: "campaign", ResourceID: campaign.ID})
	writeJSON(w, http.StatusOK, campaign)
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
