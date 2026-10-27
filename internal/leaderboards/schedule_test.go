package leaderboards

import (
	"errors"
	"testing"
	"time"
)

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func recurring(t *testing.T, every string, interval int, anchor, timezone string) Schedule {
	t.Helper()
	at := mustTime(t, anchor)
	schedule, err := normalizeSchedule(Schedule{Type: ScheduleRecurring, Every: every, Interval: interval, Anchor: &at, Timezone: timezone}, at)
	if err != nil {
		t.Fatal(err)
	}
	return schedule
}

func TestScheduleWindow(t *testing.T) {
	tests := []struct {
		name               string
		schedule           Schedule
		at                 string
		wantStart, wantEnd string
	}{
		{
			name:     "weekly from Saturday in Tehran",
			schedule: recurring(t, "week", 1, "2026-10-24T00:00:00+03:30", "Asia/Tehran"),
			at:       "2026-10-27T12:00:00Z",
			// Saturday 00:00 Tehran to the next Saturday.
			wantStart: "2026-10-23T20:30:00Z", wantEnd: "2026-10-30T20:30:00Z",
		},
		{
			name:      "exact boundary belongs to the new window",
			schedule:  recurring(t, "week", 1, "2026-10-24T00:00:00+03:30", "Asia/Tehran"),
			at:        "2026-10-30T20:30:00Z",
			wantStart: "2026-10-30T20:30:00Z", wantEnd: "2026-11-06T20:30:00Z",
		},
		{
			name:      "before the anchor counts backwards",
			schedule:  recurring(t, "week", 1, "2026-10-24T00:00:00+03:30", "Asia/Tehran"),
			at:        "2026-10-20T00:00:00Z",
			wantStart: "2026-10-16T20:30:00Z", wantEnd: "2026-10-23T20:30:00Z",
		},
		{
			name:      "every ten days",
			schedule:  recurring(t, "day", 10, "2026-11-01T00:00:00Z", "UTC"),
			at:        "2026-11-25T08:00:00Z",
			wantStart: "2026-11-21T00:00:00Z", wantEnd: "2026-12-01T00:00:00Z",
		},
		{
			name:      "monthly",
			schedule:  recurring(t, "month", 1, "2026-01-01T00:00:00Z", "UTC"),
			at:        "2026-10-31T23:59:59Z",
			wantStart: "2026-10-01T00:00:00Z", wantEnd: "2026-11-01T00:00:00Z",
		},
		{
			name:      "quarterly across a year",
			schedule:  recurring(t, "month", 3, "2026-01-15T00:00:00Z", "UTC"),
			at:        "2027-02-01T00:00:00Z",
			wantStart: "2027-01-15T00:00:00Z", wantEnd: "2027-04-15T00:00:00Z",
		},
		{
			name:     "daily keeps local midnight across a daylight-saving change",
			schedule: recurring(t, "day", 1, "2026-03-01T00:00:00-05:00", "America/New_York"),
			at:       "2026-03-08T12:00:00-04:00",
			// The DST day is 23 hours long; the next day starts at local midnight.
			wantStart: "2026-03-08T05:00:00Z", wantEnd: "2026-03-09T04:00:00Z",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, end, err := test.schedule.Window(mustTime(t, test.at))
			if err != nil {
				t.Fatal(err)
			}
			if !start.Equal(mustTime(t, test.wantStart)) || !end.Equal(mustTime(t, test.wantEnd)) {
				t.Fatalf("window = [%s, %s); want [%s, %s)", start.Format(time.RFC3339), end.Format(time.RFC3339), test.wantStart, test.wantEnd)
			}
		})
	}
}

func TestDefaultAnchor(t *testing.T) {
	now := mustTime(t, "2026-10-27T12:00:00Z") // Tuesday, 15:30 in Tehran.
	tests := []struct {
		name     string
		schedule Schedule
		want     string
	}{
		{"week defaults to Monday", Schedule{Type: ScheduleRecurring, Every: "week", Timezone: "Asia/Tehran"}, "2026-10-26T00:00:00+03:30"},
		{"week from Saturday", Schedule{Type: ScheduleRecurring, Every: "week", Timezone: "Asia/Tehran", WeekStart: "saturday"}, "2026-10-24T00:00:00+03:30"},
		{"day", Schedule{Type: ScheduleRecurring, Every: "day", Timezone: "Asia/Tehran"}, "2026-10-27T00:00:00+03:30"},
		{"month", Schedule{Type: ScheduleRecurring, Every: "month"}, "2026-10-01T00:00:00Z"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			schedule, err := normalizeSchedule(test.schedule, now)
			if err != nil {
				t.Fatal(err)
			}
			if !schedule.Anchor.Equal(mustTime(t, test.want)) {
				t.Fatalf("anchor = %s; want %s", schedule.Anchor, test.want)
			}
			if schedule.Interval != 1 || schedule.WeekStart != "" {
				t.Fatalf("schedule = %+v; want interval 1 and no stored week_start", schedule)
			}
		})
	}
}

