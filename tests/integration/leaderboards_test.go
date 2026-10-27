//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/admin"
	"github.com/motifengine/motifengine/internal/auth"
	"github.com/motifengine/motifengine/internal/leaderboards"
	"github.com/motifengine/motifengine/internal/points"
	"github.com/motifengine/motifengine/internal/progression"
)

const leaderboardAdminKey = "leaderboard-admin-key"
const leaderboardAPIKey = "leaderboard-integration-key"

type leaderboardAPI struct {
	t       *testing.T
	admin   *httptest.Server
	public  *httptest.Server
	balance *points.Ledger
}

func newLeaderboardAPI(t *testing.T, db *pgxpool.Pool) leaderboardAPI {
	t.Helper()
	if _, err := db.Exec(context.Background(), `INSERT INTO access_keys (id, tenant_id, environment_id, name, key_hash, scopes) VALUES (md5(random()::text), 'integration-tenant', 'sandbox', 'test', $1, ARRAY['read', 'write'])`, auth.Digest(leaderboardAPIKey)); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/v1/leaderboards", keyed(db, leaderboards.NewHandler(db)))
	mux.Handle("/v1/leaderboards/", keyed(db, leaderboards.NewHandler(db)))
	mux.Handle("/v1/users/", keyed(db, points.NewHandler(db)))
	api := leaderboardAPI{t: t, admin: httptest.NewServer(admin.NewHandler(db, leaderboardAdminKey)), public: httptest.NewServer(mux), balance: points.NewLedger(db)}
	t.Cleanup(api.admin.Close)
	t.Cleanup(api.public.Close)
	return api
}

