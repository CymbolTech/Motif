package points

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
)

var currencyCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

func ValidateCurrencyRequest(request CurrencyRequest) (CurrencyRequest, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.Code = strings.TrimSpace(request.Code)
	request.Name = strings.TrimSpace(request.Name)
	if request.TenantID == "" || request.EnvironmentID == "" {
		return CurrencyRequest{}, fmt.Errorf("tenant_id and environment_id are required")
	}
	if !currencyCodePattern.MatchString(request.Code) {
		return CurrencyRequest{}, fmt.Errorf("code must contain 1-32 lowercase letters, numbers, '_' or '-'")
	}
	if request.Name == "" || len(request.Name) > 120 {
		return CurrencyRequest{}, fmt.Errorf("name must be between 1 and 120 characters")
	}
	if request.UnitScale < 0 || request.UnitScale > 6 {
		return CurrencyRequest{}, fmt.Errorf("unit_scale must be between 0 and 6")
	}
	if len(request.Policy) == 0 {
		request.Policy = json.RawMessage(`{}`)
	}
	var policy map[string]any
	decoder := json.NewDecoder(bytes.NewReader(request.Policy))
	decoder.UseNumber()
	if err := decoder.Decode(&policy); err != nil || policy == nil {
		return CurrencyRequest{}, fmt.Errorf("policy must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return CurrencyRequest{}, fmt.Errorf("policy must contain one JSON value")
	}
	if rawExpiry, ok := policy["expiry_days"]; ok {
		expiry, ok := rawExpiry.(json.Number)
		if !ok {
			return CurrencyRequest{}, fmt.Errorf("policy.expiry_days must be a non-negative integer")
		}
		value, err := expiry.Int64()
		if err != nil || value < 0 || value > math.MaxInt32 {
			return CurrencyRequest{}, fmt.Errorf("policy.expiry_days must be a non-negative integer")
		}
	}
	canonical, err := json.Marshal(policy)
	if err != nil {
		return CurrencyRequest{}, fmt.Errorf("policy could not be normalized")
	}
	request.Policy = canonical
	return request, nil
}

func ValidateAction(raw json.RawMessage) (json.RawMessage, error) {
	var action map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&action); err != nil || action == nil {
		return nil, fmt.Errorf("action must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, fmt.Errorf("action must contain one JSON value")
	}
	actionType, ok := action["type"].(string)
	if !ok || strings.TrimSpace(actionType) == "" {
		return nil, fmt.Errorf("action.type is required")
	}
	switch actionType {
	case "log":
		if message, ok := action["message"]; ok {
			if text, valid := message.(string); !valid || len(text) > 500 {
				return nil, fmt.Errorf("log action message must be a string of 500 characters or fewer")
			}
		}
	case "award_points":
		currency, ok := action["currency"].(string)
		if !ok || !currencyCodePattern.MatchString(currency) {
			return nil, fmt.Errorf("award_points.currency must be a valid currency code")
		}
		if !positiveActionAmount(action["amount"]) {
			return nil, fmt.Errorf("award_points.amount must be a positive integer")
		}
	case "award_xp":
		if !positiveActionAmount(action["amount"]) {
			return nil, fmt.Errorf("award_xp.amount must be a positive integer")
		}
	default:
		return nil, fmt.Errorf("unsupported action type %q", actionType)
	}
	canonical, err := json.Marshal(action)
	if err != nil {
		return nil, fmt.Errorf("action could not be normalized")
	}
	return canonical, nil
}

// ValidateActions accepts the current single-action object and the multi-action
// array used by the automation builder. The canonical form is always an array
// so the worker can execute each effect in order.
func ValidateActions(raw json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("actions are required")
	}
	if trimmed[0] != '[' {
		canonical, err := ValidateAction(trimmed)
		if err != nil {
			return nil, err
		}
		return json.Marshal([]json.RawMessage{canonical})
	}

	var actions []json.RawMessage
	if err := json.Unmarshal(trimmed, &actions); err != nil {
		return nil, fmt.Errorf("actions must be a JSON array: %w", err)
	}
	if len(actions) == 0 {
		return nil, fmt.Errorf("at least one action is required")
	}
	canonical := make([]json.RawMessage, 0, len(actions))
	for index, action := range actions {
		normalized, err := ValidateAction(action)
		if err != nil {
			return nil, fmt.Errorf("actions[%d]: %w", index, err)
		}
		canonical = append(canonical, normalized)
	}
	return json.Marshal(canonical)
}

func positiveActionAmount(value any) bool {
	number, ok := value.(json.Number)
	if !ok {
		return false
	}
	amount, err := number.Int64()
	return err == nil && amount > 0
}
