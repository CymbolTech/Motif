package rewards

import (
	"context"
	"encoding/json"
	"testing"
)

func TestValidateRewardRequestNormalizesDigitalUnlock(t *testing.T) {
	request, err := ValidateRewardRequest(RewardRequest{
		TenantID:      " acme ",
		EnvironmentID: " prod ",
		Name:          "Welcome unlock",
		RewardType:    "digital_unlock",
		CostCurrency:  " loyalty ",
		CostAmount:    25,
		Payload:       json.RawMessage(`{"sku":"starter"}`),
	})
	if err != nil {
		t.Fatalf("ValidateRewardRequest() error = %v", err)
	}
	if request.TenantID != "acme" || request.EnvironmentID != "prod" || request.CostCurrency != "loyalty" {
		t.Fatalf("request was not normalized: %+v", request)
	}
	if string(request.Payload) != `{"sku":"starter"}` {
		t.Fatalf("payload = %s, want canonical object", request.Payload)
	}
}

func TestValidateRewardRequestRejectsPartialCost(t *testing.T) {
	_, err := ValidateRewardRequest(RewardRequest{
		TenantID:      "acme",
		EnvironmentID: "prod",
		Name:          "Free unlock",
		RewardType:    "digital_unlock",
		CostCurrency:  "loyalty",
	})
	if err == nil {
		t.Fatal("ValidateRewardRequest() error = nil, want partial cost error")
	}
}

func TestValidateRewardRequestRejectsZeroPaidCost(t *testing.T) {
	_, err := ValidateRewardRequest(RewardRequest{
		TenantID: "acme", EnvironmentID: "prod", Name: "Invalid unlock", RewardType: "digital_unlock",
		CostCurrency: "loyalty", CostAmount: 0,
	})
	if err == nil {
		t.Fatal("ValidateRewardRequest() error = nil, want positive cost error")
	}
}

func TestMockProviderIsDeterministic(t *testing.T) {
	provider := MockProvider{}
	request := DeliveryRequest{RedemptionID: "redemption_1", RewardID: "reward_1", ExternalUserID: "user-1"}
	first, err := provider.Deliver(context.Background(), request)
	if err != nil {
		t.Fatalf("Deliver() error = %v", err)
	}
	second, err := provider.Deliver(context.Background(), request)
	if err != nil {
		t.Fatalf("second Deliver() error = %v", err)
	}
	if first.ProviderReference != second.ProviderReference || first.ProviderReference != "mock:redemption_1" {
		t.Fatalf("references = %q and %q", first.ProviderReference, second.ProviderReference)
	}
}
