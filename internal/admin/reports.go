package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	appruntime "github.com/motifengine/motifengine/internal/runtime"
)

type reportDashboardResponse struct {
	TenantID      string          `json:"tenant_id"`
	EnvironmentID string          `json:"environment_id"`
	Widgets       json.RawMessage `json:"widgets"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

type reportPoint struct {
	At    time.Time `json:"at"`
	Value float64   `json:"value"`
}

type reportQueryResponse struct {
	TenantID      string        `json:"tenant_id"`
	EnvironmentID string        `json:"environment_id"`
	Measure       string        `json:"measure"`
	ResourceID    string        `json:"resource_id,omitempty"`
	EventType     string        `json:"event_type,omitempty"`
	From          time.Time     `json:"from"`
	To            time.Time     `json:"to"`
	Interval      string        `json:"interval"`
	Total         float64       `json:"total"`
	Points        []reportPoint `json:"points"`
	GeneratedAt   time.Time     `json:"generated_at"`
}

func (h *Handler) reportDashboard(w http.ResponseWriter, r *http.Request) {
	tenantID, environmentID := strings.TrimSpace(r.URL.Query().Get("tenant_id")), strings.TrimSpace(r.URL.Query().Get("environment_id"))
	if err := validateAnalyticsScope(tenantID, environmentID, nil, nil); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_scope", err.Error())
		return
	}
	var response reportDashboardResponse
	err := h.db.QueryRow(r.Context(), `SELECT tenant_id, environment_id, widgets, updated_at FROM report_dashboards WHERE tenant_id = $1 AND environment_id = $2`, tenantID, environmentID).
		Scan(&response.TenantID, &response.EnvironmentID, &response.Widgets, &response.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusOK, reportDashboardResponse{TenantID: tenantID, EnvironmentID: environmentID, Widgets: json.RawMessage(`[]`), UpdatedAt: time.Now().UTC()})
			return
		}
		writeError(w, http.StatusInternalServerError, "report_dashboard_lookup_failed", "report dashboard could not be read")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) updateReportDashboard(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TenantID      string          `json:"tenant_id"`
		EnvironmentID string          `json:"environment_id"`
		Widgets       json.RawMessage `json:"widgets"`
	}
	if err := appruntime.DecodeJSON(http.MaxBytesReader(w, r.Body, 64*1024), &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}
	request.TenantID, request.EnvironmentID = strings.TrimSpace(request.TenantID), strings.TrimSpace(request.EnvironmentID)
	if err := validateAnalyticsScope(request.TenantID, request.EnvironmentID, nil, nil); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_scope", err.Error())
		return
	}
	if len(request.Widgets) == 0 {
		request.Widgets = json.RawMessage(`[]`)
	}
	var widgets []json.RawMessage
	if err := json.Unmarshal(request.Widgets, &widgets); err != nil || len(widgets) > 24 {
		writeError(w, http.StatusBadRequest, "invalid_widgets", "widgets must be a JSON array with at most 24 items")
		return
	}
	if _, err := h.db.Exec(r.Context(), `INSERT INTO report_dashboards (tenant_id, environment_id, widgets, updated_at) VALUES ($1, $2, $3::jsonb, now()) ON CONFLICT (tenant_id, environment_id) DO UPDATE SET widgets = EXCLUDED.widgets, updated_at = now()`, request.TenantID, request.EnvironmentID, []byte(request.Widgets)); err != nil {
		writeError(w, http.StatusInternalServerError, "report_dashboard_update_failed", "report dashboard could not be saved")
		return
	}
	var response reportDashboardResponse
	if err := h.db.QueryRow(r.Context(), `SELECT tenant_id, environment_id, widgets, updated_at FROM report_dashboards WHERE tenant_id = $1 AND environment_id = $2`, request.TenantID, request.EnvironmentID).Scan(&response.TenantID, &response.EnvironmentID, &response.Widgets, &response.UpdatedAt); err != nil {
		writeError(w, http.StatusInternalServerError, "report_dashboard_lookup_failed", "saved report dashboard could not be read")
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func parseReportTime(value string, fallback time.Time) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err == nil {
		return parsed.UTC(), nil
	}
	parsed, err = time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("time must be RFC3339 or YYYY-MM-DD")
	}
	return parsed.UTC(), nil
}

func reportBucket(interval string) (string, error) {
	switch interval {
	case "", "day":
		return "day", nil
	case "week":
		return "week", nil
	default:
		return "", fmt.Errorf("interval must be day or week")
	}
}

func appendReportCondition(conditions string, column string, value any, args *[]any) string {
	*args = append(*args, value)
	return conditions + fmt.Sprintf(" AND %s = $%d", column, len(*args))
}

func (h *Handler) reportQuery(w http.ResponseWriter, r *http.Request) {
	tenantID, environmentID := strings.TrimSpace(r.URL.Query().Get("tenant_id")), strings.TrimSpace(r.URL.Query().Get("environment_id"))
	if err := validateAnalyticsScope(tenantID, environmentID, nil, nil); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_scope", err.Error())
		return
	}
	measure := strings.TrimSpace(r.URL.Query().Get("measure"))
	resourceID := strings.TrimSpace(r.URL.Query().Get("resource_id"))
	eventType := strings.TrimSpace(r.URL.Query().Get("event_type"))
	interval, err := reportBucket(strings.TrimSpace(r.URL.Query().Get("interval")))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_report_query", err.Error())
		return
	}
	now := time.Now().UTC()
	to, err := parseReportTime(r.URL.Query().Get("to"), now)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_report_query", err.Error())
		return
	}
	from, err := parseReportTime(r.URL.Query().Get("from"), to.AddDate(0, 0, -7))
	if err != nil || !from.Before(to) {
		writeError(w, http.StatusBadRequest, "invalid_report_query", "from must be before to")
		return
	}
	points, err := h.loadReportSeries(r, tenantID, environmentID, measure, resourceID, eventType, interval, from, to)
	if err != nil {
		if strings.HasPrefix(err.Error(), "unsupported report measure") {
			writeError(w, http.StatusBadRequest, "unsupported_report_measure", err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "report_query_failed", "report data could not be calculated")
		return
	}
	var total float64
	for _, point := range points {
		total += point.Value
	}
	if isDistinctReportMeasure(measure) {
		total, err = h.loadDistinctReportTotal(r, tenantID, environmentID, measure, resourceID, eventType, from, to)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "report_query_failed", "report data could not be calculated")
			return
		}
	}
	writeJSON(w, http.StatusOK, reportQueryResponse{TenantID: tenantID, EnvironmentID: environmentID, Measure: measure, ResourceID: resourceID, EventType: eventType, From: from, To: to, Interval: interval, Total: total, Points: points, GeneratedAt: now})
}

func isDistinctReportMeasure(measure string) bool {
	return measure == "users.active" || measure == "events.unique_users" || measure == "campaigns.unique_users"
}

func (h *Handler) loadDistinctReportTotal(r *http.Request, tenantID, environmentID, measure, resourceID, eventType string, from, to time.Time) (float64, error) {
	args := []any{tenantID, environmentID, from, to}
	var query string
	switch measure {
	case "users.active", "events.unique_users":
		where := "tenant_id = $1 AND environment_id = $2 AND COALESCE(occurred_at, received_at) >= $3 AND COALESCE(occurred_at, received_at) < $4"
		if eventType != "" {
			args = append(args, eventType)
			where += fmt.Sprintf(" AND event_type = $%d", len(args))
		}
		query = "SELECT count(DISTINCT NULLIF(external_user_id, ''))::float8 FROM events WHERE " + where
	case "campaigns.unique_users":
		where := "e.tenant_id = $1 AND e.environment_id = $2 AND COALESCE(e.occurred_at, e.received_at) >= $3 AND COALESCE(e.occurred_at, e.received_at) < $4 AND (c.starts_at IS NULL OR COALESCE(e.occurred_at, e.received_at) >= c.starts_at) AND (c.ends_at IS NULL OR COALESCE(e.occurred_at, e.received_at) < c.ends_at)"
		if resourceID != "" {
			args = append(args, resourceID)
			where += fmt.Sprintf(" AND c.id = $%d", len(args))
		}
		if eventType != "" {
			args = append(args, eventType)
			where += fmt.Sprintf(" AND e.event_type = $%d", len(args))
		}
		query = "SELECT count(DISTINCT NULLIF(e.external_user_id, ''))::float8 FROM events e JOIN campaigns c ON c.tenant_id = e.tenant_id AND c.environment_id = e.environment_id AND c.event_type = e.event_type WHERE " + where
	default:
		return 0, nil
	}
	var total float64
	if err := h.db.QueryRow(r.Context(), query, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (h *Handler) loadReportSeries(r *http.Request, tenantID, environmentID, measure, resourceID, eventType, interval string, from, to time.Time) ([]reportPoint, error) {
	args := []any{tenantID, environmentID, from, to}
	conditions := ""
	dateColumn := "COALESCE(occurred_at, received_at)"
	valueExpression := "count(*)::float8"
	fromSQL := "events"
	whereSQL := "tenant_id = $1 AND environment_id = $2 AND " + dateColumn + " >= $3 AND " + dateColumn + " < $4"

	switch measure {
	case "events.count", "users.active":
		if measure == "users.active" {
			valueExpression = "count(DISTINCT NULLIF(external_user_id, ''))::float8"
		}
	case "events.unique_users":
		valueExpression = "count(DISTINCT NULLIF(external_user_id, ''))::float8"
	case "events.processed", "events.failed", "events.accepted", "events.processing":
		conditions += fmt.Sprintf(" AND status = '%s'", strings.TrimPrefix(measure, "events."))
	case "automations.executions", "automations.failures":
		fromSQL = "rule_executions x JOIN rules r ON r.id = x.rule_id"
		dateColumn = "x.created_at"
		whereSQL = "r.tenant_id = $1 AND r.environment_id = $2 AND x.created_at >= $3 AND x.created_at < $4"
		if measure == "automations.failures" {
			conditions += " AND x.status = 'failed'"
		}
		if resourceID != "" {
			conditions = appendReportCondition(conditions, "r.id", resourceID, &args)
		}
	case "achievements.unlocks", "achievements.progress":
		if measure == "achievements.unlocks" {
			fromSQL = "user_achievement_progress p JOIN achievements a ON a.id = p.achievement_id"
			dateColumn = "p.unlocked_at"
			whereSQL = "a.tenant_id = $1 AND a.environment_id = $2 AND p.unlocked_at IS NOT NULL AND p.unlocked_at >= $3 AND p.unlocked_at < $4"
		} else {
			fromSQL = "achievement_progress_events p JOIN achievements a ON a.id = p.achievement_id"
			dateColumn = "p.created_at"
			whereSQL = "a.tenant_id = $1 AND a.environment_id = $2 AND p.created_at >= $3 AND p.created_at < $4"
		}
		if resourceID != "" {
			conditions = appendReportCondition(conditions, "a.id", resourceID, &args)
		}
	case "missions.completions", "missions.progress":
		if measure == "missions.completions" {
			fromSQL = "user_mission_progress p JOIN missions m ON m.id = p.mission_id"
			dateColumn = "p.completed_at"
			whereSQL = "m.tenant_id = $1 AND m.environment_id = $2 AND p.completed_at IS NOT NULL AND p.completed_at >= $3 AND p.completed_at < $4"
		} else {
			fromSQL = "mission_progress_events p JOIN missions m ON m.id = p.mission_id"
			dateColumn = "p.created_at"
			whereSQL = "m.tenant_id = $1 AND m.environment_id = $2 AND p.created_at >= $3 AND p.created_at < $4"
		}
		if resourceID != "" {
			conditions = appendReportCondition(conditions, "m.id", resourceID, &args)
		}
	case "xp.awarded":
		fromSQL = "xp_events"
		dateColumn = "created_at"
		whereSQL = "tenant_id = $1 AND environment_id = $2 AND created_at >= $3 AND created_at < $4"
		valueExpression = "COALESCE(SUM(amount), 0)::float8"
	case "points.awarded", "points.spent", "points.expired":
		fromSQL = "point_ledger_entries le JOIN point_accounts pa ON pa.id = le.account_id JOIN currencies c ON c.id = pa.currency_id"
		dateColumn = "le.created_at"
		whereSQL = "pa.tenant_id = $1 AND pa.environment_id = $2 AND le.created_at >= $3 AND le.created_at < $4"
		entryType := map[string]string{"points.awarded": "award", "points.spent": "spend", "points.expired": "expiration"}[measure]
		conditions += fmt.Sprintf(" AND le.entry_type = '%s'", entryType)
		valueExpression = "COALESCE(SUM(ABS(le.amount)), 0)::float8"
		if resourceID != "" {
			conditions = appendReportCondition(conditions, "c.code", resourceID, &args)
		}
	case "points.conversions":
		fromSQL = "point_conversions pc JOIN currencies source_currency ON source_currency.id = pc.source_currency_id JOIN currencies target_currency ON target_currency.id = pc.target_currency_id"
		dateColumn = "pc.created_at"
		whereSQL = "pc.tenant_id = $1 AND pc.environment_id = $2 AND pc.created_at >= $3 AND pc.created_at < $4"
		valueExpression = "COALESCE(SUM(pc.target_amount), 0)::float8"
		if resourceID != "" {
			args = append(args, resourceID)
			conditions += fmt.Sprintf(" AND (source_currency.code = $%d OR target_currency.code = $%d)", len(args), len(args))
		}
	case "rewards.redemptions", "rewards.fulfilled", "rewards.failed":
		fromSQL = "redemptions"
		dateColumn = "created_at"
		whereSQL = "tenant_id = $1 AND environment_id = $2 AND created_at >= $3 AND created_at < $4"
		if measure != "rewards.redemptions" {
			dateColumn = "updated_at"
			whereSQL = "tenant_id = $1 AND environment_id = $2 AND updated_at >= $3 AND updated_at < $4"
			conditions += fmt.Sprintf(" AND status = '%s'", strings.TrimPrefix(measure, "rewards."))
		}
		if resourceID != "" {
			conditions = appendReportCondition(conditions, "reward_id", resourceID, &args)
		}
	case "campaigns.events", "campaigns.unique_users":
		fromSQL = "events e JOIN campaigns c ON c.tenant_id = e.tenant_id AND c.environment_id = e.environment_id AND c.event_type = e.event_type"
		dateColumn = "COALESCE(e.occurred_at, e.received_at)"
		whereSQL = "e.tenant_id = $1 AND e.environment_id = $2 AND " + dateColumn + " >= $3 AND " + dateColumn + " < $4 AND (c.starts_at IS NULL OR " + dateColumn + " >= c.starts_at) AND (c.ends_at IS NULL OR " + dateColumn + " < c.ends_at)"
		if measure == "campaigns.unique_users" {
			valueExpression = "count(DISTINCT NULLIF(e.external_user_id, ''))::float8"
		}
		if resourceID != "" {
			conditions = appendReportCondition(conditions, "c.id", resourceID, &args)
		}
	case "goals.events":
		fromSQL = "events e JOIN goals g ON g.tenant_id = e.tenant_id AND g.environment_id = e.environment_id AND g.event_type = e.event_type"
		dateColumn = "COALESCE(e.occurred_at, e.received_at)"
		whereSQL = "e.tenant_id = $1 AND e.environment_id = $2 AND " + dateColumn + " >= $3 AND " + dateColumn + " < $4"
		if resourceID != "" {
			conditions = appendReportCondition(conditions, "g.id", resourceID, &args)
		}
	default:
		return nil, fmt.Errorf("unsupported report measure %q", measure)
	}
	if eventType != "" {
		column := "event_type"
		if strings.Contains(fromSQL, " e ") || strings.HasPrefix(fromSQL, "events e") {
			column = "e.event_type"
		}
		conditions = appendReportCondition(conditions, column, eventType, &args)
	}
	query := fmt.Sprintf(`SELECT date_trunc('%s', %s) AS bucket, %s AS value FROM %s WHERE %s%s GROUP BY 1 ORDER BY 1`, interval, dateColumn, valueExpression, fromSQL, whereSQL, conditions)
	rows, err := h.db.Query(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	values := make(map[time.Time]float64)
	for rows.Next() {
		var bucket time.Time
		var value float64
		if err := rows.Scan(&bucket, &value); err != nil {
			return nil, err
		}
		values[bucket.UTC()] = value
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	points := make([]reportPoint, 0)
	start := from.UTC().Truncate(24 * time.Hour)
	if interval == "week" {
		start = start.AddDate(0, 0, -((int(start.Weekday()) + 6) % 7))
	}
	step := 24 * time.Hour
	if interval == "week" {
		step = 7 * 24 * time.Hour
	}
	for cursor := start; cursor.Before(to); cursor = cursor.Add(step) {
		points = append(points, reportPoint{At: cursor, Value: values[cursor]})
	}
	return points, nil
}
