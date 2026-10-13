package rewards

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type RewardRequest struct {
	TenantID      string          `json:"tenant_id"`
	EnvironmentID string          `json:"environment_id"`
	Name          string          `json:"name"`
	RewardType    string          `json:"reward_type"`
	Description   string          `json:"description"`
	CostCurrency  string          `json:"cost_currency,omitempty"`
	CostAmount    int64           `json:"cost_amount,omitempty"`
	Payload       json.RawMessage `json:"payload"`
}

type Reward struct {
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	EnvironmentID string          `json:"environment_id"`
	Name          string          `json:"name"`
	RewardType    string          `json:"reward_type"`
	Description   string          `json:"description"`
	CostCurrency  string          `json:"cost_currency,omitempty"`
	CostAmount    int64           `json:"cost_amount,omitempty"`
	Payload       json.RawMessage `json:"payload"`
	Active        bool            `json:"active"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type Redemption struct {
	ID                string    `json:"id"`
	RewardID          string    `json:"reward_id"`
	ExternalUserID    string    `json:"external_user_id"`
	Status            string    `json:"status"`
	IdempotencyKey    string    `json:"idempotency_key"`
	ProviderReference string    `json:"provider_reference,omitempty"`
	FailureReason     string    `json:"failure_reason,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

var identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

// ValidateRewardRequest normalizes the admin payload and keeps the initial
// reward surface intentionally small. A reward may be free or may charge one
// active point currency, but it must never contain a partial cost.
func ValidateRewardRequest(request RewardRequest) (RewardRequest, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.Name = strings.TrimSpace(request.Name)
	request.RewardType = strings.TrimSpace(request.RewardType)
	request.Description = strings.TrimSpace(request.Description)
	request.CostCurrency = strings.TrimSpace(request.CostCurrency)

	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) {
		return RewardRequest{}, fmt.Errorf("tenant_id and environment_id must be valid identifiers")
	}
	if request.Name == "" || len(request.Name) > 120 {
		return RewardRequest{}, fmt.Errorf("name must be between 1 and 120 characters")
	}
	if request.RewardType != "digital_unlock" {
		return RewardRequest{}, fmt.Errorf("reward_type must be digital_unlock")
	}
	if len(request.Description) > 500 {
		return RewardRequest{}, fmt.Errorf("description must be 500 characters or fewer")
	}
	if request.CostCurrency == "" && request.CostAmount != 0 {
		return RewardRequest{}, fmt.Errorf("cost_currency and a positive cost_amount must be provided together")
	}
	if request.CostCurrency != "" && request.CostAmount <= 0 {
		return RewardRequest{}, fmt.Errorf("cost_currency and a positive cost_amount must be provided together")
	}
	if request.CostCurrency != "" && !identifierPattern.MatchString(request.CostCurrency) {
		return RewardRequest{}, fmt.Errorf("cost_currency must be a valid identifier")
	}
	if len(request.Payload) == 0 {
		request.Payload = json.RawMessage(`{}`)
	}
	var payload map[string]any
	if err := json.Unmarshal(request.Payload, &payload); err != nil || payload == nil {
		return RewardRequest{}, fmt.Errorf("payload must be a JSON object")
	}
	canonical, err := json.Marshal(payload)
	if err != nil {
		return RewardRequest{}, fmt.Errorf("payload could not be normalized: %w", err)
	}
	request.Payload = canonical
	return request, nil
}
