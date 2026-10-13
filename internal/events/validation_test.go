package events

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateRequestAcceptsBoundedObjectPayload(t *testing.T) {
	req := Request{
		ExternalEventID: "checkout-123",
		EventType:       "order.completed",
		Payload:         json.RawMessage(`{"order_id":"o-1","total":42}`),
	}

	validated, err := Validate(req)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if validated.PayloadHash == "" {
		t.Fatal("Validate() returned an empty payload hash")
	}
}

func TestValidateRejectsOversizedPayload(t *testing.T) {
	req := Request{ExternalEventID: "checkout-123", EventType: "order.completed", Payload: json.RawMessage(`{"body":"` + strings.Repeat("x", maxPayloadBytes) + `"}`)}
	if _, err := Validate(req); err == nil {
		t.Fatal("Validate() error = nil, want payload size error")
	}
}

func TestValidateRejectsNonObjectPayload(t *testing.T) {
	req := Request{ExternalEventID: "checkout-123", EventType: "order.completed", Payload: json.RawMessage(`[]`)}
	if _, err := Validate(req); err == nil {
		t.Fatal("Validate() error = nil, want object payload error")
	}
}

func TestValidateRejectsNullPayload(t *testing.T) {
	req := Request{ExternalEventID: "checkout-123", EventType: "order.completed", Payload: json.RawMessage(`null`)}
	if _, err := Validate(req); err == nil {
		t.Fatal("Validate() error = nil, want null payload error")
	}
}
