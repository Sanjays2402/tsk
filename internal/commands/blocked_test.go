package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBlockedListsAllStuckTasks: `tsk blocked` returns the same
// content as `tsk depend --list` for a multi-task graph.
func TestBlockedListsAllStuckTasks(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// 3 depends on 1; 4 depends on 2,3 (chain plus fan-in).
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend 3: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "2,3"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "blocked")
	if err != nil {
		t.Fatalf("blocked: %v", err)
	}
	for _, want := range []string{"#3", "#4", "blocked by"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in blocked output, got:\n%s", want, stdout)
		}
	}
	// Match `depend --list` byte-for-byte so the two surfaces can't
	// drift in semantics.
	parallel, _, err := runCmd(t, dir, "depend", "--list")
	if err != nil {
		t.Fatalf("depend --list: %v", err)
	}
	if stdout != parallel {
		t.Fatalf("blocked vs depend --list differ:\n--- blocked ---\n%s\n--- depend --list ---\n%s",
			stdout, parallel)
	}
}

// TestBlockedJSONIsArray: --json emits a JSON array even when empty
// (no blocked tasks) — so jq pipelines never crash on null.
func TestBlockedJSONIsArray(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "blocked", "--json")
	if err != nil {
		t.Fatalf("blocked --json: %v", err)
	}
	var doc []any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(doc) != 0 {
		t.Fatalf("expected empty array, got %v", doc)
	}
}

// TestBlockedAliasStuck: `tsk stuck` is a working alias.
func TestBlockedAliasStuck(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "stuck")
	if err != nil {
		t.Fatalf("stuck alias: %v", err)
	}
	if !strings.Contains(stdout, "#2") || !strings.Contains(stdout, "blocked by #1") {
		t.Fatalf("alias should produce same output, got:\n%s", stdout)
	}
}

// TestBlockedDoneTaskNotListed: completing the prerequisite removes
// the dependent from the blocked list (it's no longer stuck).
func TestBlockedDoneTaskNotListed(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "blocked")
	if err != nil {
		t.Fatalf("blocked: %v", err)
	}
	if !strings.Contains(stdout, "no blocked tasks") {
		t.Fatalf("expected 'no blocked tasks', got:\n%s", stdout)
	}
}
