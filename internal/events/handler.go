package events

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/auth"
	"github.com/motifengine/motifengine/internal/entitlements"
	"github.com/motifengine/motifengine/internal/observability"
	appruntime "github.com/motifengine/motifengine/internal/runtime"
)

type Handler struct {
	db      *pgxpool.Pool
	limits  *entitlements.Store
	metrics *observability.Metrics
}

func NewHandler(db *pgxpool.Pool, metrics ...*observability.Metrics) http.Handler {
	var currentMetrics *observability.Metrics
	if len(metrics) > 0 {
		currentMetrics = metrics[0]
	}
	return &Handler{db: db, limits: entitlements.NewStore(db), metrics: currentMetrics}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		h.serveStatus(w, r)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/v1/events" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	if r.Header.Get("X-Motif-API-Key") == "" {
		writeError(w, http.StatusUnauthorized, "missing_api_key", "X-Motif-API-Key is required")
		return
	}
	var request Request
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, maxPayloadBytes+16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	validated, err := Validate(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_event", err.Error())
		return
	}
	var occurredAt *time.Time
	if validated.OccurredAt != "" {
		parsed, parseErr := time.Parse(time.RFC3339, validated.OccurredAt)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_occurred_at", "occurred_at must be RFC3339")
			return
		}
		occurredAt = &parsed
	}

	receipt, err := h.insert(r.Context(), r.Header.Get("X-Motif-API-Key"), validated, occurredAt)
	if err != nil {
		if h.metrics != nil {
			h.metrics.ObserveEvent("rejected")
		}
		var duplicatePayload *PayloadConflictError
		var schemaMismatch *EventSchemaError
		switch {
		case errors.Is(err, errInvalidCredential):
			writeError(w, http.StatusUnauthorized, "invalid_api_key", "credential is invalid or revoked")
		case errors.As(err, &duplicatePayload):
			writeError(w, http.StatusConflict, "event_id_conflict", duplicatePayload.Error())
		case errors.As(err, &schemaMismatch):
			writeError(w, http.StatusUnprocessableEntity, "event_schema_mismatch", err.Error())
		case errors.As(err, new(*entitlements.LimitError)):
			writeError(w, http.StatusRequestEntityTooLarge, "plan_limit_exceeded", err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "event_not_accepted", "event could not be accepted")
		}
		return
	}
	if h.metrics != nil {
		h.metrics.ObserveEvent("accepted")
	}
	writeJSON(w, http.StatusAccepted, receipt)
}

func (h *Handler) serveStatus(w http.ResponseWriter, r *http.Request) {
	externalID, ok := parseEventPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	apiKey := r.Header.Get("X-Motif-API-Key")
	if apiKey == "" {
		writeError(w, http.StatusUnauthorized, "missing_api_key", "X-Motif-API-Key is required")
		return
	}
	receipt, err := h.lookup(r.Context(), apiKey, externalID)
	if err != nil {
		if errors.Is(err, errInvalidCredential) || errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "event_not_found", "event was not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "event_lookup_failed", "event status could not be read")
		return
	}
	writeJSON(w, http.StatusOK, receipt)
}

type Receipt struct {
	EventID         int64  `json:"event_id"`
	ExternalEventID string `json:"external_event_id"`
	Status          string `json:"status"`
	SchemaVersion   int    `json:"schema_version,omitempty"`
}

type PayloadConflictError struct{ ExternalEventID string }

func (e *PayloadConflictError) Error() string {
	return "event_id already exists with a different payload: " + e.ExternalEventID
}

type EventSchemaError struct {
	EventType string
	Reason    string
}

func (e *EventSchemaError) Error() string {
	return "event type " + e.EventType + " payload does not match its schema: " + e.Reason
}

var errInvalidCredential = auth.ErrInvalidCredential

