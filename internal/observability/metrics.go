package observability

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type Metrics struct {
	mu             sync.RWMutex
	requests       map[requestMetric]int64
	requestTotal   int64
	providerTotals map[string]int64
	eventTotals    map[string]int64
}

type requestMetric struct {
	Method string
	Route  string
	Status string
}

func NewMetrics() *Metrics {
	return &Metrics{requests: make(map[requestMetric]int64), providerTotals: make(map[string]int64), eventTotals: make(map[string]int64)}
}

func (m *Metrics) ObserveRequest(method, path string, status int) {
	if m == nil {
		return
	}
	metric := requestMetric{Method: method, Route: routeLabel(path), Status: statusClass(status)}
	m.mu.Lock()
	m.requests[metric]++
	m.requestTotal++
	m.mu.Unlock()
}

func (m *Metrics) ObserveProvider(status string) {
	if m == nil {
		return
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "unknown"
	}
	m.mu.Lock()
	m.providerTotals[status]++
	m.mu.Unlock()
}

func (m *Metrics) ObserveEvent(status string) {
	if m == nil {
		return
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "unknown"
	}
	m.mu.Lock()
	m.eventTotals[status]++
	m.mu.Unlock()
}

func (m *Metrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(m.Snapshot()))
	})
}

func (m *Metrics) Snapshot() string {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var builder strings.Builder
	builder.WriteString("# HELP motif_http_requests_total Total HTTP requests handled by the API.\n# TYPE motif_http_requests_total counter\n")
	keys := make([]requestMetric, 0, len(m.requests))
	for key := range m.requests {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].Route != keys[j].Route {
			return keys[i].Route < keys[j].Route
		}
		return keys[i].Method < keys[j].Method
	})
	for _, key := range keys {
		fmt.Fprintf(&builder, "motif_http_requests_total{method=%q,route=%q,status=%q} %d\n", key.Method, key.Route, key.Status, m.requests[key])
	}
	fmt.Fprintf(&builder, "motif_http_request_count %d\n", m.requestTotal)
	builder.WriteString("# HELP motif_reward_provider_attempts_total Reward provider delivery attempts by outcome.\n# TYPE motif_reward_provider_attempts_total counter\n")
	providerKeys := make([]string, 0, len(m.providerTotals))
	for key := range m.providerTotals {
		providerKeys = append(providerKeys, key)
	}
	sort.Strings(providerKeys)
	for _, key := range providerKeys {
		fmt.Fprintf(&builder, "motif_reward_provider_attempts_total{outcome=%q} %d\n", key, m.providerTotals[key])
	}
	builder.WriteString("# HELP motif_event_receipts_total Event receipt outcomes.\n# TYPE motif_event_receipts_total counter\n")
	eventKeys := make([]string, 0, len(m.eventTotals))
	for key := range m.eventTotals {
		eventKeys = append(eventKeys, key)
	}
	sort.Strings(eventKeys)
	for _, key := range eventKeys {
		fmt.Fprintf(&builder, "motif_event_receipts_total{outcome=%q} %d\n", key, m.eventTotals[key])
	}
	return builder.String()
}

func routeLabel(path string) string {
	switch {
	case path == "/metrics":
		return "/metrics"
	case strings.HasPrefix(path, "/health/"):
		return "/health/*"
	case path == "/v1/events":
		return "/v1/events"
	case strings.HasPrefix(path, "/v1/events/"):
		return "/v1/events/:id"
	case strings.HasPrefix(path, "/v1/users/"):
		return "/v1/users/:id"
	case strings.HasPrefix(path, "/v1/rewards/"):
		return "/v1/rewards/:id"
	case strings.HasPrefix(path, "/v1/redemptions/"):
		return "/v1/redemptions/:id"
	case strings.HasPrefix(path, "/v1/admin/"):
		return "/v1/admin/*"
	default:
		return "/other"
	}
}

func statusClass(status int) string {
	return fmt.Sprintf("%dxx", status/100)
}
