package leaderboards

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	MetricPoints = "points"
	MetricXP     = "xp"

	RoundScheduled = "scheduled"
	RoundActive    = "active"
	RoundEnded     = "ended"
)

var (
	ErrNotFound          = errors.New("leaderboard was not found")
	ErrRoundNotFound     = errors.New("leaderboard round was not found")
	ErrCurrencyNotFound  = errors.New("currency was not found")
	ErrRecurringBoard    = errors.New("rounds of a recurring leaderboard follow its schedule")
	ErrRoundOverlap      = errors.New("the round would overlap another round of this leaderboard")
	ErrScheduledRounds   = errors.New("cancel the scheduled rounds before switching this leaderboard to a recurring schedule")
	ErrRoundNotActive    = errors.New("only an active round can be ended")
	ErrRoundStarted      = errors.New("only a round that has not started can be cancelled")
	ErrRoundEnded        = errors.New("an ended round cannot be changed")
	ErrMetricUnchangable = errors.New("a leaderboard's metric cannot change; create a new leaderboard instead")
)

var (
	identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
	currencyPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
)

// ValidationError carries a message that is safe to return to the caller.
type ValidationError struct{ Message string }

func (e *ValidationError) Error() string { return e.Message }

func invalid(format string, args ...any) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

// Metric is what a leaderboard ranks: points earned in one currency, or XP
// earned. Spending, expiry, conversions, and redemption refunds never count.
type Metric struct {
	Type       string `json:"type"`
	Currency   string `json:"currency,omitempty"`
	CurrencyID string `json:"currency_id,omitempty"`
}

type Leaderboard struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	EnvironmentID string    `json:"environment_id"`
	Key           string    `json:"key"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Metric        Metric    `json:"metric"`
	Schedule      Schedule  `json:"schedule"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CurrentRound  *Round    `json:"current_round"`
}

type Round struct {
	ID           string     `json:"id"`
	Number       int        `json:"number"`
	Name         string     `json:"name"`
	StartsAt     time.Time  `json:"starts_at"`
	EndsAt       *time.Time `json:"ends_at"`
	Status       string     `json:"status"`
	CreatedBy    string     `json:"created_by"`
	CreatedAt    time.Time  `json:"created_at"`
	Participants *int64     `json:"participants,omitempty"`
	Leader       *Entry     `json:"leader,omitempty"`
}

type Entry struct {
	Rank      int64     `json:"rank"`
	UserID    string    `json:"user_id"`
	Score     int64     `json:"score"`
	ReachedAt time.Time `json:"reached_at"`
}

type BoardRequest struct {
	TenantID      string    `json:"tenant_id"`
	EnvironmentID string    `json:"environment_id"`
	Key           string    `json:"key"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Metric        *Metric   `json:"metric"`
	Schedule      *Schedule `json:"schedule"`
}

type RoundRequest struct {
	TenantID      string     `json:"tenant_id"`
	EnvironmentID string     `json:"environment_id"`
	Name          string     `json:"name"`
	StartsAt      *time.Time `json:"starts_at"`
	EndsAt        *time.Time `json:"ends_at"`
	// EndCurrent ends the round that is running when the new one starts.
	// It defaults to true so one call rotates a manual board.
	EndCurrent *bool `json:"end_current"`
}

// RoundUpdate renames a round or moves its end. Sending "ends_at": null
// leaves the round open until someone ends it.
type RoundUpdate struct {
	TenantID      string          `json:"tenant_id"`
	EnvironmentID string          `json:"environment_id"`
	Name          *string         `json:"name"`
	EndsAt        json.RawMessage `json:"ends_at"`
}

func (request BoardRequest) scope() (string, string) {
	return strings.TrimSpace(request.TenantID), strings.TrimSpace(request.EnvironmentID)
}

// ValidateBoardRequest normalizes a create request. now picks the default
// anchor of a recurring schedule.
func ValidateBoardRequest(request BoardRequest, now time.Time) (BoardRequest, error) {
	request.TenantID, request.EnvironmentID = request.scope()
	request.Key = strings.TrimSpace(request.Key)
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) {
		return BoardRequest{}, invalid("tenant_id and environment_id are required")
	}
	if !identifierPattern.MatchString(request.Key) {
		return BoardRequest{}, invalid("key must contain 1-64 lowercase letters, numbers, '_' or '-'")
	}
	if request.Name == "" || len(request.Name) > 120 {
		return BoardRequest{}, invalid("name must be between 1 and 120 characters")
	}
	if len(request.Description) > 500 {
		return BoardRequest{}, invalid("description must be at most 500 characters")
	}
	if request.Metric == nil {
		return BoardRequest{}, invalid("metric is required")
	}
	metric := Metric{Type: strings.TrimSpace(request.Metric.Type), Currency: strings.TrimSpace(request.Metric.Currency)}
	switch metric.Type {
	case MetricPoints:
		if !currencyPattern.MatchString(metric.Currency) {
			return BoardRequest{}, invalid("metric.currency must be a currency code for a points leaderboard")
		}
	case MetricXP:
		if metric.Currency != "" {
			return BoardRequest{}, invalid("metric.currency is only used by points leaderboards")
		}
	default:
		return BoardRequest{}, invalid("metric.type must be points or xp")
	}
	request.Metric = &metric
	if request.Schedule == nil {
		return BoardRequest{}, invalid("schedule is required")
	}
	schedule, err := normalizeSchedule(*request.Schedule, now)
	if err != nil {
		return BoardRequest{}, invalid("%s", err.Error())
	}
	request.Schedule = &schedule
	return request, nil
}

// validateBoardUpdate normalizes an update request. Key and metric may be
// repeated but not changed: scores already recorded belong to the old metric.
func validateBoardUpdate(request BoardRequest, current Leaderboard, now time.Time) (BoardRequest, error) {
	if strings.TrimSpace(request.Key) == "" {
		request.Key = current.Key
	}
	if strings.TrimSpace(request.Key) != current.Key {
		return BoardRequest{}, invalid("key cannot change")
	}
	if request.Metric == nil {
		request.Metric = &Metric{Type: current.Metric.Type, Currency: current.Metric.Currency}
	}
	if request.Schedule == nil {
		schedule := current.Schedule
		request.Schedule = &schedule
	}
	validated, err := ValidateBoardRequest(request, now)
	if err != nil {
		return BoardRequest{}, err
	}
	if validated.Metric.Type != current.Metric.Type || validated.Metric.Currency != current.Metric.Currency {
		return BoardRequest{}, ErrMetricUnchangable
	}
	return validated, nil
}

func roundStatus(round *Round, now time.Time) {
	switch {
	case round.StartsAt.After(now):
		round.Status = RoundScheduled
	case round.EndsAt != nil && !round.EndsAt.After(now):
		round.Status = RoundEnded
	default:
		round.Status = RoundActive
	}
}
