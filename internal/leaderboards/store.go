package leaderboards

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Scope struct {
	TenantID      string
	EnvironmentID string
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

const boardSelect = `
	SELECT l.id, l.tenant_id, l.environment_id, l.key, l.name, l.description, l.metric_type, COALESCE(l.currency_id, ''), COALESCE(c.code, ''),
		l.schedule_type, COALESCE(l.interval_unit, ''), COALESCE(l.interval_count, 0), l.anchor_at, l.timezone, l.active, l.created_at, l.updated_at
	FROM leaderboards l
	LEFT JOIN currencies c ON c.id = l.currency_id`

const roundColumns = `id, number, name, starts_at, ends_at, created_by, created_at`

func scanBoard(row pgx.Row) (Leaderboard, error) {
	var board Leaderboard
	err := row.Scan(&board.ID, &board.TenantID, &board.EnvironmentID, &board.Key, &board.Name, &board.Description, &board.Metric.Type, &board.Metric.CurrencyID, &board.Metric.Currency,
		&board.Schedule.Type, &board.Schedule.Every, &board.Schedule.Interval, &board.Schedule.Anchor, &board.Schedule.Timezone, &board.Active, &board.CreatedAt, &board.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Leaderboard{}, ErrNotFound
	}
	return board, err
}

func scanRound(row pgx.Row, now time.Time) (*Round, error) {
	var round Round
	if err := row.Scan(&round.ID, &round.Number, &round.Name, &round.StartsAt, &round.EndsAt, &round.CreatedBy, &round.CreatedAt); err != nil {
		return nil, err
	}
	roundStatus(&round, now)
	return &round, nil
}

// optionalRound scans a single round and maps "no row" to nil.
func optionalRound(row pgx.Row, now time.Time) (*Round, error) {
	round, err := scanRound(row, now)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return round, err
}

func newID(prefix string) (string, error) {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return prefix + base64.RawURLEncoding.EncodeToString(value), nil
}

// resolveNow returns now, or the transaction's database time when now is
// zero. Ledger entries are stamped with database time, so handlers pass zero
// to keep round boundaries and award times on the same clock.
func resolveNow(ctx context.Context, tx pgx.Tx, now time.Time) (time.Time, error) {
	if !now.IsZero() {
		return now, nil
	}
	err := tx.QueryRow(ctx, `SELECT now()`).Scan(&now)
	return now.UTC(), err
}

// inTx runs fn in a transaction after resolving *now, so fn sees the time
// the transaction uses.
func inTx[T any](ctx context.Context, db *pgxpool.Pool, now *time.Time, fn func(pgx.Tx) (T, error)) (T, error) {
	var zero T
	tx, err := db.Begin(ctx)
	if err != nil {
		return zero, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if *now, err = resolveNow(ctx, tx, *now); err != nil {
		return zero, err
	}
	value, err := fn(tx)
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}
	return value, nil
}

// RecordPoints adds a points award to every active leaderboard that ranks
// the currency. It runs inside the ledger transaction, after the idempotency
// check, so a replayed award never counts twice.
func RecordPoints(ctx context.Context, tx pgx.Tx, tenantID, environmentID, currencyID, userID string, amount int64, at time.Time) error {
	return record(ctx, tx, tenantID, environmentID, MetricPoints, &currencyID, userID, amount, at)
}

// RecordXP adds an XP award to every active XP leaderboard.
func RecordXP(ctx context.Context, tx pgx.Tx, tenantID, environmentID, userID string, amount int64, at time.Time) error {
	return record(ctx, tx, tenantID, environmentID, MetricXP, nil, userID, amount, at)
}

func record(ctx context.Context, tx pgx.Tx, tenantID, environmentID, metric string, currencyID *string, userID string, amount int64, at time.Time) error {
	if amount <= 0 {
		return nil
	}
	rows, err := tx.Query(ctx, boardSelect+`
		WHERE l.tenant_id = $1 AND l.environment_id = $2 AND l.active = true AND l.metric_type = $3 AND l.currency_id IS NOT DISTINCT FROM $4
		ORDER BY l.id`, tenantID, environmentID, metric, currencyID)
	if err != nil {
		return fmt.Errorf("leaderboard lookup: %w", err)
	}
	boards, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Leaderboard, error) { return scanBoard(row) })
	if err != nil {
		return fmt.Errorf("leaderboard lookup: %w", err)
	}
	for _, board := range boards {
		round, err := currentRound(ctx, tx, board, at, true)
		if err != nil {
			return fmt.Errorf("leaderboard %s round: %w", board.ID, err)
		}
		if round == nil {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO leaderboard_scores (round_id, external_user_id, score, reached_at)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (round_id, external_user_id) DO UPDATE
			SET score = leaderboard_scores.score + EXCLUDED.score, reached_at = GREATEST(leaderboard_scores.reached_at, EXCLUDED.reached_at)`, round.ID, userID, amount, at); err != nil {
			return fmt.Errorf("leaderboard %s score: %w", board.ID, err)
		}
	}
	return nil
}

func activeRound(ctx context.Context, q querier, boardID string, at time.Time) (*Round, error) {
	return optionalRound(q.QueryRow(ctx, `SELECT `+roundColumns+` FROM leaderboard_rounds
		WHERE leaderboard_id = $1 AND starts_at <= $2 AND (ends_at IS NULL OR ends_at > $2)
		ORDER BY starts_at DESC LIMIT 1`, boardID, at), at)
}

// lockBoard locks a board found by id or key, so callers can name a board
// the way they know it.
func lockBoard(ctx context.Context, tx pgx.Tx, scope Scope, idOrKey string) (Leaderboard, error) {
	return scanBoard(tx.QueryRow(ctx, boardSelect+` WHERE (l.id = $1 OR l.key = $1) AND l.tenant_id = $2 AND l.environment_id = $3 ORDER BY (l.id = $1) DESC LIMIT 1 FOR UPDATE OF l`, idOrKey, scope.TenantID, scope.EnvironmentID))
}

// currentRound returns the round running at the given time. For an active
// recurring board it creates that round the first time it is needed, so
// periods roll over without a background job.
func currentRound(ctx context.Context, tx pgx.Tx, board Leaderboard, at time.Time, create bool) (*Round, error) {
	round, err := activeRound(ctx, tx, board.ID, at)
	if err != nil || round != nil {
		return round, err
	}
	if !create || !board.Active || board.Schedule.Type != ScheduleRecurring {
		return nil, nil
	}
	board, err = lockBoard(ctx, tx, Scope{board.TenantID, board.EnvironmentID}, board.ID)
	if err != nil {
		return nil, err
	}
	if !board.Active || board.Schedule.Type != ScheduleRecurring {
		return nil, nil
	}
	if round, err := activeRound(ctx, tx, board.ID, at); err != nil || round != nil {
		return round, err
	}
	start, end, err := board.Schedule.Window(at)
	if err != nil {
		return nil, err
	}
	// The first round starts when the board was created, and a round never
	// starts before the previous one ended, for example after a board moved
	// from manual rounds to a schedule.
	var previousEnd *time.Time
	if err := tx.QueryRow(ctx, `SELECT ends_at FROM leaderboard_rounds WHERE leaderboard_id = $1 AND starts_at <= $2 ORDER BY starts_at DESC LIMIT 1`, board.ID, at).Scan(&previousEnd); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if previousEnd != nil && previousEnd.After(start) {
		start = *previousEnd
	}
	if board.CreatedAt.After(start) {
		start = board.CreatedAt
	}
	var nextStart *time.Time
	if err := tx.QueryRow(ctx, `SELECT min(starts_at) FROM leaderboard_rounds WHERE leaderboard_id = $1 AND starts_at > $2`, board.ID, at).Scan(&nextStart); err != nil {
		return nil, err
	}
	if nextStart != nil && nextStart.Before(end) {
		end = *nextStart
	}
	if at.Before(start) || !end.After(at) {
		return nil, nil
	}
	id, err := newID("round_")
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO leaderboard_rounds (id, leaderboard_id, number, name, starts_at, ends_at, created_by)
		SELECT $1, $2, n, 'Round ' || n, $3, $4, 'schedule'
		FROM (SELECT COALESCE(max(number), 0) + 1 AS n FROM leaderboard_rounds WHERE leaderboard_id = $2) next
		ON CONFLICT (leaderboard_id, starts_at) DO NOTHING`, id, board.ID, start, end); err != nil {
		return nil, err
	}
	return activeRound(ctx, tx, board.ID, at)
}

