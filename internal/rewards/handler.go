package rewards

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/audit"
	"github.com/motifengine/motifengine/internal/auth"
	"github.com/motifengine/motifengine/internal/observability"
	"github.com/motifengine/motifengine/internal/points"
	appruntime "github.com/motifengine/motifengine/internal/runtime"
)

type Handler struct {
	db       *pgxpool.Pool
	provider ProviderResolver
	ledger   *points.Ledger
	metrics  *observability.Metrics
}

// RedeemForAdmin lets the authenticated admin console exercise the same
// durable redemption state machine without requiring a raw integration key in
// the browser. The scope is authorized by the admin handler before this call.
func (h *Handler) RedeemForAdmin(ctx context.Context, scope points.Scope, rewardID, externalUserID, idempotencyKey string) (Redemption, error) {
	return h.redeem(ctx, scope, rewardID, redemptionRequest{ExternalUserID: externalUserID, IdempotencyKey: idempotencyKey})
}

// RetryForAdmin performs the same retry-safe transition used by the public
// integration route for a server-authorized admin scope.
func (h *Handler) RetryForAdmin(ctx context.Context, scope points.Scope, redemptionID string) (Redemption, error) {
	return h.retry(ctx, scope, redemptionID)
}

func NewHandler(db *pgxpool.Pool, provider Provider, metrics ...*observability.Metrics) http.Handler {
	return NewHandlerWithResolver(db, StaticProviderResolver{Provider: provider}, metrics...)
}

func NewHandlerWithResolver(db *pgxpool.Pool, provider ProviderResolver, metrics ...*observability.Metrics) http.Handler {
	var currentMetrics *observability.Metrics
	if len(metrics) > 0 {
		currentMetrics = metrics[0]
	}
	return &Handler{db: db, provider: provider, ledger: points.NewLedger(db), metrics: currentMetrics}
}

