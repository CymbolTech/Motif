package admin

import (
	"net/http"
	"strings"
	"time"

	"github.com/motifengine/motifengine/internal/audit"
	"github.com/motifengine/motifengine/internal/leaderboards"
	appruntime "github.com/motifengine/motifengine/internal/runtime"
)

// leaderboardRoute is a parsed /v1/admin/leaderboards path: the board, the
// sub-resource (standings or rounds), the round, and the round action.
type leaderboardRoute struct {
	boardID, resource, roundID, action string
}

func parseLeaderboardPath(path string) (leaderboardRoute, bool) {
	if path == "/v1/admin/leaderboards" {
		return leaderboardRoute{}, true
	}
	value, ok := strings.CutPrefix(path, "/v1/admin/leaderboards/")
	if !ok {
		return leaderboardRoute{}, false
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" || len(part) > 128 {
			return leaderboardRoute{}, false
		}
	}
	route := leaderboardRoute{boardID: parts[0]}
	switch {
	case len(parts) == 1:
	case len(parts) == 2 && (parts[1] == "standings" || parts[1] == "rounds"):
		route.resource = parts[1]
	case len(parts) == 3 && parts[1] == "rounds":
		route.resource, route.roundID = "rounds", parts[2]
	case len(parts) == 4 && parts[1] == "rounds" && (parts[3] == "end" || parts[3] == "cancel"):
		route.resource, route.roundID, route.action = "rounds", parts[2], parts[3]
	default:
		return leaderboardRoute{}, false
	}
	return route, true
}

// isLeaderboardWritePath reports POST paths below a board, which carry their
// tenant and environment in the JSON body like every other admin write.
func isLeaderboardWritePath(path string) bool {
	route, ok := parseLeaderboardPath(path)
	return ok && route.boardID != "" && route.resource == "rounds" && (route.roundID == "" || route.action != "")
}

func (h *Handler) serveLeaderboards(w http.ResponseWriter, r *http.Request) {
	route, ok := parseLeaderboardPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	// Zero means database time, the clock ledger entries are stamped with.
	var now time.Time
	switch {
	case route.boardID == "" && r.Method == http.MethodPost:
		h.createLeaderboard(w, r, now)
	case route.boardID == "" && r.Method == http.MethodGet:
		boards, err := leaderboards.ListBoards(r.Context(), h.db, queryLeaderboardScope(r), false, now)
		if err != nil {
			leaderboards.WriteError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, boards)
	case route.resource == "" && r.Method == http.MethodPatch:
		h.updateLeaderboard(w, r, route, now)
	case route.boardID != "" && route.resource == "" && r.Method == http.MethodDelete:
		h.deleteLeaderboard(w, r, route)
	case route.resource == "standings" && r.Method == http.MethodGet:
		board, err := leaderboards.FindBoard(r.Context(), h.db, queryLeaderboardScope(r), route.boardID)
		if err != nil {
			leaderboards.WriteError(w, err)
			return
		}
		limit, offset, err := leaderboards.ParsePage(r.URL.Query())
		if err != nil {
			leaderboards.WriteError(w, err)
			return
		}
		standings, err := leaderboards.LoadStandings(r.Context(), h.db, board, r.URL.Query().Get("round"), limit, offset, now)
		if err != nil {
			leaderboards.WriteError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, standings)
	case route.resource == "rounds" && route.roundID == "" && r.Method == http.MethodGet:
		board, err := leaderboards.FindBoard(r.Context(), h.db, queryLeaderboardScope(r), route.boardID)
		if err != nil {
			leaderboards.WriteError(w, err)
			return
		}
		limit, _, err := leaderboards.ParsePage(r.URL.Query())
		if err != nil {
			leaderboards.WriteError(w, err)
			return
		}
		rounds, err := leaderboards.ListRounds(r.Context(), h.db, board, limit, now)
		if err != nil {
			leaderboards.WriteError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, rounds)
	case route.resource == "rounds" && route.roundID == "" && r.Method == http.MethodPost:
		var request leaderboards.RoundRequest
		if !decodeLeaderboardBody(w, r, &request) {
			return
		}
		scope := leaderboards.Scope{TenantID: strings.TrimSpace(request.TenantID), EnvironmentID: strings.TrimSpace(request.EnvironmentID)}
		round, err := leaderboards.CreateRound(r.Context(), h.db, scope, route.boardID, request, now)
		h.writeRoundResult(w, r, scope, route.boardID, "leaderboard_round.created", http.StatusCreated, round, err)
	case route.roundID != "" && route.action == "" && r.Method == http.MethodPatch:
		var request leaderboards.RoundUpdate
		if !decodeLeaderboardBody(w, r, &request) {
			return
		}
		scope := leaderboards.Scope{TenantID: strings.TrimSpace(request.TenantID), EnvironmentID: strings.TrimSpace(request.EnvironmentID)}
		round, err := leaderboards.UpdateRound(r.Context(), h.db, scope, route.boardID, route.roundID, request, now)
		h.writeRoundResult(w, r, scope, route.boardID, "leaderboard_round.updated", http.StatusOK, round, err)
	case route.action != "" && r.Method == http.MethodPost:
		var request struct {
			TenantID      string `json:"tenant_id"`
			EnvironmentID string `json:"environment_id"`
		}
		if !decodeLeaderboardBody(w, r, &request) {
			return
		}
		scope := leaderboards.Scope{TenantID: strings.TrimSpace(request.TenantID), EnvironmentID: strings.TrimSpace(request.EnvironmentID)}
		action := leaderboards.EndRound
		if route.action == "cancel" {
			action = leaderboards.CancelRound
		}
		round, err := action(r.Context(), h.db, scope, route.boardID, route.roundID, now)
		h.writeRoundResult(w, r, scope, route.boardID, "leaderboard_round."+map[string]string{"end": "ended", "cancel": "cancelled"}[route.action], http.StatusOK, round, err)
	default:
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	}
}

