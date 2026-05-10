package store

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"7d", 7 * day},
		{"30d", 30 * day},
		{"90D", 90 * day},
		{"2w", 14 * day},
		{"12w", 12 * 7 * day},
		{"1m", 30 * day},
		{"3m", 90 * day},
		{"1y", 365 * day},
		{"2Y", 2 * 365 * day},
		{"24h", 24 * time.Hour},
		{"1h30m", 90 * time.Minute},
		{"0d", 0},
	}
	for _, tc := range cases {
		got, err := ParseDuration(tc.in)
		if err != nil {
			t.Errorf("ParseDuration(%q) err: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseDuration(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestParseDurationErrors(t *testing.T) {
	bad := []string{
		"",
		"   ",
		"d",
		"abc",
		"7days",
		"-3d",
		"3.5w",
		"5z",
	}
	for _, in := range bad {
		if _, err := ParseDuration(in); err == nil {
			t.Errorf("ParseDuration(%q) expected error, got nil", in)
		}
	}
}

// TestParseDurationMonthVsMinute documents the disambiguation rule: a bare
// integer + "m" means months; a Go-duration-shaped string with "m" (e.g.
// "90m" or "1h30m") still works through the fallback.
func TestParseDurationMonthVsMinute(t *testing.T) {
	got, err := ParseDuration("1m")
	if err != nil {
		t.Fatal(err)
	}
	if got != 30*24*time.Hour {
		t.Errorf("1m should be 30 days, got %s", got)
	}
	got, err = ParseDuration("90m")
	if err != nil {
		t.Fatal(err)
	}
	// "90" is a valid integer prefix → treated as 90 months. Document this.
	if got != 90*30*24*time.Hour {
		t.Errorf("90m treated as months: got %s", got)
	}
	got, err = ParseDuration("1h30m")
	if err != nil {
		t.Fatal(err)
	}
	if got != 90*time.Minute {
		t.Errorf("1h30m should fall through to time.ParseDuration: got %s", got)
	}
}