type redemptionRequest struct {
	ExternalUserID string `json:"external_user_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key, ok := auth.RequestKey(w, r)
	if !ok {
		return
	}
	scope := points.Scope(key.Scope())
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/rewards/") && strings.HasSuffix(r.URL.Path, "/redemptions") {
		h.createRedemption(w, r, scope)
		return
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/redemptions/") && strings.HasSuffix(r.URL.Path, "/retry") {
		h.retryRedemption(w, r, scope)
		return
	}
	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/redemptions/") {
		h.getRedemption(w, r, scope)
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "route not found")
}

func (h *Handler) resolveProvider(ctx context.Context, scope points.Scope) (Provider, error) {
	if h.provider == nil {
		return nil, fmt.Errorf("reward provider is not configured")
	}
	return h.provider.Resolve(ctx, scope.TenantID, scope.EnvironmentID)
}

func (h *Handler) createRedemption(w http.ResponseWriter, r *http.Request, scope points.Scope) {
	rewardID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/rewards/"), "/redemptions")
	if rewardID == "" || strings.Contains(rewardID, "/") {
		writeError(w, http.StatusNotFound, "reward_not_found", "reward was not found")
		return
	}
	var request redemptionRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.ExternalUserID = strings.TrimSpace(request.ExternalUserID)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.ExternalUserID == "" || request.IdempotencyKey == "" {
		writeError(w, http.StatusBadRequest, "invalid_redemption", "external_user_id and idempotency_key are required")
		return
	}
	redemption, err := h.redeem(r.Context(), scope, rewardID, request)
	if errors.Is(err, points.ErrInsufficientBalance) {
		writeError(w, http.StatusConflict, "insufficient_balance", "point balance is insufficient")
		return
	}
	if errors.Is(err, points.ErrIdempotencyConflict) {
		writeError(w, http.StatusConflict, "idempotency_conflict", "idempotency key was already used with a different request")
		return
	}
	if errors.Is(err, ErrDeliveryUnknown) {
		writeJSON(w, http.StatusAccepted, redemption)
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "reward_not_found", "reward was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "redemption_failed", "reward redemption could not be completed")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: scope.TenantID, EnvironmentID: scope.EnvironmentID, ActorType: "integration", Action: "redemption.state_changed", ResourceType: "redemption", ResourceID: redemption.ID, Metadata: redemptionAuditMetadata(redemption.Status)})
	writeJSON(w, http.StatusCreated, redemption)
}

func (h *Handler) redeem(ctx context.Context, scope points.Scope, rewardID string, request redemptionRequest) (Redemption, error) {
	var reward Reward
	var currencyID *string
	var costAmount *int64
	var payload []byte
	err := h.db.QueryRow(ctx, `
		SELECT r.id, r.tenant_id, r.environment_id, r.name, r.reward_type, r.description, c.code, r.cost_amount, r.payload, r.active, r.created_at, r.updated_at
		FROM rewards r LEFT JOIN currencies c ON c.id = r.cost_currency_id
		WHERE r.id = $1 AND r.tenant_id = $2 AND r.environment_id = $3 AND r.active = true`, rewardID, scope.TenantID, scope.EnvironmentID).
		Scan(&reward.ID, &reward.TenantID, &reward.EnvironmentID, &reward.Name, &reward.RewardType, &reward.Description, &currencyID, &costAmount, &payload, &reward.Active, &reward.CreatedAt, &reward.UpdatedAt)
	if err != nil {
		return Redemption{}, err
	}
	reward.Payload = payload
	if currencyID != nil {
		reward.CostCurrency = *currencyID
	}
	if costAmount != nil {
		reward.CostAmount = *costAmount
	}
	provider, err := h.resolveProvider(ctx, scope)
	if err != nil {
		return Redemption{}, err
	}
	hashBytes, _ := json.Marshal(request)
	hash := sha256.Sum256(hashBytes)
	redemptionID, err := generateRedemptionID()
	if err != nil {
		return Redemption{}, err
	}
	var redemption Redemption
	var existingHash []byte
	if err := h.db.QueryRow(ctx, `
		SELECT id, reward_id, external_user_id, status, idempotency_key, COALESCE(provider_reference, ''), COALESCE(failure_reason, ''), created_at, updated_at, request_hash
		FROM redemptions WHERE tenant_id = $1 AND environment_id = $2 AND external_user_id = $3 AND idempotency_key = $4`, scope.TenantID, scope.EnvironmentID, request.ExternalUserID, request.IdempotencyKey).
		Scan(&redemption.ID, &redemption.RewardID, &redemption.ExternalUserID, &redemption.Status, &redemption.IdempotencyKey, &redemption.ProviderReference, &redemption.FailureReason, &redemption.CreatedAt, &redemption.UpdatedAt, &existingHash); err == nil {
		if string(existingHash) != string(hash[:]) {
			return Redemption{}, points.ErrIdempotencyConflict
		}
		return redemption, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Redemption{}, err
	}
	if _, err := h.db.Exec(ctx, `INSERT INTO redemptions (id, reward_id, tenant_id, environment_id, external_user_id, status, idempotency_key, request_hash) VALUES ($1, $2, $3, $4, $5, 'requested', $6, $7)`, redemptionID, rewardID, scope.TenantID, scope.EnvironmentID, request.ExternalUserID, request.IdempotencyKey, hash[:]); err != nil {
		// A concurrent request may have won the idempotency race. Return its
		// durable result instead of exposing a database unique violation.
		if isUniqueViolation(err) {
			return h.loadExisting(ctx, scope, request)
		}
		return Redemption{}, err
	}
	if currencyID != nil && costAmount != nil {
		if _, err := h.ledger.Spend(ctx, scope, request.ExternalUserID, reward.CostCurrency, reward.CostAmount, "redemption:"+redemptionID+":reserve", json.RawMessage(`{"source":"redemption"}`)); err != nil {
			_, _ = h.db.Exec(ctx, `UPDATE redemptions SET status = 'failed', failure_reason = $2, updated_at = now() WHERE id = $1`, redemptionID, err.Error())
			return Redemption{}, err
		}
		if _, err := h.db.Exec(ctx, `UPDATE redemptions SET status = 'reserved', updated_at = now() WHERE id = $1`, redemptionID); err != nil {
			return Redemption{}, err
		}
	}
	if _, err := h.db.Exec(ctx, `UPDATE redemptions SET status = 'delivering', updated_at = now() WHERE id = $1`, redemptionID); err != nil {
		return Redemption{}, err
	}
	var providerPayload map[string]any
	_ = json.Unmarshal(payload, &providerPayload)
	var attemptID int64
	if err := h.db.QueryRow(ctx, `INSERT INTO reward_delivery_attempts (redemption_id, status) VALUES ($1, 'started') RETURNING id`, redemptionID).Scan(&attemptID); err != nil {
		return Redemption{}, err
	}
	delivery, err := provider.Deliver(ctx, DeliveryRequest{RedemptionID: redemptionID, RewardID: rewardID, ExternalUserID: request.ExternalUserID, Payload: providerPayload})
	if err != nil {
		h.observeProvider(err)
		status := "failed"
		providerState := "failed"
		if errors.Is(err, ErrDeliveryUnknown) {
			status = "delivering"
			providerState = "unknown"
		}
		_, _ = h.db.Exec(ctx, `UPDATE reward_delivery_attempts SET status = 'failed', provider_state = $2, error_message = $3, completed_at = now() WHERE id = $1`, attemptID, providerState, err.Error())
		_, _ = h.db.Exec(ctx, `UPDATE redemptions SET status = $2, failure_reason = $3, updated_at = now() WHERE id = $1`, redemptionID, status, err.Error())
		if status == "failed" && currencyID != nil && costAmount != nil {
			_, _ = h.ledger.Release(ctx, scope, request.ExternalUserID, reward.CostCurrency, reward.CostAmount, "redemption:"+redemptionID+":release", json.RawMessage(`{"source":"redemption_release"}`))
		}
		return Redemption{ID: redemptionID, RewardID: rewardID, ExternalUserID: request.ExternalUserID, Status: status, IdempotencyKey: request.IdempotencyKey, FailureReason: err.Error()}, err
	}
	h.observeProvider(nil)
	if _, err := h.db.Exec(ctx, `UPDATE reward_delivery_attempts SET status = 'fulfilled', provider_state = 'fulfilled', provider_reference = $2, completed_at = now() WHERE id = $1`, attemptID, delivery.ProviderReference); err != nil {
		return Redemption{}, err
	}
	if err := h.db.QueryRow(ctx, `UPDATE redemptions SET status = 'fulfilled', provider_reference = $2, updated_at = now() WHERE id = $1 RETURNING id, reward_id, external_user_id, status, idempotency_key, COALESCE(provider_reference, ''), COALESCE(failure_reason, ''), created_at, updated_at`, redemptionID, delivery.ProviderReference).Scan(&redemption.ID, &redemption.RewardID, &redemption.ExternalUserID, &redemption.Status, &redemption.IdempotencyKey, &redemption.ProviderReference, &redemption.FailureReason, &redemption.CreatedAt, &redemption.UpdatedAt); err != nil {
		return Redemption{}, err
	}
	return redemption, nil
}

func (h *Handler) loadExisting(ctx context.Context, scope points.Scope, request redemptionRequest) (Redemption, error) {
	var redemption Redemption
	var storedHash []byte
	err := h.db.QueryRow(ctx, `
		SELECT id, reward_id, external_user_id, status, idempotency_key, COALESCE(provider_reference, ''), COALESCE(failure_reason, ''), created_at, updated_at, request_hash
		FROM redemptions WHERE tenant_id = $1 AND environment_id = $2 AND external_user_id = $3 AND idempotency_key = $4`, scope.TenantID, scope.EnvironmentID, request.ExternalUserID, request.IdempotencyKey).
		Scan(&redemption.ID, &redemption.RewardID, &redemption.ExternalUserID, &redemption.Status, &redemption.IdempotencyKey, &redemption.ProviderReference, &redemption.FailureReason, &redemption.CreatedAt, &redemption.UpdatedAt, &storedHash)
	if err != nil {
		return Redemption{}, err
	}
	hashBytes, _ := json.Marshal(request)
	hash := sha256.Sum256(hashBytes)
	if !bytes.Equal(storedHash, hash[:]) {
		return Redemption{}, points.ErrIdempotencyConflict
	}
	return redemption, nil
}

func (h *Handler) retryRedemption(w http.ResponseWriter, r *http.Request, scope points.Scope) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/redemptions/"), "/retry")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "redemption_not_found", "redemption was not found")
		return
	}
	redemption, err := h.retry(r.Context(), scope, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "redemption_not_found", "redemption was not found")
	case errors.Is(err, points.ErrInsufficientBalance):
		writeError(w, http.StatusConflict, "insufficient_balance", "point balance is insufficient for retry")
	case errors.Is(err, points.ErrCurrencyNotFound):
		writeError(w, http.StatusNotFound, "currency_not_found", "redemption currency was not found")
	case errors.Is(err, errRedemptionNotRetryable):
		writeError(w, http.StatusConflict, "redemption_not_retryable", "redemption cannot be retried in its current state")
	case errors.Is(err, ErrDeliveryUnknown):
		writeJSON(w, http.StatusAccepted, redemption)
	case err != nil:
		writeError(w, http.StatusInternalServerError, "redemption_retry_failed", "redemption could not be retried")
	default:
		_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: scope.TenantID, EnvironmentID: scope.EnvironmentID, ActorType: "integration", Action: "redemption.retried", ResourceType: "redemption", ResourceID: redemption.ID, Metadata: redemptionAuditMetadata(redemption.Status)})
		writeJSON(w, http.StatusOK, redemption)
	}
}

var errRedemptionNotRetryable = errors.New("redemption is not retryable")

var ErrRedemptionNotRetryable = errRedemptionNotRetryable

func (h *Handler) retry(ctx context.Context, scope points.Scope, redemptionID string) (Redemption, error) {
	var redemption Redemption
	var status string
	var payload []byte
	var currency *string
	var cost *int64
	if err := h.db.QueryRow(ctx, `
		SELECT d.id, d.reward_id, d.external_user_id, d.status, d.idempotency_key,
		       COALESCE(d.provider_reference, ''), COALESCE(d.failure_reason, ''), d.created_at, d.updated_at,
		       r.payload, c.code, r.cost_amount
		FROM redemptions d
		JOIN rewards r ON r.id = d.reward_id
		LEFT JOIN currencies c ON c.id = r.cost_currency_id
		WHERE d.id = $1 AND d.tenant_id = $2 AND d.environment_id = $3`, redemptionID, scope.TenantID, scope.EnvironmentID).
		Scan(&redemption.ID, &redemption.RewardID, &redemption.ExternalUserID, &status, &redemption.IdempotencyKey, &redemption.ProviderReference, &redemption.FailureReason, &redemption.CreatedAt, &redemption.UpdatedAt, &payload, &currency, &cost); err != nil {
		return Redemption{}, err
	}
	redemption.Status = status
	if status == "fulfilled" {
		return redemption, nil
	}
	if status == "cancelled" {
		return Redemption{}, errRedemptionNotRetryable
	}
	provider, err := h.resolveProvider(ctx, scope)
	if err != nil {
		return Redemption{}, err
	}
	var attempts int
	if err := h.db.QueryRow(ctx, `SELECT count(*) FROM reward_delivery_attempts WHERE redemption_id = $1`, redemptionID).Scan(&attempts); err != nil {
		return Redemption{}, err
	}
	spendKey := fmt.Sprintf("redemption:%s:reserve", redemptionID)
	if status == "failed" && currency != nil && cost != nil {
		spendKey = fmt.Sprintf("redemption:%s:retry:%d", redemptionID, attempts+1)
		if _, err := h.ledger.Spend(ctx, scope, redemption.ExternalUserID, *currency, *cost, spendKey, json.RawMessage(`{"source":"redemption_retry"}`)); err != nil {
			return Redemption{}, err
		}
	} else if currency != nil && cost != nil {
		var reserved bool
		if err := h.db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM point_ledger_entries e
				JOIN point_accounts a ON a.id = e.account_id
				JOIN currencies c ON c.id = a.currency_id
				WHERE a.tenant_id = $1 AND a.environment_id = $2 AND a.external_user_id = $3 AND c.code = $4 AND e.idempotency_key = $5
			)`, scope.TenantID, scope.EnvironmentID, redemption.ExternalUserID, *currency, spendKey).Scan(&reserved); err != nil {
			return Redemption{}, err
		}
		if !reserved {
			if _, err := h.ledger.Spend(ctx, scope, redemption.ExternalUserID, *currency, *cost, spendKey, json.RawMessage(`{"source":"redemption_recovery"}`)); err != nil {
				return Redemption{}, err
			}
		}
	}
	if _, err := h.db.Exec(ctx, `UPDATE redemptions SET status = 'delivering', failure_reason = NULL, updated_at = now() WHERE id = $1 AND status <> 'fulfilled'`, redemptionID); err != nil {
		return Redemption{}, err
	}
	var attemptID int64
	if err := h.db.QueryRow(ctx, `INSERT INTO reward_delivery_attempts (redemption_id, status) VALUES ($1, 'started') RETURNING id`, redemptionID).Scan(&attemptID); err != nil {
		return Redemption{}, err
	}
	var providerPayload map[string]any
	_ = json.Unmarshal(payload, &providerPayload)
	delivery, err := provider.Deliver(ctx, DeliveryRequest{RedemptionID: redemptionID, RewardID: redemption.RewardID, ExternalUserID: redemption.ExternalUserID, Payload: providerPayload})
	if err != nil {
		h.observeProvider(err)
		status = "failed"
		providerState := "failed"
		if errors.Is(err, ErrDeliveryUnknown) {
			status = "delivering"
			providerState = "unknown"
		}
		_, _ = h.db.Exec(ctx, `UPDATE reward_delivery_attempts SET status = 'failed', provider_state = $2, error_message = $3, completed_at = now() WHERE id = $1`, attemptID, providerState, err.Error())
		_, _ = h.db.Exec(ctx, `UPDATE redemptions SET status = $2, failure_reason = $3, updated_at = now() WHERE id = $1`, redemptionID, status, err.Error())
		if status == "failed" && currency != nil && cost != nil {
			_, _ = h.ledger.Release(ctx, scope, redemption.ExternalUserID, *currency, *cost, spendKey+":release", json.RawMessage(`{"source":"redemption_retry_release"}`))
		}
		redemption.Status = status
		redemption.FailureReason = err.Error()
		return redemption, err
	}
	h.observeProvider(nil)
	if _, err := h.db.Exec(ctx, `UPDATE reward_delivery_attempts SET status = 'fulfilled', provider_state = 'fulfilled', provider_reference = $2, completed_at = now() WHERE id = $1`, attemptID, delivery.ProviderReference); err != nil {
		return Redemption{}, err
	}
	if err := h.db.QueryRow(ctx, `UPDATE redemptions SET status = 'fulfilled', provider_reference = $2, failure_reason = NULL, updated_at = now() WHERE id = $1 RETURNING id, reward_id, external_user_id, status, idempotency_key, COALESCE(provider_reference, ''), COALESCE(failure_reason, ''), created_at, updated_at`, redemptionID, delivery.ProviderReference).Scan(&redemption.ID, &redemption.RewardID, &redemption.ExternalUserID, &redemption.Status, &redemption.IdempotencyKey, &redemption.ProviderReference, &redemption.FailureReason, &redemption.CreatedAt, &redemption.UpdatedAt); err != nil {
		return Redemption{}, err
	}
	return redemption, nil
}

