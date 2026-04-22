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

func TestParseRelative(t *testing.T) {
	now, loc := fixedNow(t)
	cases := []struct {
		in   string
		want time.Time
	}{
		{"3d", ymd(t, 2026, time.April, 24)},
		{"in 3d", ymd(t, 2026, time.April, 24)},
		{"IN 3 days", ymd(t, 2026, time.April, 24)},
		{"2w", ymd(t, 2026, time.May, 5)},
		{"in 2 weeks", ymd(t, 2026, time.May, 5)},
		{"1m", ymd(t, 2026, time.May, 21)},
		{"in 1 month", ymd(t, 2026, time.May, 21)},
		{"1y", ymd(t, 2027, time.April, 21)},
		{"0d", ymd(t, 2026, time.April, 21)},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got, err := Parse(tc.in, now, loc)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestParseRelativeRejectsJunk(t *testing.T) {
	now, loc := fixedNow(t)
	for _, in := range []string{"3", "3x", "d3", "in 3", "-3d", "3 hours", "in"} {
		if _, err := Parse(in, now, loc); err == nil {
			t.Fatalf("%q should fail", in)
		}
	}
}

func TestParseDSTBoundaries(t *testing.T) {
	_, loc := fixedNow(t)
	// Spring forward: 2026-03-08. Parsing "tomorrow" from 2026-03-07 noon
	// should land on 2026-03-08 midnight local.
	spring := time.Date(2026, time.March, 7, 12, 0, 0, 0, loc)
	got, err := Parse("tomorrow", spring, loc)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.March, 8, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("spring got %s want %s", got, want)
	}
	// Fall back: 2026-11-01. "in 1d" from Oct 31 noon lands on Nov 1 midnight.
	fall := time.Date(2026, time.October, 31, 12, 0, 0, 0, loc)
	got, err = Parse("in 1d", fall, loc)
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2026, time.November, 1, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("fall got %s want %s", got, want)
	}
	// Seven days straddling fall-back: "1w" from Oct 28 should be Nov 4.
	sep := time.Date(2026, time.October, 28, 12, 0, 0, 0, loc)
	got, err = Parse("1w", sep, loc)
	if err != nil {
		t.Fatal(err)
	}
	want = time.Date(2026, time.November, 4, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("1w got %s want %s", got, want)
	}
}

func TestParseMonthDay(t *testing.T) {
	now, loc := fixedNow(t)
	cases := []struct {
		in   string
		want time.Time
	}{
		// Anchor: 2026-04-21 (Tue). "jul" is still ahead -> 2026-07-01.
		{"jul", ymd(t, 2026, time.July, 1)},
		{"july", ymd(t, 2026, time.July, 1)},
		// "mar" is behind -> roll to next year.
		{"mar", ymd(t, 2027, time.March, 1)},
		// Current month: "apr" today (21st), bare month = Apr 1, which is past,
		// so it must roll to next year.
		{"apr", ymd(t, 2027, time.April, 1)},
		{"jul 4", ymd(t, 2026, time.July, 4)},
		{"july 4", ymd(t, 2026, time.July, 4)},
		{"jul 4 2027", ymd(t, 2027, time.July, 4)},
		{"4 jul", ymd(t, 2026, time.July, 4)},
		{"4 jul 2027", ymd(t, 2027, time.July, 4)},
		{"JUL   4", ymd(t, 2026, time.July, 4)},
		// Day behind current date in current month rolls to next year.
		{"apr 10", ymd(t, 2027, time.April, 10)},
		// Day today or ahead in current month stays in current year.
		{"apr 21", ymd(t, 2026, time.April, 21)},
		{"apr 30", ymd(t, 2026, time.April, 30)},
		{"sept 1", ymd(t, 2026, time.September, 1)},
		{"dec 25", ymd(t, 2026, time.December, 25)},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			got, err := Parse(tc.in, now, loc)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestParseMonthDayInvalid(t *testing.T) {
	now, loc := fixedNow(t)
	for _, in := range []string{"feb 30", "jul 32", "jul 0", "jul 4 99", "foo 4"} {
		if _, err := Parse(in, now, loc); err == nil {
			t.Fatalf("%q should fail", in)
		}
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
