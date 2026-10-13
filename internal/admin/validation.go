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

type EnvironmentRequest struct {
	TenantID string `json:"tenant_id"`
	ID       string `json:"environment_id"`
	Name     string `json:"name"`
}

type GoalRequest struct {
	TenantID      string `json:"tenant_id"`
	EnvironmentID string `json:"environment_id"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	EventType     string `json:"event_type"`
	TargetCount   int64  `json:"target_count"`
}

type CampaignRequest struct {
	TenantID      string     `json:"tenant_id"`
	EnvironmentID string     `json:"environment_id"`
	ID            string     `json:"id"`
	GoalID        string     `json:"goal_id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	EventType     string     `json:"event_type"`
	StartsAt      *time.Time `json:"starts_at"`
	EndsAt        *time.Time `json:"ends_at"`
}

type MetricRequest struct {
	TenantID      string `json:"tenant_id"`
	EnvironmentID string `json:"environment_id"`
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	EventType     string `json:"event_type"`
	Aggregation   string `json:"aggregation"`
}

type FunnelStepRequest struct {
	Name      string `json:"name"`
	EventType string `json:"event_type"`
}

type FunnelRequest struct {
	TenantID      string              `json:"tenant_id"`
	EnvironmentID string              `json:"environment_id"`
	Steps         []FunnelStepRequest `json:"steps"`
	From          *time.Time          `json:"from,omitempty"`
	To            *time.Time          `json:"to,omitempty"`
}

type RetentionRequest struct {
	TenantID        string     `json:"tenant_id"`
	EnvironmentID   string     `json:"environment_id"`
	CohortEventType string     `json:"cohort_event_type"`
	ReturnEventType string     `json:"return_event_type"`
	From            *time.Time `json:"from,omitempty"`
	To              *time.Time `json:"to,omitempty"`
	Intervals       []int      `json:"intervals,omitempty"`
}

type SegmentRequest struct {
	TenantID      string     `json:"tenant_id"`
	EnvironmentID string     `json:"environment_id"`
	EventType     string     `json:"event_type"`
	MinEvents     int        `json:"min_events"`
	From          *time.Time `json:"from,omitempty"`
	To            *time.Time `json:"to,omitempty"`
}

type ExperimentVariantRequest struct {
	ID     string `json:"id"`
	Weight int    `json:"weight"`
}

type ExperimentRequest struct {
	TenantID        string                     `json:"tenant_id"`
	EnvironmentID   string                     `json:"environment_id"`
	ID              string                     `json:"id"`
	ConversionEvent string                     `json:"conversion_event_type"`
	Variants        []ExperimentVariantRequest `json:"variants"`
	From            *time.Time                 `json:"from,omitempty"`
	To              *time.Time                 `json:"to,omitempty"`
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

func ValidateEnvironmentRequest(request EnvironmentRequest) (EnvironmentRequest, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.ID = strings.TrimSpace(request.ID)
	request.Name = strings.TrimSpace(request.Name)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.ID) {
		return EnvironmentRequest{}, fmt.Errorf("tenant_id and environment_id must contain 1-64 lowercase letters, numbers, '_' or '-'")
	}
	if request.Name == "" || len(request.Name) > 120 {
		return EnvironmentRequest{}, fmt.Errorf("name must be between 1 and 120 characters")
	}
	return request, nil
}

func ValidateGoalRequest(request GoalRequest) (GoalRequest, error) {
	request.TenantID, request.EnvironmentID, request.ID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.EnvironmentID), strings.TrimSpace(request.ID)
	request.Name, request.Description, request.EventType = strings.TrimSpace(request.Name), strings.TrimSpace(request.Description), strings.TrimSpace(request.EventType)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) || !identifierPattern.MatchString(request.ID) {
		return GoalRequest{}, fmt.Errorf("tenant_id, environment_id, and id must be valid identifiers")
	}
	if request.Name == "" || len(request.Name) > 120 || request.Description != "" && len(request.Description) > 500 {
		return GoalRequest{}, fmt.Errorf("name is required and description must be 500 characters or fewer")
	}
	if !eventTypePattern.MatchString(request.EventType) || request.TargetCount < 1 {
		return GoalRequest{}, fmt.Errorf("event_type and positive target_count are required")
	}
	return request, nil
}