func (h *Handler) observeProvider(err error) {
	if h.metrics == nil {
		return
	}
	if err == nil {
		h.metrics.ObserveProvider("fulfilled")
		return
	}
	if errors.Is(err, ErrDeliveryUnknown) {
		h.metrics.ObserveProvider("unknown")
		return
	}
	h.metrics.ObserveProvider("failed")
}

func (h *Handler) getRedemption(w http.ResponseWriter, r *http.Request, scope points.Scope) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/redemptions/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "redemption_not_found", "redemption was not found")
		return
	}
	var redemption Redemption
	err := h.db.QueryRow(r.Context(), `SELECT id, reward_id, external_user_id, status, idempotency_key, COALESCE(provider_reference, ''), COALESCE(failure_reason, ''), created_at, updated_at FROM redemptions WHERE id = $1 AND tenant_id = $2 AND environment_id = $3`, id, scope.TenantID, scope.EnvironmentID).Scan(&redemption.ID, &redemption.RewardID, &redemption.ExternalUserID, &redemption.Status, &redemption.IdempotencyKey, &redemption.ProviderReference, &redemption.FailureReason, &redemption.CreatedAt, &redemption.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "redemption_not_found", "redemption was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "redemption_lookup_failed", "redemption could not be read")
		return
	}
	writeJSON(w, http.StatusOK, redemption)
}

func generateRedemptionID() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return fmt.Sprintf("redemption_%x", value), nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func redemptionAuditMetadata(status string) json.RawMessage {
	encoded, err := json.Marshal(map[string]string{"status": status})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return encoded
}
