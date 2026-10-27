package points

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/leaderboards"
)

var (
	ErrIdempotencyConflict = errors.New("idempotency key was already used with a different request")
	ErrInsufficientBalance = errors.New("insufficient point balance")
	ErrCurrencyNotFound    = errors.New("currency was not found")
)

type Scope struct {
	TenantID      string
	EnvironmentID string
}

type LedgerEntry struct {
	ID             int64           `json:"id"`
	AccountID      int64           `json:"account_id"`
	CurrencyID     string          `json:"currency_id"`
	CurrencyCode   string          `json:"currency_code"`
	ExternalUserID string          `json:"external_user_id"`
	EntryType      string          `json:"entry_type"`
	Amount         int64           `json:"amount"`
	BalanceAfter   int64           `json:"balance_after"`
	IdempotencyKey string          `json:"idempotency_key"`
	Provenance     json.RawMessage `json:"provenance"`
	CreatedAt      time.Time       `json:"created_at"`
}

type Ledger struct{ db *pgxpool.Pool }

func NewLedger(db *pgxpool.Pool) *Ledger { return &Ledger{db: db} }

func (l *Ledger) Award(ctx context.Context, scope Scope, userID, currencyCode string, amount int64, idempotencyKey string, provenance json.RawMessage) (LedgerEntry, error) {
	return l.apply(ctx, scope, userID, currencyCode, amount, "award", idempotencyKey, provenance, true)
}

// Release returns points held for a redemption that did not complete. It is
// recorded as an award but is not new earning, so leaderboards ignore it.
func (l *Ledger) Release(ctx context.Context, scope Scope, userID, currencyCode string, amount int64, idempotencyKey string, provenance json.RawMessage) (LedgerEntry, error) {
	return l.apply(ctx, scope, userID, currencyCode, amount, "award", idempotencyKey, provenance, false)
}

func (l *Ledger) Spend(ctx context.Context, scope Scope, userID, currencyCode string, amount int64, idempotencyKey string, provenance json.RawMessage) (LedgerEntry, error) {
	if amount <= 0 {
		return LedgerEntry{}, fmt.Errorf("spend amount must be positive")
	}
	return l.apply(ctx, scope, userID, currencyCode, -amount, "spend", idempotencyKey, provenance, false)
}

