package observability

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsUsesBoundedRouteLabels(t *testing.T) {
	metrics := NewMetrics()
	metrics.ObserveRequest("GET", "/v1/events/event-a", 200)
	metrics.ObserveRequest("GET", "/v1/events/event-b", 500)
	metrics.ObserveEvent("accepted")
	recorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	body := recorder.Body.String()
	if !strings.Contains(body, `route="/v1/events/:id"`) || strings.Contains(body, "event-a") || !strings.Contains(body, `status="5xx"`) || !strings.Contains(body, `motif_event_receipts_total{outcome="accepted"} 1`) {
		t.Fatalf("unexpected metrics output: %s", body)
	}
}
