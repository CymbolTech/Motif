package rewards

import (
	"context"
	"fmt"
)

type MockProvider struct{}

func (MockProvider) Deliver(_ context.Context, request DeliveryRequest) (DeliveryResult, error) {
	return DeliveryResult{ProviderReference: fmt.Sprintf("mock:%s", request.RedemptionID)}, nil
}
