package admin

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/motifengine/motifengine/internal/points"
	"github.com/motifengine/motifengine/internal/rules"
)

var identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var eventTypePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

type TenantRequest struct {
	TenantID        string `json:"tenant_id"`
	Name            string `json:"name"`
	EnvironmentID   string `json:"environment_id"`
	EnvironmentName string `json:"environment_name"`
}

type EventTypeRequest struct {
	TenantID      string          `json:"tenant_id"`
	EnvironmentID string          `json:"environment_id"`
	Name          string          `json:"name"`
	Schema        json.RawMessage `json:"schema"`
}

type RuleRequest struct {
	TenantID      string          `json:"tenant_id"`
	EnvironmentID string          `json:"environment_id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	EventType     string          `json:"event_type"`
	Conditions    json.RawMessage `json:"conditions"`
	Action        json.RawMessage `json:"action"`
	StartsAt      *time.Time      `json:"starts_at,omitempty"`
	EndsAt        *time.Time      `json:"ends_at,omitempty"`
}

func ValidateTenantRequest(request TenantRequest) (TenantRequest, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.Name = strings.TrimSpace(request.Name)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.EnvironmentName = strings.TrimSpace(request.EnvironmentName)
	if !identifierPattern.MatchString(request.TenantID) {
		return TenantRequest{}, fmt.Errorf("tenant_id must contain 1-64 lowercase letters, numbers, '_' or '-'")
	}
	if !identifierPattern.MatchString(request.EnvironmentID) {
		return TenantRequest{}, fmt.Errorf("environment_id must contain 1-64 lowercase letters, numbers, '_' or '-'")
	}
	if request.Name == "" || len(request.Name) > 120 {
		return TenantRequest{}, fmt.Errorf("name must be between 1 and 120 characters")
	}
	if request.EnvironmentName == "" || len(request.EnvironmentName) > 120 {
		return TenantRequest{}, fmt.Errorf("environment_name must be between 1 and 120 characters")
	}
	return request, nil
}

func ValidateEventTypeRequest(request EventTypeRequest) (EventTypeRequest, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.Name = strings.TrimSpace(request.Name)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) {
		return EventTypeRequest{}, fmt.Errorf("tenant_id and environment_id must be valid identifiers")
	}
	if request.Name == "" || len(request.Name) > 128 {
		return EventTypeRequest{}, fmt.Errorf("name must be between 1 and 128 characters")
	}
	for _, character := range request.Name {
		if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '.' || character == '_' || character == '-') {
			return EventTypeRequest{}, fmt.Errorf("name contains an unsupported character")
		}
	}
	if len(request.Schema) == 0 {
		request.Schema = json.RawMessage(`{}`)
	}
	var schema map[string]any
	if err := json.Unmarshal(request.Schema, &schema); err != nil {
		return EventTypeRequest{}, fmt.Errorf("schema must be a JSON object: %w", err)
	}
	if schema == nil {
		return EventTypeRequest{}, fmt.Errorf("schema must be a JSON object")
	}
	canonical, err := json.Marshal(schema)
	if err != nil {
		return EventTypeRequest{}, fmt.Errorf("schema could not be normalized: %w", err)
	}
	request.Schema = canonical
	return request, nil
}

func ValidateRuleRequest(request RuleRequest) (RuleRequest, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	request.EventType = strings.TrimSpace(request.EventType)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) {
		return RuleRequest{}, fmt.Errorf("tenant_id and environment_id must be valid identifiers")
	}
	if request.Name == "" || len(request.Name) > 120 {
		return RuleRequest{}, fmt.Errorf("name must be between 1 and 120 characters")
	}
	if request.EventType == "" || !eventTypePattern.MatchString(request.EventType) {
		return RuleRequest{}, fmt.Errorf("event_type must contain lowercase letters, numbers, '.', '_' or '-'")
	}
	if len(request.Description) > 500 {
		return RuleRequest{}, fmt.Errorf("description must be 500 characters or fewer")
	}
	if request.StartsAt != nil && request.EndsAt != nil && !request.EndsAt.After(*request.StartsAt) {
		return RuleRequest{}, fmt.Errorf("ends_at must be later than starts_at")
	}
	if len(request.Conditions) == 0 {
		request.Conditions = json.RawMessage(`{}`)
	}
	if len(request.Action) == 0 {
		return RuleRequest{}, fmt.Errorf("action is required")
	}
	canonicalConditions, err := rules.ValidateConditions(request.Conditions)
	if err != nil {
		return RuleRequest{}, err
	}
	canonicalAction, err := points.ValidateAction(request.Action)
	if err != nil {
		return RuleRequest{}, err
	}
	request.Conditions = canonicalConditions
	request.Action = canonicalAction
	return request, nil
}