func ValidateCampaignRequest(request CampaignRequest) (CampaignRequest, error) {
	request.TenantID, request.EnvironmentID, request.ID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.EnvironmentID), strings.TrimSpace(request.ID)
	request.GoalID, request.Name, request.Description, request.EventType = strings.TrimSpace(request.GoalID), strings.TrimSpace(request.Name), strings.TrimSpace(request.Description), strings.TrimSpace(request.EventType)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) || !identifierPattern.MatchString(request.ID) {
		return CampaignRequest{}, fmt.Errorf("tenant_id, environment_id, and id must be valid identifiers")
	}
	if request.GoalID != "" && !identifierPattern.MatchString(request.GoalID) {
		return CampaignRequest{}, fmt.Errorf("goal_id must be a valid identifier")
	}
	if request.Name == "" || len(request.Name) > 120 || request.Description != "" && len(request.Description) > 500 {
		return CampaignRequest{}, fmt.Errorf("name is required and description must be 500 characters or fewer")
	}
	if !eventTypePattern.MatchString(request.EventType) {
		return CampaignRequest{}, fmt.Errorf("event_type must contain lowercase letters, numbers, '.', '_' or '-'")
	}
	if request.StartsAt != nil && request.EndsAt != nil && !request.EndsAt.After(*request.StartsAt) {
		return CampaignRequest{}, fmt.Errorf("ends_at must be later than starts_at")
	}
	return request, nil
}

func ValidateMetricRequest(request MetricRequest) (MetricRequest, error) {
	request.TenantID, request.EnvironmentID, request.ID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.EnvironmentID), strings.TrimSpace(request.ID)
	request.Name, request.Description, request.EventType, request.Aggregation = strings.TrimSpace(request.Name), strings.TrimSpace(request.Description), strings.TrimSpace(request.EventType), strings.TrimSpace(request.Aggregation)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) || !identifierPattern.MatchString(request.ID) {
		return MetricRequest{}, fmt.Errorf("tenant_id, environment_id, and id must be valid identifiers")
	}
	if request.Name == "" || len(request.Name) > 120 || len(request.Description) > 500 {
		return MetricRequest{}, fmt.Errorf("name is required and description must be 500 characters or fewer")
	}
	if !eventTypePattern.MatchString(request.EventType) {
		return MetricRequest{}, fmt.Errorf("event_type must contain lowercase letters, numbers, '.', '_' or '-'")
	}
	if request.Aggregation != "count" && request.Aggregation != "unique_users" {
		return MetricRequest{}, fmt.Errorf("aggregation must be count or unique_users")
	}
	return request, nil
}

func validateAnalyticsScope(tenantID, environmentID string, from, to *time.Time) error {
	if !identifierPattern.MatchString(strings.TrimSpace(tenantID)) || !identifierPattern.MatchString(strings.TrimSpace(environmentID)) {
		return fmt.Errorf("tenant_id and environment_id must be valid identifiers")
	}
	if from != nil && to != nil && !to.After(*from) {
		return fmt.Errorf("to must be later than from")
	}
	return nil
}

func ValidateFunnelRequest(request FunnelRequest) (FunnelRequest, error) {
	request.TenantID, request.EnvironmentID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.EnvironmentID)
	if err := validateAnalyticsScope(request.TenantID, request.EnvironmentID, request.From, request.To); err != nil {
		return FunnelRequest{}, err
	}
	if len(request.Steps) < 2 || len(request.Steps) > 8 {
		return FunnelRequest{}, fmt.Errorf("steps must contain between 2 and 8 items")
	}
	for index := range request.Steps {
		request.Steps[index].Name = strings.TrimSpace(request.Steps[index].Name)
		request.Steps[index].EventType = strings.TrimSpace(request.Steps[index].EventType)
		if request.Steps[index].Name == "" || len(request.Steps[index].Name) > 120 {
			return FunnelRequest{}, fmt.Errorf("each step name must be between 1 and 120 characters")
		}
		if !eventTypePattern.MatchString(request.Steps[index].EventType) {
			return FunnelRequest{}, fmt.Errorf("each step event_type must contain lowercase letters, numbers, '.', '_' or '-'")
		}
	}
	return request, nil
}

