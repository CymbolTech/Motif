package processing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/points"
	"github.com/motifengine/motifengine/internal/progression"
	"github.com/motifengine/motifengine/internal/rules"
	"github.com/nats-io/nats.go"
)

type Processor struct {
	db     *pgxpool.Pool
	js     nats.JetStreamContext
	logger *slog.Logger
	ledger *points.Ledger
}

type outboxMessage struct {
	EventID int64 `json:"event_id"`
}

func New(db *pgxpool.Pool, js nats.JetStreamContext, logger *slog.Logger) *Processor {
	return &Processor{db: db, js: js, logger: logger, ledger: points.NewLedger(db)}
}

func (p *Processor) Start(ctx context.Context) error {
	if _, err := p.js.AddStream(&nats.StreamConfig{Name: "MOTIF_EVENTS", Subjects: []string{"motif.events.>"}, Storage: nats.FileStorage, MaxAge: 7 * 24 * time.Hour}); err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
		return err
	}
	if _, err := p.js.Subscribe("motif.events.received", p.handleMessage, nats.Durable("motif-event-processor"), nats.ManualAck()); err != nil {
		return err
	}
	go p.pump(ctx)
	go p.expirePoints(ctx)
	return nil
}

func (p *Processor) expirePoints(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			count, err := points.Expire(ctx, p.db, time.Now().UTC(), 100)
			if err != nil {
				p.logger.Error("point expiration failed", "error", err)
			} else if count > 0 {
				p.logger.Info("point lots expired", "count", count)
			}
		}
	}
}

func (p *Processor) handleMessage(message *nats.Msg) {
	var payload outboxMessage
	if err := json.Unmarshal(message.Data, &payload); err != nil {
		p.logger.Error("event message rejected", "error", err)
		_ = message.Ack()
		return
	}
	if err := p.evaluateRules(context.Background(), payload.EventID); err != nil {
		p.logger.Error("rule evaluation failed", "event_id", payload.EventID, "error", err)
		return
	}
	if err := p.updateMissions(context.Background(), payload.EventID); err != nil {
		p.logger.Error("mission progress update failed", "event_id", payload.EventID, "error", err)
		return
	}
	if err := p.updateAchievements(context.Background(), payload.EventID); err != nil {
		p.logger.Error("achievement progress update failed", "event_id", payload.EventID, "error", err)
		return
	}
	if _, err := p.db.Exec(context.Background(), `UPDATE events SET status = 'processed', processed_at = now() WHERE id = $1`, payload.EventID); err != nil {
		p.logger.Error("event processing failed", "event_id", payload.EventID, "error", err)
		return
	}
	if err := message.Ack(); err != nil {
		p.logger.Error("event acknowledgment failed", "event_id", payload.EventID, "error", err)
	}
}

