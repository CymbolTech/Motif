package transport

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRateLimiterBurstAndRefill(t *testing.T) {
	limiter := NewRateLimiter(1000, 2)
	if !limiter.Allow("client") || !limiter.Allow("client") || limiter.Allow("client") {
		t.Fatal("limiter did not enforce burst")
	}
}

func TestProtectAddsRequestIDAndCORS(t *testing.T) {
	handler := Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestID(r.Context()) == "" {
			t.Error("request id missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	}), ProtectionConfig{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), AllowedOrigins: []string{"http://localhost:3000"}, Limiter: NewRateLimiter(10, 5)})
	req := httptest.NewRequest(http.MethodGet, "http://api.test/v1/events", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent || res.Header().Get("X-Request-ID") == "" || res.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
		t.Fatalf("unexpected protected response: %d %#v", res.Code, res.Header())
	}
}

func TestProtectRejectsUnknownCORSPreflight(t *testing.T) {
	handler := Protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}), ProtectionConfig{AllowedOrigins: []string{"http://localhost:3000"}, Limiter: NewRateLimiter(10, 5)})
	req := httptest.NewRequest(http.MethodOptions, "http://api.test/v1/events", nil)
	req.Header.Set("Origin", "https://evil.example")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden || strings.TrimSpace(res.Body.String()) != "" {
		t.Fatalf("unexpected CORS response: %d %q", res.Code, res.Body.String())
	}
}
