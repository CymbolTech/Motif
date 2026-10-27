package transport

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/motifengine/motifengine/internal/observability"
)

type contextKey string

const requestIDKey contextKey = "motif.request_id"

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

type ProtectionConfig struct {
	Logger         *slog.Logger
	AllowedOrigins []string
	Limiter        *RateLimiter
	Metrics        *observability.Metrics
	// KeyCORS selects paths whose browser access each access key decides for
	// itself. AllowedOrigins does not apply to them.
	KeyCORS func(path string) bool
}

func Protect(next http.Handler, cfg ProtectionConfig) http.Handler {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Limiter == nil {
		cfg.Limiter = NewRateLimiter(30, 60)
	}
	return withRequestID(withCORS(withRateLimit(withLogging(withMetrics(next, cfg.Metrics), cfg.Logger), cfg.Limiter), cfg.AllowedOrigins, cfg.KeyCORS))
}

func withMetrics(next http.Handler, metrics *observability.Metrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrapped := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(wrapped, r)
		status := wrapped.status
		if status == 0 {
			status = http.StatusOK
		}
		if metrics != nil {
			metrics.ObserveRequest(r.Method, r.URL.Path, status)
		}
	})
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" || len(requestID) > 128 || strings.ContainsAny(requestID, "\r\n") {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, requestID)))
	})
}

// Context is intentionally kept small in the public surface. Callers can use
// transport.RequestID(r.Context()) when they need to correlate a log entry.
func newRequestID() string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		return "local-request"
	}
	return hex.EncodeToString(value)
}

func withCORS(next http.Handler, allowedOrigins []string, keyCORS func(string) bool) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			allowed[origin] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if keyCORS != nil && keyCORS(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		_, permitted := allowed[origin]
		if origin != "" && permitted {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Motif-Admin-Key, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			if origin == "" || permitted {
				w.WriteHeader(http.StatusNoContent)
			} else {
				w.WriteHeader(http.StatusForbidden)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withRateLimit(next http.Handler, limiter *RateLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(clientKey(r)) {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"code":"rate_limited","message":"request rate limit exceeded"}}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func clientKey(r *http.Request) string {
	if host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil && host != "" {
		return host
	}
	if r.RemoteAddr != "" {
		return r.RemoteAddr
	}
	return "unknown"
}

type RateLimiter struct {
	rate  float64
	burst float64
	mu    sync.Mutex
	items map[string]bucket
}

type bucket struct {
	tokens float64
	seen   time.Time
}

func NewRateLimiter(perSecond float64, burst int) *RateLimiter {
	if perSecond <= 0 {
		perSecond = 30
	}
	if burst < 1 {
		burst = 1
	}
	return &RateLimiter{rate: perSecond, burst: float64(burst), items: make(map[string]bucket)}
}

func (l *RateLimiter) Allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	current, ok := l.items[key]
	if !ok {
		current = bucket{tokens: l.burst, seen: now}
	}
	elapsed := now.Sub(current.seen).Seconds()
	if elapsed > 0 {
		current.tokens += elapsed * l.rate
		if current.tokens > l.burst {
			current.tokens = l.burst
		}
	}
	current.seen = now
	if current.tokens < 1 {
		l.items[key] = current
		return false
	}
	current.tokens--
	l.items[key] = current
	if len(l.items) > 2048 {
		l.prune(now)
	}
	return true
}

func (l *RateLimiter) prune(now time.Time) {
	cutoff := now.Add(-10 * time.Minute)
	for key, item := range l.items {
		if item.seen.Before(cutoff) {
			delete(l.items, key)
		}
	}
	for len(l.items) > 2048 {
		for key := range l.items {
			delete(l.items, key)
			break
		}
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(value []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(value)
}

func withLogging(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(wrapped, r)
		status := wrapped.status
		if status == 0 {
			status = http.StatusOK
		}
		logger.Info("http request", "request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "status", status, "duration_ms", time.Since(started).Milliseconds())
	})
}
