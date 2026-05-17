package commands

import (
	"strings"
	"testing"
)

// TestBulkRequiresSelector verifies no selector → error.
func TestBulkRequiresSelector(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "bulk", "--set-priority", "high", "--apply")
	if err == nil {
		t.Fatal("expected error when no selector provided")
	}
}

// TestBulkRequiresMutation verifies no mutation → error.
func TestBulkRequiresMutation(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "old"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "bulk", "--tag", "old", "--apply")
	if err == nil {
		t.Fatal("expected error when no mutation provided")
	}
}

// TestBulkDryRunDefault verifies dry-run is the default (no --apply).
func TestBulkDryRunDefault(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "old"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "bulk", "--tag", "old", "--add-tag", "legacy")
	if err != nil {
		t.Fatalf("bulk dry: %v", err)
	}
	if !strings.Contains(stdout, "DRY RUN") {
		t.Errorf("expected DRY RUN, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "--apply") {
		t.Errorf("expected dry-run hint about --apply, got:\n%s", stdout)
	}
	// Source unchanged
	lsOut, _, _ := runCmd(t, dir, "ls", "--json")
	if strings.Contains(lsOut, "legacy") {
		t.Errorf("tag should not be applied in dry run:\n%s", lsOut)
	}
}

// TestBulkApplyTagAddRemove verifies --apply with tag add/remove works.
func TestBulkApplyTagAddRemove(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "t1", "-t", "old"); err != nil {
		t.Fatalf("add t1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "t2", "-t", "old", "-t", "keep"); err != nil {
		t.Fatalf("add t2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "t3", "-t", "other"); err != nil {
		t.Fatalf("add t3: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "bulk", "--tag", "old", "--add-tag", "legacy", "--remove-tag", "old", "--apply")
	if err != nil {
		t.Fatalf("bulk apply: %v", err)
	}
	if !strings.Contains(stdout, "updated 2 task") {
		t.Errorf("expected 2 updates, got:\n%s", stdout)
	}
	jsonOut, _, _ := runCmd(t, dir, "ls", "--json")
	// t1 should have legacy but not old
	// t3 should be untouched (still has "other")
	if !strings.Contains(jsonOut, "legacy") {
		t.Errorf("legacy tag missing:\n%s", jsonOut)
	}
	// t3 must still have "other"
	if !strings.Contains(jsonOut, "other") {
		t.Errorf("untouched t3 missing 'other':\n%s", jsonOut)
	}
}

// TestBulkPriorityFilterAndMutation verifies priority filter + set-priority.
func TestBulkPriorityFilterAndMutation(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lo1", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "lo2", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "hi", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "bulk", "--priority", "low", "--set-priority", "medium", "--apply")
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if !strings.Contains(stdout, "updated 2 task") {
		t.Errorf("expected 2 updates, got:\n%s", stdout)
	}
}

// TestBulkByID verifies --id selector.
func TestBulkByID(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "bulk", "--id", "1", "--id", "3", "--add-tag", "tagged", "--apply")
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if !strings.Contains(stdout, "updated 2 task") {
		t.Errorf("expected 2 updates, got:\n%s", stdout)
	}
}

// TestBulkClearDue verifies --clear-due.
func TestBulkClearDue(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-d", "2030-01-01"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "bulk", "--id", "1", "--clear-due", "--apply"); err != nil {
		t.Fatalf("bulk: %v", err)
	}
	jsonOut, _, _ := runCmd(t, dir, "ls", "--json")
	if strings.Contains(jsonOut, "2030-01-01") {
		t.Errorf("due date not cleared:\n%s", jsonOut)
	}
}

// TestBulkSetDueAndClearMutuallyExclusive verifies --set-due and --clear-due conflict.
func TestBulkSetDueAndClearMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "bulk", "--id", "1", "--set-due", "tomorrow", "--clear-due", "--apply")
	if err == nil {
		t.Fatal("expected error with both --set-due and --clear-due")
	}
}

// TestBulkNoMatchPrintsMessage verifies no-match path prints a message and exits 0.
func TestBulkNoMatchPrintsMessage(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "hello"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "bulk", "--tag", "missing", "--add-tag", "y", "--apply")
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if !strings.Contains(stdout, "no tasks matched") {
		t.Errorf("expected 'no tasks matched', got:\n%s", stdout)
	}
}

// TestBulkTagsAreCaseInsensitive verifies tag matching is case-insensitive.
func TestBulkTagsAreCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "Frontend"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "bulk", "--tag", "frontend", "--add-tag", "web", "--apply")
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if !strings.Contains(stdout, "updated 1 task") {
		t.Errorf("expected case-insensitive match, got:\n%s", stdout)
	}
}

// TestBulkAddTagDedup verifies adding an existing tag doesn't duplicate.
func TestBulkAddTagDedup(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "bulk", "--id", "1", "--add-tag", "alpha", "--apply"); err != nil {
		t.Fatalf("bulk: %v", err)
	}
	jsonOut, _, _ := runCmd(t, dir, "ls", "--json")
	// Count occurrences of "alpha" — should be exactly 1
	count := strings.Count(jsonOut, `"alpha"`)
	if count != 1 {
		t.Errorf("expected exactly 1 'alpha' tag, got %d:\n%s", count, jsonOut)
	}
}