func ValidateRetentionRequest(request RetentionRequest) (RetentionRequest, error) {
	request.TenantID, request.EnvironmentID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.EnvironmentID)
	request.CohortEventType, request.ReturnEventType = strings.TrimSpace(request.CohortEventType), strings.TrimSpace(request.ReturnEventType)
	if err := validateAnalyticsScope(request.TenantID, request.EnvironmentID, request.From, request.To); err != nil {
		return RetentionRequest{}, err
	}
	if !eventTypePattern.MatchString(request.CohortEventType) || !eventTypePattern.MatchString(request.ReturnEventType) {
		return RetentionRequest{}, fmt.Errorf("cohort_event_type and return_event_type must contain lowercase letters, numbers, '.', '_' or '-'")
	}
	if len(request.Intervals) == 0 {
		request.Intervals = []int{1, 7, 30}
	}
	if len(request.Intervals) > 5 {
		return RetentionRequest{}, fmt.Errorf("intervals must contain at most 5 items")
	}
	seen := make(map[int]struct{}, len(request.Intervals))
	for _, interval := range request.Intervals {
		if interval < 1 || interval > 90 {
			return RetentionRequest{}, fmt.Errorf("intervals must be between 1 and 90 days")
		}
		if _, ok := seen[interval]; ok {
			return RetentionRequest{}, fmt.Errorf("intervals must not contain duplicates")
		}
		seen[interval] = struct{}{}
	}
	return request, nil
}

func ValidateSegmentRequest(request SegmentRequest) (SegmentRequest, error) {
	request.TenantID, request.EnvironmentID, request.EventType = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.EnvironmentID), strings.TrimSpace(request.EventType)
	if err := validateAnalyticsScope(request.TenantID, request.EnvironmentID, request.From, request.To); err != nil {
		return SegmentRequest{}, err
	}
	if !eventTypePattern.MatchString(request.EventType) {
		return SegmentRequest{}, fmt.Errorf("event_type must contain lowercase letters, numbers, '.', '_' or '-'")
	}
	if request.MinEvents < 1 || request.MinEvents > 100000 {
		return SegmentRequest{}, fmt.Errorf("min_events must be between 1 and 100000")
	}
	return request, nil
}

func ValidateExperimentRequest(request ExperimentRequest) (ExperimentRequest, error) {
	request.TenantID, request.EnvironmentID, request.ID, request.ConversionEvent = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.EnvironmentID), strings.TrimSpace(request.ID), strings.TrimSpace(request.ConversionEvent)
	if err := validateAnalyticsScope(request.TenantID, request.EnvironmentID, request.From, request.To); err != nil {
		return ExperimentRequest{}, err
	}
	if !identifierPattern.MatchString(request.ID) || !eventTypePattern.MatchString(request.ConversionEvent) {
		return ExperimentRequest{}, fmt.Errorf("id and conversion_event_type must be valid identifiers")
	}
	if len(request.Variants) < 2 || len(request.Variants) > 4 {
		return ExperimentRequest{}, fmt.Errorf("variants must contain between 2 and 4 items")
	}
	totalWeight := 0
	seen := make(map[string]struct{}, len(request.Variants))
	for index := range request.Variants {
		request.Variants[index].ID = strings.TrimSpace(request.Variants[index].ID)
		if !identifierPattern.MatchString(request.Variants[index].ID) || request.Variants[index].Weight < 1 || request.Variants[index].Weight > 99 {
			return ExperimentRequest{}, fmt.Errorf("variant ids must be valid identifiers and weights must be between 1 and 99")
		}
		if _, exists := seen[request.Variants[index].ID]; exists {
			return ExperimentRequest{}, fmt.Errorf("variant ids must be unique")
		}
		seen[request.Variants[index].ID] = struct{}{}
		totalWeight += request.Variants[index].Weight
	}
	if totalWeight != 100 {
		return ExperimentRequest{}, fmt.Errorf("variant weights must add up to 100")
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
