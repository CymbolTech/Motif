package admin

import (
	"encoding/json"
	"testing"
	"time"
)

func TestValidateTenantRequest(t *testing.T) {
	request, err := ValidateTenantRequest(TenantRequest{
		TenantID:        "acme",
		Name:            "Acme",
		EnvironmentID:   "sandbox",
		EnvironmentName: "Sandbox",
	})
	if err != nil {
		t.Fatalf("ValidateTenantRequest() error = %v", err)
	}
	if request.TenantID != "acme" || request.EnvironmentID != "sandbox" {
		t.Fatalf("unexpected normalized request: %+v", request)
	}
}

func TestValidateTenantSignupRequestNormalizesDefaults(t *testing.T) {
	request, err := ValidateTenantSignupRequest(TenantSignupRequest{Email: " Owner@Acme.Example ", Password: "a-long-password", TenantID: " acme ", Name: " Acme Rewards "})
	if err != nil {
		t.Fatalf("ValidateTenantSignupRequest() error = %v", err)
	}
	if request.Email != "owner@acme.example" || request.EnvironmentID != "production" || request.EnvironmentName != "Production" {
		t.Fatalf("unexpected normalized signup request: %+v", request)
	}
}

func TestValidateTenantSignupRequestRejectsShortPassword(t *testing.T) {
	_, err := ValidateTenantSignupRequest(TenantSignupRequest{Email: "owner@acme.example", Password: "short", TenantID: "acme", Name: "Acme"})
	if err == nil {
		t.Fatal("ValidateTenantSignupRequest() error = nil, want password validation error")
	}
}

func TestValidateEventTypeRequest(t *testing.T) {
	request, err := ValidateEventTypeRequest(EventTypeRequest{
		TenantID:      "acme",
		EnvironmentID: "sandbox",
		Name:          "purchase.completed",
		Schema:        json.RawMessage(`{"required":["order_id"]}`),
	})
	if err != nil {
		t.Fatalf("ValidateEventTypeRequest() error = %v", err)
	}
	if request.Name != "purchase.completed" {
		t.Fatalf("unexpected event type: %+v", request)
	}
}

func TestValidateEventTypeRequestRejectsNullSchema(t *testing.T) {
	_, err := ValidateEventTypeRequest(EventTypeRequest{TenantID: "acme", EnvironmentID: "sandbox", Name: "purchase.completed", Schema: json.RawMessage(`null`)})
	if err == nil {
		t.Fatal("ValidateEventTypeRequest() error = nil, want null schema error")
	}
}

func TestValidateTenantRequestRejectsUnsafeIDs(t *testing.T) {
	_, err := ValidateTenantRequest(TenantRequest{
		TenantID:        "Acme Corp",
		Name:            "Acme",
		EnvironmentID:   "sandbox",
		EnvironmentName: "Sandbox",
	})
	if err == nil {
		t.Fatal("ValidateTenantRequest() error = nil, want unsafe tenant ID error")
	}
}

func TestValidateEnvironmentRequest(t *testing.T) {
	request, err := ValidateEnvironmentRequest(EnvironmentRequest{TenantID: " acme ", ID: " production ", Name: "Production"})
	if err != nil {
		t.Fatalf("ValidateEnvironmentRequest() error = %v", err)
	}
	if request.TenantID != "acme" || request.ID != "production" {
		t.Fatalf("unexpected normalized environment: %+v", request)
	}
}

func TestValidateRuleRequestAcceptsScheduleWindow(t *testing.T) {
	starts := time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC)
	ends := starts.Add(time.Hour)
	request, err := ValidateRuleRequest(RuleRequest{
		TenantID: "acme", EnvironmentID: "sandbox", Name: "scheduled", EventType: "purchase.completed",
		Conditions: json.RawMessage(`{}`), Action: json.RawMessage(`{"type":"log"}`), StartsAt: &starts, EndsAt: &ends,
	})
	if err != nil {
		t.Fatalf("ValidateRuleRequest() error = %v", err)
	}
	if request.StartsAt == nil || request.EndsAt == nil || !request.EndsAt.Equal(ends) {
		t.Fatalf("schedule window was not preserved: %+v", request)
	}
}

func TestValidateRuleRequestAcceptsMultipleActions(t *testing.T) {
	request, err := ValidateRuleRequest(RuleRequest{
		TenantID: "acme", EnvironmentID: "sandbox", Name: "purchase reward", EventType: "order.completed",
		Conditions: json.RawMessage(`{}`), Actions: json.RawMessage(`[
            {"type":"award_xp","amount":10},
            {"type":"award_points","currency":"coins","amount":5}
        ]`),
	})
	if err != nil {
		t.Fatalf("ValidateRuleRequest() error = %v", err)
	}
	var actions []map[string]any
	if err := json.Unmarshal(request.Action, &actions); err != nil {
		t.Fatalf("normalized action is not an array: %v", err)
	}
	if len(actions) != 2 || actions[0]["type"] != "award_xp" || actions[1]["type"] != "award_points" {
		t.Fatalf("normalized actions = %s", request.Action)
	}
}

