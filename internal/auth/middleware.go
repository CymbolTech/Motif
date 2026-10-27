package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// HeaderAPIKey carries an access key on REST integration requests.
const HeaderAPIKey = "X-Motif-API-Key"

type contextKey struct{}

// Lookup resolves a raw access key; LookupKey bound to a pool satisfies it.
type Lookup func(ctx context.Context, raw string) (Key, error)

// KeyFromContext returns the access key RequireKey authenticated.
func KeyFromContext(ctx context.Context) (Key, bool) {
	key, ok := ctx.Value(contextKey{}).(Key)
	return key, ok
}

// RequestKey returns the key RequireKey authenticated for a request. A handler
// mounted without RequireKey answers 401 instead of running unauthenticated.
func RequestKey(w http.ResponseWriter, r *http.Request) (Key, bool) {
	key, ok := KeyFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_api_key", "X-Motif-API-Key is required")
	}
	return key, ok
}

// WithKey stores an authenticated key, for handlers tested without RequireKey.
func WithKey(ctx context.Context, key Key) context.Context {
	return context.WithValue(ctx, contextKey{}, key)
}

// ScopeForMethod is the scope a REST request needs: reads need read, creating
// needs write, changing needs edit, and removing needs delete.
func ScopeForMethod(method string) string {
	switch method {
	case http.MethodGet, http.MethodHead:
		return ScopeRead
	case http.MethodPut, http.MethodPatch:
		return ScopeEdit
	case http.MethodDelete:
		return ScopeDelete
	default:
		return ScopeWrite
	}
}

// integrationPaths are the REST routes authenticated with an access key.
var integrationPaths = []string{"/v1/events", "/v1/users", "/v1/points/conversions", "/v1/leaderboards", "/v1/rewards", "/v1/redemptions"}

// IsIntegrationPath reports whether a path belongs to the REST integration
// API, whose browser access each access key decides for itself.
func IsIntegrationPath(path string) bool {
	for _, prefix := range integrationPaths {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

// RequireKey authenticates REST integration requests. It answers browser
// preflights itself, because a preflight never carries the key, and then
// enforces the key's allowed origins and scopes on the real request.
// Origins only bind browsers: a server request without an Origin header is
// judged by the key and its scopes alone.
func RequireKey(lookup Lookup, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" {
			// The origin can read the response, including errors, so a developer
			// sees why a call failed. Requests from origins a key does not allow
			// are refused below before any data is read.
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Add("Vary", "Origin")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID, Retry-After")
		}
		if r.Method == http.MethodOptions {
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Motif-API-Key, X-Request-ID")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Max-Age", "600")
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		raw := strings.TrimSpace(r.Header.Get(HeaderAPIKey))
		if raw == "" {
			writeError(w, http.StatusUnauthorized, "missing_api_key", "X-Motif-API-Key is required")
			return
		}
		key, err := lookup(r.Context(), raw)
		if errors.Is(err, ErrInvalidCredential) {
			writeError(w, http.StatusUnauthorized, "invalid_api_key", "access key is invalid, expired or revoked")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "credential_lookup_failed", "access key could not be verified")
			return
		}
		if origin != "" && !key.AllowsOrigin(origin) {
			writeError(w, http.StatusForbidden, "origin_not_allowed", "this access key does not accept requests from "+origin)
			return
		}
		if scope := ScopeForMethod(r.Method); !key.Has(scope) {
			writeError(w, http.StatusForbidden, "insufficient_scope", "this access key does not have the "+scope+" scope")
			return
		}
		next.ServeHTTP(w, r.WithContext(WithKey(r.Context(), key)))
	})
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code, "message": message}})
}
