package points

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Reconciliation struct {
	AccountID        int64 `json:"account_id"`
	LedgerBalance    int64 `json:"ledger_balance"`
	ProjectedBalance int64 `json:"projected_balance"`
	Matches          bool  `json:"matches"`
}

func Expire(ctx context.Context, db *pgxpool.Pool, before time.Time, limit int) (int, error) {
	if limit < 1 || limit > 1000 {
		return 0, fmt.Errorf("limit must be between 1 and 1000")
	}
	count := 0
	for count < limit {
		if err := expireOne(ctx, db, before); err != nil {
			if err == pgx.ErrNoRows {
				return count, nil
			}
			return count, err
		}
		count++
	}
	return count, nil
}

func expireOne(ctx context.Context, db *pgxpool.Pool, before time.Time) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var lotID, accountID, amount int64
	if err := tx.QueryRow(ctx, `SELECT id, account_id, remaining_amount FROM point_lots WHERE remaining_amount > 0 AND expires_at IS NOT NULL AND expires_at <= $1 ORDER BY expires_at, id FOR UPDATE SKIP LOCKED LIMIT 1`, before).Scan(&lotID, &accountID, &amount); err != nil {
		return err
	}
	var balance int64
	if err := tx.QueryRow(ctx, `SELECT available_balance FROM point_accounts WHERE id = $1 FOR UPDATE`, accountID).Scan(&balance); err != nil {
		return err
	}
	if balance < amount {
		amount = balance
	}
	if amount <= 0 {
		return fmt.Errorf("account balance is lower than expiring lot")
	}
	var entryID int64
	provenance, _ := json.Marshal(map[string]any{"lot_id": lotID, "reason": "expired"})
	idempotencyKey := fmt.Sprintf("expiration:lot:%d", lotID)
	hash := sha256.Sum256([]byte(idempotencyKey))
	if err := tx.QueryRow(ctx, `INSERT INTO point_ledger_entries (account_id, entry_type, amount, balance_after, idempotency_key, request_hash, provenance) VALUES ($1, 'expiration', $2, $3, $4, $5, $6::jsonb) RETURNING id`, accountID, -amount, balance-amount, idempotencyKey, hash[:], provenance).Scan(&entryID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE point_accounts SET available_balance = available_balance - $2, updated_at = now() WHERE id = $1`, accountID, amount); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE point_lots SET remaining_amount = 0 WHERE id = $1`, lotID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO point_lot_allocations (lot_id, ledger_entry_id, amount) VALUES ($1, $2, $3)`, lotID, entryID, amount); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func Reconcile(ctx context.Context, db *pgxpool.Pool, scope Scope, userID, currencyCode string) (Reconciliation, error) {
	var result Reconciliation
	err := db.QueryRow(ctx, `
		SELECT a.id, COALESCE((SELECT sum(e.amount) FROM point_ledger_entries e WHERE e.account_id = a.id), 0), a.available_balance
		FROM point_accounts a JOIN currencies c ON c.id = a.currency_id
		WHERE a.tenant_id = $1 AND a.environment_id = $2 AND a.external_user_id = $3 AND c.code = $4`, scope.TenantID, scope.EnvironmentID, userID, currencyCode).Scan(&result.AccountID, &result.LedgerBalance, &result.ProjectedBalance)
	if err != nil {
		return Reconciliation{}, err
	}
	result.Matches = result.LedgerBalance == result.ProjectedBalance
	return result, nil
}