func (api leaderboardAPI) do(server *httptest.Server, header, key, method, path, body string, target any) int {
	api.t.Helper()
	request, err := http.NewRequest(method, server.URL+path, bytes.NewBufferString(body))
	if err != nil {
		api.t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(header, key)
	response, err := server.Client().Do(request)
	if err != nil {
		api.t.Fatal(err)
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(response.Body)
	if err != nil {
		api.t.Fatal(err)
	}
	if target != nil && response.StatusCode < 300 {
		if err := json.Unmarshal(payload, target); err != nil {
			api.t.Fatalf("%s %s: decode %s: %v", method, path, payload, err)
		}
	}
	if response.StatusCode >= 300 && target != nil {
		api.t.Logf("%s %s returned %d: %s", method, path, response.StatusCode, payload)
	}
	return response.StatusCode
}

func (api leaderboardAPI) adminCall(method, path, body string, target any) int {
	api.t.Helper()
	return api.do(api.admin, "X-Motif-Admin-Key", leaderboardAdminKey, method, path, body, target)
}

func (api leaderboardAPI) standings(path string) leaderboards.Standings {
	api.t.Helper()
	var standings leaderboards.Standings
	if status := api.do(api.public, "X-Motif-API-Key", leaderboardAPIKey, http.MethodGet, path, "", &standings); status != http.StatusOK {
		api.t.Fatalf("GET %s returned %d", path, status)
	}
	return standings
}

func (api leaderboardAPI) award(user string, amount int64, key string) {
	api.t.Helper()
	if _, err := api.balance.Award(context.Background(), points.Scope{TenantID: "integration-tenant", EnvironmentID: "sandbox"}, user, "points", amount, key, nil); err != nil {
		api.t.Fatalf("award %s: %v", key, err)
	}
}

func scoreRows(standings leaderboards.Standings) string {
	result := ""
	for _, entry := range standings.Entries {
		result += fmt.Sprintf("%d:%s:%d ", entry.Rank, entry.UserID, entry.Score)
	}
	return result
}

// The café flow: a manual weekly board that staff rotate by hand, read by
// the café's backend from a stable key.
func TestLeaderboardManualRoundsScoreAndRotate(t *testing.T) {
	db := disposableDatabase(t)
	seedCurrency(t, db.pool)
	api := newLeaderboardAPI(t, db.pool)
	scope := points.Scope{TenantID: "integration-tenant", EnvironmentID: "sandbox"}
	ctx := context.Background()

	var board leaderboards.Leaderboard
	if status := api.adminCall(http.MethodPost, "/v1/admin/leaderboards", `{"tenant_id":"integration-tenant","environment_id":"sandbox","key":"weekly","name":"Weekly","metric":{"type":"points","currency":"points"},"schedule":{"type":"manual"}}`, &board); status != http.StatusCreated {
		t.Fatalf("create board returned %d", status)
	}
	if status := api.adminCall(http.MethodPost, "/v1/admin/leaderboards", `{"tenant_id":"integration-tenant","environment_id":"sandbox","key":"weekly","name":"Again","metric":{"type":"xp"},"schedule":{"type":"manual"}}`, nil); status != http.StatusConflict {
		t.Fatalf("duplicate key returned %d; want 409", status)
	}

	// Points earned before the first round count nowhere.
	api.award("user-early", 500, "early")
	if standings := api.standings("/v1/leaderboards/weekly"); standings.Round != nil || len(standings.Entries) != 0 {
		t.Fatalf("board without rounds = %+v; want no round", standings)
	}

	roundsPath := "/v1/admin/leaderboards/" + board.ID + "/rounds"
	var week1 leaderboards.Round
	if status := api.adminCall(http.MethodPost, roundsPath, `{"tenant_id":"integration-tenant","environment_id":"sandbox","name":"Week 1"}`, &week1); status != http.StatusCreated || week1.Status != leaderboards.RoundActive || week1.Number != 1 {
		t.Fatalf("create week 1 returned %d: %+v", status, week1)
	}

	api.award("user-a", 50, "a-1")
	api.award("user-a", 50, "a-1") // replay: must not count twice
	api.award("user-b", 50, "b-1")
	api.award("user-c", 30, "c-1")
	// A refund of a failed redemption and a spend do not change the ranking.
	if _, err := api.balance.Release(ctx, scope, "user-c", "points", 100, "c-refund", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := api.balance.Spend(ctx, scope, "user-a", "points", 20, "a-spend", nil); err != nil {
		t.Fatal(err)
	}
	// XP does not feed a points board.
	if _, err := progression.AwardXP(ctx, db.pool, progression.Scope{TenantID: scope.TenantID, EnvironmentID: scope.EnvironmentID}, "user-c", 1000, "c-xp", nil); err != nil {
		t.Fatal(err)
	}

	standings := api.standings("/v1/leaderboards/weekly")
	if standings.Round == nil || standings.Round.ID != week1.ID || standings.Participants != 3 {
		t.Fatalf("week 1 standings = %+v", standings)
	}
	if got := scoreRows(standings); got != "1:user-a:50 2:user-b:50 3:user-c:30 " {
		t.Fatalf("week 1 ranking = %q", got)
	}
	if page := api.standings("/v1/leaderboards/weekly?limit=1&offset=1"); scoreRows(page) != "2:user-b:50 " {
		t.Fatalf("second page = %q", scoreRows(page))
	}

	var mine struct {
		Leaderboards []leaderboards.UserStanding `json:"leaderboards"`
	}
	if status := api.do(api.public, "X-Motif-API-Key", leaderboardAPIKey, http.MethodGet, "/v1/users/user-b/leaderboards", "", &mine); status != http.StatusOK || len(mine.Leaderboards) != 1 || mine.Leaderboards[0].Entry == nil || mine.Leaderboards[0].Entry.Rank != 2 {
		t.Fatalf("user standings returned %d: %+v", status, mine)
	}

	// Without end_current a second round would overlap the running one.
	if status := api.adminCall(http.MethodPost, roundsPath, `{"tenant_id":"integration-tenant","environment_id":"sandbox","end_current":false}`, nil); status != http.StatusConflict {
		t.Fatalf("overlapping round returned %d; want 409", status)
	}
	// One call ends week 1 and starts week 2 at the same instant.
	var week2 leaderboards.Round
	if status := api.adminCall(http.MethodPost, roundsPath, `{"tenant_id":"integration-tenant","environment_id":"sandbox","name":"Week 2"}`, &week2); status != http.StatusCreated || week2.Number != 2 {
		t.Fatalf("create week 2 returned %d: %+v", status, week2)
	}
	api.award("user-c", 5, "c-2")
	current := api.standings("/v1/leaderboards/weekly")
	if current.Round == nil || current.Round.ID != week2.ID || scoreRows(current) != "1:user-c:5 " {
		t.Fatalf("week 2 standings = %+v", current)
	}
	previous := api.standings("/v1/leaderboards/weekly?round=previous")
	if previous.Round == nil || previous.Round.ID != week1.ID || previous.Round.Status != leaderboards.RoundEnded || !previous.Round.EndsAt.Equal(week2.StartsAt) {
		t.Fatalf("previous round = %+v; want week 1 ending when week 2 started", previous.Round)
	}
	if byNumber := api.standings("/v1/leaderboards/weekly?round=1"); scoreRows(byNumber) != scoreRows(standings) {
		t.Fatalf("round 1 by number = %q", scoreRows(byNumber))
	}

	// Stretch week 2 to eight days, then end it early by hand.
	end := week2.StartsAt.Add(8 * 24 * time.Hour).Format(time.RFC3339)
	var stretched leaderboards.Round
	if status := api.adminCall(http.MethodPatch, roundsPath+"/"+week2.ID, `{"tenant_id":"integration-tenant","environment_id":"sandbox","ends_at":"`+end+`"}`, &stretched); status != http.StatusOK || stretched.EndsAt == nil {
		t.Fatalf("stretch week 2 returned %d: %+v", status, stretched)
	}
	if status := api.adminCall(http.MethodPost, roundsPath+"/"+week2.ID+"/end", `{"tenant_id":"integration-tenant","environment_id":"sandbox"}`, nil); status != http.StatusOK {
		t.Fatalf("end week 2 returned %d", status)
	}
	api.award("user-c", 7, "c-3")
	// Between rounds the board shows the final standings of the last round.
	final := api.standings("/v1/leaderboards/weekly")
	if final.Round == nil || final.Round.ID != week2.ID || final.Round.Status != leaderboards.RoundEnded || scoreRows(final) != "1:user-c:5 " {
		t.Fatalf("standings between rounds = %+v", final)
	}

	// A scheduled round can be cancelled before it starts.
	next := time.Now().UTC().Add(48 * time.Hour).Format(time.RFC3339)
	var scheduled leaderboards.Round
	if status := api.adminCall(http.MethodPost, roundsPath, `{"tenant_id":"integration-tenant","environment_id":"sandbox","starts_at":"`+next+`"}`, &scheduled); status != http.StatusCreated || scheduled.Status != leaderboards.RoundScheduled {
		t.Fatalf("schedule round returned %d: %+v", status, scheduled)
	}
	if upcoming := api.standings("/v1/leaderboards/weekly"); upcoming.NextRound == nil || upcoming.NextRound.ID != scheduled.ID {
		t.Fatalf("next round = %+v; want the scheduled round", upcoming.NextRound)
	}
	if status := api.adminCall(http.MethodPost, roundsPath+"/"+scheduled.ID+"/cancel", `{"tenant_id":"integration-tenant","environment_id":"sandbox"}`, nil); status != http.StatusOK {
		t.Fatalf("cancel scheduled round returned %d", status)
	}

	// Another tenant's key cannot read the board.
	if status := api.do(api.public, "X-Motif-API-Key", "unknown-key", http.MethodGet, "/v1/leaderboards/weekly", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("unknown key returned %d; want 401", status)
	}
}

func recordXP(t *testing.T, db *pgxpool.Pool, user string, amount int64, at time.Time) {
	t.Helper()
	ctx := context.Background()
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := leaderboards.RecordXP(ctx, tx, "integration-tenant", "sandbox", user, amount, at); err != nil {
		t.Fatalf("record xp at %s: %v", at, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func insertBoard(t *testing.T, db *pgxpool.Pool, request leaderboards.BoardRequest, now time.Time) leaderboards.Leaderboard {
	t.Helper()
	request, err := leaderboards.ValidateBoardRequest(request, now)
	if err != nil {
		t.Fatal(err)
	}
	var board leaderboards.Leaderboard
	if err := pgx.BeginFunc(context.Background(), db, func(tx pgx.Tx) error {
		board, err = leaderboards.InsertBoard(context.Background(), tx, request)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return board
}

// Recurring boards create each round the first time it is needed and align
// rounds to the schedule, except the first, which starts with the board.
func TestLeaderboardRecurringRoundsRollOver(t *testing.T) {
	db := disposableDatabase(t)
	seedCurrency(t, db.pool)
	ctx := context.Background()
	day1 := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	board := insertBoard(t, db.pool, leaderboards.BoardRequest{TenantID: "integration-tenant", EnvironmentID: "sandbox", Key: "daily-xp", Name: "Daily XP",
		Metric: &leaderboards.Metric{Type: leaderboards.MetricXP}, Schedule: &leaderboards.Schedule{Type: leaderboards.ScheduleRecurring, Every: "day", Anchor: &day1}}, day1)
	created := day1.Add(10 * time.Hour)
	if _, err := db.pool.Exec(ctx, `UPDATE leaderboards SET created_at = $2 WHERE id = $1`, board.ID, created); err != nil {
		t.Fatal(err)
	}
	board.CreatedAt = created

	recordXP(t, db.pool, "user-a", 10, day1.Add(9*time.Hour)) // before the board existed
	recordXP(t, db.pool, "user-a", 10, day1.Add(11*time.Hour))
	recordXP(t, db.pool, "user-b", 15, day1.Add(20*time.Hour))
	recordXP(t, db.pool, "user-a", 3, day1.Add(25*time.Hour))

	rounds, err := leaderboards.ListRounds(ctx, db.pool, board, 10, day1.Add(26*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(rounds) != 2 {
		t.Fatalf("rounds = %+v; want 2", rounds)
	}
	second, first := rounds[0], rounds[1]
	if first.Number != 1 || !first.StartsAt.Equal(created) || !first.EndsAt.Equal(day1.Add(24*time.Hour)) || first.Leader == nil || first.Leader.UserID != "user-b" || *first.Participants != 2 {
		t.Fatalf("first round = %+v leader %+v", first, first.Leader)
	}
	if second.Number != 2 || !second.StartsAt.Equal(day1.Add(24*time.Hour)) || !second.EndsAt.Equal(day1.Add(48*time.Hour)) || second.Status != leaderboards.RoundActive || second.Leader.Score != 3 {
		t.Fatalf("second round = %+v leader %+v", second, second.Leader)
	}

	// A day with no activity leaves no round behind; reading creates it.
	standings, err := leaderboards.LoadStandings(ctx, db.pool, board, "current", 10, 0, day1.Add(74*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if standings.Round == nil || standings.Round.Number != 3 || !standings.Round.StartsAt.Equal(day1.Add(72*time.Hour)) || len(standings.Entries) != 0 {
		t.Fatalf("day 4 standings = %+v", standings)
	}

	// Manual rounds are rejected on a recurring board.
	if _, err := leaderboards.CreateRound(ctx, db.pool, leaderboards.Scope{TenantID: "integration-tenant", EnvironmentID: "sandbox"}, board.ID, leaderboards.RoundRequest{}, day1.Add(74*time.Hour)); err != leaderboards.ErrRecurringBoard {
		t.Fatalf("manual round on recurring board error = %v", err)
	}
}

// Switching a manual board to a schedule ends its open round at the next
// boundary, and scheduled rounds have to be cancelled first.
func TestLeaderboardSwitchManualToRecurring(t *testing.T) {
	db := disposableDatabase(t)
	seedCurrency(t, db.pool)
	ctx := context.Background()
	scope := leaderboards.Scope{TenantID: "integration-tenant", EnvironmentID: "sandbox"}
	now := time.Now().UTC()
	board := insertBoard(t, db.pool, leaderboards.BoardRequest{TenantID: scope.TenantID, EnvironmentID: scope.EnvironmentID, Key: "weekly", Name: "Weekly",
		Metric: &leaderboards.Metric{Type: leaderboards.MetricPoints, Currency: "points"}, Schedule: &leaderboards.Schedule{Type: leaderboards.ScheduleManual}}, now)
	open, err := leaderboards.CreateRound(ctx, db.pool, scope, board.ID, leaderboards.RoundRequest{}, now)
	if err != nil {
		t.Fatal(err)
	}
	later := now.Add(10 * 24 * time.Hour)
	scheduled, err := leaderboards.CreateRound(ctx, db.pool, scope, board.ID, leaderboards.RoundRequest{StartsAt: &later}, now)
	if err != nil {
		t.Fatal(err)
	}
	weekly := leaderboards.BoardRequest{TenantID: scope.TenantID, EnvironmentID: scope.EnvironmentID, Name: "Weekly", Schedule: &leaderboards.Schedule{Type: leaderboards.ScheduleRecurring, Every: "week"}}
	if _, err := leaderboards.UpdateBoard(ctx, db.pool, scope, board.ID, weekly, now); err != leaderboards.ErrScheduledRounds {
		t.Fatalf("switch with a scheduled round error = %v; want ErrScheduledRounds", err)
	}
	if _, err := leaderboards.CancelRound(ctx, db.pool, scope, board.ID, scheduled.ID, now); err != nil {
		t.Fatal(err)
	}
	// Scheduling ended the open round where the scheduled one would start;
	// leave it open again.
	reopened, err := leaderboards.UpdateRound(ctx, db.pool, scope, board.ID, open.ID, leaderboards.RoundUpdate{EndsAt: json.RawMessage(`null`)}, now)
	if err != nil || reopened.EndsAt != nil {
		t.Fatalf("reopen round = %+v, %v", reopened, err)
	}
	updated, err := leaderboards.UpdateBoard(ctx, db.pool, scope, board.ID, weekly, now)
	if err != nil {
		t.Fatal(err)
	}
	_, boundary, err := updated.Schedule.Window(now)
	if err != nil {
		t.Fatal(err)
	}
	if updated.CurrentRound == nil || updated.CurrentRound.ID != open.ID || updated.CurrentRound.EndsAt == nil || !updated.CurrentRound.EndsAt.Equal(boundary) {
		t.Fatalf("open round after switch = %+v; want it to end at %s", updated.CurrentRound, boundary)
	}
	next, err := leaderboards.LoadStandings(ctx, db.pool, updated, "current", 10, 0, boundary.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if next.Round == nil || next.Round.CreatedBy != "schedule" || !next.Round.StartsAt.Equal(boundary) {
		t.Fatalf("first scheduled round = %+v; want it to start at %s", next.Round, boundary)
	}
}

func TestLeaderboardPlanLimit(t *testing.T) {
	db := disposableDatabase(t)
	seedCurrency(t, db.pool)
	api := newLeaderboardAPI(t, db.pool)
	if _, err := db.pool.Exec(context.Background(), `UPDATE tenant_plans SET overrides = '{"max_leaderboards":1}'::jsonb WHERE tenant_id = 'integration-tenant'`); err != nil {
		t.Fatal(err)
	}
	body := `{"tenant_id":"integration-tenant","environment_id":"sandbox","key":"%s","name":"Board","metric":{"type":"xp"},"schedule":{"type":"manual"}}`
	if status := api.adminCall(http.MethodPost, "/v1/admin/leaderboards", fmt.Sprintf(body, "first"), nil); status != http.StatusCreated {
		t.Fatalf("first board returned %d", status)
	}
	if status := api.adminCall(http.MethodPost, "/v1/admin/leaderboards", fmt.Sprintf(body, "second"), nil); status != http.StatusConflict {
		t.Fatalf("second board returned %d; want 409 plan_limit_exceeded", status)
	}
}

// Concurrent awards that all need the same not-yet-created round must create
// it once and lose no points.
func TestLeaderboardConcurrentAwardsCreateOneRound(t *testing.T) {
	db := disposableDatabase(t)
	seedCurrency(t, db.pool)
	ctx := context.Background()
	scope := points.Scope{TenantID: "integration-tenant", EnvironmentID: "sandbox"}
	now := time.Now().UTC()
	for _, key := range []string{"daily", "weekly"} {
		insertBoard(t, db.pool, leaderboards.BoardRequest{TenantID: scope.TenantID, EnvironmentID: scope.EnvironmentID, Key: key, Name: key,
			Metric: &leaderboards.Metric{Type: leaderboards.MetricPoints, Currency: "points"}, Schedule: &leaderboards.Schedule{Type: leaderboards.ScheduleRecurring, Every: map[string]string{"daily": "day", "weekly": "week"}[key]}}, now)
	}
	ledger := points.NewLedger(db.pool)
	errs := make(chan error, 20)
	for index := range 20 {
		go func() {
			_, err := ledger.Award(ctx, scope, fmt.Sprintf("user-%d", index%4), "points", 1, fmt.Sprintf("award-%d", index), nil)
			errs <- err
		}()
	}
	for range 20 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent award: %v", err)
		}
	}
	var rounds, total int64
	if err := db.pool.QueryRow(ctx, `SELECT count(DISTINCT r.id), COALESCE(sum(s.score), 0) FROM leaderboard_rounds r LEFT JOIN leaderboard_scores s ON s.round_id = r.id`).Scan(&rounds, &total); err != nil {
		t.Fatal(err)
	}
	if rounds != 2 || total != 40 {
		t.Fatalf("rounds = %d, total score = %d; want one round per board and 40 points", rounds, total)
	}
}

// Deleting a board removes its rounds and scores and frees its plan slot;
// turning it off is the way to keep history.
func TestLeaderboardDelete(t *testing.T) {
	db := disposableDatabase(t)
	seedCurrency(t, db.pool)
	api := newLeaderboardAPI(t, db.pool)
	ctx := context.Background()
	if _, err := db.pool.Exec(ctx, `UPDATE tenant_plans SET overrides = '{"max_leaderboards":1}'::jsonb WHERE tenant_id = 'integration-tenant'`); err != nil {
		t.Fatal(err)
	}
	body := `{"tenant_id":"integration-tenant","environment_id":"sandbox","key":"weekly","name":"Weekly","metric":{"type":"points","currency":"points"},"schedule":{"type":"manual"}}`
	var board leaderboards.Leaderboard
	if status := api.adminCall(http.MethodPost, "/v1/admin/leaderboards", body, &board); status != http.StatusCreated {
		t.Fatalf("create board returned %d", status)
	}
	if status := api.adminCall(http.MethodPost, "/v1/admin/leaderboards/weekly/rounds", `{"tenant_id":"integration-tenant","environment_id":"sandbox"}`, nil); status != http.StatusCreated {
		t.Fatalf("start round by key returned %d", status)
	}
	api.award("user-a", 10, "a-1")

	scope := "?tenant_id=integration-tenant&environment_id=sandbox"
	if status := api.adminCall(http.MethodDelete, "/v1/admin/leaderboards/weekly?tenant_id=integration-tenant&environment_id=other", "", nil); status != http.StatusNotFound {
		t.Fatalf("delete in another environment returned %d; want 404", status)
	}
	var deleted struct {
		Deleted bool `json:"deleted"`
		Rounds  int  `json:"rounds_deleted"`
	}
	if status := api.adminCall(http.MethodDelete, "/v1/admin/leaderboards/weekly"+scope, "", &deleted); status != http.StatusOK || !deleted.Deleted || deleted.Rounds != 1 {
		t.Fatalf("delete returned %d: %+v", status, deleted)
	}
	var remaining int
	if err := db.pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM leaderboards) + (SELECT count(*) FROM leaderboard_rounds) + (SELECT count(*) FROM leaderboard_scores)`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("rows left after delete = %d (err %v); want 0", remaining, err)
	}
	if status := api.do(api.public, "X-Motif-API-Key", leaderboardAPIKey, http.MethodGet, "/v1/leaderboards/weekly", "", nil); status != http.StatusNotFound {
		t.Fatalf("reading a deleted board returned %d; want 404", status)
	}
	var action string
	if err := db.pool.QueryRow(ctx, `SELECT action FROM audit_log WHERE resource_id = $1 ORDER BY id DESC LIMIT 1`, board.ID).Scan(&action); err != nil || action != "leaderboard.deleted" {
		t.Fatalf("audit action = %q (err %v)", action, err)
	}
	// Awards keep working and the freed slot can be reused, even with the same key.
	api.award("user-a", 10, "a-2")
	if status := api.adminCall(http.MethodPost, "/v1/admin/leaderboards", body, nil); status != http.StatusCreated {
		t.Fatalf("recreating within the plan limit returned %d", status)
	}
}
