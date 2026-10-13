package points

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/auth"
	appruntime "github.com/motifengine/motifengine/internal/runtime"
)

type ConversionHandler struct{ db *pgxpool.Pool }

func NewConversionHandler(db *pgxpool.Pool) http.Handler { return &ConversionHandler{db: db} }

type conversionRequest struct {
	ExternalUserID  string `json:"external_user_id"`
	SourceCurrency  string `json:"source_currency"`
	TargetCurrency  string `json:"target_currency"`
	Amount          int64  `json:"amount"`
	RateNumerator   int64  `json:"rate_numerator"`
	RateDenominator int64  `json:"rate_denominator"`
	IdempotencyKey  string `json:"idempotency_key"`
}

func (h *ConversionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/points/conversions" {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
		return
	}
	if r.Header.Get("X-Motif-API-Key") == "" {
		writeError(w, http.StatusUnauthorized, "missing_api_key", "X-Motif-API-Key is required")
		return
	}
	scope, err := lookupIntegrationScope(r.Context(), h.db, r.Header.Get("X-Motif-API-Key"))
	if errors.Is(err, errInvalidCredential) {
		writeError(w, http.StatusUnauthorized, "invalid_api_key", "credential is invalid or revoked")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "credential_lookup_failed", "credential could not be verified")
		return
	}
	var request conversionRequest
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 16*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.ExternalUserID = strings.TrimSpace(request.ExternalUserID)
	request.SourceCurrency = strings.TrimSpace(request.SourceCurrency)
	request.TargetCurrency = strings.TrimSpace(request.TargetCurrency)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.ExternalUserID == "" || request.Amount <= 0 || request.RateNumerator <= 0 || request.RateDenominator <= 0 || request.IdempotencyKey == "" || request.SourceCurrency == "" || request.TargetCurrency == "" {
		writeError(w, http.StatusBadRequest, "invalid_conversion", "user, currencies, positive amount, positive rate, and idempotency_key are required")
		return
	}
	conversion, err := Convert(r.Context(), h.db, ConversionRequest{Scope: scope, ExternalUserID: request.ExternalUserID, SourceCurrency: request.SourceCurrency, TargetCurrency: request.TargetCurrency, Amount: request.Amount, RateNumerator: request.RateNumerator, RateDenominator: request.RateDenominator, IdempotencyKey: request.IdempotencyKey})
	switch {
	case errors.Is(err, ErrCurrencyNotFound):
		writeError(w, http.StatusNotFound, "currency_not_found", "currency was not found")
	case errors.Is(err, ErrInsufficientBalance):
		writeError(w, http.StatusConflict, "insufficient_balance", "point balance is insufficient")
	case errors.Is(err, ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", "idempotency key was already used with a different request")
	case err != nil:
		writeError(w, http.StatusInternalServerError, "conversion_failed", "point conversion could not be completed")
	default:
		writeJSON(w, http.StatusCreated, conversion)
	}
}

func lookupIntegrationScope(ctx context.Context, db *pgxpool.Pool, apiKey string) (Scope, error) {
	scope, err := auth.LookupIntegrationCredential(ctx, db, apiKey)
	return Scope{TenantID: scope.TenantID, EnvironmentID: scope.EnvironmentID}, err
}
