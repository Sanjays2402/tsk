package commands

import (
	"strings"
	"testing"
)

// TestGraphReachableFiltersToSubgraph: setup is two disconnected
// chains: 2->1 and 4->3. `tsk graph --reachable 2` should emit only
// edges from chain {2,1}; chain {4,3} must be absent.
func TestGraphReachableFiltersToSubgraph(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2->1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend 4->3: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2")
	if err != nil {
		t.Fatalf("graph --reachable: %v", err)
	}
	// Should contain #2 → #1 line.
	if !strings.Contains(stdout, "#2 -> #1") {
		t.Fatalf("expected '#2 -> #1' in output, got:\n%s", stdout)
	}
	// Should NOT contain #4 → #3 line (different subgraph).
	if strings.Contains(stdout, "#4 -> #3") {
		t.Fatalf("unrelated subgraph #4 -> #3 must be filtered out, got:\n%s", stdout)
	}
}

// TestGraphReachableTransitivePrereqs: chain 4->3->2->1, asking
// --reachable 4 should include EVERY edge in the chain (transitive
// closure, not just direct prereqs).
func TestGraphReachableTransitivePrereqs(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "4")
	if err != nil {
		t.Fatalf("graph --reachable: %v", err)
	}
	for _, expected := range []string{"#4 -> #3", "#3 -> #2", "#2 -> #1"} {
		if !strings.Contains(stdout, expected) {
			t.Fatalf("expected %q in transitive subgraph, got:\n%s", expected, stdout)
		}
	}
}

// TestGraphReachableNoDepsRoot: a root with no prereqs of its own
// should emit a clear "no dependencies reachable from #N" message
// rather than the generic whole-store empty text.
func TestGraphReachableNoDepsRoot(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "with-deps"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Ask for the subgraph reachable from #1, which has no deps.
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1")
	if err != nil {
		t.Fatalf("graph --reachable: %v", err)
	}
	if !strings.Contains(stdout, "no dependencies reachable from #1") {
		t.Fatalf("expected specific 'no dependencies reachable from #1', got:\n%s", stdout)
	}
}

// TestGraphReachableUnknownID: a non-existent id should produce a
// clear error, not a silent empty subgraph.
func TestGraphReachableUnknownID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "99")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Fatalf("expected error to mention #99, got %v", err)
	}
}

// TestGraphReachableDOTFormat: the --reachable filter composes with
// --format dot. Output should still be valid DOT scaffolding.
func TestGraphReachableDOTFormat(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--format", "dot")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if !strings.HasPrefix(stdout, "digraph tsk {") {
		t.Fatalf("expected DOT skeleton, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "2 -> 1;") {
		t.Fatalf("expected '2 -> 1;' edge, got:\n%s", stdout)
	}
	// Task #3 has no deps and is outside the subgraph — should NOT
	// appear in the DOT node list.
	if strings.Contains(stdout, `"3 `) || strings.Contains(stdout, "  3 [") {
		t.Fatalf("task 3 must be absent from filtered DOT, got:\n%s", stdout)
	}
}

// TestGraphReachableFanIn: when multiple tasks depend on the SAME
// prereq, --reachable from a downstream root should include only
// the ancestors of that root — not sibling tasks that also depend
// on the same prereq.
//
// Layout:
//   - #1 is the prereq (no deps)
//   - #2 and #3 both depend on #1 (siblings)
//   - --reachable 2 should include only the 2 -> 1 edge, not 3 -> 1
func TestGraphReachableFanIn(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "left", "right"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if !strings.Contains(stdout, "#2 -> #1") {
		t.Fatalf("expected #2 -> #1 in subgraph, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#3 -> #1") {
		t.Fatalf("sibling edge #3 -> #1 must be filtered out (not reachable from #2), got:\n%s", stdout)
	}
}

// TestGraphReachableComposeWithOpen: the --reachable filter should
// stack on top of --open without breaking either. With a chain
// containing a done prereq, --reachable + --open should drop that
// done edge.
func TestGraphReachableComposeWithOpen(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"deep", "mid", "top"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Mark #1 done. Without --open, --reachable should include
	// the 2 -> 1 edge. With --open, that edge drops.
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "3", "--open")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if !strings.Contains(stdout, "#3 -> #2") {
		t.Fatalf("expected '#3 -> #2' to remain, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#2 -> #1") {
		t.Fatalf("done-prereq edge #2 -> #1 must drop under --open, got:\n%s", stdout)
	}
}
