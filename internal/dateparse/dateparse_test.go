package dateparse

import (
	"errors"
	"testing"
	"time"
)

// fixedNow is a Tuesday so weekday math is deterministic.
// 2026-04-21 19:00 -07:00 (PDT, America/Los_Angeles).
func fixedNow(t *testing.T) (time.Time, *time.Location) {
	t.Helper()
	loc, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatalf("load LA: %v", err)
	}
	return time.Date(2026, time.April, 21, 19, 0, 0, 0, loc), loc
}

func ymd(t *testing.T, year int, month time.Month, day int) time.Time {
	t.Helper()
	_, loc := fixedNow(t)
	return time.Date(year, month, day, 0, 0, 0, 0, loc)
}

func TestParseKeywordsAndWeekdays(t *testing.T) {
	now, loc := fixedNow(t)
	type want struct {
		t    time.Time
		fail bool
	}
	cases := []struct {
		in   string
		want want
	}{
		{"today", want{t: ymd(t, 2026, time.April, 21)}},
		{"  TODAY ", want{t: ymd(t, 2026, time.April, 21)}},
		{"tomorrow", want{t: ymd(t, 2026, time.April, 22)}},
		{"tmrw", want{t: ymd(t, 2026, time.April, 22)}},
		{"yesterday", want{t: ymd(t, 2026, time.April, 20)}},
		// Tuesday anchor: tue -> 7 days out, wed -> +1, mon -> +6.
		{"tue", want{t: ymd(t, 2026, time.April, 28)}},
		{"tuesday", want{t: ymd(t, 2026, time.April, 28)}},
		{"wed", want{t: ymd(t, 2026, time.April, 22)}},
		{"thu", want{t: ymd(t, 2026, time.April, 23)}},
		{"fri", want{t: ymd(t, 2026, time.April, 24)}},
		{"sat", want{t: ymd(t, 2026, time.April, 25)}},
		{"sun", want{t: ymd(t, 2026, time.April, 26)}},
		{"mon", want{t: ymd(t, 2026, time.April, 27)}},
		{"monday", want{t: ymd(t, 2026, time.April, 27)}},
		// ISO still works.
		{"2026-05-01", want{t: ymd(t, 2026, time.May, 1)}},
		{"2026-12-31", want{t: ymd(t, 2026, time.December, 31)}},
		// Nonsense.
		{"", want{fail: true}},
		{"   ", want{fail: true}},
		{"sometime", want{fail: true}},
		{"bloop", want{fail: true}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got, err := Parse(tc.in, now, loc)
			if tc.want.fail {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tc.want.t) {
				t.Fatalf("got %s want %s", got, tc.want.t)
			}
		})
	}
}

func TestParseErrorMessage(t *testing.T) {
	now, loc := fixedNow(t)
	_, err := Parse("sometime", now, loc)
	if err == nil {
		t.Fatal("expected error")
	}
	var pe *ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("expected *ParseError, got %T", err)
	}
	msg := pe.Error()
	if msg == "" || !containsAll(msg, "sometime", "tomorrow", "2026-05-01") {
		t.Fatalf("unhelpful error: %q", msg)
	}
}

func TestParseEmptyIsSentinel(t *testing.T) {
	now, loc := fixedNow(t)
	_, err := Parse("  ", now, loc)
	if !errors.Is(err, ErrEmpty) {
		t.Fatalf("want ErrEmpty, got %v", err)
	}
}

func TestParseConsistentWithISO(t *testing.T) {
	now, loc := fixedNow(t)
	// Round-trip YYYY-MM-DD: format the parsed time and re-parse; they must match.
	in := "2027-02-14"
	got, err := Parse(in, now, loc)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Format("2006-01-02") != in {
		t.Fatalf("round-trip mismatch: %s != %s", got.Format("2006-01-02"), in)
	}
	if got.Location().String() != loc.String() {
		t.Fatalf("wrong loc: %s", got.Location())
	}
}

func TestParseNilLocDefaults(t *testing.T) {
	now, _ := fixedNow(t)
	got, err := Parse("today", now, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.IsZero() {
		t.Fatal("zero time")
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !containsFold(s, sub) {
			return false
		}
	}
	return true
}

func containsFold(s, sub string) bool {
	return len(sub) == 0 || indexFold(s, sub) >= 0
}

func indexFold(s, sub string) int {
	// Simple case-insensitive substring search, avoids pulling strings for a helper.
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			a, b := s[i+j], sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 'a' - 'A'
			}
			if b >= 'A' && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
