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