// latestRound is what a board shows when no round is running: the most
// recent round that has started.
func latestRound(ctx context.Context, q querier, boardID string, now time.Time) (*Round, error) {
	return optionalRound(q.QueryRow(ctx, `SELECT `+roundColumns+` FROM leaderboard_rounds WHERE leaderboard_id = $1 AND starts_at <= $2 ORDER BY starts_at DESC LIMIT 1`, boardID, now), now)
}

func displayRound(ctx context.Context, tx pgx.Tx, board Leaderboard, now time.Time) (*Round, error) {
	round, err := currentRound(ctx, tx, board, now, true)
	if err != nil || round != nil {
		return round, err
	}
	return latestRound(ctx, tx, board.ID, now)
}

// attachDisplayRounds resolves each board's round in id order. Creating a
// recurring round locks its board, and awards lock boards in the same order,
// so two transactions never wait on each other's boards.
func attachDisplayRounds(ctx context.Context, tx pgx.Tx, boards []Leaderboard, now time.Time) error {
	order := make([]int, len(boards))
	for index := range order {
		order[index] = index
	}
	sort.Slice(order, func(a, b int) bool { return boards[order[a]].ID < boards[order[b]].ID })
	for _, index := range order {
		round, err := displayRound(ctx, tx, boards[index], now)
		if err != nil {
			return err
		}
		boards[index].CurrentRound = round
	}
	return nil
}

