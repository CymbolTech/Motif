package rewards

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/points"
)

const (
	defaultReconciliationInterval = 30 * time.Second
	reconciliationLease           = 5 * time.Minute
	maxReconciliationBatch        = 50
)

// Reconciler polls providers that expose Status and resolves redemptions left
// in delivering after an ambiguous network outcome. It is deliberately
// separate from the request path so a provider outage cannot block new API
// requests indefinitely.
type Reconciler struct {
	db       *pgxpool.Pool
	provider ProviderResolver
	ledger   *points.Ledger
	logger   *slog.Logger
	interval time.Duration
	now      func() time.Time
}

type reconciliationItem struct {
	ID                string
	RewardID          string
	TenantID          string
	EnvironmentID     string
	ExternalUserID    string
	Payload           []byte
	Currency          *string
	Cost              *int64
	Attempts          int
	ProviderReference string
}

func NewReconciler(db *pgxpool.Pool, provider Provider, logger *slog.Logger) *Reconciler {
	return NewReconcilerWithResolver(db, StaticProviderResolver{Provider: provider}, logger)
}

func NewReconcilerWithResolver(db *pgxpool.Pool, provider ProviderResolver, logger *slog.Logger) *Reconciler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Reconciler{
		db:       db,
		provider: provider,
		ledger:   points.NewLedger(db),
		logger:   logger,
		interval: defaultReconciliationInterval,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// Start launches the reconciliation loop and returns immediately.
func (r *Reconciler) Start(ctx context.Context) {
	if r == nil || r.db == nil || r.provider == nil {
		return
	}
	go r.run(ctx)
}

func (r *Reconciler) run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := r.ReconcileOnce(ctx, maxReconciliationBatch); err != nil {
				r.logger.Error("reward reconciliation failed", "error", err)
			}
		}
	}
}

// ReconcileOnce claims a bounded batch and resolves each item. It is exported
// for operator-triggered jobs and deterministic integration tests.
func (r *Reconciler) ReconcileOnce(ctx context.Context, limit int) (int, error) {
	if r == nil || r.db == nil || r.provider == nil {
		return 0, nil
	}
	if limit < 1 {
		limit = maxReconciliationBatch
	}
	items, err := r.claim(ctx, limit)
	if err != nil {
		return 0, err
	}
	for _, item := range items {
		if err := r.reconcileItem(ctx, item); err != nil {
			r.logger.Error("reward reconciliation item failed", "redemption_id", item.ID, "error", err)
		}
	}
	return len(items), nil
}

func (r *Reconciler) claim(ctx context.Context, limit int) ([]reconciliationItem, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `
		SELECT d.id, d.reward_id, d.tenant_id, d.environment_id, d.external_user_id,
		       r.payload, c.code, r.cost_amount, d.reconciliation_attempts,
		       COALESCE(d.provider_reference, '')
		FROM redemptions d
		JOIN rewards r ON r.id = d.reward_id
		LEFT JOIN currencies c ON c.id = r.cost_currency_id
		WHERE d.status = 'delivering'
		  AND (d.next_reconciliation_at IS NULL OR d.next_reconciliation_at <= now())
		  AND (d.reconciliation_locked_until IS NULL OR d.reconciliation_locked_until <= now())
		ORDER BY d.updated_at ASC, d.id ASC
		LIMIT $1
		FOR UPDATE OF d SKIP LOCKED`, limit)
	if err != nil {
		return nil, err
	}
	items := make([]reconciliationItem, 0, limit)
	for rows.Next() {
		var item reconciliationItem
		if err := rows.Scan(&item.ID, &item.RewardID, &item.TenantID, &item.EnvironmentID, &item.ExternalUserID, &item.Payload, &item.Currency, &item.Cost, &item.Attempts, &item.ProviderReference); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for index := range items {
		item := &items[index]
		item.Attempts++
		_, err := tx.Exec(ctx, `
			UPDATE redemptions
			SET reconciliation_attempts = $2,
			    reconciliation_locked_until = now() + interval '5 minutes',
			    last_provider_check_at = now(),
			    last_provider_check_error = NULL
			WHERE id = $1`, item.ID, item.Attempts)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Reconciler) reconcileItem(ctx context.Context, item reconciliationItem) error {
	attemptID, err := r.startAttempt(ctx, item.ID)
	if err != nil {
		return err
	}
	provider, err := r.provider.Resolve(ctx, item.TenantID, item.EnvironmentID)
	if err != nil {
		return r.recordUnknownWithAttempt(ctx, item, attemptID, err)
	}
	statusProvider, ok := provider.(StatusProvider)
	if !ok {
		return r.recordUnsupported(ctx, item, attemptID, ErrReconciliationUnsupported)
	}
	var payload map[string]any
	if len(item.Payload) > 0 {
		if err := json.Unmarshal(item.Payload, &payload); err != nil {
			return r.recordUnknownWithAttempt(ctx, item, attemptID, fmt.Errorf("reward payload is invalid: %w", err))
		}
	}
	status, err := statusProvider.Status(ctx, DeliveryRequest{RedemptionID: item.ID, RewardID: item.RewardID, ExternalUserID: item.ExternalUserID, Payload: payload})
	if err != nil {
		if errors.Is(err, ErrReconciliationUnsupported) {
			return r.recordUnsupported(ctx, item, attemptID, err)
		}
		return r.recordUnknownWithAttempt(ctx, item, attemptID, err)
	}
	switch status.State {
	case "fulfilled":
		return r.recordFulfilled(ctx, item, attemptID, status.ProviderReference)
	case "failed":
		reason := status.FailureReason
		if reason == "" {
			reason = "provider reported delivery failure"
		}
		return r.recordFailed(ctx, item, attemptID, reason)
	case "pending":
		return r.recordPending(ctx, item, attemptID)
	default:
		return r.recordUnknownWithAttempt(ctx, item, attemptID, fmt.Errorf("provider returned unsupported state %q", status.State))
	}
}

func (r *Reconciler) startAttempt(ctx context.Context, redemptionID string) (int64, error) {
	var attemptID int64
	err := r.db.QueryRow(ctx, `
		INSERT INTO reward_delivery_attempts (redemption_id, status, kind)
		VALUES ($1, 'started', 'reconciliation') RETURNING id`, redemptionID).Scan(&attemptID)
	return attemptID, err
}

func (r *Reconciler) recordFulfilled(ctx context.Context, item reconciliationItem, attemptID int64, providerReference string) error {
	if providerReference == "" {
		providerReference = item.ProviderReference
	}
	if _, err := r.db.Exec(ctx, `UPDATE reward_delivery_attempts SET status = 'fulfilled', provider_state = 'fulfilled', provider_reference = NULLIF($2, ''), completed_at = now() WHERE id = $1`, attemptID, providerReference); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `
		UPDATE redemptions
		SET status = 'fulfilled', provider_reference = NULLIF($2, ''), failure_reason = NULL,
		    next_reconciliation_at = NULL, reconciliation_locked_until = NULL,
		    last_provider_check_error = NULL, updated_at = now()
		WHERE id = $1 AND status = 'delivering'`, item.ID, providerReference)
	return err
}