func (h *Handler) insert(ctx context.Context, apiKey string, request ValidatedRequest, occurredAt *time.Time) (Receipt, error) {
	tx, err := h.db.Begin(ctx)
	if err != nil {
		return Receipt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var tenantID, environmentID string
	var credentialID int64
	if err := tx.QueryRow(ctx, `
		SELECT c.id, c.tenant_id, c.environment_id
		FROM integration_credentials c
		JOIN tenants t ON t.id = c.tenant_id
		WHERE c.key_hash = $1 AND c.revoked_at IS NULL AND t.status = 'active'`, auth.Digest(apiKey)).Scan(&credentialID, &tenantID, &environmentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Receipt{}, errInvalidCredential
		}
		return Receipt{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE integration_credentials SET last_used_at = now() WHERE id = $1`, credentialID); err != nil {
		return Receipt{}, err
	}
	if err := h.limits.Check(ctx, tenantID, "max_event_payload_bytes", int64(len(request.CanonicalPayload)), 0); err != nil {
		return Receipt{}, err
	}
	var schema json.RawMessage
	schemaVersion := 1
	schemaErr := tx.QueryRow(ctx, `
		SELECT schema, current_version
		FROM event_types
		WHERE tenant_id = $1 AND environment_id = $2 AND name = $3 AND active = true`, tenantID, environmentID, request.EventType).Scan(&schema, &schemaVersion)
	if schemaErr == nil {
		if err := ValidateSchema(schema, request.CanonicalPayload); err != nil {
			return Receipt{}, &EventSchemaError{EventType: request.EventType, Reason: err.Error()}
		}
	} else if !errors.Is(schemaErr, pgx.ErrNoRows) {
		return Receipt{}, schemaErr
	}

	var eventID int64
	var status string
	err = tx.QueryRow(ctx, `
		INSERT INTO events (tenant_id, environment_id, external_event_id, event_type, occurred_at, external_user_id, payload, payload_hash, event_schema_version)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, decode($8, 'hex'), $9)
		ON CONFLICT (tenant_id, environment_id, external_event_id) DO NOTHING
		RETURNING id, status`, tenantID, environmentID, request.ExternalEventID, request.EventType, occurredAt, nullableString(request.UserID), request.CanonicalPayload, request.PayloadHash, schemaVersion).Scan(&eventID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		var existingHash []byte
		var existingSchemaVersion int
		if queryErr := tx.QueryRow(ctx, `SELECT id, status, payload_hash, event_schema_version FROM events WHERE tenant_id = $1 AND environment_id = $2 AND external_event_id = $3`, tenantID, environmentID, request.ExternalEventID).Scan(&eventID, &status, &existingHash, &existingSchemaVersion); queryErr != nil {
			return Receipt{}, queryErr
		}
		if !bytes.Equal(existingHash, mustDecodeHex(request.PayloadHash)) {
			return Receipt{}, &PayloadConflictError{ExternalEventID: request.ExternalEventID}
		}
		if err := tx.Commit(ctx); err != nil {
			return Receipt{}, err
		}
		return Receipt{EventID: eventID, ExternalEventID: request.ExternalEventID, Status: status, SchemaVersion: existingSchemaVersion}, nil
	}
	if err != nil {
		return Receipt{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO event_outbox (event_id) VALUES ($1)`, eventID); err != nil {
		return Receipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Receipt{}, err
	}
	return Receipt{EventID: eventID, ExternalEventID: request.ExternalEventID, Status: status, SchemaVersion: schemaVersion}, nil
}

func (h *Handler) lookup(ctx context.Context, apiKey, externalID string) (Receipt, error) {
	var receipt Receipt
	err := h.db.QueryRow(ctx, `
		SELECT e.id, e.external_event_id, e.status, e.event_schema_version
		FROM events e
		JOIN integration_credentials c
		  ON c.tenant_id = e.tenant_id AND c.environment_id = e.environment_id
		JOIN tenants t ON t.id = c.tenant_id
		WHERE c.key_hash = $1 AND c.revoked_at IS NULL AND t.status = 'active' AND e.external_event_id = $2`, auth.Digest(apiKey), externalID).
		Scan(&receipt.EventID, &receipt.ExternalEventID, &receipt.Status, &receipt.SchemaVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return Receipt{}, errInvalidCredential
	}
	if err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func parseEventPath(path string) (string, bool) {
	value := strings.TrimPrefix(path, "/v1/events/")
	value = strings.TrimSuffix(value, "/")
	if value == "" || strings.Contains(value, "/") || value == path {
		return "", false
	}
	return value, true
}

func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func mustDecodeHex(value string) []byte {
	decoded, _ := hex.DecodeString(value)
	return decoded
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
