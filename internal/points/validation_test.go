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
