package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var dueWeekdays = map[string]time.Weekday{
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
	"sun": time.Sunday, "sunday": time.Sunday,
}

// parseNaturalDue accepts YYYY-MM-DD, today, tomorrow, +Nd, +Nw, and weekday
// names (optionally prefixed with "next"). Weekdays mean the next occurrence,
// strictly after today. An empty input clears the due date.
func parseNaturalDue(value string, now time.Time) (*time.Time, error) {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return nil, nil
	}
	if parsed, err := time.Parse("2006-01-02", text); err == nil {
		due := parsed.UTC()
		return &due, nil
	}
	local := now.In(time.Local)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
	switch {
	case text == "today":
	case text == "tomorrow":
		day = day.AddDate(0, 0, 1)
	case strings.HasPrefix(text, "+") && (strings.HasSuffix(text, "d") || strings.HasSuffix(text, "w")):
		amount, err := strconv.Atoi(text[1 : len(text)-1])
		if err != nil || amount < 0 {
			return nil, fmt.Errorf("due date offset must be a positive number of days or weeks")
		}
		if strings.HasSuffix(text, "w") {
			amount *= 7
		}
		day = day.AddDate(0, 0, amount)
	default:
		weekday, ok := dueWeekdays[strings.TrimPrefix(text, "next ")]
		if !ok {
			return nil, fmt.Errorf("due date must be YYYY-MM-DD, today, tomorrow, +Nd, +Nw, or a weekday")
		}
		delta := (int(weekday) - int(day.Weekday()) + 7) % 7
		if delta == 0 {
			delta = 7
		}
		day = day.AddDate(0, 0, delta)
	}
	due := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	return &due, nil
}
