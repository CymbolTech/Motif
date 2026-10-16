package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/motifengine/motifengine/internal/rules"
)

type eventTraceResponse struct {
	Event          traceEvent           `json:"event"`
	RuleExecutions []traceRuleExecution `json:"rule_executions"`
	PointEntries   []tracePointEntry    `json:"point_entries"`
	XPEntries      []traceXPEntry       `json:"xp_entries"`
	Missions       []traceProgress      `json:"missions"`
	Achievements   []traceProgress      `json:"achievements"`
	Evaluations    []traceEvaluation    `json:"evaluations"`
	Note           string               `json:"note"`
}

type traceEvent struct {
	EventID       string          `json:"event_id"`
	EventType     string          `json:"event_type"`
	UserID        string          `json:"user_id,omitempty"`
	Status        string          `json:"status"`
	SchemaVersion int             `json:"schema_version"`
	OccurredAt    *time.Time      `json:"occurred_at,omitempty"`
	ReceivedAt    time.Time       `json:"received_at"`
	ProcessedAt   *time.Time      `json:"processed_at,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

type traceRuleExecution struct {
	RuleID      string          `json:"rule_id"`
	RuleName    string          `json:"rule_name"`
	Version     int             `json:"version"`
	Status      string          `json:"status"`
	Attempts    int             `json:"attempts"`
	LastError   string          `json:"last_error,omitempty"`
	Actions     json.RawMessage `json:"actions"`
	CreatedAt   time.Time       `json:"created_at"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

type tracePointEntry struct {
	Currency       string    `json:"currency"`
	EntryType      string    `json:"entry_type"`
	Amount         int64     `json:"amount"`
	BalanceAfter   int64     `json:"balance_after"`
	IdempotencyKey string    `json:"idempotency_key"`
	CreatedAt      time.Time `json:"created_at"`
}

type traceXPEntry struct {
	Amount         int64     `json:"amount"`
	IdempotencyKey string    `json:"idempotency_key"`
	CreatedAt      time.Time `json:"created_at"`
}

type traceProgress struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type traceEvaluation struct {
	RuleID      string             `json:"rule_id"`
	RuleName    string             `json:"rule_name"`
	Version     int                `json:"version"`
	InWindow    bool               `json:"in_window"`
	Matched     bool               `json:"matched"`
	Reason      string             `json:"reason"`
	Explanation *rules.Explanation `json:"explanation,omitempty"`
}

func isEventTracePath(path string) bool {
	id, ok := eventTraceID(path)
	return ok && id != ""
}

func eventTraceID(path string) (string, bool) {
	if !strings.HasPrefix(path, "/v1/admin/events/") || !strings.HasSuffix(path, "/trace") {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/admin/events/"), "/trace")
	return id, id != "" && !strings.Contains(id, "/")
}

// eventTrace explains what one event did: the receipt, the rules that ran and
// what they awarded, and a fresh evaluation of every currently published rule
// for the event type so a non-match can be explained.
func (h *Handler) eventTrace(w http.ResponseWriter, r *http.Request) {
	externalID, _ := eventTraceID(r.URL.Path)
	tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
	environmentID := strings.TrimSpace(r.URL.Query().Get("environment_id"))
	if !identifierPattern.MatchString(tenantID) || !identifierPattern.MatchString(environmentID) {
		writeError(w, http.StatusBadRequest, "invalid_scope", "tenant_id and environment_id are required")
		return
	}
	ctx := r.Context()
	var eventID int64
	var userID *string
	response := eventTraceResponse{
		RuleExecutions: []traceRuleExecution{}, PointEntries: []tracePointEntry{}, XPEntries: []traceXPEntry{},
		Missions: []traceProgress{}, Achievements: []traceProgress{}, Evaluations: []traceEvaluation{},
		Note: "evaluations re-run the currently published rule versions against the stored payload; they explain matches now, which can differ from processing time if a rule changed since",
	}
	err := h.db.QueryRow(ctx, `
		SELECT id, external_event_id, event_type, external_user_id, status, event_schema_version, occurred_at, received_at, processed_at, payload
		FROM events WHERE tenant_id = $1 AND environment_id = $2 AND external_event_id = $3`, tenantID, environmentID, externalID).
		Scan(&eventID, &response.Event.EventID, &response.Event.EventType, &userID, &response.Event.Status, &response.Event.SchemaVersion,
			&response.Event.OccurredAt, &response.Event.ReceivedAt, &response.Event.ProcessedAt, &response.Event.Payload)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "event_not_found", "event was not found in this environment")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "event_trace_failed", "event trace could not be read")
		return
	}
	if userID != nil {
		response.Event.UserID = *userID
	}
	if err := h.traceDetails(r, eventID, tenantID, environmentID, &response); err != nil {
		writeError(w, http.StatusInternalServerError, "event_trace_failed", "event trace could not be read")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) traceDetails(r *http.Request, eventID int64, tenantID, environmentID string, response *eventTraceResponse) error {
	ctx := r.Context()
	rows, err := h.db.Query(ctx, `
		SELECT x.rule_id, ru.name, COALESCE(rv.version, 0), x.status, x.attempts, COALESCE(x.last_error, ''), x.action, x.created_at, x.completed_at
		FROM rule_executions x
		JOIN rules ru ON ru.id = x.rule_id
		LEFT JOIN rule_versions rv ON rv.id = x.rule_version_id
		WHERE x.event_id = $1 ORDER BY x.id`, eventID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var execution traceRuleExecution
		if err := rows.Scan(&execution.RuleID, &execution.RuleName, &execution.Version, &execution.Status, &execution.Attempts, &execution.LastError, &execution.Actions, &execution.CreatedAt, &execution.CompletedAt); err != nil {
			rows.Close()
			return err
		}
		response.RuleExecutions = append(response.RuleExecutions, execution)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	keyPrefix := "event:" + strconv.FormatInt(eventID, 10) + ":%"
	rows, err = h.db.Query(ctx, `
		SELECT c.code, l.entry_type, l.amount, l.balance_after, l.idempotency_key, l.created_at
		FROM point_ledger_entries l
		JOIN point_accounts a ON a.id = l.account_id
		JOIN currencies c ON c.id = a.currency_id
		WHERE a.tenant_id = $1 AND a.environment_id = $2 AND l.idempotency_key LIKE $3
		ORDER BY l.id`, tenantID, environmentID, keyPrefix)
	if err != nil {
		return err
	}
	for rows.Next() {
		var entry tracePointEntry
		if err := rows.Scan(&entry.Currency, &entry.EntryType, &entry.Amount, &entry.BalanceAfter, &entry.IdempotencyKey, &entry.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		response.PointEntries = append(response.PointEntries, entry)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	rows, err = h.db.Query(ctx, `
		SELECT amount, idempotency_key, created_at FROM xp_events
		WHERE tenant_id = $1 AND environment_id = $2 AND idempotency_key LIKE $3 ORDER BY id`, tenantID, environmentID, keyPrefix)
	if err != nil {
		return err
	}
	for rows.Next() {
		var entry traceXPEntry
		if err := rows.Scan(&entry.Amount, &entry.IdempotencyKey, &entry.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		response.XPEntries = append(response.XPEntries, entry)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, progress := range []struct {
		query  string
		target *[]traceProgress
	}{
		{`SELECT m.id, m.name FROM mission_progress_events p JOIN missions m ON m.id = p.mission_id WHERE p.event_id = $1 ORDER BY m.name`, &response.Missions},
		{`SELECT a.id, a.name FROM achievement_progress_events p JOIN achievements a ON a.id = p.achievement_id WHERE p.event_id = $1 ORDER BY a.name`, &response.Achievements},
	} {
		rows, err := h.db.Query(ctx, progress.query, eventID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var item traceProgress
			if err := rows.Scan(&item.ID, &item.Name); err != nil {
				rows.Close()
				return err
			}
			*progress.target = append(*progress.target, item)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}

	rows, err = h.db.Query(ctx, `
		SELECT r.id, r.name, rv.version, rv.conditions,
		       (rv.starts_at IS NULL OR rv.starts_at <= now()) AND (rv.ends_at IS NULL OR rv.ends_at > now())
		FROM rules r
		JOIN rule_versions rv ON rv.rule_id = r.id AND rv.status = 'published'
		WHERE r.tenant_id = $1 AND r.environment_id = $2 AND rv.event_type = $3
		ORDER BY r.name`, tenantID, environmentID, response.Event.EventType)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var evaluation traceEvaluation
		var conditions json.RawMessage
		if err := rows.Scan(&evaluation.RuleID, &evaluation.RuleName, &evaluation.Version, &conditions, &evaluation.InWindow); err != nil {
			return err
		}
		explanation, err := rules.ExplainConditions(conditions, response.Event.Payload)
		if err != nil {
			evaluation.Reason = err.Error()
		} else {
			evaluation.Explanation = &explanation
			evaluation.Matched = explanation.Matched && explanation.Error == ""
			evaluation.Reason = explanation.Reason()
		}
		if !evaluation.InWindow {
			evaluation.Reason = "outside the rule's schedule window; " + evaluation.Reason
		}
		response.Evaluations = append(response.Evaluations, evaluation)
	}
	return rows.Err()
}
