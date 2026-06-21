package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestUpstreamListsDirectDependents: when several tasks depend on a
// target id, --upstream surfaces each of them with the right status
// annotation.
func TestUpstreamListsDirectDependents(t *testing.T) {
	dir := t.TempDir()
	// Layout: 1 is target. 2 depends only on 1 (would unblock).
	// 3 depends on 1 and 99 missing (also "unblocks" — missing
	// counts as satisfied per unmetBlockers' policy). 4 depends
	// on 1 AND 5 (5 is open, so it's still blocked even after 1).
	// 5 doesn't depend on 1.
	for _, title := range []string{"target", "only-1", "1-and-missing", "1-and-5", "unrelated"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1,5"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--upstream")
	if err != nil {
		t.Fatalf("depend --upstream: %v", err)
	}
	// Header says target #1 has 2 upstream.
	if !strings.Contains(stdout, "#1  target  (upstream: 2)") {
		t.Fatalf("expected header 'upstream: 2', got:\n%s", stdout)
	}
	// #2 would unblock (it only depends on #1).
	if !strings.Contains(stdout, "#2  only-1  (unblocks)") {
		t.Fatalf("expected '#2 (unblocks)', got:\n%s", stdout)
	}
	// #4 is still blocked (also depends on open #5).
	if !strings.Contains(stdout, "#4  1-and-5  (blocked)") {
		t.Fatalf("expected '#4 (blocked)', got:\n%s", stdout)
	}
	// #5 should NOT appear — it doesn't depend on #1.
	if strings.Contains(stdout, "#5  unrelated") {
		t.Fatalf("unrelated task #5 must not appear, got:\n%s", stdout)
	}
}

// TestUpstreamHonorsTransitiveSatisfaction: a dependent that names
// the target plus an already-done task should still classify as
// "unblocks" — the other dep is satisfied, the target IS the gating
// edge.
func TestUpstreamHonorsTransitiveSatisfaction(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"target", "done-prereq", "dependent"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// #3 depends on #1 and #2. Mark #2 done.
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--upstream")
	if err != nil {
		t.Fatalf("depend --upstream: %v", err)
	}
	if !strings.Contains(stdout, "#3  dependent  (unblocks)") {
		t.Fatalf("expected dependent with all-other-deps-done to be 'unblocks', got:\n%s", stdout)
	}
}

// TestUpstreamMarksDoneDependents: a dependent that is itself done
// should classify as "done" — the edge is historical, no longer
// relevant to whether to close the target.
func TestUpstreamMarksDoneDependents(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"target", "old-dependent"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Close prereq first so we can close the dependent.
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	// Undo #1 (it's done at the moment) just so we have a meaningful
	// "upstream of #1" query that returns a done dependent.
	if _, _, err := runCmd(t, dir, "undo", "1"); err != nil {
		t.Fatalf("undo: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--upstream")
	if err != nil {
		t.Fatalf("depend --upstream: %v", err)
	}
	if !strings.Contains(stdout, "#2  old-dependent  (done)") {
		t.Fatalf("expected done dependent with (done) annotation, got:\n%s", stdout)
	}
}

// TestUpstreamEmptyResult: a task no one depends on should produce
// a clear plain-language message, and JSON should emit an empty
// upstream array (not null).
func TestUpstreamEmptyResult(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--upstream")
	if err != nil {
		t.Fatalf("depend --upstream: %v", err)
	}
	if !strings.Contains(stdout, "no tasks depend on #1") {
		t.Fatalf("expected 'no tasks depend on #1' message, got:\n%s", stdout)
	}
	// JSON form.
	stdout, _, err = runCmd(t, dir, "depend", "1", "--upstream", "--json")
	if err != nil {
		t.Fatalf("depend --upstream --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	arr, ok := doc["upstream"].([]any)
	if !ok {
		t.Fatalf("expected upstream array, got %T:\n%s", doc["upstream"], stdout)
	}
	if len(arr) != 0 {
		t.Fatalf("expected empty array, got %d items:\n%s", len(arr), stdout)
	}
	if got, _ := doc["total_count"].(float64); int(got) != 0 {
		t.Fatalf("expected total_count=0, got %v", doc["total_count"])
	}
}

// TestUpstreamJSONStructure: --upstream --json emits the documented
// schema (id/title/upstream array with id/title/status per row).
func TestUpstreamJSONStructure(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"target", "dep-a", "dep-b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend 3: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--upstream", "--json")
	if err != nil {
		t.Fatalf("depend --upstream --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if id, _ := doc["id"].(float64); int(id) != 1 {
		t.Fatalf("expected id=1, got %v", doc["id"])
	}
	if got, _ := doc["total_count"].(float64); int(got) != 2 {
		t.Fatalf("expected total_count=2, got %v", doc["total_count"])
	}
	arr := doc["upstream"].([]any)
	if len(arr) != 2 {
		t.Fatalf("expected 2 upstream rows, got %d", len(arr))
	}
	// Sorted by id asc.
	for i, want := range []int{2, 3} {
		row := arr[i].(map[string]any)
		if got, _ := row["id"].(float64); int(got) != want {
			t.Fatalf("row %d: expected id=%d, got %v", i, want, row["id"])
		}
		if status, _ := row["status"].(string); status != "unblocks" {
			t.Fatalf("row %d: expected status=unblocks, got %v", i, row["status"])
		}
	}
}

// TestUpstreamMutexWithMutation: --upstream cannot be combined with
// mutation flags (it's read-only).
func TestUpstreamMutexWithMutation(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "depend", "1", "--upstream", "--clear")
	if err == nil {
		t.Fatal("expected error combining --upstream with --clear")
	}
}

// TestUpstreamMutexWithOtherReadOnly: --upstream is mutually
// exclusive with --tree and --justify — each is a distinct view.
func TestUpstreamMutexWithOtherReadOnly(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "y"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	for _, other := range []string{"--tree", "--justify"} {
		_, _, err := runCmd(t, dir, "depend", "2", "--upstream", other)
		if err == nil {
			t.Fatalf("expected error combining --upstream with %s", other)
		}
	}
}

// TestUpstreamRequiresID: --upstream needs a positional id (otherwise
// what would "what depends on me?" even mean?).
func TestUpstreamRequiresID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "depend", "--upstream")
	if err == nil {
		t.Fatal("expected error: --upstream without id")
	}
}
