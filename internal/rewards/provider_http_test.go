package rewards

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPProviderDeliversIdempotently(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Idempotency-Key") != "redemption-1" {
			t.Fatalf("unexpected provider request: %s %s auth=%q key=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"), r.Header.Get("Idempotency-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"provider_reference":"provider-1"}`))
	}))
	defer server.Close()

	provider := NewHTTPProvider(server.URL, "secret", time.Second)
	result, err := provider.Deliver(context.Background(), DeliveryRequest{RedemptionID: "redemption-1", RewardID: "reward-1", ExternalUserID: "user-1", Payload: map[string]any{"kind": "unlock"}})
	if err != nil || result.ProviderReference != "provider-1" {
		t.Fatalf("Deliver() = %+v, %v", result, err)
	}
}

func TestHTTPProviderMapsServerFailureToUnknownOutcome(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "upstream", http.StatusBadGateway) }))
	defer server.Close()

	_, err := NewHTTPProvider(server.URL, "", time.Second).Deliver(context.Background(), DeliveryRequest{RedemptionID: "redemption-1"})
	if !errors.Is(err, ErrDeliveryUnknown) {
		t.Fatalf("Deliver() error = %v, want ErrDeliveryUnknown", err)
	}
}

func TestHTTPProviderRejectsMissingReference(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{}`)) }))
	defer server.Close()
	_, err := NewHTTPProvider(server.URL, "", time.Second).Deliver(context.Background(), DeliveryRequest{RedemptionID: "redemption-1"})
	if err == nil {
		t.Fatal("Deliver() error = nil, want missing reference error")
	}
}

func TestHTTPProviderStatusReadsProviderOutcome(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/status/redemption-1" {
			t.Fatalf("unexpected status request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Idempotency-Key") != "redemption-1" {
			t.Fatalf("missing status auth headers: auth=%q key=%q", r.Header.Get("Authorization"), r.Header.Get("Idempotency-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"fulfilled","provider_reference":"provider-1"}`))
	}))
	defer server.Close()

	provider := NewHTTPProviderWithStatus(server.URL+"/deliver", server.URL+"/status/{redemption_id}", "secret", time.Second)
	status, err := provider.Status(context.Background(), DeliveryRequest{RedemptionID: "redemption-1"})
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.State != "fulfilled" || status.ProviderReference != "provider-1" {
		t.Fatalf("Status() = %+v", status)
	}
}

func TestHTTPProviderStatusRejectsUnknownState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"surprising"}`))
	}))
	defer server.Close()

	_, err := NewHTTPProviderWithStatus(server.URL, server.URL, "", time.Second).Status(context.Background(), DeliveryRequest{RedemptionID: "redemption-1"})
	if err == nil {
		t.Fatal("Status() error = nil, want invalid status error")
	}
}

func TestHTTPProviderStatusRequiresEndpoint(t *testing.T) {
	_, err := NewHTTPProviderWithStatus("http://provider.test/deliver", "", "", time.Second).Status(context.Background(), DeliveryRequest{RedemptionID: "redemption-1"})
	if !errors.Is(err, ErrReconciliationUnsupported) {
		t.Fatalf("Status() error = %v, want ErrReconciliationUnsupported", err)
	}
}

func TestHTTPProviderStatusMapsPending(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"state":"processing"}`))
	}))
	defer server.Close()

	status, err := NewHTTPProviderWithStatus(server.URL, server.URL, "", time.Second).Status(context.Background(), DeliveryRequest{RedemptionID: "redemption-1"})
	if err != nil || status.State != "pending" {
		t.Fatalf("Status() = %+v, %v, want pending", status, err)
	}
}

func TestHTTPProviderStatusTimeoutIsUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	_, err := NewHTTPProviderWithStatus(server.URL, server.URL, "", 5*time.Millisecond).Status(context.Background(), DeliveryRequest{RedemptionID: "redemption-1"})
	if !errors.Is(err, ErrDeliveryUnknown) {
		t.Fatalf("Status() error = %v, want ErrDeliveryUnknown", err)
	}
}
