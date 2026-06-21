package commands

import (
	"strings"
	"testing"
)

// TestExportGraphDotEmitsDOT: the basic case — a store with a
// dependency edge produces DOT syntax with the expected
// "digraph tsk" header and an arrow between the two ids.
func TestExportGraphDotEmitsDOT(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "blocker"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot")
	if err != nil {
		t.Fatalf("export --graph-dot: %v", err)
	}
	if !strings.Contains(stdout, "digraph tsk {") {
		t.Fatalf("expected DOT header 'digraph tsk {', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "rankdir=LR") {
		t.Fatalf("expected rankdir=LR, got:\n%s", stdout)
	}
	// The arrow #2 -> #1 (we depend on prereq).
	if !strings.Contains(stdout, "2 -> 1;") {
		t.Fatalf("expected '2 -> 1;' edge, got:\n%s", stdout)
	}
}

// TestExportGraphDotByteIdenticalToGraphFormat: the export-verb
// surface must produce byte-identical output to the equivalent
// `tsk graph --format dot` call. This is a regression guard
// against drift between the two surfaces.
func TestExportGraphDotByteIdenticalToGraphFormat(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
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
	exportOut, _, err := runCmd(t, dir, "export", "--graph-dot")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	graphOut, _, err := runCmd(t, dir, "graph", "--format", "dot")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if exportOut != graphOut {
		t.Fatalf("export --graph-dot must be byte-identical to `graph --format dot`\nEXPORT:\n%s\nGRAPH:\n%s", exportOut, graphOut)
	}
}

// TestExportGraphDotFormatAlias: --format graph-dot and --format
// dot both resolve to graph-dot output (plus the no-dash form).
func TestExportGraphDotFormatAlias(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	for _, alias := range []string{"graph-dot", "dot", "graphdot", "GRAPH-DOT"} {
		t.Run(alias, func(t *testing.T) {
			stdout, _, err := runCmd(t, dir, "export", "--format", alias)
			if err != nil {
				t.Fatalf("export --format %s: %v", alias, err)
			}
			if !strings.Contains(stdout, "digraph tsk {") {
				t.Fatalf("alias %s should produce DOT, got:\n%s", alias, stdout)
			}
		})
	}
}

// TestExportGraphDotWithReachable: --graph-dot --reachable scopes
// the emitted graph to the subgraph rooted at the given id.
func TestExportGraphDotWithReachable(t *testing.T) {
	dir := t.TempDir()
	// Build two disjoint chains: 2->1 and 4->3.
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
	// Scope to subgraph rooted at #2 → must include 2 -> 1 edge,
	// must NOT include 4 -> 3 edge.
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot", "--reachable", "2")
	if err != nil {
		t.Fatalf("export --graph-dot --reachable 2: %v", err)
	}
	if !strings.Contains(stdout, "2 -> 1;") {
		t.Fatalf("expected scoped edge '2 -> 1;', got:\n%s", stdout)
	}
	if strings.Contains(stdout, "4 -> 3;") {
		t.Fatalf("edge from unrelated chain should be excluded, got:\n%s", stdout)
	}
}

// TestExportGraphDotWithOpen: --graph-dot --open drops done tasks
// from the emitted graph.
func TestExportGraphDotWithOpen(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"done-one", "open-two"} {
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
	// Without --open, the edge 2 -> 1 appears (1 is satisfied but
	// the relationship still renders so the user sees the history).
	plain, _, err := runCmd(t, dir, "export", "--graph-dot")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(plain, "2 -> 1;") {
		t.Fatalf("expected '2 -> 1;' in default mode, got:\n%s", plain)
	}
	// With --open: 1 is done so the edge is dropped (per
	// collectGraphEdges' "satisfied dep is not actively blocking"
	// policy).
	openOut, _, err := runCmd(t, dir, "export", "--graph-dot", "--open")
	if err != nil {
		t.Fatalf("export --open: %v", err)
	}
	if strings.Contains(openOut, "2 -> 1;") {
		t.Fatalf("--open should drop edges to satisfied prereqs, got:\n%s", openOut)
	}
}

// TestExportGraphDotReachableMissingID: pointing --reachable at an
// id with no task surfaces a clear error matching `tsk graph`'s
// message.
func TestExportGraphDotReachableMissingID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "export", "--graph-dot", "--reachable", "999")
	if err == nil {
		t.Fatal("expected error for missing --reachable id")
	}
	if !strings.Contains(err.Error(), "no task with id 999") {
		t.Fatalf("expected 'no task with id 999', got: %v", err)
	}
}

// TestExportGraphDotEmptyStore: a store with no deps emits the
// "no dependencies" marker, NOT an empty DOT skeleton.
func TestExportGraphDotEmptyStore(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(stdout, "no dependencies") {
		t.Fatalf("expected 'no dependencies' marker on empty store, got:\n%s", stdout)
	}
}

// TestExportRejectsMultipleShortcuts: --json + --graph-dot must
// error rather than silently picking one. Re-uses the existing
// "multiple formats" check; this case extends coverage to the
// new --graph-dot shortcut.
func TestExportRejectsGraphDotPlusJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "export", "--graph-dot", "--json")
	if err == nil {
		t.Fatal("expected error: --graph-dot + --json")
	}
	if !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("error should say 'exactly one', got: %v", err)
	}
}

// TestExportGraphDotReachableRejectedOnNonGraphFormat: passing
// --reachable with a non-graph format must error so users don't
// silently get JSON when they meant DOT.
func TestExportGraphDotReachableRejectedOnNonGraphFormat(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "export", "--json", "--reachable", "1")
	if err == nil {
		t.Fatal("expected error: --reachable with --json")
	}
	if !strings.Contains(err.Error(), "--graph-dot") {
		t.Fatalf("error should mention --graph-dot, got: %v", err)
	}
}
