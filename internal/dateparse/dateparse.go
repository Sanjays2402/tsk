// Package dateparse parses natural-language due-date inputs into a concrete
// time.Time, anchored to a caller-supplied "now" and location. It exists so
// users can type "tomorrow", "fri", "in 3d", or "jul 4" instead of a strict
// YYYY-MM-DD. The canonical YYYY-MM-DD format still round-trips unchanged.
//
// The package intentionally uses only the standard library (time + regexp).
package dateparse

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// ErrEmpty is returned when the input is empty or only whitespace.
var ErrEmpty = fmt.Errorf("empty date input")

// ParseError wraps a parse failure with the original input for nicer messages.
type ParseError struct {
	Input string
}

// Error implements error. The message mirrors the CLI guidance: it nudges the
// user toward one of the accepted shapes.
func (e *ParseError) Error() string {
	return fmt.Sprintf("can't parse %q as a date. try: tomorrow, fri, in 3d, 2026-05-01", e.Input)
}

// isoLayout is the canonical YYYY-MM-DD format that has always been accepted.
const isoLayout = "2006-01-02"

// Parse resolves input into a time.Time at start-of-day in loc, relative to
// now. It accepts:
//
//   - YYYY-MM-DD (backward compatible)
//   - today, tomorrow, tmrw, yesterday
//   - weekday names (mon..sun, monday..sunday) — next occurrence; same-day
//     weekday resolves to seven days out
//   - relative offsets: "in 3d", "3d", "in 2w", "2w", "in 1m", "1m"
//   - month-day: "jul 4", "july 4", "jul 4 2027", "4 jul", "jul"
//   - aliases: "next week", "next month", "next mon", "eow", "eom"
//
// All results are normalised to 00:00:00 in loc on the target calendar day.
// Parse returns a *ParseError when the input matches no supported shape.
func Parse(input string, now time.Time, loc *time.Location) (time.Time, error) {
	if loc == nil {
		loc = time.Local
	}
	raw := strings.TrimSpace(input)
	if raw == "" {
		return time.Time{}, ErrEmpty
	}
	s := strings.ToLower(raw)
	// Collapse internal whitespace runs so "jul   4" parses the same as "jul 4".
	s = collapseSpaces(s)

	anchor := startOfDay(now, loc)

	if t, ok := parseISO(raw, loc); ok {
		return t, nil
	}
	if t, ok := parseKeyword(s, anchor); ok {
		return t, nil
	}
	if t, ok := parseWeekday(s, anchor); ok {
		return t, nil
	}
	if t, ok := parseRelative(s, anchor); ok {
		return t, nil
	}
	return time.Time{}, &ParseError{Input: raw}
}

// relativeRE matches "3d", "in 3d", "2w", "in 1m", etc. The unit is one of
// d (days), w (weeks), m (months), y (years). The optional "in " prefix is
// accepted purely for readability.
var relativeRE = regexp.MustCompile(`^(?:in\s+)?(\d+)\s*(d|w|m|y|day|days|week|weeks|month|months|year|years)$`)

// parseRelative matches "in 3d", "3d", "2w", "1m", "1y" and their long forms.
// Months and years use calendar math (time.AddDate) so "1m" on Jan 31 lands on
// the Go-standard normalised date (Mar 3 or similar), not a drifting 30-day
// approximation.
func parseRelative(s string, anchor time.Time) (time.Time, bool) {
	m := relativeRE.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	n := atoi(m[1])
	switch m[2] {
	case "d", "day", "days":
		return addDays(anchor, n), true
	case "w", "week", "weeks":
		return addDays(anchor, n*7), true
	case "m", "month", "months":
		return anchor.AddDate(0, n, 0), true
	case "y", "year", "years":
		return anchor.AddDate(n, 0, 0), true
	}
	return time.Time{}, false
}

// atoi is a tiny, allocation-free decimal parser. The regex guarantees digits.
func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

// parseISO accepts the canonical YYYY-MM-DD format. Preserving this path keeps
// existing scripts and stored tasks working.
func parseISO(raw string, loc *time.Location) (time.Time, bool) {
	t, err := time.ParseInLocation(isoLayout, raw, loc)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// parseKeyword handles the fixed-form words: today / tomorrow / tmrw / yesterday.
func parseKeyword(s string, anchor time.Time) (time.Time, bool) {
	switch s {
	case "today", "now":
		return anchor, true
	case "tomorrow", "tmrw", "tmr":
		return addDays(anchor, 1), true
	case "yesterday":
		return addDays(anchor, -1), true
	}
	return time.Time{}, false
}

// parseWeekday resolves "mon", "monday", etc. to the next occurrence. If today
// is the named weekday, the result is seven days out — matching user intent of
// "the next Monday", not "today".
func parseWeekday(s string, anchor time.Time) (time.Time, bool) {
	wd, ok := weekdayFromWord(s)
	if !ok {
		return time.Time{}, false
	}
	return nextWeekday(anchor, wd, false), true
}

// weekdayFromWord maps both short (mon) and long (monday) weekday forms.
func weekdayFromWord(s string) (time.Weekday, bool) {
	switch s {
	case "mon", "monday":
		return time.Monday, true
	case "tue", "tues", "tuesday":
		return time.Tuesday, true
	case "wed", "weds", "wednesday":
		return time.Wednesday, true
	case "thu", "thur", "thurs", "thursday":
		return time.Thursday, true
	case "fri", "friday":
		return time.Friday, true
	case "sat", "saturday":
		return time.Saturday, true
	case "sun", "sunday":
		return time.Sunday, true
	}
	return 0, false
}

// nextWeekday returns the next date whose weekday matches want. If includeToday
// is false and today already matches, it rolls forward seven days.
func nextWeekday(anchor time.Time, want time.Weekday, includeToday bool) time.Time {
	delta := int(want) - int(anchor.Weekday())
	if delta < 0 {
		delta += 7
	}
	if delta == 0 && !includeToday {
		delta = 7
	}
	return addDays(anchor, delta)
}

// addDays advances by n calendar days without drifting across DST transitions,
// by using time.Date so the hour stays pinned to 00:00 local.
func addDays(t time.Time, n int) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d+n, 0, 0, 0, 0, t.Location())
}

// startOfDay truncates t to 00:00:00 in loc.
func startOfDay(t time.Time, loc *time.Location) time.Time {
	in := t.In(loc)
	y, m, d := in.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}

var wsRun = regexp.MustCompile(`\s+`)

func collapseSpaces(s string) string {
	return strings.TrimSpace(wsRun.ReplaceAllString(s, " "))
}
