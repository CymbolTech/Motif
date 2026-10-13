package health

import (
	"encoding/json"
	"errors"
	"net/http"
)

var errDependencyUnavailable = errors.New("dependency unavailable")

type Dependencies struct {
	Database func() error
	NATS     func() error
}

type Handler struct {
	dependencies Dependencies
}

func NewHandler(dependencies Dependencies) http.Handler {
	return &Handler{dependencies: dependencies}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/health/live":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case "/health/ready":
		if err := h.ready(); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) ready() error {
	for _, check := range []func() error{h.dependencies.Database, h.dependencies.NATS} {
		if check != nil {
			if err := check(); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
