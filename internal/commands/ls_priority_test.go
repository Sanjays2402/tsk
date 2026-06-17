package commands

import (
	"strings"
	"testing"
)

// TestLsPriorityFilterMatches verifies --priority narrows the list to tasks of
// that exact priority. Exercises resolvePriorityFilter's success path.
func TestLsPriorityFilterMatches(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "urgent thing", "-p", "urgent"); err != nil {
		t.Fatalf("add urgent: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "low thing", "-p", "low"); err != nil {
		t.Fatalf("add low: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "ls", "--priority", "urgent")
	if err != nil {
		t.Fatalf("ls --priority urgent: %v", err)
	}
	if !strings.Contains(stdout, "urgent thing") {
		t.Fatalf("expected urgent task in output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "low thing") {
		t.Fatalf("low task should be filtered out, got:\n%s", stdout)
	}
}

// TestLsPriorityFilterRejectsBogus verifies an unknown --priority value is a
// hard error rather than a silent empty list.
func TestLsPriorityFilterRejectsBogus(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "ls", "--priority", "supercritical")
	if err == nil {
		t.Fatal("expected error for unknown --priority value")
	}
}

// TestLsPriorityFilterCombinesWithAll verifies --priority composes with --all
// so a done task of the right priority still shows.
func TestLsPriorityFilterCombinesWithAll(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "done high", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "open high", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}

	// Without --all, the done task is hidden by the default state filter.
	stdout, _, err := runCmd(t, dir, "ls", "--priority", "high")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if strings.Contains(stdout, "done high") {
		t.Fatalf("done task should be hidden without --all, got:\n%s", stdout)
	}

	// With --all, both high-priority tasks appear.
	stdout, _, err = runCmd(t, dir, "ls", "--all", "--priority", "high")
	if err != nil {
		t.Fatalf("ls --all: %v", err)
	}
	if !strings.Contains(stdout, "done high") || !strings.Contains(stdout, "open high") {
		t.Fatalf("expected both high tasks with --all, got:\n%s", stdout)
	}
}

// TestLsEmptyPriorityShowsAll confirms an empty --priority (the default) does
// not filter by priority — resolvePriorityFilter's no-filter branch.
func TestLsEmptyPriorityShowsAll(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"low", "medium", "high", "urgent"} {
		if _, _, err := runCmd(t, dir, "add", "task-"+p, "-p", p); err != nil {
			t.Fatalf("add %s: %v", p, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	for _, p := range []string{"low", "medium", "high", "urgent"} {
		if !strings.Contains(stdout, "task-"+p) {
			t.Fatalf("expected task-%s with no priority filter, got:\n%s", p, stdout)
		}
	}
}