func (r *Reconciler) recordPending(ctx context.Context, item reconciliationItem, attemptID int64) error {
	if _, err := r.db.Exec(ctx, `UPDATE reward_delivery_attempts SET status = 'pending', provider_state = 'pending' WHERE id = $1`, attemptID); err != nil {
		return err
	}
	return r.scheduleNext(ctx, item, "")
}

func (r *Reconciler) recordUnknownWithAttempt(ctx context.Context, item reconciliationItem, attemptID int64, err error) error {
	if updateErr := r.db.QueryRow(ctx, `UPDATE reward_delivery_attempts SET status = 'failed', provider_state = 'unknown', error_message = $2, completed_at = now() WHERE id = $1 RETURNING id`, attemptID, err.Error()).Scan(&attemptID); updateErr != nil {
		return updateErr
	}
	return r.scheduleNext(ctx, item, err.Error())
}

func (r *Reconciler) recordUnsupported(ctx context.Context, item reconciliationItem, attemptID int64, err error) error {
	if _, updateErr := r.db.Exec(ctx, `UPDATE reward_delivery_attempts SET status = 'failed', provider_state = 'unsupported', error_message = $2, completed_at = now() WHERE id = $1`, attemptID, err.Error()); updateErr != nil {
		return updateErr
	}
	return r.scheduleNext(ctx, item, err.Error())
}

func (r *Reconciler) scheduleNext(ctx context.Context, item reconciliationItem, reason string) error {
	next := r.now().Add(reconciliationBackoff(item.Attempts))
	_, err := r.db.Exec(ctx, `
		UPDATE redemptions
		SET next_reconciliation_at = $2, reconciliation_locked_until = NULL,
		    last_provider_check_error = NULLIF($3, ''), updated_at = now()
		WHERE id = $1 AND status = 'delivering'`, item.ID, next, reason)
	return err
}

func (r *Reconciler) recordFailed(ctx context.Context, item reconciliationItem, attemptID int64, reason string) error {
	if _, err := r.db.Exec(ctx, `UPDATE reward_delivery_attempts SET status = 'failed', provider_state = 'failed', error_message = $2, completed_at = now() WHERE id = $1`, attemptID, reason); err != nil {
		return err
	}
	result, err := r.db.Exec(ctx, `
		UPDATE redemptions
		SET status = 'failed', failure_reason = $2, next_reconciliation_at = NULL,
		    reconciliation_locked_until = NULL, last_provider_check_error = NULL,
		    updated_at = now()
		WHERE id = $1 AND status = 'delivering'`, item.ID, reason)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 1 && item.Currency != nil && item.Cost != nil {
		_, err = r.ledger.Award(ctx, points.Scope{TenantID: item.TenantID, EnvironmentID: item.EnvironmentID}, item.ExternalUserID, *item.Currency, *item.Cost, "redemption:"+item.ID+":reconciliation_release", json.RawMessage(`{"source":"redemption_reconciliation_release"}`))
	}
	return err
}

func reconciliationBackoff(attempt int) time.Duration {
	switch {
	case attempt <= 0:
		return time.Minute
	case attempt == 1:
		return 5 * time.Minute
	case attempt == 2:
		return 15 * time.Minute
	default:
		return time.Hour
	}
}