func (l *Ledger) apply(ctx context.Context, scope Scope, userID, currencyCode string, delta int64, entryType, idempotencyKey string, provenance json.RawMessage, countsAsEarning bool) (LedgerEntry, error) {
	if userID == "" || strings.TrimSpace(currencyCode) == "" || idempotencyKey == "" || delta == 0 {
		return LedgerEntry{}, fmt.Errorf("user, currency, amount, and idempotency_key are required")
	}
	if delta < 0 && entryType == "award" {
		return LedgerEntry{}, fmt.Errorf("award amount must be positive")
	}
	if len(provenance) == 0 {
		provenance = json.RawMessage(`{}`)
	}
	var provenanceObject map[string]any
	if err := json.Unmarshal(provenance, &provenanceObject); err != nil || provenanceObject == nil {
		return LedgerEntry{}, fmt.Errorf("provenance must be a JSON object")
	}
	canonicalProvenance, err := json.Marshal(provenanceObject)
	if err != nil {
		return LedgerEntry{}, err
	}
	requestBytes, _ := json.Marshal(struct {
		CurrencyCode string `json:"currency_code"`
		Delta        int64  `json:"delta"`
		EntryType    string `json:"entry_type"`
		Provenance   []byte `json:"provenance"`
	}{currencyCode, delta, entryType, canonicalProvenance})
	requestHash := sha256.Sum256(requestBytes)

	tx, err := l.db.Begin(ctx)
	if err != nil {
		return LedgerEntry{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var currencyID string
	var expiryDays *int
	if err := tx.QueryRow(ctx, `
		SELECT c.id, NULLIF((p.policy->>'expiry_days')::integer, 0)
		FROM currencies c
		JOIN currency_policy_versions p ON p.currency_id = c.id AND p.version = c.policy_version
		WHERE c.tenant_id = $1 AND c.environment_id = $2 AND c.code = $3 AND c.active = true`, scope.TenantID, scope.EnvironmentID, currencyCode).Scan(&currencyID, &expiryDays); errors.Is(err, pgx.ErrNoRows) {
		return LedgerEntry{}, ErrCurrencyNotFound
	} else if err != nil {
		return LedgerEntry{}, err
	}
	var accountID, balance int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO point_accounts (tenant_id, environment_id, currency_id, external_user_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, environment_id, currency_id, external_user_id) DO UPDATE SET updated_at = now()
		RETURNING id, available_balance`, scope.TenantID, scope.EnvironmentID, currencyID, userID).Scan(&accountID, &balance); err != nil {
		return LedgerEntry{}, err
	}
	if err := tx.QueryRow(ctx, `SELECT available_balance FROM point_accounts WHERE id = $1 FOR UPDATE`, accountID).Scan(&balance); err != nil {
		return LedgerEntry{}, err
	}
	var existing LedgerEntry
	var existingHash []byte
	err = tx.QueryRow(ctx, `
		SELECT e.id, e.account_id, c.id, c.code, a.external_user_id, e.entry_type, e.amount, e.balance_after, e.idempotency_key, e.provenance, e.created_at, e.request_hash
		FROM point_ledger_entries e
		JOIN point_accounts a ON a.id = e.account_id
		JOIN currencies c ON c.id = a.currency_id
		WHERE e.account_id = $1 AND e.idempotency_key = $2`, accountID, idempotencyKey).
		Scan(&existing.ID, &existing.AccountID, &existing.CurrencyID, &existing.CurrencyCode, &existing.ExternalUserID, &existing.EntryType, &existing.Amount, &existing.BalanceAfter, &existing.IdempotencyKey, &existing.Provenance, &existing.CreatedAt, &existingHash)
	if err == nil {
		if !bytes.Equal(existingHash, requestHash[:]) {
			return LedgerEntry{}, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return LedgerEntry{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return LedgerEntry{}, err
	}
	if delta < 0 && balance < -delta {
		return LedgerEntry{}, ErrInsufficientBalance
	}
	newBalance := balance + delta
	if newBalance < 0 {
		return LedgerEntry{}, ErrInsufficientBalance
	}
	if _, err := tx.Exec(ctx, `UPDATE point_accounts SET available_balance = $2, updated_at = now() WHERE id = $1`, accountID, newBalance); err != nil {
		return LedgerEntry{}, err
	}
	var entry LedgerEntry
	if err := tx.QueryRow(ctx, `
		INSERT INTO point_ledger_entries (account_id, entry_type, amount, balance_after, idempotency_key, request_hash, provenance)
			VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb)
			RETURNING id, account_id, entry_type, amount, balance_after, idempotency_key, provenance, created_at`, accountID, entryType, delta, newBalance, idempotencyKey, requestHash[:], canonicalProvenance).
		Scan(&entry.ID, &entry.AccountID, &entry.EntryType, &entry.Amount, &entry.BalanceAfter, &entry.IdempotencyKey, &entry.Provenance, &entry.CreatedAt); err != nil {
		return LedgerEntry{}, err
	}
	entry.CurrencyID = currencyID
	entry.CurrencyCode = currencyCode
	entry.ExternalUserID = userID
	if entryType == "award" {
		var expiresAt *time.Time
		if expiryDays != nil {
			value := time.Now().UTC().AddDate(0, 0, *expiryDays)
			expiresAt = &value
		}
		if _, err := tx.Exec(ctx, `INSERT INTO point_lots (account_id, award_entry_id, original_amount, remaining_amount, expires_at) VALUES ($1, $2, $3, $3, $4)`, accountID, entry.ID, delta, expiresAt); err != nil {
			return LedgerEntry{}, err
		}
		if countsAsEarning {
			if err := leaderboards.RecordPoints(ctx, tx, scope.TenantID, scope.EnvironmentID, currencyID, userID, delta, entry.CreatedAt); err != nil {
				return LedgerEntry{}, err
			}
		}
	} else if delta < 0 {
		if err := allocateLots(ctx, tx, accountID, entry.ID, -delta); err != nil {
			return LedgerEntry{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return LedgerEntry{}, err
	}
	return entry, nil
}

func allocateLots(ctx context.Context, tx pgx.Tx, accountID, ledgerEntryID int64, amount int64) error {
	rows, err := tx.Query(ctx, `SELECT id, remaining_amount FROM point_lots WHERE account_id = $1 AND remaining_amount > 0 ORDER BY expires_at NULLS LAST, id FOR UPDATE`, accountID)
	if err != nil {
		return err
	}
	lots := make([]struct{ id, remaining int64 }, 0)
	for rows.Next() {
		var lot struct{ id, remaining int64 }
		if err := rows.Scan(&lot.id, &lot.remaining); err != nil {
			rows.Close()
			return err
		}
		lots = append(lots, lot)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	remaining := amount
	for _, lot := range lots {
		if remaining <= 0 {
			break
		}
		lotID, available := lot.id, lot.remaining
		allocated := available
		if allocated > remaining {
			allocated = remaining
		}
		if _, err := tx.Exec(ctx, `UPDATE point_lots SET remaining_amount = remaining_amount - $2 WHERE id = $1`, lotID, allocated); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO point_lot_allocations (lot_id, ledger_entry_id, amount) VALUES ($1, $2, $3)`, lotID, ledgerEntryID, allocated); err != nil {
			return err
		}
		remaining -= allocated
	}
	if remaining > 0 {
		return fmt.Errorf("point lots could not cover ledger entry")
	}
	return nil
}
