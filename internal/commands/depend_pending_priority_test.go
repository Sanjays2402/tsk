package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPendingPriorityFiltersByLevel: among multiple freshly-
// unblocked tasks, --priority urgent keeps only the urgent one.
func TestPendingPriorityFiltersByLevel(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "low-thing", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "urgent-thing", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Default (no priority): both surface.
	stdout, _, err := runCmd(t, dir, "depend", "--pending")
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if !strings.Contains(stdout, "low-thing") || !strings.Contains(stdout, "urgent-thing") {
		t.Fatalf("default pending should show both, got:\n%s", stdout)
	}
	// --priority urgent: only the urgent one.
	stdout, _, err = runCmd(t, dir, "depend", "--pending", "--priority", "urgent")
	if err != nil {
		t.Fatalf("pending --priority urgent: %v", err)
	}
	if !strings.Contains(stdout, "urgent-thing") {
		t.Fatalf("urgent-thing must appear, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "low-thing") {
		t.Fatalf("low-thing should NOT appear under --priority urgent, got:\n%s", stdout)
	}
	// Header annotation present.
	if !strings.Contains(stdout, "priority=urgent") {
		t.Fatalf("header should include 'priority=urgent', got:\n%s", stdout)
	}
}

// TestPendingPriorityShortFormAccepted: --priority u (the short
// alias for urgent in ParsePriority) must work too — consistent
// with what tsk add accepts.
func TestPendingPriorityShortFormAccepted(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "task", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Short form "h" for high.
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--priority", "h")
	if err != nil {
		t.Fatalf("pending --priority h: %v", err)
	}
	if !strings.Contains(stdout, "task") {
		t.Fatalf("expected task under --priority h (short form), got:\n%s", stdout)
	}
}

// TestPendingPriorityComposesWithTag: --priority intersects with
// --tag — both must match for a task to surface.
func TestPendingPriorityComposesWithTag(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "work-low", "-p", "low", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "work-high", "-p", "high", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "home-high", "-p", "high", "-t", "home"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, id := range []string{"2", "3", "4"} {
		if _, _, err := runCmd(t, dir, "depend", id, "--on", "1"); err != nil {
			t.Fatalf("depend %s: %v", id, err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--tag", "work", "--priority", "high")
	if err != nil {
		t.Fatalf("pending --tag work --priority high: %v", err)
	}
	if !strings.Contains(stdout, "work-high") {
		t.Fatalf("work-high should appear (matches both filters), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "work-low") {
		t.Fatalf("work-low should NOT appear (priority mismatch), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "home-high") {
		t.Fatalf("home-high should NOT appear (tag mismatch), got:\n%s", stdout)
	}
	// Both filters in header.
	if !strings.Contains(stdout, "tag=work") {
		t.Fatalf("header should include tag=work, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "priority=high") {
		t.Fatalf("header should include priority=high, got:\n%s", stdout)
	}
}

// TestPendingPriorityEmptyValueIsNoFilter: --priority "" mirrors
// --tag ""'s defensive policy (no filter, no header annotation).
func TestPendingPriorityEmptyValueIsNoFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "blocked", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--priority", "")
	if err != nil {
		t.Fatalf("pending --priority '': %v", err)
	}
	if !strings.Contains(stdout, "blocked") {
		t.Fatalf("empty --priority should not filter; expected 'blocked', got:\n%s", stdout)
	}
	if strings.Contains(stdout, "priority=") {
		t.Fatalf("empty --priority should NOT annotate header with priority=, got:\n%s", stdout)
	}
}

// TestPendingPriorityRejectsBogusValue: an invalid priority
// (typo) must error with exit-2 — silent degradation would
// confuse the user, since they clearly meant SOMETHING.
func TestPendingPriorityRejectsBogusValue(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "depend", "--pending", "--priority", "blazing")
	if err == nil {
		t.Fatal("expected error for bogus --priority")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
	if !strings.Contains(err.Error(), "blazing") {
		t.Fatalf("error should quote the bad value, got: %v", err)
	}
}

// TestPendingPriorityEmptyResultMentionsFilter: when --priority
// matches nothing pending, the empty message must include the
// active priority so the user understands WHY the result is empty.
func TestPendingPriorityEmptyResultMentionsFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "low-task", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--priority", "urgent")
	if err != nil {
		t.Fatalf("pending --priority urgent: %v", err)
	}
	if !strings.Contains(stdout, "no tasks") {
		t.Fatalf("empty result expected, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "priority=urgent") {
		t.Fatalf("empty message should mention 'priority=urgent', got:\n%s", stdout)
	}
}

// TestPendingPriorityJSONStaysClean: --priority + --json must
// produce a valid JSON array narrowed to the priority filter
// (schema unchanged — same pendingRow shape).
func TestPendingPriorityJSONStaysClean(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "low", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "high", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, id := range []string{"2", "3"} {
		if _, _, err := runCmd(t, dir, "depend", id, "--on", "1"); err != nil {
			t.Fatalf("depend %s: %v", id, err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--pending", "--priority", "high", "--json")
	if err != nil {
		t.Fatalf("pending --priority high --json: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d:\n%s", len(rows), stdout)
	}
	if id, _ := rows[0]["id"].(float64); int(id) != 3 {
		t.Fatalf("expected id=3 (high task), got %v", rows[0]["id"])
	}
	// Priority field still present on the row for downstream tools.
	if prio, _ := rows[0]["priority"].(string); prio != "high" {
		t.Fatalf("expected priority=high in JSON row, got %v", rows[0]["priority"])
	}
}
