package progression

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
	ErrIdempotencyConflict = errors.New("xp idempotency key was already used with a different request")
	ErrInvalidXP           = errors.New("xp amount must be positive")
)

type Scope struct {
	TenantID      string
	EnvironmentID string
}

type XPEntry struct {
	ID             int64           `json:"id"`
	ExternalUserID string          `json:"external_user_id"`
	Amount         int64           `json:"amount"`
	TotalXP        int64           `json:"total_xp"`
	IdempotencyKey string          `json:"idempotency_key"`
	Provenance     json.RawMessage `json:"provenance"`
	CreatedAt      time.Time       `json:"created_at"`
}

type Level struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	EnvironmentID string    `json:"environment_id"`
	Level         int       `json:"level"`
	Name          string    `json:"name"`
	XPThreshold   int64     `json:"xp_threshold"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
}

type UserProgression struct {
	UserID      string `json:"user_id"`
	TotalXP     int64  `json:"total_xp"`
	Level       int    `json:"level"`
	LevelName   string `json:"level_name"`
	NextLevelXP *int64 `json:"next_level_xp,omitempty"`
}

func AwardXP(ctx context.Context, db *pgxpool.Pool, scope Scope, userID string, amount int64, idempotencyKey string, provenance json.RawMessage) (XPEntry, error) {
	if strings.TrimSpace(scope.TenantID) == "" || strings.TrimSpace(scope.EnvironmentID) == "" || strings.TrimSpace(userID) == "" || amount <= 0 || strings.TrimSpace(idempotencyKey) == "" {
		return XPEntry{}, ErrInvalidXP
	}
	if len(provenance) == 0 {
		provenance = json.RawMessage(`{}`)
	}
	var object map[string]any
	if err := json.Unmarshal(provenance, &object); err != nil || object == nil {
		return XPEntry{}, fmt.Errorf("provenance must be a JSON object")
	}
	canonical, err := json.Marshal(object)
	if err != nil {
		return XPEntry{}, err
	}
	requestBytes, _ := json.Marshal(struct {
		Amount     int64  `json:"amount"`
		Provenance []byte `json:"provenance"`
	}{amount, canonical})
	hash := sha256.Sum256(requestBytes)
	tx, err := db.Begin(ctx)
	if err != nil {
		return XPEntry{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var entry XPEntry
	var existingHash []byte
	err = tx.QueryRow(ctx, `
		SELECT id, external_user_id, amount, idempotency_key, provenance, created_at, request_hash
		FROM xp_events WHERE tenant_id = $1 AND environment_id = $2 AND external_user_id = $3 AND idempotency_key = $4`, scope.TenantID, scope.EnvironmentID, userID, idempotencyKey).
		Scan(&entry.ID, &entry.ExternalUserID, &entry.Amount, &entry.IdempotencyKey, &entry.Provenance, &entry.CreatedAt, &existingHash)
	if err == nil {
		if !bytes.Equal(existingHash, hash[:]) {
			return XPEntry{}, ErrIdempotencyConflict
		}
		if err := tx.QueryRow(ctx, `SELECT total_xp FROM user_xp WHERE tenant_id = $1 AND environment_id = $2 AND external_user_id = $3`, scope.TenantID, scope.EnvironmentID, userID).Scan(&entry.TotalXP); err != nil {
			return XPEntry{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return XPEntry{}, err
		}
		return entry, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return XPEntry{}, err
	}
	if err := tx.QueryRow(ctx, `INSERT INTO xp_events (tenant_id, environment_id, external_user_id, amount, idempotency_key, request_hash, provenance) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb) RETURNING id, created_at`, scope.TenantID, scope.EnvironmentID, userID, amount, idempotencyKey, hash[:], canonical).Scan(&entry.ID, &entry.CreatedAt); err != nil {
		return XPEntry{}, err
	}
	entry.ExternalUserID, entry.Amount, entry.IdempotencyKey, entry.Provenance = userID, amount, idempotencyKey, canonical
	if err := tx.QueryRow(ctx, `
		INSERT INTO user_xp (tenant_id, environment_id, external_user_id, total_xp)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (tenant_id, environment_id, external_user_id) DO UPDATE SET total_xp = user_xp.total_xp + EXCLUDED.total_xp, updated_at = now()
		RETURNING total_xp`, scope.TenantID, scope.EnvironmentID, userID, amount).Scan(&entry.TotalXP); err != nil {
		return XPEntry{}, err
	}
	if err := leaderboards.RecordXP(ctx, tx, scope.TenantID, scope.EnvironmentID, userID, amount, entry.CreatedAt); err != nil {
		return XPEntry{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return XPEntry{}, err
	}
	return entry, nil
}

func LoadUserProgression(ctx context.Context, db *pgxpool.Pool, scope Scope, userID string) (UserProgression, error) {
	var progression UserProgression
	progression.UserID = userID
	if err := db.QueryRow(ctx, `SELECT COALESCE(total_xp, 0) FROM user_xp WHERE tenant_id = $1 AND environment_id = $2 AND external_user_id = $3`, scope.TenantID, scope.EnvironmentID, userID).Scan(&progression.TotalXP); errors.Is(err, pgx.ErrNoRows) {
		progression.TotalXP = 0
	} else if err != nil {
		return UserProgression{}, err
	}
	if err := db.QueryRow(ctx, `SELECT level, name FROM level_definitions WHERE tenant_id = $1 AND environment_id = $2 AND active = true AND xp_threshold <= $3 ORDER BY xp_threshold DESC, level DESC LIMIT 1`, scope.TenantID, scope.EnvironmentID, progression.TotalXP).Scan(&progression.Level, &progression.LevelName); errors.Is(err, pgx.ErrNoRows) {
		progression.Level, progression.LevelName = 0, "Unranked"
	} else if err != nil {
		return UserProgression{}, err
	}
	var next int64
	if err := db.QueryRow(ctx, `SELECT xp_threshold FROM level_definitions WHERE tenant_id = $1 AND environment_id = $2 AND active = true AND xp_threshold > $3 ORDER BY xp_threshold, level LIMIT 1`, scope.TenantID, scope.EnvironmentID, progression.TotalXP).Scan(&next); errors.Is(err, pgx.ErrNoRows) {
		progression.NextLevelXP = nil
	} else if err != nil {
		return UserProgression{}, err
	} else {
		progression.NextLevelXP = &next
	}
	return progression, nil
}