func TestScheduleValidation(t *testing.T) {
	now := mustTime(t, "2026-10-27T12:00:00Z")
	late := mustTime(t, "2026-01-31T00:00:00Z")
	for name, schedule := range map[string]Schedule{
		"unknown type":      {Type: "weekly"},
		"unknown unit":      {Type: ScheduleRecurring, Every: "year"},
		"bad timezone":      {Type: ScheduleRecurring, Every: "day", Timezone: "Mars/Olympus"},
		"interval too long": {Type: ScheduleRecurring, Every: "day", Interval: 400},
		"bad week start":    {Type: ScheduleRecurring, Every: "week", WeekStart: "someday"},
		"month day 31":      {Type: ScheduleRecurring, Every: "month", Anchor: &late},
	} {
		if _, err := normalizeSchedule(schedule, now); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	manual, err := normalizeSchedule(Schedule{Type: ScheduleManual, Every: "week", Interval: 2}, now)
	if err != nil {
		t.Fatal(err)
	}
	if manual.Every != "" || manual.Interval != 0 || manual.Anchor != nil || manual.Timezone != "UTC" {
		t.Fatalf("manual schedule kept recurring fields: %+v", manual)
	}
}

func TestValidateBoardRequest(t *testing.T) {
	now := mustTime(t, "2026-10-27T12:00:00Z")
	valid := BoardRequest{TenantID: "acme", EnvironmentID: "prod", Key: "weekly", Name: "Weekly", Metric: &Metric{Type: MetricPoints, Currency: "coins"}, Schedule: &Schedule{Type: ScheduleManual}}
	if _, err := ValidateBoardRequest(valid, now); err != nil {
		t.Fatalf("valid request: %v", err)
	}
	cases := map[string]func(*BoardRequest){
		"missing key":         func(r *BoardRequest) { r.Key = "" },
		"uppercase key":       func(r *BoardRequest) { r.Key = "Weekly" },
		"missing metric":      func(r *BoardRequest) { r.Metric = nil },
		"points without coin": func(r *BoardRequest) { r.Metric = &Metric{Type: MetricPoints} },
		"xp with currency":    func(r *BoardRequest) { r.Metric = &Metric{Type: MetricXP, Currency: "coins"} },
		"missing schedule":    func(r *BoardRequest) { r.Schedule = nil },
		"missing environment": func(r *BoardRequest) { r.EnvironmentID = "" },
	}
	for name, mutate := range cases {
		request := valid
		mutate(&request)
		_, err := ValidateBoardRequest(request, now)
		var validation *ValidationError
		if !errors.As(err, &validation) {
			t.Errorf("%s: error = %v; want a validation error", name, err)
		}
	}
}

func TestValidateBoardUpdateKeepsMetric(t *testing.T) {
	now := mustTime(t, "2026-10-27T12:00:00Z")
	current := Leaderboard{Key: "weekly", Metric: Metric{Type: MetricPoints, Currency: "coins"}, Schedule: Schedule{Type: ScheduleManual, Timezone: "UTC"}}
	update := BoardRequest{TenantID: "acme", EnvironmentID: "prod", Name: "Weekly coins"}
	validated, err := validateBoardUpdate(update, current, now)
	if err != nil {
		t.Fatal(err)
	}
	if validated.Metric.Currency != "coins" || validated.Schedule.Type != ScheduleManual {
		t.Fatalf("update did not keep the current metric and schedule: %+v", validated)
	}
	update.Metric = &Metric{Type: MetricXP}
	if _, err := validateBoardUpdate(update, current, now); !errors.Is(err, ErrMetricUnchangable) {
		t.Fatalf("metric change error = %v; want ErrMetricUnchangable", err)
	}
	update.Metric, update.Key = nil, "monthly"
	if _, err := validateBoardUpdate(update, current, now); err == nil {
		t.Fatal("key change was accepted")
	}
}

func TestParsePath(t *testing.T) {
	for path, want := range map[string][2]string{
		"/v1/leaderboards":               {"", ""},
		"/v1/leaderboards/weekly":        {"weekly", ""},
		"/v1/leaderboards/weekly/":       {"weekly", ""},
		"/v1/leaderboards/weekly/rounds": {"weekly", "rounds"},
	} {
		key, resource, ok := parsePath(path)
		if !ok || key != want[0] || resource != want[1] {
			t.Errorf("parsePath(%q) = %q, %q, %v; want %q, %q", path, key, resource, ok, want[0], want[1])
		}
	}
	for _, path := range []string{"/v1/leaderboards/weekly/standings", "/v1/leaderboards/weekly/rounds/1", "/v1/leaderboardsx"} {
		if _, _, ok := parsePath(path); ok {
			t.Errorf("parsePath(%q) accepted an unknown route", path)
		}
	}
}
