package leaderboards

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DefaultLimit = 10
	MaxLimit     = 100
	MaxOffset    = 10000
)

type Standings struct {
	Leaderboard  Leaderboard `json:"leaderboard"`
	Round        *Round      `json:"round"`
	NextRound    *Round      `json:"next_round"`
	Participants int64       `json:"participants"`
	Entries      []Entry     `json:"entries"`
	Limit        int         `json:"limit"`
	Offset       int         `json:"offset"`
}

type UserStanding struct {
	Leaderboard  Leaderboard `json:"leaderboard"`
	Round        *Round      `json:"round"`
	Participants int64       `json:"participants"`
	Entry        *Entry      `json:"entry"`
}

// LoadStandings returns one page of a round's ranking. selector is
// "current" (or empty), "previous", or a round number. "current" falls back
// to the last round that started, so a page between two manual rounds shows
// final results instead of nothing.
func LoadStandings(ctx context.Context, db *pgxpool.Pool, board Leaderboard, selector string, limit, offset int, now time.Time) (Standings, error) {
	return inTx(ctx, db, &now, func(tx pgx.Tx) (Standings, error) {
		result := Standings{Leaderboard: board, Entries: make([]Entry, 0), Limit: limit, Offset: offset}
		var round *Round
		var err error
		switch selector = strings.TrimSpace(selector); selector {
		case "", "current":
			round, err = displayRound(ctx, tx, board, now)
		case "previous":
			round, err = optionalRound(tx.QueryRow(ctx, `SELECT `+roundColumns+` FROM leaderboard_rounds WHERE leaderboard_id = $1 AND ends_at <= $2 ORDER BY ends_at DESC LIMIT 1`, board.ID, now), now)
			if err == nil && round == nil {
				err = ErrRoundNotFound
			}
		default:
			number, parseErr := strconv.Atoi(selector)
			if parseErr != nil || number < 1 {
				return Standings{}, invalid("round must be current, previous, or a round number")
			}
			round, err = optionalRound(tx.QueryRow(ctx, `SELECT `+roundColumns+` FROM leaderboard_rounds WHERE leaderboard_id = $1 AND number = $2`, board.ID, number), now)
			if err == nil && round == nil {
				err = ErrRoundNotFound
			}
		}
		if err != nil {
			return Standings{}, err
		}
		result.Round = round
		if result.NextRound, err = optionalRound(tx.QueryRow(ctx, `SELECT `+roundColumns+` FROM leaderboard_rounds WHERE leaderboard_id = $1 AND starts_at > $2 ORDER BY starts_at LIMIT 1`, board.ID, now), now); err != nil {
			return Standings{}, err
		}
		if round == nil {
			return result, nil
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM leaderboard_scores WHERE round_id = $1`, round.ID).Scan(&result.Participants); err != nil {
			return Standings{}, err
		}
		rows, err := tx.Query(ctx, `
			SELECT external_user_id, score, reached_at FROM leaderboard_scores
			WHERE round_id = $1
			ORDER BY score DESC, reached_at, external_user_id
			LIMIT $2 OFFSET $3`, round.ID, limit, offset)
		if err != nil {
			return Standings{}, err
		}
		defer rows.Close()
		for rows.Next() {
			entry := Entry{Rank: int64(offset + len(result.Entries) + 1)}
			if err := rows.Scan(&entry.UserID, &entry.Score, &entry.ReachedAt); err != nil {
				return Standings{}, err
			}
			result.Entries = append(result.Entries, entry)
		}
		return result, rows.Err()
	})
}

// LoadUserStandings returns a user's place on every active board's current
// round. Entry is null when the user has not scored in that round.
func LoadUserStandings(ctx context.Context, db *pgxpool.Pool, scope Scope, userID string, now time.Time) ([]UserStanding, error) {
	return inTx(ctx, db, &now, func(tx pgx.Tx) ([]UserStanding, error) {
		rows, err := tx.Query(ctx, boardSelect+` WHERE l.tenant_id = $1 AND l.environment_id = $2 AND l.active = true ORDER BY l.name, l.key`, scope.TenantID, scope.EnvironmentID)
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
		result := make([]UserStanding, 0, len(boards))
		for _, board := range boards {
			standing := UserStanding{Leaderboard: board, Round: board.CurrentRound}
			standing.Leaderboard.CurrentRound = nil
			if standing.Round != nil {
				if err := tx.QueryRow(ctx, `SELECT count(*) FROM leaderboard_scores WHERE round_id = $1`, standing.Round.ID).Scan(&standing.Participants); err != nil {
					return nil, err
				}
				if standing.Entry, err = userEntry(ctx, tx, standing.Round.ID, userID); err != nil {
					return nil, err
				}
			}
			result = append(result, standing)
		}
		return result, nil
	})
}

func userEntry(ctx context.Context, q querier, roundID, userID string) (*Entry, error) {
	entry := Entry{UserID: userID}
	err := q.QueryRow(ctx, `SELECT score, reached_at FROM leaderboard_scores WHERE round_id = $1 AND external_user_id = $2`, roundID, userID).Scan(&entry.Score, &entry.ReachedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Rank follows the ranking order: higher score, then whoever reached it
	// first, then user id so every rank is unique.
	if err := q.QueryRow(ctx, `
		SELECT count(*) + 1 FROM leaderboard_scores
		WHERE round_id = $1 AND (score > $2 OR (score = $2 AND (reached_at < $3 OR (reached_at = $3 AND external_user_id < $4))))`,
		roundID, entry.Score, entry.ReachedAt, userID).Scan(&entry.Rank); err != nil {
		return nil, err
	}
	return &entry, nil
}
