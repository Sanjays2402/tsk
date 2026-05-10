package store

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseDuration parses a human-friendly duration string used by tsk's
// archive/purge commands.
//
// Accepted suffixes (integer prefix required):
//
//   - Nd: days (e.g. "7d", "30d", "90d")
//   - Nw: weeks (e.g. "2w", "12w") — N * 7 days
//   - Nm: months (e.g. "1m", "3m") — approximated as N * 30 days
//   - Ny: years (e.g. "1y", "2y") — approximated as N * 365 days
//
// As a fallback, a Go duration string containing more than one unit
// (e.g. "24h", "1h30m") is accepted via time.ParseDuration. Note that a
// bare "Nm" always means months in this helper, not minutes — sub-hour
// granularity is not useful for archive/purge windows. Use "24h" for hours.
//
// Note: month and year approximations are intentionally fixed (30 / 365
// days) — they're meant for "is this older than ~3 months" checks, not
// calendar arithmetic.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if d, ok, err := parseUnitDuration(s); ok || err != nil {
		return d, err
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (try 7d, 2w, 1m, 1y, or 24h)", s)
	}
	if d < 0 {
		return 0, fmt.Errorf("invalid duration %q (negative)", s)
	}
	return d, nil
}

// parseUnitDuration handles the d/w/m/y suffix forms. Returns (d, true, nil) on
// a clean match, (0, false, nil) when the input doesn't carry a recognized
// suffix (caller should try the Go duration fallback), or (0, _, err) on an
// outright invalid suffix-form input.
func parseUnitDuration(s string) (time.Duration, bool, error) {
	last := s[len(s)-1]
	prefix := s[:len(s)-1]
	switch last {
	case 'd', 'D':
		d, err := scaleUnit(prefix, 24*time.Hour, "days", s)
		return d, true, err
	case 'w', 'W':
		d, err := scaleUnit(prefix, 7*24*time.Hour, "weeks", s)
		return d, true, err
	case 'y', 'Y':
		d, err := scaleUnit(prefix, 365*24*time.Hour, "years", s)
		return d, true, err
	case 'm', 'M':
		// Months only when the prefix is a bare integer. Otherwise fall
		// through to the Go duration fallback (e.g. "90m", "1h30m").
		if n, err := strconv.Atoi(prefix); err == nil && n >= 0 {
			return time.Duration(n) * 30 * 24 * time.Hour, true, nil
		}
	}
	return 0, false, nil
}

func scaleUnit(prefix string, unit time.Duration, label, raw string) (time.Duration, error) {
	n, err := strconv.Atoi(prefix)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid %s duration %q", label, raw)
	}
	return time.Duration(n) * unit, nil
}
