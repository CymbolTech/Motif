package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	eventdomain "github.com/motifengine/motifengine/internal/events"
	"github.com/motifengine/motifengine/internal/rules"
	appruntime "github.com/motifengine/motifengine/internal/runtime"
)

// RuleCheckRequest is a rule definition plus an optional sample payload. The
// check validates and evaluates the rule without saving anything.
type RuleCheckRequest struct {
	RuleRequest
	SamplePayload json.RawMessage `json:"sample_payload,omitempty"`
}

type ruleCheckResponse struct {
	Valid      bool             `json:"valid"`
	Errors     []string         `json:"errors"`
	Warnings   []string         `json:"warnings"`
	Conditions json.RawMessage  `json:"conditions,omitempty"`
	Actions    json.RawMessage  `json:"actions,omitempty"`
	Contract   *ruleCheckSchema `json:"event_type_contract,omitempty"`
	Sample     *ruleCheckSample `json:"sample,omitempty"`
}

type ruleCheckSchema struct {
	Registered bool `json:"registered"`
	Version    int  `json:"version,omitempty"`
}

type ruleCheckSample struct {
	SchemaValid bool               `json:"schema_valid"`
	SchemaError string             `json:"schema_error,omitempty"`
	Matched     bool               `json:"matched"`
	Reason      string             `json:"reason"`
	Explanation *rules.Explanation `json:"explanation,omitempty"`
	WouldApply  json.RawMessage    `json:"would_apply,omitempty"`
}

type checkedAction struct {
	Type     string `json:"type"`
	Currency string `json:"currency"`
}

func (h *Handler) checkRule(w http.ResponseWriter, r *http.Request) {
	var request RuleCheckRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 128*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	if request.Name == "" {
		request.Name = "rule check"
	}
	response := ruleCheckResponse{Errors: []string{}, Warnings: []string{}}
	validated, err := ValidateRuleRequest(request.RuleRequest)
	if err != nil {
		response.Errors = append(response.Errors, err.Error())
		writeJSON(w, http.StatusOK, response)
		return
	}
	response.Conditions, response.Actions = validated.Conditions, validated.Action
	ctx := r.Context()
	var environmentExists bool
	if err := h.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM environments WHERE tenant_id = $1 AND id = $2)`, validated.TenantID, validated.EnvironmentID).Scan(&environmentExists); err != nil {
		writeError(w, http.StatusInternalServerError, "rule_check_failed", "rule could not be checked")
		return
	}
	if !environmentExists {
		writeError(w, http.StatusNotFound, "environment_not_found", "tenant environment was not found")
		return
	}

	var schema json.RawMessage
	contract := &ruleCheckSchema{}
	err = h.db.QueryRow(ctx, `SELECT schema, current_version FROM event_types WHERE tenant_id = $1 AND environment_id = $2 AND name = $3 AND active = true`,
		validated.TenantID, validated.EnvironmentID, validated.EventType).Scan(&schema, &contract.Version)
	switch {
	case err == nil:
		contract.Registered = true
	case errors.Is(err, pgx.ErrNoRows):
		response.Warnings = append(response.Warnings, fmt.Sprintf("event type %q has no active contract in this environment; its events are accepted without schema validation", validated.EventType))
	default:
		writeError(w, http.StatusInternalServerError, "rule_check_failed", "rule could not be checked")
		return
	}
	response.Contract = contract

	currencyErrors, err := h.checkActionCurrencies(r, validated)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rule_check_failed", "rule could not be checked")
		return
	}
	response.Errors = append(response.Errors, currencyErrors...)
	if awardsUser(validated.Action) {
		response.Warnings = append(response.Warnings, "award_points and award_xp need the event's user_id; events sent without one fail these actions")
	}
	if validated.EndsAt != nil && !validated.EndsAt.After(time.Now()) {
		response.Warnings = append(response.Warnings, "ends_at is in the past, so the published rule would never run")
	}

	if len(bytes.TrimSpace(request.SamplePayload)) > 0 {
		sample := &ruleCheckSample{SchemaValid: true}
		if contract.Registered {
			if err := eventdomain.ValidateSchema(schema, request.SamplePayload); err != nil {
				sample.SchemaValid, sample.SchemaError = false, err.Error()
			}
		}
		explanation, err := rules.ExplainConditions(validated.Conditions, request.SamplePayload)
		if err != nil {
			response.Errors = append(response.Errors, "sample_payload: "+err.Error())
		} else {
			sample.Explanation = &explanation
			sample.Matched = explanation.Matched && explanation.Error == ""
			sample.Reason = explanation.Reason()
			if sample.Matched {
				sample.WouldApply = validated.Action
			}
		}
		response.Sample = sample
	}
	response.Valid = len(response.Errors) == 0
	writeJSON(w, http.StatusOK, response)
}

// checkActionCurrencies reports award_points actions whose currency is missing
// or inactive in the environment; the worker would fail those actions.
func (h *Handler) checkActionCurrencies(r *http.Request, rule RuleRequest) ([]string, error) {
	actions, err := decodeCheckedActions(rule.Action)
	if err != nil {
		return nil, err
	}
	problems := []string{}
	for index, action := range actions {
		if action.Type != "award_points" {
			continue
		}
		var active bool
		err := h.db.QueryRow(r.Context(), `SELECT active FROM currencies WHERE tenant_id = $1 AND environment_id = $2 AND code = $3`, rule.TenantID, rule.EnvironmentID, action.Currency).Scan(&active)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			problems = append(problems, fmt.Sprintf("actions[%d]: currency %q does not exist in this environment", index, action.Currency))
		case err != nil:
			return nil, err
		case !active:
			problems = append(problems, fmt.Sprintf("actions[%d]: currency %q is inactive", index, action.Currency))
		}
	}
	return problems, nil
}

func decodeCheckedActions(raw json.RawMessage) ([]checkedAction, error) {
	var actions []checkedAction
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		err := json.Unmarshal(trimmed, &actions)
		return actions, err
	}
	var action checkedAction
	if err := json.Unmarshal(trimmed, &action); err != nil {
		return nil, err
	}
	return append(actions, action), nil
}

func awardsUser(raw json.RawMessage) bool {
	actions, err := decodeCheckedActions(raw)
	if err != nil {
		return false
	}
	for _, action := range actions {
		if action.Type == "award_points" || action.Type == "award_xp" {
			return true
		}
	}
	return false
}
