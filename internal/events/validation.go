package events

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const maxPayloadBytes = 64 * 1024

type Request struct {
	ExternalEventID string          `json:"event_id"`
	EventType       string          `json:"event_type"`
	OccurredAt      string          `json:"occurred_at,omitempty"`
	UserID          string          `json:"user_id,omitempty"`
	Payload         json.RawMessage `json:"payload"`
}

type ValidatedRequest struct {
	Request
	CanonicalPayload json.RawMessage
	PayloadHash      string
}

func Validate(request Request) (ValidatedRequest, error) {
	request.ExternalEventID = strings.TrimSpace(request.ExternalEventID)
	request.EventType = strings.TrimSpace(request.EventType)
	if request.ExternalEventID == "" || len(request.ExternalEventID) > 128 {
		return ValidatedRequest{}, fmt.Errorf("event_id must be between 1 and 128 characters")
	}
	if request.EventType == "" || len(request.EventType) > 128 {
		return ValidatedRequest{}, fmt.Errorf("event_type must be between 1 and 128 characters")
	}
	if len(request.Payload) == 0 {
		request.Payload = json.RawMessage(`{}`)
	}
	if len(request.Payload) > maxPayloadBytes {
		return ValidatedRequest{}, fmt.Errorf("payload exceeds %d bytes", maxPayloadBytes)
	}
	var object map[string]any
	if err := json.Unmarshal(request.Payload, &object); err != nil {
		return ValidatedRequest{}, fmt.Errorf("payload must be a JSON object: %w", err)
	}
	if object == nil {
		return ValidatedRequest{}, fmt.Errorf("payload must be a JSON object")
	}
	canonical, err := json.Marshal(object)
	if err != nil {
		return ValidatedRequest{}, fmt.Errorf("payload could not be normalized: %w", err)
	}
	if len(canonical) > maxPayloadBytes {
		return ValidatedRequest{}, fmt.Errorf("payload exceeds %d bytes", maxPayloadBytes)
	}
	hash := sha256.Sum256(canonical)
	return ValidatedRequest{Request: request, CanonicalPayload: canonical, PayloadHash: hex.EncodeToString(hash[:])}, nil
}
