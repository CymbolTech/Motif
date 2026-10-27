package mcpserver

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type getLeaderboardInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
	Leaderboard string `json:"leaderboard" jsonschema:"The board's key, such as weekly, or its id"`
	Round       string `json:"round,omitempty" jsonschema:"current (default), previous for the last finished round, or a round number"`
	Limit       int    `json:"limit,omitempty" jsonschema:"How many ranked entries to return (default 10, maximum 100)"`
}

type createLeaderboardInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
	Key         string `json:"key" jsonschema:"Stable key the product backend reads the board by, such as weekly: lowercase letters, numbers, hyphens and underscores. It cannot change later"`
	Name        string `json:"name" jsonschema:"Display name, such as Weekly top customers"`
	Description string `json:"description,omitempty" jsonschema:"Who wins and what they get"`
	Rank        string `json:"rank" jsonschema:"What to rank: points earned in one currency, or XP earned. It cannot change later"`
	Currency    string `json:"currency,omitempty" jsonschema:"Currency code to rank; required when rank is points"`
	Rounds      string `json:"rounds" jsonschema:"manual: an operator starts each round, so rounds can differ in length. recurring: a new round starts at every boundary on its own"`
	Every       string `json:"every,omitempty" jsonschema:"Recurring boards: the period unit"`
	Interval    int    `json:"interval,omitempty" jsonschema:"Recurring boards: repeat every this many units (default 1), for example 2 with week for fortnightly rounds"`
	Timezone    string `json:"timezone,omitempty" jsonschema:"IANA time zone whose midnight starts each recurring round, such as Asia/Tehran (default UTC)"`
	WeekStart   string `json:"week_start,omitempty" jsonschema:"Weekly boards: the weekday rounds start on (default monday)"`
}

type startRoundInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
	Leaderboard string `json:"leaderboard" jsonschema:"The manual board's key or id"`
	Name        string `json:"name,omitempty" jsonschema:"Round name, such as Week 45 (default Round N)"`
	StartsAt    string `json:"starts_at,omitempty" jsonschema:"RFC 3339 start in the future to schedule the round; omit to start now"`
	EndsAt      string `json:"ends_at,omitempty" jsonschema:"RFC 3339 time the round closes on its own; omit to keep it open until someone ends it"`
	EndCurrent  *bool  `json:"end_current,omitempty" jsonschema:"End the round that is running when this one starts (default true), so no award falls between rounds"`
}

type updateRoundInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
	Leaderboard string `json:"leaderboard" jsonschema:"The board's key or id"`
	RoundID     string `json:"round_id,omitempty" jsonschema:"The round to change; omit for the running round"`
	Name        string `json:"name,omitempty" jsonschema:"A new round name"`
	EndsAt      string `json:"ends_at,omitempty" jsonschema:"Move the end of a running or scheduled manual round to this RFC 3339 time, for example to run 8 days instead of 7"`
	OpenEnded   bool   `json:"open_ended,omitempty" jsonschema:"Remove the end so the round runs until someone ends it"`
}

type deleteLeaderboardInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
	Leaderboard string `json:"leaderboard" jsonschema:"The board's key or id"`
}

type endRoundInput struct {
	Environment string `json:"environment,omitempty" jsonschema:"Environment id. Optional when the token can use exactly one environment."`
	Leaderboard string `json:"leaderboard" jsonschema:"The manual board's key or id"`
	RoundID     string `json:"round_id,omitempty" jsonschema:"The round to end now, or a scheduled round to cancel; omit for the running round"`
}

var createLeaderboardSchema = inputSchema[createLeaderboardInput](map[string][]string{
	"rank": {"points", "xp"}, "rounds": {"manual", "recurring"}, "every": {"day", "week", "month"},
	"week_start": {"saturday", "sunday", "monday", "tuesday", "wednesday", "thursday", "friday"},
})

func leaderboardPath(board string, suffix ...string) string {
	path := "/v1/admin/leaderboards/" + url.PathEscape(board)
	for _, part := range suffix {
		path += "/" + url.PathEscape(part)
	}
	return path
}

func registerLeaderboardInspectTools(server *mcp.Server, session *Session) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_leaderboard",
		Description: "Read a leaderboard's ranking for its current round, the last finished round, or a numbered round, plus its round history with each round's leader. Between two manual rounds the current view shows the last round's final standings.",
		Annotations: readOnly("Read a leaderboard"),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input getLeaderboardInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		query := session.Scope(environment)
		query.Set("round", input.Round)
		if input.Round == "" {
			query.Set("round", "current")
		}
		query.Set("limit", strconv.Itoa(clamp(input.Limit, 10, 100)))
		standings, err := session.Client.Get(ctx, leaderboardPath(input.Leaderboard, "standings"), query)
		if err != nil {
			return nil, nil, err
		}
		history := session.Scope(environment)
		history.Set("limit", "10")
		rounds, err := session.Client.Get(ctx, leaderboardPath(input.Leaderboard, "rounds"), history)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"environment": environment, "standings": standings, "recent_rounds": rounds}, nil
	})
}

