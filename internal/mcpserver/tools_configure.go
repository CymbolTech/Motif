package mcpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type registerEventTypeInput struct {
	Environment string         `json:"environment,omitempty" jsonschema:"Environment id. Optional: an access key reaches exactly one environment."`
	Name        string         `json:"name" jsonschema:"Stable event name such as order.completed: lowercase letters, numbers, dots, underscores and hyphens"`
	Schema      map[string]any `json:"schema" jsonschema:"Payload schema in the supported JSON Schema subset: type, required, properties, enum, minimum, maximum, additionalProperties"`
}

type createCurrencyInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional: an access key reaches exactly one environment."`
	Code        string `json:"code" jsonschema:"Lowercase currency code used in rules, for example loyalty or coins"`
	Name        string `json:"name" jsonschema:"Display name, for example Loyalty points"`
	UnitScale   int    `json:"unit_scale,omitempty" jsonschema:"Decimal precision from 0 to 6; 0 for whole points"`
	ExpiryDays  int    `json:"expiry_days,omitempty" jsonschema:"Days after which each award expires; omit for points that never expire"`
}

type createLevelInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional: an access key reaches exactly one environment."`
	Level       int    `json:"level" jsonschema:"Level number, starting at 1"`
	Name        string `json:"name" jsonschema:"Level name, for example Getting started"`
	XPThreshold int64  `json:"xp_threshold" jsonschema:"Total XP needed to reach this level; level 1 is usually 0"`
}

type createAchievementInput struct {
	Environment   string `json:"environment,omitempty" jsonschema:"Environment id. Optional: an access key reaches exactly one environment."`
	Name          string `json:"name" jsonschema:"Achievement name"`
	Description   string `json:"description,omitempty" jsonschema:"What the user did to earn it"`
	EventType     string `json:"event_type" jsonschema:"The one event type that counts toward it"`
	RequiredCount int    `json:"required_count" jsonschema:"How many events unlock it"`
}

type createMissionInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional: an access key reaches exactly one environment."`
	Name        string `json:"name" jsonschema:"Mission name"`
	EventType   string `json:"event_type" jsonschema:"The one event type that advances it"`
	TargetCount int    `json:"target_count" jsonschema:"How many events complete it"`
}

type createRewardInput struct {
	Environment  string         `json:"environment,omitempty" jsonschema:"Environment id. Optional: an access key reaches exactly one environment."`
	Name         string         `json:"name" jsonschema:"Reward name"`
	Description  string         `json:"description,omitempty" jsonschema:"What the user receives"`
	CostCurrency string         `json:"cost_currency,omitempty" jsonschema:"Currency code a paid reward charges; omit for a free reward"`
	CostAmount   int64          `json:"cost_amount,omitempty" jsonschema:"Points a paid reward charges; omit for a free reward"`
	Payload      map[string]any `json:"payload,omitempty" jsonschema:"Data passed to the delivery provider, such as a coupon value or entitlement key"`
}

type updateCatalogInput struct {
	Environment string         `json:"environment,omitempty" jsonschema:"Environment id. Optional: an access key reaches exactly one environment."`
	Kind        string         `json:"kind" jsonschema:"The catalog item type"`
	ID          string         `json:"id" jsonschema:"The item id from list_catalog"`
	Active      *bool          `json:"active,omitempty" jsonschema:"Activate (true) or deactivate (false) the item"`
	Definition  map[string]any `json:"definition,omitempty" jsonschema:"A complete replacement definition with the same fields as when it was created"`
}

