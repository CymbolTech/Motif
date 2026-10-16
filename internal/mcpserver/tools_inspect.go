package mcpserver

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type environmentInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
}

type listCatalogInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
	Kind        string `json:"kind" jsonschema:"Which catalog to list"`
}

type listRulesInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
	RuleID      string `json:"rule_id,omitempty" jsonschema:"Return every version of this rule instead of the rule list"`
}

type checkRuleInput struct {
	Environment   string           `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
	EventType     string           `json:"event_type" jsonschema:"The event type the rule listens to, for example order.completed"`
	Conditions    map[string]any   `json:"conditions,omitempty" jsonschema:"Condition expression with exactly one operator per object: all, any, exists, eq, neq, gt, gte, lt, lte. Paths look like $.total. Omit to match every event"`
	Actions       []map[string]any `json:"actions" jsonschema:"Ordered actions: {type: award_xp, amount}, {type: award_points, currency, amount} or {type: log, message}"`
	StartsAt      string           `json:"starts_at,omitempty" jsonschema:"Optional inclusive UTC start, RFC 3339"`
	EndsAt        string           `json:"ends_at,omitempty" jsonschema:"Optional exclusive UTC end, RFC 3339"`
	SamplePayload map[string]any   `json:"sample_payload,omitempty" jsonschema:"A realistic event payload to evaluate the conditions against. Strongly recommended"`
}

type eventTraceInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
	EventID     string `json:"event_id" jsonschema:"The event_id the producer sent"`
}

type listEventsInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
	Status      string `json:"status,omitempty" jsonschema:"Only events with this status"`
	Limit       int    `json:"limit,omitempty" jsonschema:"At most this many events, newest first (default 20, maximum 100)"`
}

type userStateInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
	UserID      string `json:"user_id" jsonschema:"The external user id the product sends as user_id"`
}

var (
	listCatalogSchema = inputSchema[listCatalogInput](map[string][]string{"kind": {"currencies", "levels", "achievements", "missions", "rewards", "goals", "campaigns"}})
	listEventsSchema  = inputSchema[listEventsInput](map[string][]string{"status": {"accepted", "processing", "processed", "failed"}})
)

var catalogPaths = map[string]string{
	"currencies": "/v1/admin/currencies", "levels": "/v1/admin/levels", "achievements": "/v1/admin/achievements",
	"missions": "/v1/admin/missions", "rewards": "/v1/admin/rewards", "goals": "/v1/admin/goals", "campaigns": "/v1/admin/campaigns",
}

func readOnly(title string) *mcp.ToolAnnotations {
	closed := false
	return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, OpenWorldHint: &closed}
}

func registerInspectTools(server *mcp.Server, session *Session) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_workspace",
		Description: "Start here. Returns the workspace this token belongs to, the environments it can use, its permissions, plan usage, processing health per environment, and the URLs a backend integration needs.",
		Annotations: readOnly("Workspace overview"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		usage, err := session.Client.Get(ctx, "/v1/admin/usage", url.Values{"tenant_id": {session.TenantID}})
		if err != nil {
			usage = map[string]any{"unavailable": err.Error()}
		}
		health := map[string]any{}
		for index, environment := range session.Environments {
			if index == 3 {
				break
			}
			diagnostics, err := session.Client.Get(ctx, "/v1/admin/diagnostics", session.Scope(environment.ID))
			if err != nil {
				diagnostics = map[string]any{"unavailable": err.Error()}
			}
			health[environment.ID] = diagnostics
		}
		return nil, map[string]any{
			"workspace":           map[string]any{"name": session.Workspace, "status": session.Status},
			"token":               map[string]any{"id": session.TokenID, "scopes": session.Scopes, "created_by": session.Email},
			"environments":        session.Environments,
			"plan_usage":          usage,
			"health":              health,
			"console_url":         session.ConsoleURL,
			"integration_api_url": session.PublicAPIURL,
			"how_this_fits":       "This server configures and inspects the workspace. The product's own backend sends real events to integration_api_url with the environment's REST integration key (X-Motif-API-Key), which a workspace administrator copies from the console; never put that key in client-side code.",
		}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_event_types",
		Description: "List the event contracts registered in an environment, with their current schema version. Events of an unregistered type are accepted without validation.",
		Annotations: readOnly("List event contracts"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input environmentInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		items, err := session.Client.Get(ctx, "/v1/admin/event-types", session.Scope(environment))
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"environment": environment, "event_types": items}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_catalog",
		Description: "List one catalog in an environment: currencies, levels, achievements, missions, rewards, goals or campaigns, including inactive items.",
		Annotations: readOnly("List catalog"),
		InputSchema: listCatalogSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input listCatalogInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		items, err := session.Client.Get(ctx, catalogPaths[input.Kind], session.Scope(environment))
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"environment": environment, "kind": input.Kind, "items": items}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_rules",
		Description: "List the rules in an environment with their status and current version, or every version of one rule. Only published versions run.",
		Annotations: readOnly("List rules"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input listRulesInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		if input.RuleID != "" {
			versions, err := session.Client.Get(ctx, "/v1/admin/rules/"+url.PathEscape(input.RuleID)+"/versions", session.Scope(environment))
			if err != nil {
				return nil, nil, err
			}
			return nil, map[string]any{"environment": environment, "rule_id": input.RuleID, "versions": versions}, nil
		}
		rules, err := session.Client.Get(ctx, "/v1/admin/rules", session.Scope(environment))
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"environment": environment, "rules": rules}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "check_rule",
		Description: "Check a rule definition without saving it: validates conditions and actions, confirms the event contract and currencies exist, and evaluates a sample payload, naming the exact expression that decided a non-match. Run this before save_rule_draft.",
		Annotations: readOnly("Check a rule"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input checkRuleInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		fields := ruleFields(input.EventType, input.Conditions, input.Actions, input.StartsAt, input.EndsAt)
		if input.SamplePayload != nil {
			fields["sample_payload"] = input.SamplePayload
		}
		result, err := session.Client.Send(ctx, http.MethodPost, "/v1/admin/rules/check", nil, session.Body(environment, fields))
		if err != nil {
			return nil, nil, err
		}
		return nil, result, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_event_trace",
		Description: "Explain what one event did: its processing status, the rules that ran and what they awarded (points, XP, missions, achievements), and why each published rule for the event type matched or not.",
		Annotations: readOnly("Trace an event"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input eventTraceInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		trace, err := session.Client.Get(ctx, "/v1/admin/events/"+url.PathEscape(input.EventID)+"/trace", session.Scope(environment))
		if err != nil {
			return nil, nil, err
		}
		return nil, trace, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_events",
		Description: "List recent events received in an environment, newest first, optionally by status.",
		Annotations: readOnly("List events"),
		InputSchema: listEventsSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input listEventsInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		query := session.Scope(environment)
		if input.Status != "" {
			query.Set("status", input.Status)
		}
		query.Set("limit", strconv.Itoa(clamp(input.Limit, 20, 100)))
		events, err := session.Client.Get(ctx, "/v1/admin/events", query)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"environment": environment, "events": events}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_user_state",
		Description: "Read one user's state in an environment: balances, XP and level, mission and achievement progress, and recent ledger entries.",
		Annotations: readOnly("Read user state"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input userStateInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		query := session.Scope(environment)
		query.Set("user_id", input.UserID)
		profile, err := session.Client.Get(ctx, "/v1/admin/user-profile", query)
		if err != nil {
			return nil, nil, err
		}
		query.Set("limit", "20")
		transactions, err := session.Client.Get(ctx, "/v1/admin/user-transactions", query)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"environment": environment, "user_id": input.UserID, "profile": profile, "recent_transactions": transactions}, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_diagnostics",
		Description: "Find operational problems in an environment: processing backlog, failed rule executions with their last error, and failed redemptions.",
		Annotations: readOnly("Diagnose problems"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input environmentInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		summary, err := session.Client.Get(ctx, "/v1/admin/diagnostics", session.Scope(environment))
		if err != nil {
			return nil, nil, err
		}
		failed := session.Scope(environment)
		failed.Set("status", "failed")
		failed.Set("limit", "20")
		executions, err := session.Client.Get(ctx, "/v1/admin/rule-executions", failed)
		if err != nil {
			return nil, nil, err
		}
		redemptions, err := session.Client.Get(ctx, "/v1/admin/redemptions", failed)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"environment": environment, "summary": summary, "failed_rule_executions": executions, "failed_redemptions": redemptions}, nil
	})
}

func ruleFields(eventType string, conditions map[string]any, actions []map[string]any, startsAt, endsAt string) map[string]any {
	fields := map[string]any{"event_type": eventType, "actions": actions}
	if conditions != nil {
		fields["conditions"] = conditions
	}
	if startsAt != "" {
		fields["starts_at"] = startsAt
	}
	if endsAt != "" {
		fields["ends_at"] = endsAt
	}
	return fields
}

func clamp(value, fallback, maximum int) int {
	if value <= 0 {
		return fallback
	}
	if value > maximum {
		return maximum
	}
	return value
}