func registerLeaderboardConfigureTools(server *mcp.Server, session *Session) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "create_leaderboard",
		Description: "Create a leaderboard that ranks users by points earned in one currency or XP earned during each round. Spending, expiry, conversions and refunds never change a rank. Manual boards need start_leaderboard_round before anything counts; recurring boards start rounds on their own. The product backend reads it with GET /v1/leaderboards/{key}.",
		Annotations: writes("Create a leaderboard", false, false),
		InputSchema: createLeaderboardSchema,
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input createLeaderboardInput) (*mcp.CallToolResult, any, error) {
		metric := map[string]any{"type": input.Rank}
		if input.Rank == "points" {
			metric["currency"] = input.Currency
		}
		schedule := map[string]any{"type": input.Rounds, "timezone": input.Timezone}
		if input.Rounds == "recurring" {
			schedule["every"], schedule["interval"] = input.Every, input.Interval
			if input.WeekStart != "" {
				schedule["week_start"] = input.WeekStart
			}
		}
		_, created, err := send(ctx, session, input.Environment, http.MethodPost, "/v1/admin/leaderboards", map[string]any{"key": input.Key, "name": input.Name, "description": input.Description, "metric": metric, "schedule": schedule})
		if err != nil {
			return nil, nil, err
		}
		result := created.(map[string]any)
		result["next"] = "The product backend reads the board with GET /v1/leaderboards/" + input.Key + " and its integration key."
		if input.Rounds == "manual" {
			result["next"] = "Nothing counts until a round runs: call start_leaderboard_round. " + result["next"].(string)
		}
		return nil, result, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "start_leaderboard_round",
		Description: "Start or schedule a round of a manual leaderboard. By default the running round ends at the same instant, so one call rotates the board and no award falls between rounds. Scores start from zero in the new round.",
		Annotations: writes("Start a leaderboard round", true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input startRoundInput) (*mcp.CallToolResult, any, error) {
		fields := map[string]any{}
		for key, value := range map[string]string{"name": input.Name, "starts_at": input.StartsAt, "ends_at": input.EndsAt} {
			if value != "" {
				fields[key] = value
			}
		}
		if input.EndCurrent != nil {
			fields["end_current"] = *input.EndCurrent
		}
		return send(ctx, session, input.Environment, http.MethodPost, leaderboardPath(input.Leaderboard, "rounds"), fields)
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "update_leaderboard_round",
		Description: "Rename a round, or on a manual board move its end (for example to run 8 days instead of 7) or remove the end so it runs until someone ends it.",
		Annotations: writes("Update a leaderboard round", false, true),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input updateRoundInput) (*mcp.CallToolResult, any, error) {
		if input.Name == "" && input.EndsAt == "" && !input.OpenEnded {
			return nil, nil, errors.New("set name, ends_at, or open_ended")
		}
		if input.EndsAt != "" && input.OpenEnded {
			return nil, nil, errors.New("set ends_at or open_ended, not both")
		}
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		roundID, err := resolveRound(ctx, session, environment, input.Leaderboard, input.RoundID)
		if err != nil {
			return nil, nil, err
		}
		fields := map[string]any{}
		if input.Name != "" {
			fields["name"] = input.Name
		}
		if input.EndsAt != "" {
			fields["ends_at"] = input.EndsAt
		}
		if input.OpenEnded {
			fields["ends_at"] = nil
		}
		updated, err := session.Client.Send(ctx, http.MethodPatch, leaderboardPath(input.Leaderboard, "rounds", roundID), nil, session.Body(environment, fields))
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"environment": environment, "result": updated}, nil
	})

	registerLeaderboardDeleteTool(server, session)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "end_leaderboard_round",
		Description: "End the running round of a manual leaderboard now, freezing its final standings, or cancel a scheduled round before it starts. Confirm with the human first: an ended round cannot be reopened.",
		Annotations: writes("End a leaderboard round", true, true),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input endRoundInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		roundID, err := resolveRound(ctx, session, environment, input.Leaderboard, input.RoundID)
		if err != nil {
			return nil, nil, err
		}
		action := "end"
		if input.RoundID != "" {
			var rounds []map[string]any
			if err := session.Client.Do(ctx, http.MethodGet, leaderboardPath(input.Leaderboard, "rounds"), session.Scope(environment), nil, &rounds); err != nil {
				return nil, nil, err
			}
			for _, round := range rounds {
				if round["id"] == roundID && round["status"] == "scheduled" {
					action = "cancel"
				}
			}
		}
		ended, err := session.Client.Send(ctx, http.MethodPost, leaderboardPath(input.Leaderboard, "rounds", roundID, action), nil, session.Body(environment, nil))
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"environment": environment, "action": map[string]string{"end": "ended", "cancel": "cancelled"}[action], "round": ended}, nil
	})
}

func registerLeaderboardDeleteTool(server *mcp.Server, session *Session) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_leaderboard",
		Description: "Permanently delete a leaderboard with all its rounds and scores; GET /v1/leaderboards/{key} then returns 404. Balances and XP are not touched. Confirm with the human first, and prefer update_catalog_item with active false to stop scoring but keep history.",
		Annotations: writes("Delete a leaderboard", true, true),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input deleteLeaderboardInput) (*mcp.CallToolResult, any, error) {
		environment, err := session.Environment(input.Environment)
		if err != nil {
			return nil, nil, err
		}
		deleted, err := session.Client.Send(ctx, http.MethodDelete, leaderboardPath(input.Leaderboard), session.Scope(environment), nil)
		if err != nil {
			return nil, nil, err
		}
		return nil, map[string]any{"environment": environment, "result": deleted}, nil
	})
}

// resolveRound returns roundID, or the id of the board's running round.
func resolveRound(ctx context.Context, session *Session, environment, board, roundID string) (string, error) {
	if roundID != "" {
		return roundID, nil
	}
	var standings struct {
		Round *struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"round"`
	}
	query := session.Scope(environment)
	query.Set("limit", "1")
	if err := session.Client.Do(ctx, http.MethodGet, leaderboardPath(board, "standings"), query, nil, &standings); err != nil {
		return "", err
	}
	if standings.Round == nil || standings.Round.Status != "active" {
		return "", errors.New("no round is running on this leaderboard; pass round_id, or start one with start_leaderboard_round")
	}
	return standings.Round.ID, nil
}
