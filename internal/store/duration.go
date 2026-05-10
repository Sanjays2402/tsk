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
	last := s[len(s)-1]
	prefix := s[:len(s)-1]
	switch last {
	case 'd', 'D':
		n, err := strconv.Atoi(prefix)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid days duration %q", s)
		}
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w', 'W':
		n, err := strconv.Atoi(prefix)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid weeks duration %q", s)
		}
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case 'm', 'M':
		// Months only when the prefix is a bare integer. Otherwise fall
		// through to the Go duration fallback (e.g. "90m", "1h30m").
		if n, err := strconv.Atoi(prefix); err == nil && n >= 0 {
			return time.Duration(n) * 30 * 24 * time.Hour, nil
		}
	case 'y', 'Y':
		n, err := strconv.Atoi(prefix)
		if err != nil || n < 0 {
			return 0, fmt.Errorf("invalid years duration %q", s)
		}
		return time.Duration(n) * 365 * 24 * time.Hour, nil
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
