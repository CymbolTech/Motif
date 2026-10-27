package points

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/auth"
	"github.com/motifengine/motifengine/internal/leaderboards"
	"github.com/motifengine/motifengine/internal/progression"
	appruntime "github.com/motifengine/motifengine/internal/runtime"
)

type Handler struct {
	db     *pgxpool.Pool
	ledger *Ledger
}

func NewHandler(db *pgxpool.Pool) http.Handler {
	return &Handler{db: db, ledger: NewLedger(db)}
}

type transactionRequest struct {
	CurrencyCode   string          `json:"currency"`
	Amount         int64           `json:"amount"`
	Type           string          `json:"type"`
	IdempotencyKey string          `json:"idempotency_key"`
	Provenance     json.RawMessage `json:"provenance"`
}

type balanceResponse struct {
	CurrencyID string `json:"currency_id"`
	Currency   string `json:"currency"`
	Name       string `json:"name"`
	UnitScale  int    `json:"unit_scale"`
	Balance    int64  `json:"balance"`
	AccountID  *int64 `json:"account_id,omitempty"`
}

// BalanceResponse is the tenant-scoped balance shape shared by integration and
// admin read paths. Keep the JSON contract identical for both callers.
type BalanceResponse = balanceResponse

type UserSnapshot struct {
	UserID       string                        `json:"user_id"`
	Balances     []balanceResponse             `json:"balances"`
	Transactions []LedgerEntry                 `json:"transactions"`
	NextCursor   string                        `json:"next_cursor,omitempty"`
	Missions     []map[string]any              `json:"missions"`
	Achievements []progression.UserAchievement `json:"achievements"`
	Progression  progression.UserProgression   `json:"progression"`
}

// LoadUserSnapshot reads all loyalty state for one user inside an explicit
// tenant/environment scope. It is used by the authenticated admin console so
// the browser never has to guess which integration credential belongs to the
// selected workspace.
func LoadUserSnapshot(ctx context.Context, db *pgxpool.Pool, scope Scope, userID string, limit int) (UserSnapshot, error) {
	h := &Handler{db: db}
	balances, err := h.loadBalances(ctx, scope, userID)
	if err != nil {
		return UserSnapshot{}, err
	}
	transactions, nextCursor, err := LoadUserTransactions(ctx, db, scope, userID, limit, "")
	if err != nil {
		return UserSnapshot{}, err
	}
	missions, err := h.loadMissions(ctx, scope, userID)
	if err != nil {
		return UserSnapshot{}, err
	}
	achievements, err := h.loadAchievements(ctx, scope, userID)
	if err != nil {
		return UserSnapshot{}, err
	}
	userProgression, err := progression.LoadUserProgression(ctx, db, progression.Scope{TenantID: scope.TenantID, EnvironmentID: scope.EnvironmentID}, userID)
	if err != nil {
		return UserSnapshot{}, err
	}
	return UserSnapshot{UserID: userID, Balances: balances, Transactions: transactions, NextCursor: nextCursor, Missions: missions, Achievements: achievements, Progression: userProgression}, nil
}