type saveRuleDraftInput struct {
	Environment string           `json:"environment,omitempty" jsonschema:"Environment id. Optional: an access key reaches exactly one environment."`
	RuleID      string           `json:"rule_id,omitempty" jsonschema:"An existing rule to add a new draft version to; omit to create a new rule"`
	Name        string           `json:"name,omitempty" jsonschema:"Rule name, unique in the environment; required for a new rule"`
	Description string           `json:"description,omitempty" jsonschema:"Why the rule exists"`
	EventType   string           `json:"event_type" jsonschema:"The event type the rule listens to"`
	Conditions  map[string]any   `json:"conditions,omitempty" jsonschema:"Condition expression; omit to match every event of the type"`
	Actions     []map[string]any `json:"actions" jsonschema:"Ordered actions: {type: award_xp, amount}, {type: award_points, currency, amount} or {type: log, message}"`
	StartsAt    string           `json:"starts_at,omitempty" jsonschema:"Optional inclusive UTC start, RFC 3339"`
	EndsAt      string           `json:"ends_at,omitempty" jsonschema:"Optional exclusive UTC end, RFC 3339"`
}

type publishRuleInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional: an access key reaches exactly one environment."`
	RuleID      string `json:"rule_id" jsonschema:"The rule to publish"`
	Version     int    `json:"version" jsonschema:"The exact draft version to publish, from save_rule_draft or list_rules"`
}

type sendTestEventInput struct {
	Environment string         `json:"environment,omitempty" jsonschema:"Environment id. Optional: an access key reaches exactly one environment."`
	EventType   string         `json:"event_type" jsonschema:"The event type to send"`
	UserID      string         `json:"user_id" jsonschema:"A test user id; award actions need one"`
	Payload     map[string]any `json:"payload" jsonschema:"The event payload, shaped like the product will send it"`
	EventID     string         `json:"event_id,omitempty" jsonschema:"Optional event id; reuse one to verify that a retry does not award twice"`
	Wait        *bool          `json:"wait,omitempty" jsonschema:"Wait up to eight seconds for processing and return the trace (default true)"`
}

var (
	updateCatalogSchema = inputSchema[updateCatalogInput](map[string][]string{"kind": {"currency", "level", "achievement", "mission", "reward", "leaderboard"}})
	catalogItemPaths    = map[string]string{"currency": "currencies", "level": "levels", "achievement": "achievements", "mission": "missions", "reward": "rewards", "leaderboard": "leaderboards"}
	unsafeIDCharacters  = regexp.MustCompile(`[^a-z0-9._-]+`)
)

func writes(title string, destructive, idempotent bool) *mcp.ToolAnnotations {
	closed := false
	return &mcp.ToolAnnotations{Title: title, DestructiveHint: &destructive, IdempotentHint: idempotent, OpenWorldHint: &closed}
}