// InsertBoard creates a validated board inside the caller's transaction so
// the caller can enforce plan limits in the same transaction.
func InsertBoard(ctx context.Context, tx pgx.Tx, request BoardRequest) (Leaderboard, error) {
	var currencyID *string
	if request.Metric.Type == MetricPoints {
		var id string
		err := tx.QueryRow(ctx, `SELECT id FROM currencies WHERE tenant_id = $1 AND environment_id = $2 AND code = $3`, request.TenantID, request.EnvironmentID, request.Metric.Currency).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return Leaderboard{}, ErrCurrencyNotFound
		}
		if err != nil {
			return Leaderboard{}, err
		}
		currencyID = &id
	}
	id, err := newID("leaderboard_")
	if err != nil {
		return Leaderboard{}, err
	}
	schedule := request.Schedule
	var unit *string
	var interval *int
	if schedule.Type == ScheduleRecurring {
		unit, interval = &schedule.Every, &schedule.Interval
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO leaderboards (id, tenant_id, environment_id, key, name, description, metric_type, currency_id, schedule_type, interval_unit, interval_count, anchor_at, timezone)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		id, request.TenantID, request.EnvironmentID, request.Key, request.Name, request.Description, request.Metric.Type, currencyID,
		schedule.Type, unit, interval, schedule.Anchor, schedule.Timezone); err != nil {
		return Leaderboard{}, err
	}
	return scanBoard(tx.QueryRow(ctx, boardSelect+` WHERE l.id = $1`, id))
}

// UpdateBoard renames a board or changes its schedule. A new schedule takes
// effect at the next boundary: a running round keeps its window, and an
// open manual round ends where the first scheduled round begins.
func UpdateBoard(ctx context.Context, db *pgxpool.Pool, scope Scope, boardID string, request BoardRequest, now time.Time) (Leaderboard, error) {
	return inTx(ctx, db, &now, func(tx pgx.Tx) (Leaderboard, error) {
		board, err := lockBoard(ctx, tx, scope, boardID)
		if err != nil {
			return Leaderboard{}, err
		}
		request, err := validateBoardUpdate(request, board, now)
		if err != nil {
			return Leaderboard{}, err
		}
		schedule := *request.Schedule
		if board.Schedule.Type == ScheduleManual && schedule.Type == ScheduleRecurring {
			var scheduled int
			if err := tx.QueryRow(ctx, `SELECT count(*) FROM leaderboard_rounds WHERE leaderboard_id = $1 AND starts_at > $2`, board.ID, now).Scan(&scheduled); err != nil {
				return Leaderboard{}, err
			}
			if scheduled > 0 {
				return Leaderboard{}, ErrScheduledRounds
			}
			_, boundary, err := schedule.Window(now)
			if err != nil {
				return Leaderboard{}, err
			}
			if _, err := tx.Exec(ctx, `UPDATE leaderboard_rounds SET ends_at = $3 WHERE leaderboard_id = $1 AND starts_at <= $2 AND ends_at IS NULL AND starts_at < $3`, board.ID, now, boundary); err != nil {
				return Leaderboard{}, err
			}
		}
		var unit *string
		var interval *int
		if schedule.Type == ScheduleRecurring {
			unit, interval = &schedule.Every, &schedule.Interval
		}
		if _, err := tx.Exec(ctx, `
			UPDATE leaderboards SET name = $2, description = $3, schedule_type = $4, interval_unit = $5, interval_count = $6, anchor_at = $7, timezone = $8, updated_at = now()
			WHERE id = $1`, board.ID, request.Name, request.Description, schedule.Type, unit, interval, schedule.Anchor, schedule.Timezone); err != nil {
			return Leaderboard{}, err
		}
		board, err = scanBoard(tx.QueryRow(ctx, boardSelect+` WHERE l.id = $1`, board.ID))
		if err != nil {
			return Leaderboard{}, err
		}
		board.CurrentRound, err = displayRound(ctx, tx, board, now)
		return board, err
	})
}