// userLeaderboards returns one user's place on every active board, for the
// console and MCP clients that cannot use the integration key.
func (h *Handler) userLeaderboards(w http.ResponseWriter, r *http.Request) {
	scope := queryLeaderboardScope(r)
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if !identifierPattern.MatchString(scope.TenantID) || !identifierPattern.MatchString(scope.EnvironmentID) || userID == "" || len(userID) > 200 {
		writeError(w, http.StatusBadRequest, "invalid_user_scope", "tenant_id, environment_id, and user_id are required")
		return
	}
	standings, err := leaderboards.LoadUserStandings(r.Context(), h.db, scope, userID, time.Time{})
	if err != nil {
		leaderboards.WriteError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user_id": userID, "leaderboards": standings})
}

func queryLeaderboardScope(r *http.Request) leaderboards.Scope {
	return leaderboards.Scope{TenantID: strings.TrimSpace(r.URL.Query().Get("tenant_id")), EnvironmentID: strings.TrimSpace(r.URL.Query().Get("environment_id"))}
}

func decodeLeaderboardBody(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return false
	}
	return true
}

func (h *Handler) createLeaderboard(w http.ResponseWriter, r *http.Request, now time.Time) {
	var request leaderboards.BoardRequest
	if !decodeLeaderboardBody(w, r, &request) {
		return
	}
	request, err := leaderboards.ValidateBoardRequest(request, time.Now().UTC())
	if err != nil {
		leaderboards.WriteError(w, err)
		return
	}
	tx, err := h.db.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "leaderboard_creation_failed", "leaderboard could not be created")
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()
	if err := h.enforceResourceLimitTx(r.Context(), tx, request.TenantID, request.EnvironmentID, "max_leaderboards", "leaderboards"); err != nil {
		h.writePlanLimitError(w, err)
		return
	}
	board, err := leaderboards.InsertBoard(r.Context(), tx, request)
	if err != nil {
		switch {
		case isUniqueViolation(err):
			writeError(w, http.StatusConflict, "leaderboard_exists", "a leaderboard with this key already exists in the environment")
		case isForeignKeyViolation(err):
			writeError(w, http.StatusNotFound, "environment_not_found", "tenant environment was not found")
		default:
			leaderboards.WriteError(w, err)
		}
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "leaderboard_creation_failed", "leaderboard could not be created")
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: board.TenantID, EnvironmentID: board.EnvironmentID, ActorType: "platform_admin", Action: "leaderboard.created", ResourceType: "leaderboard", ResourceID: board.ID, Metadata: auditMetadata(map[string]any{"key": board.Key, "schedule": board.Schedule.Type})})
	writeJSON(w, http.StatusCreated, board)
}

func (h *Handler) updateLeaderboard(w http.ResponseWriter, r *http.Request, route leaderboardRoute, now time.Time) {
	var request leaderboards.BoardRequest
	if !decodeLeaderboardBody(w, r, &request) {
		return
	}
	scope := leaderboards.Scope{TenantID: strings.TrimSpace(request.TenantID), EnvironmentID: strings.TrimSpace(request.EnvironmentID)}
	board, err := leaderboards.UpdateBoard(r.Context(), h.db, scope, route.boardID, request, now)
	if err != nil {
		leaderboards.WriteError(w, err)
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: board.TenantID, EnvironmentID: board.EnvironmentID, ActorType: "platform_admin", Action: "leaderboard.updated", ResourceType: "leaderboard", ResourceID: board.ID, Metadata: auditMetadata(map[string]any{"schedule": board.Schedule.Type})})
	writeJSON(w, http.StatusOK, board)
}

// deleteLeaderboard removes a board with its rounds and scores. Turning a
// board off keeps its history; deleting it does not.
func (h *Handler) deleteLeaderboard(w http.ResponseWriter, r *http.Request, route leaderboardRoute) {
	scope := queryLeaderboardScope(r)
	board, rounds, err := leaderboards.DeleteBoard(r.Context(), h.db, scope, route.boardID)
	if err != nil {
		leaderboards.WriteError(w, err)
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: board.TenantID, EnvironmentID: board.EnvironmentID, ActorType: "platform_admin", Action: "leaderboard.deleted", ResourceType: "leaderboard", ResourceID: board.ID, Metadata: auditMetadata(map[string]any{"key": board.Key, "name": board.Name, "rounds": rounds})})
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": board.ID, "key": board.Key, "rounds_deleted": rounds})
}

func (h *Handler) writeRoundResult(w http.ResponseWriter, r *http.Request, scope leaderboards.Scope, boardID, action string, status int, round leaderboards.Round, err error) {
	if err != nil {
		leaderboards.WriteError(w, err)
		return
	}
	_ = audit.Record(r.Context(), h.db, audit.Entry{TenantID: scope.TenantID, EnvironmentID: scope.EnvironmentID, ActorType: "platform_admin", Action: action, ResourceType: "leaderboard_round", ResourceID: round.ID, Metadata: auditMetadata(map[string]any{"leaderboard_id": boardID, "number": round.Number, "starts_at": round.StartsAt, "ends_at": round.EndsAt})})
	writeJSON(w, status, round)
}
