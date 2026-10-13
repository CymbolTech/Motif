package health

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandlerReportsLiveness(t *testing.T) {
	h := NewHandler(Dependencies{})
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := res.Body.String(); got != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q, want healthy JSON", got)
	}
}

func TestHandlerReportsReadinessFailureWhenDependencyUnavailable(t *testing.T) {
	h := NewHandler(Dependencies{Database: func() error { return errDependencyUnavailable }})
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	res := httptest.NewRecorder()

	h.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusServiceUnavailable)
	}
}