// LoadUserTransactions returns a cursor-paginated ledger for an explicit
// tenant/environment scope. The cursor format is the same opaque value used
// by the public points endpoint.
func LoadUserTransactions(ctx context.Context, db *pgxpool.Pool, scope Scope, userID string, limit int, cursor string) ([]LedgerEntry, string, error) {
	if limit < 1 || limit > 100 {
		return nil, "", errors.New("limit must be between 1 and 100")
	}
	var cursorTime *time.Time
	var cursorID int64
	if strings.TrimSpace(cursor) != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return nil, "", err
		}
		var parsed transactionCursor
		if err := json.Unmarshal(decoded, &parsed); err != nil || parsed.ID < 1 {
			return nil, "", errors.New("cursor is invalid")
		}
		parsedTime, err := time.Parse(time.RFC3339Nano, parsed.CreatedAt)
		if err != nil {
			return nil, "", errors.New("cursor is invalid")
		}
		cursorTime, cursorID = &parsedTime, parsed.ID
	}
	rows, err := db.Query(ctx, `
		SELECT e.id, e.account_id, c.id, c.code, a.external_user_id, e.entry_type, e.amount, e.balance_after, e.idempotency_key, e.provenance, e.created_at
		FROM point_ledger_entries e
		JOIN point_accounts a ON a.id = e.account_id
		JOIN currencies c ON c.id = a.currency_id
		WHERE a.tenant_id = $1 AND a.environment_id = $2 AND a.external_user_id = $3
		  AND ($4::timestamptz IS NULL OR e.created_at < $4 OR (e.created_at = $4 AND e.id < $5))
		ORDER BY e.created_at DESC, e.id DESC LIMIT $6`, scope.TenantID, scope.EnvironmentID, userID, cursorTime, cursorID, limit+1)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	entries := make([]LedgerEntry, 0, limit)
	for rows.Next() {
		var entry LedgerEntry
		if err := rows.Scan(&entry.ID, &entry.AccountID, &entry.CurrencyID, &entry.CurrencyCode, &entry.ExternalUserID, &entry.EntryType, &entry.Amount, &entry.BalanceAfter, &entry.IdempotencyKey, &entry.Provenance, &entry.CreatedAt); err != nil {
			return nil, "", err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	var nextCursor string
	if len(entries) > limit {
		last := entries[limit-1]
		entries = entries[:limit]
		payload, _ := json.Marshal(transactionCursor{CreatedAt: last.CreatedAt.UTC().Format(time.RFC3339Nano), ID: last.ID})
		nextCursor = base64.RawURLEncoding.EncodeToString(payload)
	}
	return entries, nextCursor, nil
}

type balancesResponse struct {
	UserID   string            `json:"user_id"`
	Balances []balanceResponse `json:"balances"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, resource, ok := parseUserPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	key, ok := auth.RequestKey(w, r)
	if !ok {
		return
	}
	scope := Scope(key.Scope())
	switch {
	case r.Method == http.MethodGet && resource == "points":
		h.listBalances(w, r, scope, userID)
	case r.Method == http.MethodGet && resource == "transactions":
		h.listTransactions(w, r, scope, userID)
	case r.Method == http.MethodPost && resource == "transactions":
		h.createTransaction(w, r, scope, userID)
	case r.Method == http.MethodGet && resource == "profile":
		h.getProfile(w, r, scope, userID)
	case r.Method == http.MethodGet && resource == "missions":
		h.listMissions(w, r, scope, userID)
	case r.Method == http.MethodGet && resource == "achievements":
		h.listAchievements(w, r, scope, userID)
	case r.Method == http.MethodGet && resource == "progression":
		h.getProgression(w, r, scope, userID)
	case r.Method == http.MethodGet && resource == "streaks":
		h.getStreaks(w, r, scope, userID)
	case r.Method == http.MethodGet && resource == "leaderboards":
		standings, err := leaderboards.LoadUserStandings(r.Context(), h.db, leaderboards.Scope{TenantID: scope.TenantID, EnvironmentID: scope.EnvironmentID}, userID, time.Time{})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "leaderboard_lookup_failed", "leaderboards could not be read")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user_id": userID, "leaderboards": standings})
	default:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

func (h *Handler) listBalances(w http.ResponseWriter, r *http.Request, scope Scope, userID string) {
	balances, err := h.loadBalances(r.Context(), scope, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "balance_lookup_failed", "balances could not be read")
		return
	}
	writeJSON(w, http.StatusOK, balancesResponse{UserID: userID, Balances: balances})
}

func (h *Handler) loadBalances(ctx context.Context, scope Scope, userID string) ([]balanceResponse, error) {
	rows, err := h.db.Query(ctx, `
		SELECT c.id, c.code, c.name, c.unit_scale, COALESCE(a.available_balance, 0), a.id
		FROM currencies c
		LEFT JOIN point_accounts a
		  ON a.currency_id = c.id AND a.external_user_id = $3
		 AND a.tenant_id = c.tenant_id AND a.environment_id = c.environment_id
		WHERE c.tenant_id = $1 AND c.environment_id = $2 AND c.active = true
		ORDER BY c.code`, scope.TenantID, scope.EnvironmentID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]balanceResponse, 0)
	for rows.Next() {
		var balance balanceResponse
		if err := rows.Scan(&balance.CurrencyID, &balance.Currency, &balance.Name, &balance.UnitScale, &balance.Balance, &balance.AccountID); err != nil {
			return nil, err
		}
		result = append(result, balance)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (h *Handler) getProfile(w http.ResponseWriter, r *http.Request, scope Scope, userID string) {
	balances, err := h.loadBalances(r.Context(), scope, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "profile_lookup_failed", "profile could not be read")
		return
	}
	var eventCount int
	if err := h.db.QueryRow(r.Context(), `SELECT count(*) FROM events WHERE tenant_id = $1 AND environment_id = $2 AND external_user_id = $3`, scope.TenantID, scope.EnvironmentID, userID).Scan(&eventCount); err != nil {
		writeError(w, http.StatusInternalServerError, "profile_lookup_failed", "profile could not be read")
		return
	}
	missions, err := h.loadMissions(r.Context(), scope, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "profile_lookup_failed", "profile could not be read")
		return
	}
	achievements, err := h.loadAchievements(r.Context(), scope, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "profile_lookup_failed", "profile could not be read")
		return
	}
	userProgression, err := progression.LoadUserProgression(r.Context(), h.db, progression.Scope{TenantID: scope.TenantID, EnvironmentID: scope.EnvironmentID}, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "profile_lookup_failed", "profile could not be read")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": userID, "event_count": eventCount, "balances": balances, "missions": missions, "achievements": achievements, "progression": userProgression})
}

func (h *Handler) getProgression(w http.ResponseWriter, r *http.Request, scope Scope, userID string) {
	userProgression, err := progression.LoadUserProgression(r.Context(), h.db, progression.Scope{TenantID: scope.TenantID, EnvironmentID: scope.EnvironmentID}, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "progression_lookup_failed", "progression could not be read")
		return
	}
	writeJSON(w, http.StatusOK, userProgression)
}

func (h *Handler) getStreaks(w http.ResponseWriter, r *http.Request, scope Scope, userID string) {
	eventType := strings.TrimSpace(r.URL.Query().Get("event_type"))
	rows, err := h.db.Query(r.Context(), `SELECT DISTINCT (COALESCE(occurred_at, received_at) AT TIME ZONE 'UTC')::date FROM events WHERE tenant_id = $1 AND environment_id = $2 AND external_user_id = $3 AND ($4 = '' OR event_type = $4) ORDER BY 1 DESC`, scope.TenantID, scope.EnvironmentID, userID, eventType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streak_lookup_failed", "streak could not be read")
		return
	}
	defer rows.Close()
	dates := make([]time.Time, 0)
	for rows.Next() {
		var date time.Time
		if err := rows.Scan(&date); err != nil {
			writeError(w, http.StatusInternalServerError, "streak_lookup_failed", "streak could not be read")
			return
		}
		dates = append(dates, date)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "streak_lookup_failed", "streak could not be read")
		return
	}
	current, longest := 0, 0
	if len(dates) > 0 {
		for index := 0; index < len(dates); index++ {
			length := 1
			for index+length < len(dates) && dates[index].Sub(dates[index+length]) == 24*time.Hour {
				length++
			}
			if index == 0 && (dates[0].Equal(time.Now().UTC().Truncate(24*time.Hour)) || dates[0].Equal(time.Now().UTC().Add(-24*time.Hour).Truncate(24*time.Hour))) {
				current = length
			}
			if length > longest {
				longest = length
			}
		}
	}
	var lastActive any
	if len(dates) > 0 {
		lastActive = dates[0].Format("2006-01-02")
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": userID, "event_type": eventType, "current_streak": current, "longest_streak": longest, "last_active_date": lastActive})
}

func (h *Handler) listMissions(w http.ResponseWriter, r *http.Request, scope Scope, userID string) {
	missions, err := h.loadMissions(r.Context(), scope, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "mission_lookup_failed", "missions could not be read")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": userID, "missions": missions})
}

func (h *Handler) listAchievements(w http.ResponseWriter, r *http.Request, scope Scope, userID string) {
	achievements, err := h.loadAchievements(r.Context(), scope, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "achievement_lookup_failed", "achievements could not be read")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": userID, "achievements": achievements})
}

func (h *Handler) loadMissions(ctx context.Context, scope Scope, userID string) ([]map[string]any, error) {
	rows, err := h.db.Query(ctx, `
		SELECT m.id, m.name, m.event_type, m.target_count, COALESCE(p.progress, 0), p.completed_at
		FROM missions m LEFT JOIN user_mission_progress p ON p.mission_id = m.id AND p.external_user_id = $3
		WHERE m.tenant_id = $1 AND m.environment_id = $2 AND m.active = true ORDER BY m.created_at, m.id`, scope.TenantID, scope.EnvironmentID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, name, eventType string
		var target, progress int
		var completedAt *time.Time
		if err := rows.Scan(&id, &name, &eventType, &target, &progress, &completedAt); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "name": name, "event_type": eventType, "target_count": target, "progress": progress, "completed": completedAt != nil, "completed_at": completedAt})
	}
	return result, rows.Err()
}

func (h *Handler) loadAchievements(ctx context.Context, scope Scope, userID string) ([]progression.UserAchievement, error) {
	rows, err := h.db.Query(ctx, `
		SELECT a.id, a.name, a.description, a.event_type, a.required_count,
		       COALESCE(p.progress, 0), p.unlocked_at
		FROM achievements a
		LEFT JOIN user_achievement_progress p ON p.achievement_id = a.id AND p.external_user_id = $3
		WHERE a.tenant_id = $1 AND a.environment_id = $2 AND a.active = true
		ORDER BY a.created_at, a.id`, scope.TenantID, scope.EnvironmentID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]progression.UserAchievement, 0)
	for rows.Next() {
		var achievement progression.UserAchievement
		if err := rows.Scan(&achievement.ID, &achievement.Name, &achievement.Description, &achievement.EventType, &achievement.RequiredCount, &achievement.Progress, &achievement.UnlockedAt); err != nil {
			return nil, err
		}
		achievement.Unlocked = achievement.UnlockedAt != nil
		result = append(result, achievement)
	}
	return result, rows.Err()
}

func (h *Handler) listTransactions(w http.ResponseWriter, r *http.Request, scope Scope, userID string) {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be between 1 and 100")
			return
		}
		limit = parsed
	}
	currency := r.URL.Query().Get("currency")
	var cursorTime *time.Time
	var cursorID int64
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_cursor", "cursor is invalid")
			return
		}
		var cursor transactionCursor
		if err := json.Unmarshal(decoded, &cursor); err != nil || cursor.ID < 1 {
			writeError(w, http.StatusBadRequest, "invalid_cursor", "cursor is invalid")
			return
		}
		parsed, err := time.Parse(time.RFC3339Nano, cursor.CreatedAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_cursor", "cursor is invalid")
			return
		}
		cursorTime, cursorID = &parsed, cursor.ID
	}
	rows, err := h.db.Query(r.Context(), `
		SELECT e.id, e.account_id, c.id, c.code, a.external_user_id, e.entry_type, e.amount, e.balance_after, e.idempotency_key, e.provenance, e.created_at
		FROM point_ledger_entries e
		JOIN point_accounts a ON a.id = e.account_id
		JOIN currencies c ON c.id = a.currency_id
		WHERE a.tenant_id = $1 AND a.environment_id = $2 AND a.external_user_id = $3
		  AND ($4 = '' OR c.code = $4)
		  AND ($5::timestamptz IS NULL OR e.created_at < $5 OR (e.created_at = $5 AND e.id < $6))
		ORDER BY e.created_at DESC, e.id DESC LIMIT $7`, scope.TenantID, scope.EnvironmentID, userID, currency, cursorTime, cursorID, limit+1)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "transaction_lookup_failed", "transactions could not be read")
		return
	}
	defer rows.Close()
	entries := make([]LedgerEntry, 0, limit)
	for rows.Next() {
		var entry LedgerEntry
		if err := rows.Scan(&entry.ID, &entry.AccountID, &entry.CurrencyID, &entry.CurrencyCode, &entry.ExternalUserID, &entry.EntryType, &entry.Amount, &entry.BalanceAfter, &entry.IdempotencyKey, &entry.Provenance, &entry.CreatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "transaction_lookup_failed", "transactions could not be read")
			return
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "transaction_lookup_failed", "transactions could not be read")
		return
	}
	var nextCursor string
	if len(entries) > limit {
		last := entries[limit-1]
		entries = entries[:limit]
		payload, _ := json.Marshal(transactionCursor{CreatedAt: last.CreatedAt.UTC().Format(time.RFC3339Nano), ID: last.ID})
		nextCursor = base64.RawURLEncoding.EncodeToString(payload)
	}
	response := map[string]any{"user_id": userID, "transactions": entries}
	if nextCursor != "" {
		response["next_cursor"] = nextCursor
	}
	writeJSON(w, http.StatusOK, response)
}

type transactionCursor struct {
	CreatedAt string `json:"created_at"`
	ID        int64  `json:"id"`
}

func (h *Handler) createTransaction(w http.ResponseWriter, r *http.Request, scope Scope, userID string) {
	var request transactionRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 32*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.CurrencyCode = strings.TrimSpace(request.CurrencyCode)
	request.Type = strings.TrimSpace(request.Type)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.Amount <= 0 || request.CurrencyCode == "" || request.IdempotencyKey == "" || (request.Type != "award" && request.Type != "spend") {
		writeError(w, http.StatusBadRequest, "invalid_transaction", "currency, positive amount, type, and idempotency_key are required")
		return
	}
	var entry LedgerEntry
	var err error
	if request.Type == "award" {
		entry, err = h.ledger.Award(r.Context(), scope, userID, request.CurrencyCode, request.Amount, request.IdempotencyKey, request.Provenance)
	} else {
		entry, err = h.ledger.Spend(r.Context(), scope, userID, request.CurrencyCode, request.Amount, request.IdempotencyKey, request.Provenance)
	}
	switch {
	case errors.Is(err, ErrCurrencyNotFound):
		writeError(w, http.StatusNotFound, "currency_not_found", "currency was not found")
	case errors.Is(err, ErrInsufficientBalance):
		writeError(w, http.StatusConflict, "insufficient_balance", "point balance is insufficient")
	case errors.Is(err, ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", "idempotency key was already used with a different request")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "transaction_failed", "point transaction could not be recorded")
	default:
		writeJSON(w, http.StatusCreated, entry)
	}
}

func parseUserPath(path string) (string, string, bool) {
	value := strings.TrimPrefix(path, "/v1/users/")
	parts := strings.Split(strings.TrimSuffix(value, "/"), "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || (parts[1] != "points" && parts[1] != "profile" && parts[1] != "missions" && parts[1] != "achievements" && parts[1] != "progression" && parts[1] != "streaks" && parts[1] != "leaderboards") {
		return "", "", false
	}
	userID, err := url.PathUnescape(parts[0])
	if err != nil || userID == "" || strings.Contains(userID, "/") {
		return "", "", false
	}
	if len(parts) == 2 && parts[1] == "profile" {
		return userID, "profile", true
	}
	if len(parts) == 2 && parts[1] == "missions" {
		return userID, "missions", true
	}
	if len(parts) == 2 && parts[1] == "achievements" {
		return userID, "achievements", true
	}
	if len(parts) == 2 && parts[1] == "progression" {
		return userID, "progression", true
	}
	if len(parts) == 2 && parts[1] == "streaks" {
		return userID, "streaks", true
	}
	if len(parts) == 2 && parts[1] == "leaderboards" {
		return userID, "leaderboards", true
	}
	if len(parts) == 3 && parts[1] == "points" && parts[2] == "transactions" {
		return userID, "transactions", true
	}
	if len(parts) != 2 {
		return "", "", false
	}
	return userID, "points", true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