// DeleteBoard removes a board with its rounds and scores. It reports how many
// rounds went with it, for the audit trail.
func DeleteBoard(ctx context.Context, db *pgxpool.Pool, scope Scope, idOrKey string) (Leaderboard, int, error) {
	type deleted struct {
		board  Leaderboard
		rounds int
	}
	now := time.Time{}
	result, err := inTx(ctx, db, &now, func(tx pgx.Tx) (deleted, error) {
		board, err := lockBoard(ctx, tx, scope, idOrKey)
		if err != nil {
			return deleted{}, err
		}
		var rounds int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM leaderboard_rounds WHERE leaderboard_id = $1`, board.ID).Scan(&rounds); err != nil {
			return deleted{}, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM leaderboards WHERE id = $1`, board.ID); err != nil {
			return deleted{}, err
		}
		return deleted{board, rounds}, nil
	})
	return result.board, result.rounds, err
}

// ListBoards returns the boards of a scope with the round each one shows.
func ListBoards(ctx context.Context, db *pgxpool.Pool, scope Scope, activeOnly bool, now time.Time) ([]Leaderboard, error) {
	return inTx(ctx, db, &now, func(tx pgx.Tx) ([]Leaderboard, error) {
		rows, err := tx.Query(ctx, boardSelect+` WHERE l.tenant_id = $1 AND l.environment_id = $2 AND ($3 = false OR l.active = true) ORDER BY l.name, l.key`, scope.TenantID, scope.EnvironmentID, activeOnly)
		if err != nil {
			return nil, err
		}
		boards, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Leaderboard, error) { return scanBoard(row) })
		if err != nil {
			return nil, err
		}
		if err := attachDisplayRounds(ctx, tx, boards, now); err != nil {
			return nil, err
		}
		return boards, nil
	})
}

// FindBoard resolves a board by id or by key within a scope.
func FindBoard(ctx context.Context, q querier, scope Scope, idOrKey string) (Leaderboard, error) {
	return scanBoard(q.QueryRow(ctx, boardSelect+` WHERE l.tenant_id = $1 AND l.environment_id = $2 AND (l.id = $3 OR l.key = $3) ORDER BY (l.id = $3) DESC LIMIT 1`, scope.TenantID, scope.EnvironmentID, idOrKey))
}

// ListRounds returns a board's rounds, newest first, with each round's
// participant count and leader.
func ListRounds(ctx context.Context, db *pgxpool.Pool, board Leaderboard, limit int, now time.Time) ([]Round, error) {
	return inTx(ctx, db, &now, func(tx pgx.Tx) ([]Round, error) {
		// Materialize the running recurring round so the history includes it.
		if _, err := currentRound(ctx, tx, board, now, true); err != nil {
			return nil, err
		}
		rows, err := tx.Query(ctx, `
			SELECT r.id, r.number, r.name, r.starts_at, r.ends_at, r.created_by, r.created_at,
				(SELECT count(*) FROM leaderboard_scores s WHERE s.round_id = r.id),
				leader.external_user_id, leader.score, leader.reached_at
			FROM leaderboard_rounds r
			LEFT JOIN LATERAL (
				SELECT external_user_id, score, reached_at FROM leaderboard_scores s
				WHERE s.round_id = r.id ORDER BY score DESC, reached_at, external_user_id LIMIT 1
			) leader ON true
			WHERE r.leaderboard_id = $1
			ORDER BY r.starts_at DESC
			LIMIT $2`, board.ID, limit)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		result := make([]Round, 0)
		for rows.Next() {
			var round Round
			var participants int64
			var leaderID *string
			var leaderScore *int64
			var leaderReached *time.Time
			if err := rows.Scan(&round.ID, &round.Number, &round.Name, &round.StartsAt, &round.EndsAt, &round.CreatedBy, &round.CreatedAt, &participants, &leaderID, &leaderScore, &leaderReached); err != nil {
				return nil, err
			}
			roundStatus(&round, now)
			round.Participants = &participants
			if leaderID != nil {
				round.Leader = &Entry{Rank: 1, UserID: *leaderID, Score: *leaderScore, ReachedAt: *leaderReached}
			}
			result = append(result, round)
		}
		return result, rows.Err()
	})
}

