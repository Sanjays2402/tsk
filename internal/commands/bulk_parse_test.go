package commands

import (
	"testing"
)

// TestParseOptionalStatus exercises every recognized status alias plus the
// empty (nil) and invalid cases. parseOptionalStatus was largely uncovered.
func TestParseOptionalStatus(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		got, err := parseOptionalStatus("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("empty status should return nil, got %v", *got)
		}
	})

	openAliases := []string{"open", "todo", "pending", "OPEN", "ToDo"}
	for _, a := range openAliases {
		t.Run("open/"+a, func(t *testing.T) {
			got, err := parseOptionalStatus(a)
			if err != nil {
				t.Fatalf("alias %q: unexpected error %v", a, err)
			}
			if got == nil || *got != false {
				t.Fatalf("alias %q should mean done=false, got %v", a, got)
			}
		})
	}

	doneAliases := []string{"done", "complete", "completed", "DONE", "Completed"}
	for _, a := range doneAliases {
		t.Run("done/"+a, func(t *testing.T) {
			got, err := parseOptionalStatus(a)
			if err != nil {
				t.Fatalf("alias %q: unexpected error %v", a, err)
			}
			if got == nil || *got != true {
				t.Fatalf("alias %q should mean done=true, got %v", a, got)
			}
		})
	}

	t.Run("invalid errors with exit code 2", func(t *testing.T) {
		_, err := parseOptionalStatus("halfway")
		if err == nil {
			t.Fatal("expected error for invalid status")
		}
		var ec ExitCoder
		if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
			t.Fatalf("expected ExitCode 2, got %v", err)
		}
	})
}

// TestParseOptionalDue verifies the nil, valid, and invalid branches of
// parseOptionalDue.
func TestParseOptionalDue(t *testing.T) {
	t.Run("empty returns nil", func(t *testing.T) {
		got, err := parseOptionalDue("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != nil {
			t.Fatalf("empty due should return nil, got %v", *got)
		}
	})

	t.Run("absolute date parses", func(t *testing.T) {
		got, err := parseOptionalDue("2030-07-04")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil {
			t.Fatal("expected a parsed time, got nil")
		}
		if got.Year() != 2030 || got.Month() != 7 || got.Day() != 4 {
			t.Fatalf("parsed wrong date: %v", got)
		}
	})

	t.Run("invalid errors with exit code 2", func(t *testing.T) {
		_, err := parseOptionalDue("not-a-date")
		if err == nil {
			t.Fatal("expected error for invalid due")
		}
		var ec ExitCoder
		if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
			t.Fatalf("expected ExitCode 2, got %v", err)
		}
	})
}
