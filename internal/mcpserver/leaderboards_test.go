package mcpserver

import (
	"slices"
	"strings"
	"testing"
)

func TestLeaderboardToolsFollowTheTokenScopes(t *testing.T) {
	reader, err := connect(t, &fakeAPI{scopes: []string{"read"}}, "motif_at_valid")
	if err != nil {
		t.Fatal(err)
	}
	names := toolNames(t, reader)
	if !slices.Contains(names, "get_leaderboard") || slices.Contains(names, "create_leaderboard") || slices.Contains(names, "end_leaderboard_round") {
		t.Fatalf("read-only tools = %v, want get_leaderboard only", names)
	}
	writer, err := connect(t, &fakeAPI{scopes: []string{"read", "write"}}, "motif_at_valid")
	if err != nil {
		t.Fatal(err)
	}
	names = toolNames(t, writer)
	for _, want := range []string{"create_leaderboard", "start_leaderboard_round", "update_leaderboard_round", "end_leaderboard_round", "delete_leaderboard"} {
		if !slices.Contains(names, want) {
			t.Fatalf("write tools %v are missing %s", names, want)
		}
	}
}

func TestCreateLeaderboardSendsMetricAndSchedule(t *testing.T) {
	api := &fakeAPI{scopes: []string{"read", "write"}, environments: []string{"sandbox"}}
	session, err := connect(t, api, "motif_at_valid")
	if err != nil {
		t.Fatal(err)
	}
	result, text := call(t, session, "create_leaderboard", map[string]any{"key": "monthly", "name": "Monthly", "rank": "points", "currency": "coins", "rounds": "recurring", "every": "month", "timezone": "Asia/Tehran"})
	if result.IsError || !strings.Contains(text, "/v1/leaderboards/monthly") {
		t.Fatalf("create_leaderboard = %s", text)
	}
	sent := api.last("/v1/admin/leaderboards")
	metric, _ := sent.Body["metric"].(map[string]any)
	schedule, _ := sent.Body["schedule"].(map[string]any)
	if sent.Body["environment_id"] != "sandbox" || metric["currency"] != "coins" || schedule["every"] != "month" || schedule["timezone"] != "Asia/Tehran" {
		t.Fatalf("create_leaderboard sent %+v", sent.Body)
	}

	_, text = call(t, session, "create_leaderboard", map[string]any{"key": "weekly", "name": "Weekly", "rank": "xp", "rounds": "manual"})
	if !strings.Contains(text, "start_leaderboard_round") {
		t.Fatalf("a manual board should point at start_leaderboard_round: %s", text)
	}
	if schedule, _ := api.last("/v1/admin/leaderboards").Body["schedule"].(map[string]any); schedule["every"] != nil {
		t.Fatalf("a manual board must not send recurring fields: %+v", schedule)
	}
}

func TestRoundToolsDefaultToTheRunningRound(t *testing.T) {
	api := &fakeAPI{scopes: []string{"read", "write"}, environments: []string{"sandbox"}}
	session, err := connect(t, api, "motif_at_valid")
	if err != nil {
		t.Fatal(err)
	}
	if result, text := call(t, session, "end_leaderboard_round", map[string]any{"leaderboard": "weekly"}); result.IsError || !strings.Contains(text, `"ended"`) {
		t.Fatalf("end_leaderboard_round = %s", text)
	}
	if sent := api.last("/v1/admin/leaderboards/weekly/rounds/round_7/end"); sent.Method != "POST" || sent.Body["environment_id"] != "sandbox" {
		t.Fatalf("end_leaderboard_round sent %+v", sent)
	}

	if result, text := call(t, session, "end_leaderboard_round", map[string]any{"leaderboard": "weekly", "round_id": "round_8"}); result.IsError || !strings.Contains(text, `"cancelled"`) {
		t.Fatalf("ending a scheduled round should cancel it: %s", text)
	}

	if result, text := call(t, session, "update_leaderboard_round", map[string]any{"leaderboard": "weekly", "open_ended": true}); result.IsError {
		t.Fatalf("update_leaderboard_round = %s", text)
	}
	sent := api.last("/v1/admin/leaderboards/weekly/rounds/round_7")
	if value, ok := sent.Body["ends_at"]; sent.Method != "PATCH" || !ok || value != nil {
		t.Fatalf("open_ended must send ends_at null, sent %+v", sent)
	}
	if result, _ := call(t, session, "update_leaderboard_round", map[string]any{"leaderboard": "weekly"}); !result.IsError {
		t.Fatal("an update without changes must be rejected before calling the API")
	}
}

func TestUserStateIncludesLeaderboardRanks(t *testing.T) {
	api := &fakeAPI{scopes: []string{"read"}, environments: []string{"sandbox"}}
	session, err := connect(t, api, "motif_at_valid")
	if err != nil {
		t.Fatal(err)
	}
	if result, text := call(t, session, "get_user_state", map[string]any{"user_id": "user-42"}); result.IsError || !strings.Contains(text, `"leaderboards"`) {
		t.Fatalf("get_user_state = %s", text)
	}
	if sent := api.last("/v1/admin/user-leaderboards"); !strings.Contains(sent.Query, "user_id=user-42") {
		t.Fatalf("get_user_state sent %+v", sent)
	}
}

func TestDeleteLeaderboardSendsAScopedDelete(t *testing.T) {
	api := &fakeAPI{scopes: []string{"read", "write"}, environments: []string{"sandbox"}}
	session, err := connect(t, api, "motif_at_valid")
	if err != nil {
		t.Fatal(err)
	}
	if result, text := call(t, session, "delete_leaderboard", map[string]any{"leaderboard": "weekly"}); result.IsError {
		t.Fatalf("delete_leaderboard = %s", text)
	}
	if sent := api.last("/v1/admin/leaderboards/weekly"); sent.Method != "DELETE" || !strings.Contains(sent.Query, "environment_id=sandbox") {
		t.Fatalf("delete_leaderboard sent %+v", sent)
	}
}
