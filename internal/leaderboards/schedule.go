package leaderboards

import (
	"fmt"
	"strings"
	"time"

	// Embed the zone database so recurring boards work in minimal images
	// that ship without /usr/share/zoneinfo.
	_ "time/tzdata"
)

const (
	ScheduleManual    = "manual"
	ScheduleRecurring = "recurring"
)

// Schedule decides who creates a board's rounds. Manual boards get rounds
// from an operator. Recurring boards repeat every Interval units of Every,
// counted in Timezone from Anchor, so weekly rounds that start on Saturday
// use an anchor that falls on a Saturday.
type Schedule struct {
	Type     string     `json:"type"`
	Every    string     `json:"every,omitempty"`
	Interval int        `json:"interval,omitempty"`
	Anchor   *time.Time `json:"anchor,omitempty"`
	Timezone string     `json:"timezone,omitempty"`
	// WeekStart only picks the default anchor for weekly boards created
	// without one. It is not stored.
	WeekStart string `json:"week_start,omitempty"`
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday, "wednesday": time.Wednesday,
	"thursday": time.Thursday, "friday": time.Friday, "saturday": time.Saturday,
}

// normalizeSchedule validates a schedule and fills its defaults. now picks
// the default anchor: the current day, week, or month start in the timezone.
func normalizeSchedule(schedule Schedule, now time.Time) (Schedule, error) {
	schedule.Type = strings.TrimSpace(schedule.Type)
	schedule.Timezone = strings.TrimSpace(schedule.Timezone)
	if schedule.Timezone == "" {
		schedule.Timezone = "UTC"
	}
	location, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return Schedule{}, fmt.Errorf("schedule.timezone must be an IANA time zone such as Asia/Tehran")
	}
	switch schedule.Type {
	case ScheduleManual:
		return Schedule{Type: ScheduleManual, Timezone: schedule.Timezone}, nil
	case ScheduleRecurring:
	default:
		return Schedule{}, fmt.Errorf("schedule.type must be manual or recurring")
	}
	schedule.Every = strings.TrimSpace(schedule.Every)
	if schedule.Every != "day" && schedule.Every != "week" && schedule.Every != "month" {
		return Schedule{}, fmt.Errorf("schedule.every must be day, week, or month")
	}
	if schedule.Interval == 0 {
		schedule.Interval = 1
	}
	if schedule.Interval < 1 || schedule.Interval > 366 {
		return Schedule{}, fmt.Errorf("schedule.interval must be between 1 and 366")
	}
	if schedule.Anchor == nil {
		local := now.In(location)
		anchor := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
		switch schedule.Every {
		case "week":
			start := time.Monday
			if schedule.WeekStart != "" {
				day, ok := weekdays[strings.ToLower(strings.TrimSpace(schedule.WeekStart))]
				if !ok {
					return Schedule{}, fmt.Errorf("schedule.week_start must be a weekday name")
				}
				start = day
			}
			anchor = anchor.AddDate(0, 0, -((int(anchor.Weekday()) - int(start) + 7) % 7))
		case "month":
			anchor = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, location)
		}
		schedule.Anchor = &anchor
	}
	anchor := schedule.Anchor.UTC().Truncate(time.Microsecond)
	schedule.Anchor = &anchor
	if schedule.Every == "month" && anchor.In(location).Day() > 28 {
		return Schedule{}, fmt.Errorf("schedule.anchor must fall on day 1-28 for monthly boards")
	}
	schedule.WeekStart = ""
	return schedule, nil
}

// Window returns the recurring period [start, end) that contains at.
// Periods are counted on the wall clock of the schedule's timezone, so a
// daily board keeps starting at midnight across daylight-saving changes.
func (s Schedule) Window(at time.Time) (time.Time, time.Time, error) {
	if s.Type != ScheduleRecurring || s.Anchor == nil || s.Interval < 1 {
		return time.Time{}, time.Time{}, fmt.Errorf("schedule is not recurring")
	}
	location, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	anchor := s.Anchor.In(location)
	local := at.In(location)
	start := func(k int) time.Time {
		switch s.Every {
		case "day":
			return anchor.AddDate(0, 0, k*s.Interval)
		case "week":
			return anchor.AddDate(0, 0, 7*k*s.Interval)
		default:
			return anchor.AddDate(0, k*s.Interval, 0)
		}
	}
	var k int
	switch s.Every {
	case "day":
		k = floorDiv(int(local.Sub(anchor).Hours()/24), s.Interval)
	case "week":
		k = floorDiv(int(local.Sub(anchor).Hours()/24), 7*s.Interval)
	case "month":
		k = floorDiv((local.Year()-anchor.Year())*12+int(local.Month())-int(anchor.Month()), s.Interval)
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("unsupported schedule unit %q", s.Every)
	}
	// The estimate can be one period off around daylight-saving changes and
	// month ends; step until the window contains at.
	for start(k).After(at) {
		k--
	}
	for !start(k + 1).After(at) {
		k++
	}
	return start(k).UTC(), start(k + 1).UTC(), nil
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}