func TestValidateRuleRequestRejectsInvertedScheduleWindow(t *testing.T) {
	starts := time.Date(2026, time.January, 2, 11, 0, 0, 0, time.UTC)
	ends := starts
	_, err := ValidateRuleRequest(RuleRequest{
		TenantID: "acme", EnvironmentID: "sandbox", Name: "scheduled", EventType: "purchase.completed",
		Conditions: json.RawMessage(`{}`), Action: json.RawMessage(`{"type":"log"}`), StartsAt: &starts, EndsAt: &ends,
	})
	if err == nil {
		t.Fatal("ValidateRuleRequest() error = nil, want inverted schedule error")
	}
}

func TestValidateGoalAndCampaignRequests(t *testing.T) {
	goal, err := ValidateGoalRequest(GoalRequest{TenantID: "acme", EnvironmentID: "sandbox", ID: "repeat_purchase", Name: "Repeat purchase", EventType: "order.completed", TargetCount: 2})
	if err != nil || goal.ID != "repeat_purchase" {
		t.Fatalf("ValidateGoalRequest() = %+v, %v", goal, err)
	}
	starts := time.Date(2026, time.January, 2, 10, 0, 0, 0, time.UTC)
	ends := starts.Add(24 * time.Hour)
	campaign, err := ValidateCampaignRequest(CampaignRequest{TenantID: "acme", EnvironmentID: "sandbox", ID: "january_return", GoalID: "repeat_purchase", Name: "January return", EventType: "order.completed", StartsAt: &starts, EndsAt: &ends})
	if err != nil || campaign.GoalID != "repeat_purchase" {
		t.Fatalf("ValidateCampaignRequest() = %+v, %v", campaign, err)
	}
}

func TestValidateMetricRequest(t *testing.T) {
	metric, err := ValidateMetricRequest(MetricRequest{TenantID: " acme ", EnvironmentID: " sandbox ", ID: "repeat_buyers", Name: "Repeat buyers", Description: "Users with a second order", EventType: "order.completed", Aggregation: "unique_users"})
	if err != nil {
		t.Fatalf("ValidateMetricRequest() error = %v", err)
	}
	if metric.TenantID != "acme" || metric.EnvironmentID != "sandbox" || metric.Aggregation != "unique_users" {
		t.Fatalf("unexpected normalized metric: %+v", metric)
	}
}

func TestValidateMetricRequestRejectsUnsupportedAggregation(t *testing.T) {
	_, err := ValidateMetricRequest(MetricRequest{TenantID: "acme", EnvironmentID: "sandbox", ID: "orders", Name: "Orders", EventType: "order.completed", Aggregation: "sum"})
	if err == nil {
		t.Fatal("ValidateMetricRequest() error = nil, want unsupported aggregation error")
	}
}

func TestValidateFunnelRequest(t *testing.T) {
	request, err := ValidateFunnelRequest(FunnelRequest{TenantID: "acme", EnvironmentID: "sandbox", Steps: []FunnelStepRequest{{Name: "Viewed", EventType: "product.viewed"}, {Name: "Purchased", EventType: "order.completed"}}})
	if err != nil || len(request.Steps) != 2 {
		t.Fatalf("ValidateFunnelRequest() = %+v, %v", request, err)
	}
}

func TestValidateFunnelRequestRejectsShortFunnel(t *testing.T) {
	_, err := ValidateFunnelRequest(FunnelRequest{TenantID: "acme", EnvironmentID: "sandbox", Steps: []FunnelStepRequest{{Name: "Viewed", EventType: "product.viewed"}}})
	if err == nil {
		t.Fatal("ValidateFunnelRequest() error = nil, want at least two steps")
	}
}

func TestValidateRetentionRequestDefaultsIntervals(t *testing.T) {
	request, err := ValidateRetentionRequest(RetentionRequest{TenantID: "acme", EnvironmentID: "sandbox", CohortEventType: "signup.completed", ReturnEventType: "session.started"})
	if err != nil || len(request.Intervals) != 3 || request.Intervals[1] != 7 {
		t.Fatalf("ValidateRetentionRequest() = %+v, %v", request, err)
	}
}

func TestValidateSegmentAndExperimentRequests(t *testing.T) {
	segment, err := ValidateSegmentRequest(SegmentRequest{TenantID: "acme", EnvironmentID: "sandbox", EventType: "order.completed", MinEvents: 2})
	if err != nil || segment.MinEvents != 2 {
		t.Fatalf("ValidateSegmentRequest() = %+v, %v", segment, err)
	}
	experiment, err := ValidateExperimentRequest(ExperimentRequest{TenantID: "acme", EnvironmentID: "sandbox", ID: "checkout_copy", ConversionEvent: "order.completed", Variants: []ExperimentVariantRequest{{ID: "control", Weight: 50}, {ID: "variant", Weight: 50}}})
	if err != nil || len(experiment.Variants) != 2 {
		t.Fatalf("ValidateExperimentRequest() = %+v, %v", experiment, err)
	}
}

func TestValidateExperimentRequestRejectsUnbalancedWeights(t *testing.T) {
	_, err := ValidateExperimentRequest(ExperimentRequest{TenantID: "acme", EnvironmentID: "sandbox", ID: "checkout_copy", ConversionEvent: "order.completed", Variants: []ExperimentVariantRequest{{ID: "control", Weight: 40}, {ID: "variant", Weight: 40}}})
	if err == nil {
		t.Fatal("ValidateExperimentRequest() error = nil, want weight validation error")
	}
}