// CreateRound adds a round to a manual board. With EndCurrent (the default)
// the running round ends exactly when the new one starts, so no award falls
// between rounds.
func CreateRound(ctx context.Context, db *pgxpool.Pool, scope Scope, boardID string, request RoundRequest, now time.Time) (Round, error) {
	request.Name = strings.TrimSpace(request.Name)
	if len(request.Name) > 120 {
		return Round{}, invalid("name must be at most 120 characters")
	}
	endCurrent := request.EndCurrent == nil || *request.EndCurrent
	return inTx(ctx, db, &now, func(tx pgx.Tx) (Round, error) {
		start := now
		if request.StartsAt != nil {
			if request.StartsAt.Before(now.Add(-time.Minute)) {
				return Round{}, invalid("starts_at cannot be in the past")
			}
			if request.StartsAt.After(now) {
				start = *request.StartsAt
			}
		}
		start = start.UTC().Truncate(time.Microsecond)
		var end *time.Time
		if request.EndsAt != nil {
			value := request.EndsAt.UTC().Truncate(time.Microsecond)
			if !value.After(start) {
				return Round{}, invalid("ends_at must be after the round starts")
			}
			end = &value
		}
		board, err := lockBoard(ctx, tx, scope, boardID)
		if err != nil {
			return Round{}, err
		}
		if board.Schedule.Type != ScheduleManual {
			return Round{}, ErrRecurringBoard
		}
		rows, err := tx.Query(ctx, `SELECT `+roundColumns+` FROM leaderboard_rounds
			WHERE leaderboard_id = $1 AND (ends_at IS NULL OR ends_at > $2) AND ($3::timestamptz IS NULL OR starts_at < $3)`, board.ID, start, end)
		if err != nil {
			return Round{}, err
		}
		overlapping, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Round, error) { return scanRound(row, now) })
		if err != nil {
			return Round{}, err
		}
		for _, round := range overlapping {
			if !endCurrent || !round.StartsAt.Before(start) {
				return Round{}, ErrRoundOverlap
			}
			if _, err := tx.Exec(ctx, `UPDATE leaderboard_rounds SET ends_at = $2 WHERE id = $1`, round.ID, start); err != nil {
				return Round{}, err
			}
		}
		id, err := newID("round_")
		if err != nil {
			return Round{}, err
		}
		round, err := scanRound(tx.QueryRow(ctx, `
			INSERT INTO leaderboard_rounds (id, leaderboard_id, number, name, starts_at, ends_at, created_by)
			SELECT $1, $2, n, COALESCE(NULLIF($3, ''), 'Round ' || n), $4, $5, 'admin'
			FROM (SELECT COALESCE(max(number), 0) + 1 AS n FROM leaderboard_rounds WHERE leaderboard_id = $2) next
			RETURNING `+roundColumns, id, board.ID, request.Name, start, end), now)
		if err != nil {
			return Round{}, err
		}
		return *round, nil
	})
}

func lockRound(ctx context.Context, tx pgx.Tx, scope Scope, boardID, roundID string, now time.Time) (Leaderboard, *Round, error) {
	board, err := lockBoard(ctx, tx, scope, boardID)
	if err != nil {
		return Leaderboard{}, nil, err
	}
	round, err := scanRound(tx.QueryRow(ctx, `SELECT `+roundColumns+` FROM leaderboard_rounds WHERE leaderboard_id = $1 AND id = $2`, board.ID, roundID), now)
	if errors.Is(err, pgx.ErrNoRows) {
		return Leaderboard{}, nil, ErrRoundNotFound
	}
	return board, round, err
}

