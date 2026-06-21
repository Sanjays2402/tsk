package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestTopPinnedOnlyRestrictsPool: --pinned-only must reduce the
// result to ONLY pinned tasks, regardless of priority. Without the
// flag, all tasks are eligible (priority drives selection).
func TestTopPinnedOnlyRestrictsPool(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"urgent-unpinned", "low-pinned", "medium-unpinned", "low-pinned-too"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "pri", "1", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "2", "low"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "4", "low"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pin", "2", "4"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	// --pinned-only output: only #2 and #4 should appear.
	stdout, _, err := runCmd(t, dir, "top", "--pinned-only")
	if err != nil {
		t.Fatalf("top --pinned-only: %v", err)
	}
	if !strings.Contains(stdout, "low-pinned") || !strings.Contains(stdout, "low-pinned-too") {
		t.Fatalf("expected both pinned tasks, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "urgent-unpinned") {
		t.Fatalf("urgent-unpinned must NOT appear with --pinned-only, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "medium-unpinned") {
		t.Fatalf("medium-unpinned must NOT appear with --pinned-only, got:\n%s", stdout)
	}
}

// TestTopPinnedOnlyEmptyWhenNonePinned: with no pinned tasks in the
// store, --pinned-only should emit the "no tasks" empty marker —
// not silently fall back to listing everything.
func TestTopPinnedOnlyEmptyWhenNonePinned(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "bravo", "charlie"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "top", "--pinned-only")
	if err != nil {
		t.Fatalf("top --pinned-only: %v", err)
	}
	if !strings.Contains(stdout, "no tasks") {
		t.Fatalf("expected 'no tasks' empty marker, got:\n%s", stdout)
	}
	// And specifically none of the unpinned task titles appear (they
	// have distinct multi-letter names so substring matches are safe).
	for _, title := range []string{"alpha", "bravo", "charlie"} {
		if strings.Contains(stdout, title) {
			t.Fatalf("unpinned task %q must not appear, got:\n%s", title, stdout)
		}
	}
}

// TestTopPinnedOnlyStacksWithRespectDeps: --pinned-only and
// --respect-deps both apply — pinned AND unblocked.
func TestTopPinnedOnlyStacksWithRespectDeps(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "pinned-blocked", "pinned-free", "unpinned-free"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// #2 is pinned but blocked by open #1.
	// #3 is pinned and unblocked (no deps).
	// #4 is unpinned and unblocked (filtered by --pinned-only).
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pin", "2", "3"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "top", "--pinned-only", "--respect-deps")
	if err != nil {
		t.Fatalf("top --pinned-only --respect-deps: %v", err)
	}
	if !strings.Contains(stdout, "pinned-free") {
		t.Fatalf("expected pinned-free in output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "pinned-blocked") {
		t.Fatalf("pinned-blocked has unmet dep — must be filtered, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "unpinned-free") {
		t.Fatalf("unpinned-free must be filtered by --pinned-only, got:\n%s", stdout)
	}
}

// TestTopPinnedOnlyJSONShape: --pinned-only --json emits the same
// task-array shape `top --json` uses, just narrowed to pinned tasks.
// Empty case is `[]`, not null.
func TestTopPinnedOnlyJSONShape(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"unpinned", "pinned"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "pin", "2"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "top", "--pinned-only", "--json")
	if err != nil {
		t.Fatalf("top --pinned-only --json: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 pinned row, got %d:\n%s", len(rows), stdout)
	}
	if title, _ := rows[0]["Title"].(string); title != "pinned" {
		t.Fatalf("expected Title=pinned, got %v", rows[0]["Title"])
	}
	// Empty case → still well-formed empty array, not null.
	dir2 := t.TempDir()
	if _, _, err := runCmd(t, dir2, "add", "nothing-pinned"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err = runCmd(t, dir2, "top", "--pinned-only", "--json")
	if err != nil {
		t.Fatalf("top --pinned-only --json (empty): %v", err)
	}
	trimmed := strings.TrimSpace(stdout)
	if trimmed != "[]" {
		t.Fatalf("expected '[]' for no pinned tasks, got %q", trimmed)
	}
}

// TestTopPinnedOnlyHonorsLimit: --pinned-only respects the N
// positional limit just like the unfiltered view does.
func TestTopPinnedOnlyHonorsLimit(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"p1", "p2", "p3", "p4"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "pin", "1", "2", "3", "4"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "top", "2", "--pinned-only", "--json")
	if err != nil {
		t.Fatalf("top 2 --pinned-only --json: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (N=2 cap), got %d:\n%s", len(rows), stdout)
	}
}

// TestTopPinnedOnlyExcludesDoneByDefault: pinned + done tasks
// should be excluded unless --all is set, matching the default
// behavior of `tsk top` (which is undone-only).
func TestTopPinnedOnlyExcludesDoneByDefault(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"pinned-done", "pinned-open"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "pin", "1", "2"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Without --all: only pinned-open.
	stdout, _, err := runCmd(t, dir, "top", "--pinned-only")
	if err != nil {
		t.Fatalf("top --pinned-only: %v", err)
	}
	if !strings.Contains(stdout, "pinned-open") {
		t.Fatalf("expected pinned-open, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "pinned-done") {
		t.Fatalf("pinned-done is done; must be excluded by default, got:\n%s", stdout)
	}
	// With --all: both should appear.
	stdout, _, err = runCmd(t, dir, "top", "--pinned-only", "--all")
	if err != nil {
		t.Fatalf("top --pinned-only --all: %v", err)
	}
	if !strings.Contains(stdout, "pinned-open") || !strings.Contains(stdout, "pinned-done") {
		t.Fatalf("expected both with --all, got:\n%s", stdout)
	}
}
