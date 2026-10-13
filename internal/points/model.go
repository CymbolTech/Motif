package points

import (
	"encoding/json"
	"time"
)

type CurrencyRequest struct {
	TenantID      string          `json:"tenant_id"`
	EnvironmentID string          `json:"environment_id"`
	Code          string          `json:"code"`
	Name          string          `json:"name"`
	UnitScale     int             `json:"unit_scale"`
	Policy        json.RawMessage `json:"policy"`
}

type Currency struct {
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	EnvironmentID string          `json:"environment_id"`
	Code          string          `json:"code"`
	Name          string          `json:"name"`
	UnitScale     int             `json:"unit_scale"`
	PolicyVersion int             `json:"policy_version"`
	Policy        json.RawMessage `json:"policy"`
	Active        bool            `json:"active"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}
