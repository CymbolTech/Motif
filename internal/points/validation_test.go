package points

import (
	"encoding/json"
	"testing"
)

func TestValidateCurrencyRequest(t *testing.T) {
	request, err := ValidateCurrencyRequest(CurrencyRequest{TenantID: "demo-tenant", EnvironmentID: "sandbox", Code: "loyalty", Name: "Loyalty points", UnitScale: 0, Policy: json.RawMessage(`{"expiry_days":30}`)})
	if err != nil {
		t.Fatalf("ValidateCurrencyRequest() error = %v", err)
	}
	if string(request.Policy) != `{"expiry_days":30}` {
		t.Fatalf("policy = %s", request.Policy)
	}
	for _, request := range []CurrencyRequest{
		{TenantID: "tenant", EnvironmentID: "sandbox", Code: "Bad", Name: "Points"},
		{TenantID: "tenant", EnvironmentID: "sandbox", Code: "points", Name: "Points", UnitScale: 7},
		{TenantID: "tenant", EnvironmentID: "sandbox", Code: "points", Name: "Points", Policy: json.RawMessage(`[]`)},
		{TenantID: "tenant", EnvironmentID: "sandbox", Code: "points", Name: "Points", Policy: json.RawMessage(`{"expiry_days":"30"}`)},
		{TenantID: "tenant", EnvironmentID: "sandbox", Code: "points", Name: "Points", Policy: json.RawMessage(`{"expiry_days":-1}`)},
	} {
		if _, err := ValidateCurrencyRequest(request); err == nil {
			t.Fatal("expected invalid currency request")
		}
	}
}

func TestValidateAction(t *testing.T) {
	if _, err := ValidateAction(json.RawMessage(`{"type":"award_points","currency":"loyalty","amount":100}`)); err != nil {
		t.Fatalf("award_points action rejected: %v", err)
	}
	if _, err := ValidateAction(json.RawMessage(`{"type":"award_xp","amount":25}`)); err != nil {
		t.Fatalf("award_xp action rejected: %v", err)
	}
	if _, err := ValidateAction(json.RawMessage(`{"type":"award_xp","amount":9000000000000000000}`)); err != nil {
		t.Fatalf("large integer award_xp action rejected: %v", err)
	}
	for _, raw := range []string{
		`{"type":"award_points","currency":"Loyalty","amount":100}`,
		`{"type":"award_points","currency":"loyalty","amount":1.5}`,
		`{"type":"award_xp","amount":0}`,
		`{"type":"http","url":"https://example.test"}`,
		`{"type":"award_xp","amount":1.5}`,
		`{"type":"award_xp","amount":25} {"type":"log"}`,
	} {
		if _, err := ValidateAction(json.RawMessage(raw)); err == nil {
			t.Fatalf("ValidateAction(%s) accepted invalid action", raw)
		}
	}
}

func TestValidateActionsAcceptsMultipleTypedActions(t *testing.T) {
	raw := json.RawMessage(`[
        {"type":"award_xp","amount":10},
        {"type":"award_points","currency":"coins","amount":5}
    ]`)
	canonical, err := ValidateActions(raw)
	if err != nil {
		t.Fatalf("ValidateActions() error = %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(canonical, &got); err != nil {
		t.Fatalf("canonical actions are not JSON: %v", err)
	}
	if len(got) != 2 || got[0]["type"] != "award_xp" || got[0]["amount"] != float64(10) || got[1]["type"] != "award_points" || got[1]["currency"] != "coins" || got[1]["amount"] != float64(5) {
		t.Fatalf("canonical actions = %s", canonical)
	}
}

func TestValidateActionsWrapsLegacySingleAction(t *testing.T) {
	canonical, err := ValidateActions(json.RawMessage(`{"type":"award_xp","amount":10}`))
	if err != nil {
		t.Fatalf("ValidateActions() error = %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(canonical, &got); err != nil {
		t.Fatalf("canonical action is not JSON: %v", err)
	}
	if len(got) != 1 || got[0]["type"] != "award_xp" || got[0]["amount"] != float64(10) {
		t.Fatalf("canonical legacy action = %s", canonical)
	}
}

func TestValidateActionsRejectsEmptyOrInvalidLists(t *testing.T) {
	for _, raw := range []string{
		`[]`,
		`[{"type":"award_xp","amount":0}]`,
		`[{"type":"award_xp","amount":10}, {"type":"unknown"}]`,
		`["award_xp"]`,
	} {
		if _, err := ValidateActions(json.RawMessage(raw)); err == nil {
			t.Fatalf("ValidateActions(%s) returned nil error", raw)
		}
	}
}
