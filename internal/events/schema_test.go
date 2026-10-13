package events

import (
	"encoding/json"
	"testing"
)

func TestValidateSchemaRequiredFields(t *testing.T) {
	schema := json.RawMessage(`{"required":["order_id","amount"]}`)
	payload := json.RawMessage(`{"order_id":"o-1","amount":10}`)
	if err := ValidateSchema(schema, payload); err != nil {
		t.Fatalf("ValidateSchema() error = %v", err)
	}
}

func TestValidateSchemaReportsMissingRequiredField(t *testing.T) {
	schema := json.RawMessage(`{"required":["order_id","amount"]}`)
	payload := json.RawMessage(`{"order_id":"o-1"}`)
	if err := ValidateSchema(schema, payload); err == nil {
		t.Fatal("ValidateSchema() error = nil, want missing field error")
	}
}

func TestValidateSchemaSupportsTypesRangesAndEnums(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","required":["amount","kind"],"properties":{"amount":{"type":"integer","minimum":1,"maximum":10},"kind":{"type":"string","enum":["purchase","refund"]}},"additionalProperties":false}`)
	if err := ValidateSchema(schema, json.RawMessage(`{"amount":5,"kind":"purchase"}`)); err != nil {
		t.Fatalf("ValidateSchema() error = %v", err)
	}
	for _, payload := range []string{`{"amount":0,"kind":"purchase"}`, `{"amount":5,"kind":"other"}`, `{"amount":"5","kind":"purchase"}`, `{"amount":5,"kind":"purchase","extra":true}`} {
		if err := ValidateSchema(schema, json.RawMessage(payload)); err == nil {
			t.Fatalf("ValidateSchema(%s) error = nil, want mismatch", payload)
		}
	}
}
