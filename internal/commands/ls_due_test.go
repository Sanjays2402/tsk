package commands

import (
	"strings"
	"testing"
	"time"
)

// dueDate formats an offset (in days) from now as a YYYY-MM-DD string in the
// store's resolved timezone, matching how `add -d` parses absolute dates.
func dueDate(offsetDays int) string {
	loc := ResolveTZ()
	return time.Now().In(loc).AddDate(0, 0, offsetDays).Format("2006-01-02")
}

// TestLsOverdueFilter verifies --overdue surfaces only tasks whose due date is
// in the past (and not done).
func TestLsOverdueFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "past due", "-d", dueDate(-3)); err != nil {
		t.Fatalf("add past: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "future due", "-d", dueDate(5)); err != nil {
		t.Fatalf("add future: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls", "--overdue")
	if err != nil {
		t.Fatalf("ls --overdue: %v", err)
	}
	if !strings.Contains(stdout, "past due") {
		t.Fatalf("expected overdue task, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "future due") {
		t.Fatalf("future task should not be overdue, got:\n%s", stdout)
	}
}

// TestLsTodayFilter verifies --today surfaces only tasks due today.
func TestLsTodayFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "due today", "-d", dueDate(0)); err != nil {
		t.Fatalf("add today: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "due tomorrow", "-d", dueDate(1)); err != nil {
		t.Fatalf("add tomorrow: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls", "--today")
	if err != nil {
		t.Fatalf("ls --today: %v", err)
	}
	if !strings.Contains(stdout, "due today") {
		t.Fatalf("expected today's task, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "due tomorrow") {
		t.Fatalf("tomorrow's task should be excluded, got:\n%s", stdout)
	}
}

// TestLsUpcomingFilter verifies --upcoming surfaces only future-dated tasks.
func TestLsUpcomingFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "soon", "-d", dueDate(2)); err != nil {
		t.Fatalf("add soon: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "was due", "-d", dueDate(-2)); err != nil {
		t.Fatalf("add past: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls", "--upcoming")
	if err != nil {
		t.Fatalf("ls --upcoming: %v", err)
	}
	if !strings.Contains(stdout, "soon") {
		t.Fatalf("expected upcoming task, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "was due") {
		t.Fatalf("past-due task should not be upcoming, got:\n%s", stdout)
	}
}

// TestLsNoDueFilterShowsUndated confirms that with no due filter, an undated
// task is still listed (passDueFilter's all-pass default).
func TestLsNoDueFilterShowsUndated(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "no date here"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(stdout, "no date here") {
		t.Fatalf("undated task should list by default, got:\n%s", stdout)
	}
}
