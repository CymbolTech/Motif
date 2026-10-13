package rewards

import (
	"context"
	"errors"
	"time"
)

type DeliveryRequest struct {
	RedemptionID   string
	RewardID       string
	ExternalUserID string
	Payload        map[string]any
}

type DeliveryResult struct {
	ProviderReference string
}

// DeliveryStatus is the provider's durable view of a redemption. Providers
// may report pending while an external operation is still being resolved.
type DeliveryStatus struct {
	State             string
	ProviderReference string
	FailureReason     string
}

type Provider interface {
	Deliver(ctx context.Context, request DeliveryRequest) (DeliveryResult, error)
}

// StatusProvider is optional. A provider that implements it can reconcile
// ambiguous deliveries without issuing a second external operation.
type StatusProvider interface {
	Status(ctx context.Context, request DeliveryRequest) (DeliveryStatus, error)
}

var ErrDeliveryUnknown = errors.New("reward provider outcome is unknown")

var ErrReconciliationUnsupported = errors.New("reward provider does not support reconciliation")

type ProviderConfig struct {
	Kind      string
	URL       string
	StatusURL string
	Token     string
	Timeout   time.Duration
}

func NewProvider(config ProviderConfig) Provider {
	if config.Kind == "http" {
		return NewHTTPProviderWithStatus(config.URL, config.StatusURL, config.Token, config.Timeout)
	}
	return MockProvider{}
}
