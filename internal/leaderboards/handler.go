package leaderboards

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/auth"
)

// Handler serves the integration API under /v1/leaderboards for a
// customer's backend, authenticated by the integration key.
type Handler struct {
	db *pgxpool.Pool
}

func NewHandler(db *pgxpool.Pool) http.Handler {
	return &Handler{db: db}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key, resource, ok := parsePath(r.URL.Path)
	if !ok || r.Method != http.MethodGet {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	credential, ok := auth.RequestKey(w, r)
	if !ok {
		return
	}
	scope := Scope(credential.Scope())
	// Zero means database time, the clock ledger entries are stamped with.
	var now time.Time
	if key == "" {
		boards, err := ListBoards(r.Context(), h.db, scope, true, now)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "leaderboard_lookup_failed", "leaderboards could not be read")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"leaderboards": boards})
		return
	}
	board, err := FindBoard(r.Context(), h.db, scope, key)
	if err != nil {
		WriteError(w, err)
		return
	}
	if resource == "rounds" {
		limit, _, err := ParsePage(r.URL.Query())
		if err != nil {
			WriteError(w, err)
			return
		}
		rounds, err := ListRounds(r.Context(), h.db, board, limit, now)
		if err != nil {
			WriteError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"leaderboard": board, "rounds": rounds})
		return
	}
	limit, offset, err := ParsePage(r.URL.Query())
	if err != nil {
		WriteError(w, err)
		return
	}
	standings, err := LoadStandings(r.Context(), h.db, board, r.URL.Query().Get("round"), limit, offset, now)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, standings)
}

func parsePath(path string) (string, string, bool) {
	if path == "/v1/leaderboards" || path == "/v1/leaderboards/" {
		return "", "", true
	}
	value, ok := strings.CutPrefix(path, "/v1/leaderboards/")
	if !ok {
		return "", "", false
	}
	parts := strings.Split(strings.TrimSuffix(value, "/"), "/")
	key, err := url.PathUnescape(parts[0])
	if err != nil || key == "" || len(key) > 128 {
		return "", "", false
	}
	switch {
	case len(parts) == 1:
		return key, "", true
	case len(parts) == 2 && parts[1] == "rounds":
		return key, "rounds", true
	default:
		return "", "", false
	}
}

// ParsePage reads limit and offset query parameters.
func ParsePage(query url.Values) (int, int, error) {
	limit, offset := DefaultLimit, 0
	if value := query.Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 1 || parsed > MaxLimit {
			return 0, 0, invalid("limit must be between 1 and %d", MaxLimit)
		}
		limit = parsed
	}
	if value := query.Get("offset"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed < 0 || parsed > MaxOffset {
			return 0, 0, invalid("offset must be between 0 and %d", MaxOffset)
		}
		offset = parsed
	}
	return limit, offset, nil
}

// WriteError maps leaderboard errors to the shared API error shape.
func WriteError(w http.ResponseWriter, err error) {
	var validation *ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusBadRequest, "invalid_leaderboard_request", validation.Message)
	case errors.Is(err, ErrNotFound):
		writeError(w, http.StatusNotFound, "leaderboard_not_found", err.Error())
	case errors.Is(err, ErrRoundNotFound):
		writeError(w, http.StatusNotFound, "leaderboard_round_not_found", err.Error())
	case errors.Is(err, ErrCurrencyNotFound):
		writeError(w, http.StatusNotFound, "currency_not_found", err.Error())
	case errors.Is(err, ErrMetricUnchangable):
		writeError(w, http.StatusBadRequest, "invalid_leaderboard_request", err.Error())
	case errors.Is(err, ErrRecurringBoard), errors.Is(err, ErrRoundOverlap), errors.Is(err, ErrScheduledRounds), errors.Is(err, ErrRoundNotActive), errors.Is(err, ErrRoundStarted), errors.Is(err, ErrRoundEnded):
		writeError(w, http.StatusConflict, "leaderboard_round_conflict", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "leaderboard_request_failed", "the leaderboard request could not be completed")
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