func registerConfigureTools(server *mcp.Server, session *Session) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "register_event_type",
		Description: "Register an event contract, or a new immutable version of an existing one. Once registered, events whose payload does not match are rejected with 422 instead of reaching rules.",
		Annotations: writes("Register an event contract", false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input registerEventTypeInput) (*mcp.CallToolResult, any, error) {
		return send(ctx, session, input.Environment, http.MethodPost, "/v1/admin/event-types", map[string]any{"name": input.Name, "schema": input.Schema})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_currency",
		Description: "Create a spendable point currency. Rules award it with award_points; rewards can charge it. XP is separate and needs no currency.",
		Annotations: writes("Create a currency", false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input createCurrencyInput) (*mcp.CallToolResult, any, error) {
		policy := map[string]any{}
		if input.ExpiryDays > 0 {
			policy["expiry_days"] = input.ExpiryDays
		}
		return send(ctx, session, input.Environment, http.MethodPost, "/v1/admin/currencies", map[string]any{"code": input.Code, "name": input.Name, "unit_scale": input.UnitScale, "policy": policy})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_level",
		Description: "Create a level: a name reached at a cumulative XP threshold.",
		Annotations: writes("Create a level", false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input createLevelInput) (*mcp.CallToolResult, any, error) {
		return send(ctx, session, input.Environment, http.MethodPost, "/v1/admin/levels", map[string]any{"level": input.Level, "name": input.Name, "xp_threshold": input.XPThreshold})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_achievement",
		Description: "Create an achievement: a durable milestone unlocked after a number of events of one type. It records recognition only; add a rule to award points or XP.",
		Annotations: writes("Create an achievement", false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input createAchievementInput) (*mcp.CallToolResult, any, error) {
		return send(ctx, session, input.Environment, http.MethodPost, "/v1/admin/achievements", map[string]any{"name": input.Name, "description": input.Description, "event_type": input.EventType, "required_count": input.RequiredCount})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_mission",
		Description: "Create a mission: progress toward a target count of events of one type, for checklists and challenges. It tracks progress only; add a rule to award points or XP.",
		Annotations: writes("Create a mission", false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input createMissionInput) (*mcp.CallToolResult, any, error) {
		return send(ctx, session, input.Environment, http.MethodPost, "/v1/admin/missions", map[string]any{"name": input.Name, "event_type": input.EventType, "target_count": input.TargetCount})
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_reward",
		Description: "Create a digital_unlock reward, free or costing one currency. The product's backend redeems it for a user over REST with an access key; this server never redeems.",
		Annotations: writes("Create a reward", false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input createRewardInput) (*mcp.CallToolResult, any, error) {
		fields := map[string]any{"name": input.Name, "reward_type": "digital_unlock", "description": input.Description, "payload": input.Payload}
		if input.Payload == nil {
			fields["payload"] = map[string]any{}
		}
		if input.CostCurrency != "" || input.CostAmount > 0 {
			fields["cost_currency"], fields["cost_amount"] = input.CostCurrency, input.CostAmount
		}
		return send(ctx, session, input.Environment, http.MethodPost, "/v1/admin/rewards", fields)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "save_rule_draft",
		Description: "Save a rule as a draft: a new rule, or a new version of an existing one. Drafts never run; the published version keeps running until publish_rule replaces it. Run check_rule first.",
		Annotations: writes("Save a rule draft", false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input saveRuleDraftInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		fields := ruleFields(input.EventType, input.Conditions, input.Actions, input.StartsAt, input.EndsAt)
		if input.Description != "" {
			fields["description"] = input.Description
		}
		var draft any
		if input.RuleID == "" {
			if strings.TrimSpace(input.Name) == "" {
				return nil, nil, errors.New("name is required for a new rule")
			}
			fields["name"] = input.Name
			draft, err = session.Client.Send(ctx, http.MethodPost, "/v1/admin/rules", nil, session.Body(environment, fields))
		} else {
			if input.Name != "" {
				fields["name"] = input.Name
			}
			draft, err = session.Client.Send(ctx, http.MethodPost, "/v1/admin/rules/"+url.PathEscape(input.RuleID)+"/versions", session.Scope(environment), fields)
		}
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{
			"environment": environment,
			"draft":       draft,
			"review_url":  session.ConsoleURL + "/automations",
			"next":        "The draft does not run. Show the human the rule and ask before publishing; then call publish_rule with rule_id and the draft's version.",
		}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "send_test_event",
		Description: "Send a real event into an environment, as the product's backend would, and by default wait for processing and return its trace. Matching rules award real points and XP in that environment, so prefer a test or sandbox environment and a test user id.",
		Annotations: writes("Send a test event", true, true),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input sendTestEventInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		eventID := strings.TrimSpace(input.EventID)
		if eventID == "" {
			eventID = testEventID(input.EventType)
		}
		receipt, err := session.Client.Send(ctx, http.MethodPost, "/v1/admin/events/test", nil, session.Body(environment, map[string]any{
			"event_id": eventID, "event_type": input.EventType, "user_id": input.UserID, "payload": input.Payload,
		}))
		if err != nil {
			return nil, nil, err
		}
		result := map[string]any{"environment": environment, "event_id": eventID, "receipt": receipt}
		if input.Wait != nil && !*input.Wait {
			result["next"] = "Processing is asynchronous; call get_event_trace with this event_id."
			return nil, result, nil
		}
		trace, processed, err := waitForTrace(ctx, session, environment, eventID, 8*time.Second)
		if err != nil {
			return nil, nil, err
		}
		result["trace"] = trace
		if !processed {
			result["next"] = "The event was accepted but not processed within eight seconds; check get_diagnostics, then get_event_trace again."
		}
		return nil, result, nil
	})

	registerLeaderboardConfigureTools(server, session)
}

// registerEditTools needs the edit scope: they change what already exists,
// including which rule version runs.
func registerEditTools(server *mcp.Server, session *Session) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_catalog_item",
		Description: "Activate or deactivate a currency, level, achievement, mission, reward or leaderboard, or replace its definition. A leaderboard definition takes name, description and schedule; its key and metric cannot change, and a new schedule takes effect at the next round boundary. A deactivated leaderboard stops scoring.",
		Annotations: writes("Update a catalog item", false, true),
		InputSchema: updateCatalogSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input updateCatalogInput) (*mcp.CallToolResult, any, error) {
		if input.Active == nil && input.Definition == nil {
			return nil, nil, errors.New("set active, definition, or both")
		}
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		result := map[string]any{"environment": environment}
		if input.Definition != nil {
			updated, err := session.Client.Send(ctx, http.MethodPatch, "/v1/admin/"+catalogItemPaths[input.Kind]+"/"+url.PathEscape(input.ID), nil, session.Body(environment, input.Definition))
			if err != nil {
				return nil, nil, err
			}
			result["updated"] = updated
		}
		if input.Active != nil {
			status, err := session.Client.Send(ctx, http.MethodPost, "/v1/admin/catalog-status", nil, session.Body(environment, map[string]any{"resource": input.Kind, "id": input.ID, "active": *input.Active}))
			if err != nil {
				return nil, nil, err
			}
			result["status"] = status
		}
		return nil, result, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "publish_rule",
		Description: "Publish one draft version of a rule so it runs on every new matching event in the environment, replacing the previously published version. Confirm with the human first.",
		Annotations: writes("Publish a rule", true, true),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input publishRuleInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		if input.Version < 1 {
			return nil, nil, errors.New("version must name the draft version to publish")
		}
		published, err := session.Client.Send(ctx, http.MethodPost, "/v1/admin/rules/"+url.PathEscape(input.RuleID)+"/versions/"+strconv.Itoa(input.Version)+"/publish", session.Scope(environment), nil)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"environment": environment, "published": published, "next": "Send a test event with send_test_event to confirm the rule awards what you expect."}, nil
	})

	registerLeaderboardEditTools(server, session)
}

