package model

import (
	"testing"
	"time"
)

func TestParseRecurrenceValid(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"daily", "daily"},
		{"DAILY", "daily"},
		{"weekly", "weekly"},
		{"monthly", "monthly"},
		{"yearly", "yearly"},
		{"annually", "yearly"},
		{"weekdays", "weekdays"},
		{"every:3d", "every:3d"},
		{"every:1w", "every:1w"},
		{"every:2m", "every:2m"},
		{"every:5y", "every:5y"},
		{"  every:3d  ", "every:3d"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseRecurrence(tc.in)
			if err != nil {
				t.Fatalf("ParseRecurrence(%q) err: %v", tc.in, err)
			}
			if got.String() != tc.want {
				t.Fatalf("ParseRecurrence(%q).String() = %q, want %q", tc.in, got.String(), tc.want)
			}
			// Roundtrip: String -> ParseRecurrence -> String.
			rt, err := ParseRecurrence(got.String())
			if err != nil {
				t.Fatalf("roundtrip err: %v", err)
			}
			if rt.String() != got.String() {
				t.Fatalf("roundtrip mismatch: %q -> %q", got.String(), rt.String())
			}
		})
	}
}

func TestParseRecurrenceInvalid(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"never",
		"every",
		"every:",
		"every:0d",
		"every:-2d",
		"every:3x",
		"every:3",
		"every:abc",
		"every:3dd",
		"weekdayss",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseRecurrence(in); err == nil {
				t.Fatalf("expected error for %q", in)
			}
		})
	}
}

func TestRecurrenceNext(t *testing.T) {
	loc := time.UTC
	mk := func(y, m, d int) time.Time {
		return time.Date(y, time.Month(m), d, 0, 0, 0, 0, loc)
	}

	cases := []struct {
		name string
		rule string
		from time.Time
		want time.Time
	}{
		// daily always advances one day, including across month boundaries.
		{"daily basic", "daily", mk(2026, 5, 9), mk(2026, 5, 10)},
		{"daily over month", "daily", mk(2026, 5, 31), mk(2026, 6, 1)},

		// weekly advances 7 days; same weekday next week.
		{"weekly wednesday", "weekly", mk(2026, 5, 13), mk(2026, 5, 20)}, // Wed -> Wed
		{"weekly friday", "weekly", mk(2026, 5, 8), mk(2026, 5, 15)},     // Fri -> Fri

		// monthly with day-of-month clamping.
		{"monthly normal", "monthly", mk(2026, 5, 15), mk(2026, 6, 15)},
		{"monthly jan 31 -> feb 28", "monthly", mk(2026, 1, 31), mk(2026, 2, 28)},
		{"monthly jan 31 leap year", "monthly", mk(2024, 1, 31), mk(2024, 2, 29)},
		{"monthly mar 31 -> apr 30", "monthly", mk(2026, 3, 31), mk(2026, 4, 30)},
		{"monthly dec -> jan", "monthly", mk(2026, 12, 15), mk(2027, 1, 15)},

		// yearly with leap-day clamping.
		{"yearly basic", "yearly", mk(2026, 7, 4), mk(2027, 7, 4)},
		{"yearly feb 29 -> feb 28", "yearly", mk(2024, 2, 29), mk(2025, 2, 28)},
		{"yearly feb 29 -> next leap", "yearly", mk(2020, 2, 29), mk(2021, 2, 28)},

		// weekdays: Friday -> next Monday, weekday -> next weekday, Sunday -> Monday.
		{"weekdays friday -> monday", "weekdays", mk(2026, 5, 8), mk(2026, 5, 11)},
		{"weekdays sunday -> monday", "weekdays", mk(2026, 5, 10), mk(2026, 5, 11)},
		{"weekdays saturday -> monday", "weekdays", mk(2026, 5, 9), mk(2026, 5, 11)},
		{"weekdays monday -> tuesday", "weekdays", mk(2026, 5, 11), mk(2026, 5, 12)},
		{"weekdays thursday -> friday", "weekdays", mk(2026, 5, 14), mk(2026, 5, 15)},

		// every:Nd / Nw.
		{"every:3d", "every:3d", mk(2026, 5, 9), mk(2026, 5, 12)},
		{"every:2w", "every:2w", mk(2026, 5, 9), mk(2026, 5, 23)},

		// every:Nm with clamping.
		{"every:1m mar 31 -> apr 30", "every:1m", mk(2026, 3, 31), mk(2026, 4, 30)},
		{"every:2m jan 31 -> mar 31", "every:2m", mk(2026, 1, 31), mk(2026, 3, 31)},
		{"every:13m wraps year", "every:13m", mk(2026, 1, 15), mk(2027, 2, 15)},

		// every:Ny with clamping.
		{"every:1y feb 29 -> feb 28", "every:1y", mk(2024, 2, 29), mk(2025, 2, 28)},
		{"every:4y feb 29 -> feb 29", "every:4y", mk(2024, 2, 29), mk(2028, 2, 29)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ParseRecurrence(tc.rule)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.rule, err)
			}
			got := r.Next(tc.from)
			if !got.Equal(tc.want) {
				t.Fatalf("Next(%s) under %q = %s, want %s",
					tc.from.Format("2006-01-02"), tc.rule,
					got.Format("2006-01-02"), tc.want.Format("2006-01-02"))
			}
		})
	}
}

func TestRecurrenceNextAdvancesAtLeastOneStep(t *testing.T) {
	// Calling Next on Wednesday with weekly returns next Wednesday (not the same day).
	loc := time.UTC
	wed := time.Date(2026, 5, 13, 12, 0, 0, 0, loc) // Wed afternoon
	r, _ := ParseRecurrence("weekly")
	nextDate := r.Next(wed)
	want := time.Date(2026, 5, 20, 0, 0, 0, 0, loc)
	if !nextDate.Equal(want) {
		t.Fatalf("weekly Next(Wed) = %s, want %s", nextDate, want)
	}
}

func TestRecurrenceNextPreservesLocation(t *testing.T) {
	la, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Skip("zoneinfo unavailable")
	}
	from := time.Date(2026, 5, 9, 14, 30, 0, 0, la)
	r, _ := ParseRecurrence("daily")
	got := r.Next(from)
	if got.Location().String() != la.String() {
		t.Fatalf("Next lost location: got %s want %s", got.Location(), la)
	}
	// Should be midnight of the next day in LA.
	want := time.Date(2026, 5, 10, 0, 0, 0, 0, la)
	if !got.Equal(want) {
		t.Fatalf("Next = %s, want %s", got, want)
	}
}