// EndRound closes the running round of a manual board now.
func EndRound(ctx context.Context, db *pgxpool.Pool, scope Scope, boardID, roundID string, now time.Time) (Round, error) {
	return inTx(ctx, db, &now, func(tx pgx.Tx) (Round, error) {
		board, round, err := lockRound(ctx, tx, scope, boardID, roundID, now)
		if err != nil {
			return Round{}, err
		}
		if board.Schedule.Type != ScheduleManual {
			return Round{}, ErrRecurringBoard
		}
		if round.Status != RoundActive {
			return Round{}, ErrRoundNotActive
		}
		updated, err := scanRound(tx.QueryRow(ctx, `UPDATE leaderboard_rounds SET ends_at = $2 WHERE id = $1 RETURNING `+roundColumns, round.ID, now), now)
		if err != nil {
			return Round{}, err
		}
		return *updated, nil
	})
}

// CancelRound removes a scheduled round before it starts.
func CancelRound(ctx context.Context, db *pgxpool.Pool, scope Scope, boardID, roundID string, now time.Time) (Round, error) {
	return inTx(ctx, db, &now, func(tx pgx.Tx) (Round, error) {
		_, round, err := lockRound(ctx, tx, scope, boardID, roundID, now)
		if err != nil {
			return Round{}, err
		}
		if round.Status != RoundScheduled {
			return Round{}, ErrRoundStarted
		}
		if _, err := tx.Exec(ctx, `DELETE FROM leaderboard_rounds WHERE id = $1`, round.ID); err != nil {
			return Round{}, err
		}
		return *round, nil
	})
}

// UpdateRound renames a round or, on a manual board, moves its end so a
// round can run six or eight days instead of seven.
func UpdateRound(ctx context.Context, db *pgxpool.Pool, scope Scope, boardID, roundID string, request RoundUpdate, now time.Time) (Round, error) {
	var name *string
	if request.Name != nil {
		value := strings.TrimSpace(*request.Name)
		if value == "" || len(value) > 120 {
			return Round{}, invalid("name must be between 1 and 120 characters")
		}
		name = &value
	}
	moveEnd := len(request.EndsAt) > 0
	var end *time.Time
	if moveEnd && string(request.EndsAt) != "null" {
		var value time.Time
		if err := json.Unmarshal(request.EndsAt, &value); err != nil {
			return Round{}, invalid("ends_at must be an RFC 3339 time or null")
		}
		value = value.UTC().Truncate(time.Microsecond)
		end = &value
	}
	return inTx(ctx, db, &now, func(tx pgx.Tx) (Round, error) {
		board, round, err := lockRound(ctx, tx, scope, boardID, roundID, now)
		if err != nil {
			return Round{}, err
		}
		if moveEnd {
			if board.Schedule.Type != ScheduleManual {
				return Round{}, ErrRecurringBoard
			}
			if round.Status == RoundEnded {
				return Round{}, ErrRoundEnded
			}
			if end != nil && (!end.After(now) || !end.After(round.StartsAt)) {
				return Round{}, invalid("ends_at must be in the future and after the round starts")
			}
			var nextStart *time.Time
			if err := tx.QueryRow(ctx, `SELECT min(starts_at) FROM leaderboard_rounds WHERE leaderboard_id = $1 AND starts_at > $2`, board.ID, round.StartsAt).Scan(&nextStart); err != nil {
				return Round{}, err
			}
			if nextStart != nil && (end == nil || end.After(*nextStart)) {
				return Round{}, ErrRoundOverlap
			}
			if _, err := tx.Exec(ctx, `UPDATE leaderboard_rounds SET ends_at = $2 WHERE id = $1`, round.ID, end); err != nil {
				return Round{}, err
			}
		}
		if name != nil {
			if _, err := tx.Exec(ctx, `UPDATE leaderboard_rounds SET name = $2 WHERE id = $1`, round.ID, *name); err != nil {
				return Round{}, err
			}
		}
		updated, err := scanRound(tx.QueryRow(ctx, `SELECT `+roundColumns+` FROM leaderboard_rounds WHERE id = $1`, round.ID), now)
		if err != nil {
			return Round{}, err
		}
		return *updated, nil
	})
}
