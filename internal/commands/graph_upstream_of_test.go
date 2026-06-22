package commands

import (
	"strings"
	"testing"
)

// TestGraphUpstreamOfBasicChain: a linear chain #1 ← #2 ← #3 (each
// task depends on the next-lower id, so #2 depends on #1, #3 depends
// on #2). --upstream-of 1 should produce edges #2->#1 and #3->#2,
// which together with #1 (the root) form the upstream subgraph.
func TestGraphUpstreamOfBasicChain(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add root: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "middle"); err != nil {
		t.Fatalf("add middle: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "top"); err != nil {
		t.Fatalf("add top: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2->1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend 3->2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1")
	if err != nil {
		t.Fatalf("graph --upstream-of: %v", err)
	}
	// Both edges should be present in the adjacency listing.
	if !strings.Contains(stdout, "#2 -> #1") {
		t.Fatalf("expected #2 -> #1 in upstream-of 1, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "#3 -> #2") {
		t.Fatalf("expected #3 -> #2 in upstream-of 1, got:\n%s", stdout)
	}
}

// TestGraphUpstreamOfExcludesOffChainPrereqs: when an upstream task
// has additional prereqs OUTSIDE the upstream chain (e.g. "ship"
// depends on root AND on an unrelated "release-notes"), --upstream-of
// must DROP the off-chain edge so the rendered subgraph stays
// focused on the chain leading to root. This is the critical
// distinction from filterReachableEdges, which keeps edges with
// any reachable source.
func TestGraphUpstreamOfExcludesOffChainPrereqs(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root", "ship", "release-notes"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// #2 (ship) depends on BOTH #1 (root) and #3 (release-notes).
	// upstream-of 1 should include #2->#1 but NOT #2->#3.
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1,3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1")
	if err != nil {
		t.Fatalf("upstream-of: %v", err)
	}
	if !strings.Contains(stdout, "#2 -> #1") {
		t.Fatalf("expected #2 -> #1 in subgraph, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#2 -> #3") {
		t.Fatalf("off-chain edge #2 -> #3 must be EXCLUDED, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#3") {
		t.Fatalf("#3 (off-chain prereq) must not appear, got:\n%s", stdout)
	}
}

// TestGraphUpstreamOfRootWithNoDependents: when root has nobody
// upstream, the result is empty — message is "no tasks depend on #N"
// (distinct from --reachable's "no dependencies reachable from #N").
func TestGraphUpstreamOfRootWithNoDependents(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1")
	if err != nil {
		t.Fatalf("upstream-of lonely: %v", err)
	}
	if !strings.Contains(stdout, "no tasks depend on #1") {
		t.Fatalf("expected 'no tasks depend on #1', got:\n%s", stdout)
	}
}

// TestGraphUpstreamOfRejectsCombinedWithReachable: the two filters
// answer opposite-direction questions; combining them would muddle
// the subgraph definition. Exit 2 (usage error).
func TestGraphUpstreamOfRejectsCombinedWithReachable(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--upstream-of", "1")
	if err == nil {
		t.Fatal("expected error combining --reachable and --upstream-of")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestGraphUpstreamOfRejectsMissingID: an --upstream-of pointing
// at a non-existent task surfaces the same "no task with id N"
// error --reachable uses.
func TestGraphUpstreamOfRejectsMissingID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--upstream-of", "99")
	if err == nil {
		t.Fatal("expected error for nonexistent --upstream-of id")
	}
	if !strings.Contains(err.Error(), "no task with id 99") {
		t.Fatalf("expected 'no task with id 99' error, got %v", err)
	}
}

// TestGraphUpstreamOfDotFormatRenders: DOT format works the same
// as --reachable. The root node, every transitive dependent, and
// the in-chain edges all appear in the DOT body.
func TestGraphUpstreamOfDotFormat(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root", "mid", "top"} {
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
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--format", "dot")
	if err != nil {
		t.Fatalf("dot upstream-of: %v", err)
	}
	if !strings.Contains(stdout, "digraph tsk {") {
		t.Fatalf("expected DOT skeleton, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "2 -> 1") {
		t.Fatalf("expected 2 -> 1 edge in DOT, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "3 -> 2") {
		t.Fatalf("expected 3 -> 2 edge in DOT, got:\n%s", stdout)
	}
	// All three nodes should be declared.
	for _, want := range []string{`1 [label="#1 root"`, `2 [label="#2 mid"`, `3 [label="#3 top"`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected node decl %q, got:\n%s", want, stdout)
		}
	}
}

// TestGraphUpstreamOfWithHighlight: --upstream-of composes with
// --highlight so the user can pin the root visually inside the
// impact subgraph. Common recipe: "show the chain still blocked
// by #X, with #X highlighted as the focus".
func TestGraphUpstreamOfWithHighlight(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root", "dep"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--format", "dot", "--highlight", "1")
	if err != nil {
		t.Fatalf("upstream-of + highlight: %v", err)
	}
	if !strings.Contains(stdout, "gold") {
		t.Fatalf("expected highlight (gold fill) on #1, got:\n%s", stdout)
	}
}

// TestGraphUpstreamOfDiamondJoin: when two upstream chains converge
// (#3 depends on #2, #4 depends on #2, both #2 depends on root),
// every node on both chains appears in the subgraph and every
// in-chain edge is rendered. Validates the BFS correctly handles
// a multi-parent visit pattern.
func TestGraphUpstreamOfDiamondJoin(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root", "deploy", "ship", "release"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// #2 -> #1, #3 -> #2, #4 -> #2 (so #3 and #4 both transitively
	// depend on #1).
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1")
	if err != nil {
		t.Fatalf("upstream-of diamond: %v", err)
	}
	for _, want := range []string{"#2 -> #1", "#3 -> #2", "#4 -> #2"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in diamond subgraph, got:\n%s", want, stdout)
		}
	}
}

// TestGraphUpstreamOfRespectsOpenFilter: --upstream-of + --open
// composes correctly — done dependents are filtered out before the
// upstream BFS runs (because collectGraphEdges removes them under
// --open). Done tasks can't be made `tsk done` directly when they
// have open prereqs (depend.go's invariant), so we use --clear to
// drop the dependency before marking the task done; the upstream
// chain link survives because we restored it AFTER the done flip.
func TestGraphUpstreamOfRespectsOpenFilter(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root", "open-dep", "done-dep"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend #2->#1: %v", err)
	}
	// First mark #3 done without any prereq...
	if _, _, err := runCmd(t, dir, "done", "3"); err != nil {
		t.Fatalf("done 3: %v", err)
	}
	// ...then add the upstream dep (now from a done task, which is
	// fine — depend doesn't reject the source-being-done case).
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend #3->#1 post-done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--open")
	if err != nil {
		t.Fatalf("upstream-of --open: %v", err)
	}
	if !strings.Contains(stdout, "#2 -> #1") {
		t.Fatalf("expected #2 -> #1 (still open), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#3 ->") {
		t.Fatalf("done dependent #3 must be filtered by --open, got:\n%s", stdout)
	}
}

// TestGraphReachableStillWorksAfterRefactor: regression guard that
// --reachable wasn't broken by the new --upstream-of plumbing
// (emitGraph signature change, runE refactor).
func TestGraphReachableStillWorksAfterRefactor(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root", "dep"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2")
	if err != nil {
		t.Fatalf("--reachable regression: %v", err)
	}
	if !strings.Contains(stdout, "#2 -> #1") {
		t.Fatalf("expected #2 -> #1 in reachable view, got:\n%s", stdout)
	}
	// Empty-message wording for --reachable should be unchanged.
	stdout2, _, err := runCmd(t, dir, "graph", "--reachable", "1")
	if err != nil {
		t.Fatalf("--reachable lonely: %v", err)
	}
	if !strings.Contains(stdout2, "no dependencies reachable from #1") {
		t.Fatalf("expected 'no dependencies reachable from #1', got:\n%s", stdout2)
	}
}
