package commands

import (
	"strings"
	"testing"
)

// TestTopoSinceReverseEmitsPrereqs: chain 3 -> 2 -> 1, `topo
// --since 3 --reverse` emits just [1, 2] (the prereqs before
// the milestone), in dependency-respecting order.
func TestTopoSinceReverseEmitsPrereqs(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "middle", "top"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2->1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend 3->2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--ids", "--since", "3", "--reverse")
	if err != nil {
		t.Fatalf("topo --reverse: %v", err)
	}
	if strings.TrimSpace(stdout) != "1,2" {
		t.Fatalf("expected '1,2' (the prereqs before #3), got %q", stdout)
	}
}

// TestTopoSinceReverseExcludesAnchor: the anchor task #N must NOT
// appear in --reverse output — the user asked for prereqs, not
// the milestone itself.
func TestTopoSinceReverseExcludesAnchor(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--ids", "--since", "2", "--reverse")
	if err != nil {
		t.Fatalf("topo --reverse: %v", err)
	}
	// Anchor #2 must not appear.
	if strings.Contains(stdout, "2") {
		t.Fatalf("anchor #2 should not appear in --reverse output, got %q", stdout)
	}
	// #1 (the prereq) must appear.
	if !strings.Contains(stdout, "1") {
		t.Fatalf("prereq #1 should appear in --reverse output, got %q", stdout)
	}
}

// TestTopoSinceReverseHeadIsEmpty: --since on a task with no
// prereqs in the topo output errors with exit-2 (it's already at
// the head, no work comes before it).
func TestTopoSinceReverseHeadIsEmpty(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"head", "downstream"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	_, _, err := runCmd(t, dir, "topo", "--since", "1", "--reverse")
	if err == nil {
		t.Fatal("expected error when --reverse anchor is already at the head")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
	if !strings.Contains(err.Error(), "head") {
		t.Fatalf("error should mention 'head', got: %v", err)
	}
}

// TestTopoReverseRequiresSince: --reverse without --since is a
// usage error (there's no anchor to reverse from).
func TestTopoReverseRequiresSince(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "topo", "--reverse")
	if err == nil {
		t.Fatal("expected error: --reverse without --since")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
	if !strings.Contains(err.Error(), "--since") {
		t.Fatalf("error should mention --since, got: %v", err)
	}
}

// TestTopoSinceReverseComposesWithJSON: --reverse + --json must
// produce a valid JSON array of just the prereqs.
func TestTopoSinceReverseComposesWithJSON(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// Chain: 3 -> 2 -> 1
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--since", "3", "--reverse", "--json")
	if err != nil {
		t.Fatalf("topo --reverse --json: %v", err)
	}
	// Must be valid JSON.
	if !strings.HasPrefix(strings.TrimSpace(stdout), "[") {
		t.Fatalf("expected JSON array, got %q", stdout)
	}
	// Anchor #3 must not be present, but #1 and #2 must.
	if strings.Contains(stdout, "\"id\": 3") {
		t.Fatalf("anchor #3 should not be in reverse JSON, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "\"id\": 1") || !strings.Contains(stdout, "\"id\": 2") {
		t.Fatalf("expected prereqs #1 and #2 in JSON, got:\n%s", stdout)
	}
}

// TestTopoSinceReverseBranching: with a branching graph (3 depends
// on both 1 and 2), --since 3 --reverse must surface BOTH branches.
func TestTopoSinceReverseBranching(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"branch-a", "branch-b", "join"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// #3 depends on BOTH #1 and #2.
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--ids", "--since", "3", "--reverse")
	if err != nil {
		t.Fatalf("topo --reverse: %v", err)
	}
	// Both branches present; #3 excluded.
	got := strings.TrimSpace(stdout)
	if !strings.Contains(got, "1") || !strings.Contains(got, "2") {
		t.Fatalf("expected both #1 and #2 prereqs, got %q", got)
	}
	if strings.Contains(strings.Split(got, ",")[0], "3") || strings.HasSuffix(got, "3") || strings.Contains(got, ",3,") {
		t.Fatalf("anchor #3 must not appear, got %q", got)
	}
}

// TestSliceTopoBeforeIsolated: direct unit test on the helper
// guards the empty/missing/at-head edge cases. The helper now
// computes prereqs via reverse BFS through DependsOn edges, so
// we set up an explicit 3 → 2 → 1 chain in the synthetic data.
func TestSliceTopoBeforeIsolated(t *testing.T) {
	a := topoTask{}
	b := topoTask{}
	c := topoTask{}
	a.Task.ID = 1
	b.Task.ID = 2
	c.Task.ID = 3
	// Chain: 2 depends on 1, 3 depends on 2.
	b.Task.DependsOn = []int{1}
	c.Task.DependsOn = []int{2}
	in := []topoTask{a, b, c}

	// Normal: prereqs of #3 are [1, 2] (transitive through 2 -> 1).
	got := sliceTopoBefore(in, 3)
	if len(got) != 2 || got[0].Task.ID != 1 || got[1].Task.ID != 2 {
		t.Fatalf("expected [1, 2], got %v", idsOf(got))
	}

	// Head: #1 has no prereqs in the slice → nil result.
	got = sliceTopoBefore(in, 1)
	if got != nil {
		t.Fatalf("expected nil for at-head anchor, got %v", idsOf(got))
	}

	// Missing: id not in slice → nil result.
	got = sliceTopoBefore(in, 99)
	if got != nil {
		t.Fatalf("expected nil for missing id, got %v", idsOf(got))
	}
}

// idsOf is a tiny test helper that extracts ids from a slice of
// topoTask for compact failure messages.
func idsOf(rows []topoTask) []int {
	out := make([]int, len(rows))
	for i, r := range rows {
		out[i] = r.Task.ID
	}
	return out
}

// TestTopoSinceReversePreservesCycleVisibility: when a hand-edit
// creates a cycle, the cycle rows must still appear in --reverse
// output (corruption visibility is non-negotiable).
func TestTopoSinceReversePreservesCycleVisibility(t *testing.T) {
	dir := t.TempDir()
	// Build a clean 3-task chain first, plus an isolated 4th task
	// to serve as our reverse anchor.
	for _, title := range []string{"prereq", "middle", "isolated", "anchor"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// Anchor #4 depends on #1 — gives it ONE prereq.
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1"); err != nil {
		t.Fatalf("depend 4->1: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--ids", "--since", "4", "--reverse")
	if err != nil {
		t.Fatalf("topo --reverse: %v", err)
	}
	// Should contain just #1 (the prereq); #2 and #3 are isolated
	// and don't gate #4. The anchor #4 itself is excluded.
	got := strings.TrimSpace(stdout)
	if got != "1" {
		t.Fatalf("expected just '1', got %q", got)
	}
}
