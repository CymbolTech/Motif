package points

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConversionRequest struct {
	Scope           Scope
	ExternalUserID  string
	SourceCurrency  string
	TargetCurrency  string
	Amount          int64
	RateNumerator   int64
	RateDenominator int64
	IdempotencyKey  string
}

type Conversion struct {
	ID              string    `json:"id"`
	ExternalUserID  string    `json:"external_user_id"`
	SourceCurrency  string    `json:"source_currency"`
	TargetCurrency  string    `json:"target_currency"`
	SourceAmount    int64     `json:"source_amount"`
	TargetAmount    int64     `json:"target_amount"`
	RateNumerator   int64     `json:"rate_numerator"`
	RateDenominator int64     `json:"rate_denominator"`
	IdempotencyKey  string    `json:"idempotency_key"`
	CreatedAt       time.Time `json:"created_at"`
}

func Convert(ctx context.Context, db *pgxpool.Pool, request ConversionRequest) (Conversion, error) {
	if request.ExternalUserID == "" || request.SourceCurrency == "" || request.TargetCurrency == "" || request.Amount <= 0 || request.RateNumerator <= 0 || request.RateDenominator <= 0 || request.IdempotencyKey == "" {
		return Conversion{}, fmt.Errorf("source, target, positive amount, positive rate, user, and idempotency_key are required")
	}
	if request.SourceCurrency == request.TargetCurrency {
		return Conversion{}, fmt.Errorf("source and target currencies must differ")
	}
	product := new(big.Int).Mul(big.NewInt(request.Amount), big.NewInt(request.RateNumerator))
	target := new(big.Int).Quo(product, big.NewInt(request.RateDenominator))
	if !target.IsInt64() || target.Sign() <= 0 {
		return Conversion{}, fmt.Errorf("conversion result is outside the supported integer range")
	}
	targetAmount := target.Int64()
	requestBytes, _ := json.Marshal(request)
	hash := sha256.Sum256(requestBytes)
	tx, err := db.Begin(ctx)
	if err != nil {
		return Conversion{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Serialize requests for the same idempotency key before checking for an
	// existing conversion. Account locks alone cannot cover requests that use
	// the same key with different source or target currencies.
	keyBytes, _ := json.Marshal([4]string{request.Scope.TenantID, request.Scope.EnvironmentID, request.ExternalUserID, request.IdempotencyKey})
	keyHash := sha256.Sum256(keyBytes)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(binary.BigEndian.Uint64(keyHash[:8]))); err != nil {
		return Conversion{}, err
	}
	var existing Conversion
	var existingHash []byte
	err = tx.QueryRow(ctx, `
		SELECT x.id, x.external_user_id, sc.code, tc.code, x.source_amount, x.target_amount, x.rate_numerator, x.rate_denominator, x.idempotency_key, x.created_at, x.request_hash
		FROM point_conversions x
		JOIN currencies sc ON sc.id = x.source_currency_id
		JOIN currencies tc ON tc.id = x.target_currency_id
		WHERE x.tenant_id = $1 AND x.environment_id = $2 AND x.external_user_id = $3 AND x.idempotency_key = $4
		FOR UPDATE`, request.Scope.TenantID, request.Scope.EnvironmentID, request.ExternalUserID, request.IdempotencyKey).
		Scan(&existing.ID, &existing.ExternalUserID, &existing.SourceCurrency, &existing.TargetCurrency, &existing.SourceAmount, &existing.TargetAmount, &existing.RateNumerator, &existing.RateDenominator, &existing.IdempotencyKey, &existing.CreatedAt, &existingHash)
	if err == nil {
		if string(existingHash) != string(hash[:]) {
			return Conversion{}, ErrIdempotencyConflict
		}
		if err := tx.Commit(ctx); err != nil {
			return Conversion{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Conversion{}, err
	}
	var sourceID, targetID string
	var targetExpiryDays *int
	if err := tx.QueryRow(ctx, `SELECT id FROM currencies WHERE tenant_id = $1 AND environment_id = $2 AND code = $3 AND active = true`, request.Scope.TenantID, request.Scope.EnvironmentID, request.SourceCurrency).Scan(&sourceID); errors.Is(err, pgx.ErrNoRows) {
		return Conversion{}, ErrCurrencyNotFound
	} else if err != nil {
		return Conversion{}, err
	}
	if err := tx.QueryRow(ctx, `
		SELECT c.id, NULLIF((p.policy->>'expiry_days')::integer, 0)
		FROM currencies c
		JOIN currency_policy_versions p ON p.currency_id = c.id AND p.version = c.policy_version
		WHERE c.tenant_id = $1 AND c.environment_id = $2 AND c.code = $3 AND c.active = true`, request.Scope.TenantID, request.Scope.EnvironmentID, request.TargetCurrency).Scan(&targetID, &targetExpiryDays); errors.Is(err, pgx.ErrNoRows) {
		return Conversion{}, ErrCurrencyNotFound
	} else if err != nil {
		return Conversion{}, err
	}
	accounts, err := ensureLockedAccounts(ctx, tx, request.Scope, request.ExternalUserID, sourceID, targetID)
	if err != nil {
		return Conversion{}, err
	}
	if accounts[0].balance < request.Amount {
		return Conversion{}, ErrInsufficientBalance
	}
	conversionID, err := generateConversionID()
	if err != nil {
		return Conversion{}, err
	}
	debitBalance := accounts[0].balance - request.Amount
	creditBalance := accounts[1].balance + targetAmount
	if _, err := tx.Exec(ctx, `UPDATE point_accounts SET available_balance = $2, updated_at = now() WHERE id = $1`, accounts[0].id, debitBalance); err != nil {
		return Conversion{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE point_accounts SET available_balance = $2, updated_at = now() WHERE id = $1`, accounts[1].id, creditBalance); err != nil {
		return Conversion{}, err
	}
	debitKey := "conversion:" + conversionID + ":debit"
	creditKey := "conversion:" + conversionID + ":credit"
	var debitEntryID int64
	if err := tx.QueryRow(ctx, `INSERT INTO point_ledger_entries (account_id, entry_type, amount, balance_after, idempotency_key, request_hash, provenance) VALUES ($1, 'conversion_debit', $2, $3, $4, $5, $6::jsonb) RETURNING id`, accounts[0].id, -request.Amount, debitBalance, debitKey, hash[:], `{"conversion_id":"`+conversionID+`"}`).Scan(&debitEntryID); err != nil {
		return Conversion{}, err
	}
	if err := allocateLots(ctx, tx, accounts[0].id, debitEntryID, request.Amount); err != nil {
		return Conversion{}, err
	}
	var creditEntryID int64
	if err := tx.QueryRow(ctx, `INSERT INTO point_ledger_entries (account_id, entry_type, amount, balance_after, idempotency_key, request_hash, provenance) VALUES ($1, 'conversion_credit', $2, $3, $4, $5, $6::jsonb) RETURNING id`, accounts[1].id, targetAmount, creditBalance, creditKey, hash[:], `{"conversion_id":"`+conversionID+`"}`).Scan(&creditEntryID); err != nil {
		return Conversion{}, err
	}
	var expiresAt *time.Time
	if targetExpiryDays != nil {
		value := time.Now().UTC().AddDate(0, 0, *targetExpiryDays)
		expiresAt = &value
	}
	if _, err := tx.Exec(ctx, `INSERT INTO point_lots (account_id, award_entry_id, original_amount, remaining_amount, expires_at) VALUES ($1, $2, $3, $3, $4)`, accounts[1].id, creditEntryID, targetAmount, expiresAt); err != nil {
		return Conversion{}, err
	}
	var conversion Conversion
	if err := tx.QueryRow(ctx, `
		INSERT INTO point_conversions (id, tenant_id, environment_id, external_user_id, source_currency_id, target_currency_id, source_amount, target_amount, rate_numerator, rate_denominator, idempotency_key, request_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, external_user_id, source_amount, target_amount, rate_numerator, rate_denominator, idempotency_key, created_at`, conversionID, request.Scope.TenantID, request.Scope.EnvironmentID, request.ExternalUserID, sourceID, targetID, request.Amount, targetAmount, request.RateNumerator, request.RateDenominator, request.IdempotencyKey, hash[:]).Scan(&conversion.ID, &conversion.ExternalUserID, &conversion.SourceAmount, &conversion.TargetAmount, &conversion.RateNumerator, &conversion.RateDenominator, &conversion.IdempotencyKey, &conversion.CreatedAt); err != nil {
		return Conversion{}, err
	}
	conversion.SourceCurrency = request.SourceCurrency
	conversion.TargetCurrency = request.TargetCurrency
	if err := tx.Commit(ctx); err != nil {
		return Conversion{}, err
	}
	return conversion, nil
}

type lockedAccount struct{ id, balance int64 }

func ensureLockedAccounts(ctx context.Context, tx pgx.Tx, scope Scope, userID, sourceID, targetID string) ([2]lockedAccount, error) {
	for _, currencyID := range []string{sourceID, targetID} {
		if _, err := tx.Exec(ctx, `INSERT INTO point_accounts (tenant_id, environment_id, currency_id, external_user_id) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING`, scope.TenantID, scope.EnvironmentID, currencyID, userID); err != nil {
			return [2]lockedAccount{}, err
		}
	}
	rows, err := tx.Query(ctx, `SELECT id, currency_id, available_balance FROM point_accounts WHERE tenant_id = $1 AND environment_id = $2 AND external_user_id = $3 AND currency_id IN ($4, $5) ORDER BY currency_id FOR UPDATE`, scope.TenantID, scope.EnvironmentID, userID, sourceID, targetID)
	if err != nil {
		return [2]lockedAccount{}, err
	}
	defer rows.Close()
	var source, target lockedAccount
	for rows.Next() {
		var id, balance int64
		var currencyID string
		if err := rows.Scan(&id, &currencyID, &balance); err != nil {
			return [2]lockedAccount{}, err
		}
		if currencyID == sourceID {
			source = lockedAccount{id: id, balance: balance}
		} else {
			target = lockedAccount{id: id, balance: balance}
		}
	}
	if err := rows.Err(); err != nil {
		return [2]lockedAccount{}, err
	}
	return [2]lockedAccount{source, target}, nil
}

func generateConversionID() (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return fmt.Sprintf("conversion_%x", value), nil
}