func send(ctx context.Context, session *Session, requestedEnvironment, method, path string, fields map[string]any) (*mcp.CallToolResult, any, error) {
	environment, err := session.Environment(requestedEnvironment)
	if err != nil {
		return nil, nil, err
	}
	created, err := session.Client.Send(ctx, method, path, nil, session.Body(environment, fields))
	if err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"environment": environment, "result": created}, nil
}

func testEventID(eventType string) string {
	random := make([]byte, 4)
	_, _ = rand.Read(random)
	prefix := unsafeIDCharacters.ReplaceAllString(strings.ToLower(eventType), "-")
	if len(prefix) > 80 {
		prefix = prefix[:80]
	}
	return fmt.Sprintf("test_%s_%s", prefix, hex.EncodeToString(random))
}

// waitForTrace polls the event trace until the worker has processed the event.
func waitForTrace(ctx context.Context, session *Session, environment, eventID string, timeout time.Duration) (any, bool, error) {
	deadline := time.Now().Add(timeout)
	path := "/v1/admin/events/" + url.PathEscape(eventID) + "/trace"
	for {
		trace, err := session.Client.Get(ctx, path, session.Scope(environment))
		if err != nil {
			return nil, false, err
		}
		if processed(trace) || time.Now().After(deadline) {
			return trace, processed(trace), nil
		}
		select {
		case <-ctx.Done():
			return trace, false, ctx.Err()
		case <-time.After(400 * time.Millisecond):
		}
	}
}

func processed(trace any) bool {
	document, ok := trace.(map[string]any)
	if !ok {
		return false
	}
	event, ok := document["event"].(map[string]any)
	return ok && event["status"] == "processed"
}
