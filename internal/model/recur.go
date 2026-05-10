package model

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RecurKind identifies a recurrence rule's family.
type RecurKind int

// Supported recurrence kinds.
const (
	// RecurDaily fires every day.
	RecurDaily RecurKind = iota + 1
	// RecurWeekly fires every week on the same weekday.
	RecurWeekly
	// RecurMonthly fires every month on the same day-of-month, with
	// month-end clamping (Jan 31 + 1 month -> Feb 28/29).
	RecurMonthly
	// RecurYearly fires every year on the same month/day, with leap-day
	// clamping (Feb 29 + 1 year -> Feb 28 in non-leap years).
	RecurYearly
	// RecurWeekdays fires every Mon-Fri (skipping Sat/Sun).
	RecurWeekdays
	// RecurEvery fires every N units (Every.N + Every.Unit).
	RecurEvery
)

// RecurUnit is the unit field of an `every:Nu` rule.
type RecurUnit byte

// Supported every-units.
const (
	UnitDay   RecurUnit = 'd'
	UnitWeek  RecurUnit = 'w'
	UnitMonth RecurUnit = 'm'
	UnitYear  RecurUnit = 'y'
)

// EveryRule captures the N + unit pair for `every:Nu` recurrences.
type EveryRule struct {
	N    int
	Unit RecurUnit
}

// Recurrence describes how a task repeats. The zero value is invalid;
// callers should always go through ParseRecurrence.
type Recurrence struct {
	Kind  RecurKind
	Every EveryRule // populated only when Kind == RecurEvery
}

// String returns the canonical storage form (e.g. "daily", "weekdays",
// "every:3d"). The output is stable and round-trips through ParseRecurrence.
func (r Recurrence) String() string {
	switch r.Kind {
	case RecurDaily:
		return "daily"
	case RecurWeekly:
		return "weekly"
	case RecurMonthly:
		return "monthly"
	case RecurYearly:
		return "yearly"
	case RecurWeekdays:
		return "weekdays"
	case RecurEvery:
		return fmt.Sprintf("every:%d%c", r.Every.N, r.Every.Unit)
	}
	return ""
}

// ParseRecurrence resolves a recurrence string to a Recurrence value. It is
// strict: unknown keywords or malformed `every:` rules return an error.
func ParseRecurrence(s string) (Recurrence, error) {
	raw := strings.ToLower(strings.TrimSpace(s))
	if raw == "" {
		return Recurrence{}, fmt.Errorf("recurrence required")
	}
	switch raw {
	case "daily":
		return Recurrence{Kind: RecurDaily}, nil
	case "weekly":
		return Recurrence{Kind: RecurWeekly}, nil
	case "monthly":
		return Recurrence{Kind: RecurMonthly}, nil
	case "yearly", "annually":
		return Recurrence{Kind: RecurYearly}, nil
	case "weekdays":
		return Recurrence{Kind: RecurWeekdays}, nil
	}
	if !strings.HasPrefix(raw, "every:") {
		return Recurrence{}, fmt.Errorf("unknown recurrence %q", s)
	}
	rest := raw[len("every:"):]
	if rest == "" {
		return Recurrence{}, fmt.Errorf("invalid recurrence %q: missing N<unit>", s)
	}
	unit := rest[len(rest)-1]
	switch RecurUnit(unit) {
	case UnitDay, UnitWeek, UnitMonth, UnitYear:
	default:
		return Recurrence{}, fmt.Errorf("invalid recurrence %q: unit must be d/w/m/y", s)
	}
	nStr := rest[:len(rest)-1]
	n, err := strconv.Atoi(nStr)
	if err != nil || n <= 0 {
		return Recurrence{}, fmt.Errorf("invalid recurrence %q: N must be a positive integer", s)
	}
	return Recurrence{Kind: RecurEvery, Every: EveryRule{N: n, Unit: RecurUnit(unit)}}, nil
}

// Next returns the next due date strictly after `from`. The returned time is
// at midnight in `from`'s location. Next always advances at least one full
// step from `from`; calling Next with a Wednesday and a weekly rule returns
// the following Wednesday, never the same day.
//
// Behavior notes:
//   - Monthly rules clamp to the last day of the target month when the
//     anchor day-of-month doesn't exist (e.g. Jan 31 + 1mo -> Feb 28/29).
//   - Yearly rules clamp Feb 29 to Feb 28 in non-leap years.
//   - Weekdays skip Saturday and Sunday: Friday + weekdays -> Monday.
//   - For an unknown/zero Recurrence, Next returns `from` unchanged.
func (r Recurrence) Next(from time.Time) time.Time {
	loc := from.Location()
	day := startOfDay(from.In(loc))
	switch r.Kind {
	case RecurDaily:
		return day.AddDate(0, 0, 1)
	case RecurWeekly:
		return day.AddDate(0, 0, 7)
	case RecurMonthly:
		return addMonthsClamp(day, 1)
	case RecurYearly:
		return addYearsClamp(day, 1)
	case RecurWeekdays:
		next := day.AddDate(0, 0, 1)
		for {
			wd := next.Weekday()
			if wd != time.Saturday && wd != time.Sunday {
				return next
			}
			next = next.AddDate(0, 0, 1)
		}
	case RecurEvery:
		n := r.Every.N
		if n <= 0 {
			n = 1
		}
		switch r.Every.Unit {
		case UnitDay:
			return day.AddDate(0, 0, n)
		case UnitWeek:
			return day.AddDate(0, 0, 7*n)
		case UnitMonth:
			return addMonthsClamp(day, n)
		case UnitYear:
			return addYearsClamp(day, n)
		}
	}
	return from
}

// addMonthsClamp adds n months to t and clamps overflow days to the last day
// of the resulting month (e.g. Jan 31 + 1 -> Feb 28/29). t is assumed to be
// at start-of-day in its location.
func addMonthsClamp(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	// time.Date normalizes month overflow for us, so just compute the target
	// year/month and clamp the day to the resulting month's last day.
	total := int(m) + n - 1
	yAdj := y + total/12
	mAdj := time.Month(total%12) + 1
	if d > daysInMonth(yAdj, mAdj) {
		d = daysInMonth(yAdj, mAdj)
	}
	return time.Date(yAdj, mAdj, d, 0, 0, 0, 0, t.Location())
}

// addYearsClamp adds n years to t, clamping Feb 29 to Feb 28 when the target
// year isn't a leap year.
func addYearsClamp(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	loc := t.Location()
	yAdj := y + n
	if d > daysInMonth(yAdj, m) {
		d = daysInMonth(yAdj, m)
	}
	return time.Date(yAdj, m, d, 0, 0, 0, 0, loc)
}

func daysInMonth(year int, m time.Month) int {
	// Day 0 of (year, m+1) == last day of (year, m).
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