func (p *Processor) updateMissions(ctx context.Context, eventID int64) error {
	rows, err := p.db.Query(ctx, `
		SELECT m.id, e.external_user_id
		FROM events e JOIN missions m
		  ON m.tenant_id = e.tenant_id AND m.environment_id = e.environment_id AND m.event_type = e.event_type AND m.active = true
		WHERE e.id = $1 AND e.external_user_id IS NOT NULL`, eventID)
	if err != nil {
		return err
	}
	type missionEvent struct{ missionID, userID string }
	items := make([]missionEvent, 0)
	for rows.Next() {
		var item missionEvent
		if err := rows.Scan(&item.missionID, &item.userID); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range items {
		tx, err := p.db.Begin(ctx)
		if err != nil {
			return err
		}
		var inserted bool
		if err := tx.QueryRow(ctx, `INSERT INTO mission_progress_events (mission_id, event_id, external_user_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING RETURNING true`, item.missionID, eventID, item.userID).Scan(&inserted); errors.Is(err, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			continue
		} else if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_mission_progress (mission_id, external_user_id, progress)
			SELECT $1, $2, LEAST(1, m.target_count) FROM missions m WHERE m.id = $1
			ON CONFLICT (mission_id, external_user_id) DO UPDATE SET progress = LEAST(user_mission_progress.progress + 1, (SELECT target_count FROM missions WHERE id = $1)), completed_at = CASE WHEN user_mission_progress.progress + 1 >= (SELECT target_count FROM missions WHERE id = $1) THEN COALESCE(user_mission_progress.completed_at, now()) ELSE user_mission_progress.completed_at END, updated_at = now()`, item.missionID, item.userID); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) updateAchievements(ctx context.Context, eventID int64) error {
	rows, err := p.db.Query(ctx, `
		SELECT a.id, e.external_user_id
		FROM events e JOIN achievements a
		  ON a.tenant_id = e.tenant_id AND a.environment_id = e.environment_id AND a.event_type = e.event_type AND a.active = true
		WHERE e.id = $1 AND e.external_user_id IS NOT NULL`, eventID)
	if err != nil {
		return err
	}
	type achievementEvent struct{ achievementID, userID string }
	items := make([]achievementEvent, 0)
	for rows.Next() {
		var item achievementEvent
		if err := rows.Scan(&item.achievementID, &item.userID); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, item := range items {
		tx, err := p.db.Begin(ctx)
		if err != nil {
			return err
		}
		var inserted bool
		if err := tx.QueryRow(ctx, `INSERT INTO achievement_progress_events (achievement_id, event_id, external_user_id) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING RETURNING true`, item.achievementID, eventID, item.userID).Scan(&inserted); errors.Is(err, pgx.ErrNoRows) {
			_ = tx.Rollback(ctx)
			continue
		} else if err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO user_achievement_progress (achievement_id, external_user_id, progress, unlocked_at)
			SELECT $1, $2, LEAST(1, a.required_count), CASE WHEN a.required_count = 1 THEN now() ELSE NULL END FROM achievements a WHERE a.id = $1
			ON CONFLICT (achievement_id, external_user_id) DO UPDATE SET
				progress = LEAST(user_achievement_progress.progress + 1, (SELECT required_count FROM achievements WHERE id = $1)),
				unlocked_at = CASE WHEN user_achievement_progress.progress + 1 >= (SELECT required_count FROM achievements WHERE id = $1) THEN COALESCE(user_achievement_progress.unlocked_at, now()) ELSE user_achievement_progress.unlocked_at END,
				updated_at = now()`, item.achievementID, item.userID); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) evaluateRules(ctx context.Context, eventID int64) error {
	rows, err := p.db.Query(ctx, `
		SELECT rv.rule_id, rv.id, rv.conditions, rv.action, e.payload, e.tenant_id, e.environment_id, COALESCE(e.external_user_id, '')
		FROM events e
		JOIN rules r
		  ON r.tenant_id = e.tenant_id
		 AND r.environment_id = e.environment_id
		JOIN rule_versions rv
		  ON rv.rule_id = r.id
		 AND rv.event_type = e.event_type
		 AND rv.status = 'published'
		WHERE e.id = $1
		  AND (rv.starts_at IS NULL OR rv.starts_at <= now())
		  AND (rv.ends_at IS NULL OR rv.ends_at > now())`, eventID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var ruleID, ruleVersionID, tenantID, environmentID, externalUserID string
		var conditions, action, payload []byte
		if err := rows.Scan(&ruleID, &ruleVersionID, &conditions, &action, &payload, &tenantID, &environmentID, &externalUserID); err != nil {
			return err
		}
		matched, err := rules.MatchConditions(conditions, payload)
		if err != nil {
			return err
		}
		if !matched {
			continue
		}
		if err := p.executeAction(ctx, ruleID, ruleVersionID, eventID, tenantID, environmentID, externalUserID, action); err != nil {
			return err
		}
	}
	return rows.Err()
}

const maxActionAttempts = 3

type actionDefinition struct {
	Type     string      `json:"type"`
	Currency string      `json:"currency,omitempty"`
	Amount   json.Number `json:"amount,omitempty"`
	Message  string      `json:"message,omitempty"`
}

func decodeActions(raw []byte) ([]actionDefinition, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("action definition is empty")
	}
	definitions := []json.RawMessage{trimmed}
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &definitions); err != nil {
			return nil, err
		}
	}
	if len(definitions) == 0 {
		return nil, fmt.Errorf("at least one action is required")
	}
	actions := make([]actionDefinition, 0, len(definitions))
	for index, definition := range definitions {
		var action actionDefinition
		if err := json.Unmarshal(definition, &action); err != nil {
			return nil, fmt.Errorf("action %d is invalid: %w", index, err)
		}
		if action.Type == "" {
			return nil, fmt.Errorf("action %d type is required", index)
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func actionIdempotencyKey(eventID int64, ruleVersionID string, ordinal int) string {
	return fmt.Sprintf("event:%d:rule:%s:action:%d", eventID, ruleVersionID, ordinal)
}

func (p *Processor) executeAction(ctx context.Context, ruleID, ruleVersionID string, eventID int64, tenantID, environmentID, externalUserID string, action []byte) error {
	actions, err := decodeActions(action)
	if err != nil {
		return err
	}
	actionType := actions[0].Type
	if len(actions) > 1 {
		actionType = "multi_action"
	}
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var executionID int64
	var status string
	var attempts int
	var actionOrdinal int
	err = tx.QueryRow(ctx, `SELECT id, status, attempts, action_ordinal FROM rule_executions WHERE rule_id = $1 AND event_id = $2 FOR UPDATE`, ruleID, eventID).Scan(&executionID, &status, &attempts, &actionOrdinal)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
			INSERT INTO rule_executions (rule_id, rule_version_id, event_id, action_type, action, status, attempts, started_at)
			VALUES ($1, $2, $3, $4, $5::jsonb, 'pending', 1, now())
			RETURNING id`, ruleID, ruleVersionID, eventID, actionType, action).Scan(&executionID)
		attempts = 1
	} else if err == nil {
		if status == "recorded" || (status == "failed" && attempts >= maxActionAttempts) {
			return tx.Commit(ctx)
		}
		attempts++
		_, err = tx.Exec(ctx, `UPDATE rule_executions SET status = 'pending', attempts = $2, last_error = NULL, started_at = now(), completed_at = NULL WHERE id = $1`, executionID, attempts)
	}
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	for ordinal := actionOrdinal; ordinal < len(actions); ordinal++ {
		definition := actions[ordinal]
		if err = p.executeSingleAction(ctx, ruleID, ruleVersionID, eventID, tenantID, environmentID, externalUserID, ordinal, definition); err != nil {
			_, updateErr := p.db.Exec(ctx, `UPDATE rule_executions SET status = 'failed', last_error = $2, completed_at = now() WHERE id = $1`, executionID, err.Error())
			if updateErr != nil {
				return updateErr
			}
			return nil
		}
		if _, err = p.db.Exec(ctx, `UPDATE rule_executions SET action_ordinal = $2, last_error = NULL WHERE id = $1`, executionID, ordinal+1); err != nil {
			return err
		}
	}
	_, err = p.db.Exec(ctx, `UPDATE rule_executions SET status = 'recorded', last_error = NULL, completed_at = now() WHERE id = $1`, executionID)
	return err
}

func (p *Processor) executeSingleAction(ctx context.Context, ruleID, ruleVersionID string, eventID int64, tenantID, environmentID, externalUserID string, ordinal int, definition actionDefinition) error {
	key := actionIdempotencyKey(eventID, ruleVersionID, ordinal)
	switch definition.Type {
	case "log":
		p.logger.Info("rule action recorded", "rule_id", ruleID, "rule_version_id", ruleVersionID, "event_id", eventID, "action_ordinal", ordinal)
		return nil
	case "award_points":
		amount, err := definition.Amount.Int64()
		if err != nil || amount <= 0 || externalUserID == "" {
			return fmt.Errorf("award_points requires an external user and positive integer amount")
		}
		_, err = p.ledger.Award(ctx, points.Scope{TenantID: tenantID, EnvironmentID: environmentID}, externalUserID, definition.Currency, amount, key, json.RawMessage(`{"source":"rule"}`))
		return err
	case "award_xp":
		amount, err := definition.Amount.Int64()
		if err != nil || amount <= 0 || externalUserID == "" {
			return fmt.Errorf("award_xp requires an external user and positive integer amount")
		}
		provenance, _ := json.Marshal(map[string]any{"source": "rule", "rule_id": ruleID, "rule_version_id": ruleVersionID, "event_id": eventID, "action_ordinal": ordinal})
		_, err = progression.AwardXP(ctx, p.db, progression.Scope{TenantID: tenantID, EnvironmentID: environmentID}, externalUserID, amount, key, provenance)
		return err
	default:
		return fmt.Errorf("unsupported action type %q", definition.Type)
	}
}

func (p *Processor) pump(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.publishOne(ctx); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				p.logger.Error("event outbox attempt failed", "error", err)
			}
		}
	}
}

func (p *Processor) publishOne(ctx context.Context) error {
	tx, err := p.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var eventID int64
	if err := tx.QueryRow(ctx, `
		SELECT event_id
		FROM event_outbox
		WHERE published_at IS NULL
		ORDER BY created_at
		FOR UPDATE SKIP LOCKED
		LIMIT 1`).Scan(&eventID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE event_outbox SET attempts = attempts + 1 WHERE event_id = $1`, eventID); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	data, err := json.Marshal(outboxMessage{EventID: eventID})
	if err != nil {
		return err
	}
	if _, err := p.js.Publish("motif.events.received", data); err != nil {
		_, _ = p.db.Exec(ctx, `UPDATE event_outbox SET last_error = $2 WHERE event_id = $1`, eventID, err.Error())
		return err
	}
	_, err = p.db.Exec(ctx, `UPDATE event_outbox SET published_at = now(), last_error = NULL WHERE event_id = $1`, eventID)
	return err
}
