package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestNextSkipExcludesNamedTask: with #2 the highest-priority task,
// `tsk next --skip 2` should fall through to the runner-up #1.
func TestNextSkipExcludesNamedTask(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "second-best", "-p", "medium"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "winner", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Sanity check: without --skip, the high-priority #2 wins.
	stdout, _, err := runCmd(t, dir, "next")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	if !strings.Contains(stdout, "winner") {
		t.Fatalf("expected winner without skip, got:\n%s", stdout)
	}
	// With --skip 2, the second-best #1 wins.
	stdout, _, err = runCmd(t, dir, "next", "--skip", "2")
	if err != nil {
		t.Fatalf("next --skip 2: %v", err)
	}
	if !strings.Contains(stdout, "second-best") {
		t.Fatalf("expected second-best after --skip 2, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "winner") {
		t.Fatalf("winner must be skipped, got:\n%s", stdout)
	}
}

// TestNextSkipAcceptsMultipleIDsAndHashPrefix: comma-separated and
// "#7" notation both work.
func TestNextSkipAcceptsMultipleIDsAndHashPrefix(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "next", "--skip", "#1,#2")
	if err != nil {
		t.Fatalf("next --skip: %v", err)
	}
	if !strings.Contains(stdout, "#3") || !strings.Contains(stdout, "c") {
		t.Fatalf("expected #3 c (#1 and #2 skipped), got:\n%s", stdout)
	}
}

// TestNextSkipEmptyResult: when every task is in the skip list,
// `next` should report "all caught up" (the legitimate empty
// state). It must not crash or surface a skipped task.
func TestNextSkipEmptyResult(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "next", "--skip", "1,2")
	if err != nil {
		t.Fatalf("next --skip all: %v", err)
	}
	if !strings.Contains(stdout, "all caught up") {
		t.Fatalf("expected 'all caught up' when every task skipped, got:\n%s", stdout)
	}
}

// TestNextSkipStacksWithRespectDeps: --skip and --respect-deps should
// compose — skip the explicit ids AND skip the blocked ones.
func TestNextSkipStacksWithRespectDeps(t *testing.T) {
	dir := t.TempDir()
	// #1 prereq, #2 depends on #1 (blocked), #3 plain, #4 plain.
	// All have the same priority so id-order ties decide.
	for _, title := range []string{"prereq", "blocked-by-1", "third", "fourth"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// --respect-deps alone: #2 is blocked, so #1 wins on id.
	// Now also --skip 1: #2 is blocked-out, #1 is skipped, #3 wins.
	stdout, _, err := runCmd(t, dir, "next", "--respect-deps", "--skip", "1")
	if err != nil {
		t.Fatalf("next stack: %v", err)
	}
	if !strings.Contains(stdout, "third") {
		t.Fatalf("expected third (after skipping prereq and dep-blocked), got:\n%s", stdout)
	}
}

// TestNextSkipJSONReflectsExclusion: --json output must respect the
// skip set the same way plain text does.
func TestNextSkipJSONReflectsExclusion(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "next", "--json", "--skip", "1")
	if err != nil {
		t.Fatalf("next --json --skip: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if id, _ := doc["id"].(float64); int(id) != 2 {
		t.Fatalf("expected id=2 after --skip 1, got %v", doc["id"])
	}
}

// TestNextSkipJSONEmpty: --skip-everything → {"empty": true}.
func TestNextSkipJSONEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "next", "--json", "--skip", "1")
	if err != nil {
		t.Fatalf("next --json --skip 1: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if got, _ := doc["empty"].(bool); !got {
		t.Fatalf("expected empty=true, got %v", doc)
	}
}

// TestNextSkipRejectsBadID: a non-numeric token in --skip should
// fail up-front with a usage error.
func TestNextSkipRejectsBadID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "next", "--skip", "banana")
	if err == nil {
		t.Fatal("expected error for non-numeric --skip token")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2 (usage error), got %v", err)
	}
}

// TestNextSkipIgnoresMissingIDsAndDupes: stale ids in the skip set
// should not error, and duplicates collapse silently.
func TestNextSkipIgnoresMissingIDsAndDupes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// 99 doesn't exist; the duplicate 1,1 should collapse.
	stdout, _, err := runCmd(t, dir, "next", "--skip", "99,1,1,99")
	if err != nil {
		t.Fatalf("next --skip stale+dupe: %v", err)
	}
	if !strings.Contains(stdout, "all caught up") {
		t.Fatalf("expected all-caught-up (only task #1 skipped), got:\n%s", stdout)
	}
}
