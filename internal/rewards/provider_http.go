package rewards

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HTTPProvider struct {
	URL       string
	StatusURL string
	Token     string
	Client    *http.Client
	UserAgent string
}

type httpDeliveryRequest struct {
	RedemptionID   string         `json:"redemption_id"`
	RewardID       string         `json:"reward_id"`
	ExternalUserID string         `json:"external_user_id"`
	Payload        map[string]any `json:"payload"`
}

type httpDeliveryResponse struct {
	ProviderReference string `json:"provider_reference"`
	Reference         string `json:"reference"`
}

func NewHTTPProvider(url, token string, timeout time.Duration) *HTTPProvider {
	return NewHTTPProviderWithStatus(url, "", token, timeout)
}

// NewHTTPProviderWithStatus creates an HTTP provider. statusURL may contain
// the literal {redemption_id} placeholder; when omitted, reconciliation is
// reported as unsupported instead of guessing a provider URL shape.
func NewHTTPProviderWithStatus(url, statusURL, token string, timeout time.Duration) *HTTPProvider {
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	return &HTTPProvider{
		URL:       strings.TrimRight(strings.TrimSpace(url), "/"),
		StatusURL: strings.TrimRight(strings.TrimSpace(statusURL), "/"),
		Token:     token,
		Client:    &http.Client{Timeout: timeout},
		UserAgent: "motifengine-reward-provider/1",
	}
}

func (p *HTTPProvider) Deliver(ctx context.Context, request DeliveryRequest) (DeliveryResult, error) {
	if p == nil || p.URL == "" {
		return DeliveryResult{}, fmt.Errorf("reward provider URL is not configured")
	}
	body, err := json.Marshal(httpDeliveryRequest{RedemptionID: request.RedemptionID, RewardID: request.RewardID, ExternalUserID: request.ExternalUserID, Payload: request.Payload})
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("encode delivery request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(body))
	if err != nil {
		return DeliveryResult{}, fmt.Errorf("create delivery request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", p.UserAgent)
	req.Header.Set("Idempotency-Key", request.RedemptionID)
	req.Header.Set("X-Motif-Redemption-ID", request.RedemptionID)
	if p.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.Token)
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return DeliveryResult{}, fmt.Errorf("%w: %v", ErrDeliveryUnknown, err)
		}
		return DeliveryResult{}, fmt.Errorf("provider request failed: %w", err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if readErr != nil {
		return DeliveryResult{}, fmt.Errorf("read provider response: %w", readErr)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if response.StatusCode >= 500 {
			return DeliveryResult{}, fmt.Errorf("%w: provider returned HTTP %d", ErrDeliveryUnknown, response.StatusCode)
		}
		return DeliveryResult{}, fmt.Errorf("provider rejected delivery with HTTP %d", response.StatusCode)
	}
	var decoded httpDeliveryResponse
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		return DeliveryResult{}, fmt.Errorf("provider response is not valid JSON: %w", err)
	}
	reference := strings.TrimSpace(decoded.ProviderReference)
	if reference == "" {
		reference = strings.TrimSpace(decoded.Reference)
	}
	if reference == "" {
		return DeliveryResult{}, fmt.Errorf("provider response did not include provider_reference")
	}
	return DeliveryResult{ProviderReference: reference}, nil
}

func (p *HTTPProvider) Status(ctx context.Context, request DeliveryRequest) (DeliveryStatus, error) {
	if p == nil || strings.TrimSpace(p.StatusURL) == "" {
		return DeliveryStatus{}, ErrReconciliationUnsupported
	}
	statusURL := strings.ReplaceAll(p.StatusURL, "{redemption_id}", url.PathEscape(request.RedemptionID))
	if !strings.Contains(p.StatusURL, "{redemption_id}") {
		statusURL = strings.TrimRight(statusURL, "/") + "/" + url.PathEscape(request.RedemptionID)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, statusURL, nil)
	if err != nil {
		return DeliveryStatus{}, fmt.Errorf("create status request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", p.UserAgent)
	req.Header.Set("Idempotency-Key", request.RedemptionID)
	req.Header.Set("X-Motif-Redemption-ID", request.RedemptionID)
	if p.Token != "" {
		req.Header.Set("Authorization", "Bearer "+p.Token)
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return DeliveryStatus{}, fmt.Errorf("%w: %v", ErrDeliveryUnknown, err)
		}
		return DeliveryStatus{}, fmt.Errorf("provider status request failed: %w", err)
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 64*1024))
	if readErr != nil {
		return DeliveryStatus{}, fmt.Errorf("read provider status response: %w", readErr)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		if response.StatusCode == http.StatusNotFound || response.StatusCode >= http.StatusInternalServerError {
			return DeliveryStatus{}, fmt.Errorf("%w: provider status returned HTTP %d", ErrDeliveryUnknown, response.StatusCode)
		}
		return DeliveryStatus{}, fmt.Errorf("provider status rejected request with HTTP %d", response.StatusCode)
	}
	var decoded struct {
		State             string `json:"state"`
		Status            string `json:"status"`
		ProviderReference string `json:"provider_reference"`
		Reference         string `json:"reference"`
		FailureReason     string `json:"failure_reason"`
		Error             string `json:"error"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return DeliveryStatus{}, fmt.Errorf("provider status response is not valid JSON: %w", err)
	}
	state := strings.ToLower(strings.TrimSpace(decoded.State))
	if state == "" {
		state = strings.ToLower(strings.TrimSpace(decoded.Status))
	}
	switch state {
	case "fulfilled", "success", "succeeded", "completed":
		state = "fulfilled"
	case "failed", "failure", "rejected", "cancelled", "canceled":
		state = "failed"
	case "pending", "processing", "requested", "reserved", "delivering", "unknown":
		state = "pending"
	default:
		return DeliveryStatus{}, fmt.Errorf("provider status response has unsupported status %q", state)
	}
	reference := strings.TrimSpace(decoded.ProviderReference)
	if reference == "" {
		reference = strings.TrimSpace(decoded.Reference)
	}
	failureReason := strings.TrimSpace(decoded.FailureReason)
	if failureReason == "" {
		failureReason = strings.TrimSpace(decoded.Error)
	}
	return DeliveryStatus{State: state, ProviderReference: reference, FailureReason: failureReason}, nil
}
