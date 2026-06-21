package commands

import (
	"strings"
	"testing"
)

// TestExportGraphDotHighlightEmitsFocusStyle: --graph-dot --highlight
// wraps the focus node in the gold/bold style and leaves other nodes
// untouched.
func TestExportGraphDotHighlightEmitsFocusStyle(t *testing.T) {
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
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot", "--highlight", "2")
	if err != nil {
		t.Fatalf("export --graph-dot --highlight 2: %v", err)
	}
	// The highlighted node line must contain the gold fillcolor.
	if !strings.Contains(stdout, `2 [label="#2 b" style="filled,bold", fillcolor="gold", color="black", penwidth=2];`) {
		// More forgiving check: just look for the gold marker on the
		// #2 line. If the label rendering changes we still want to
		// catch the highlight signal.
		if !lineHasIDAndGold(stdout, 2) {
			t.Fatalf("expected #2 node to be styled with gold fill, got:\n%s", stdout)
		}
	}
	// Other nodes (#1, #3) must NOT have the gold marker.
	for _, otherID := range []int{1, 3} {
		if lineHasIDAndGold(stdout, otherID) {
			t.Fatalf("non-highlighted #%d unexpectedly got gold styling:\n%s", otherID, stdout)
		}
	}
}

// lineHasIDAndGold reports whether the line for node `id` in the
// DOT output contains the "gold" fillcolor marker. Reused by a
// couple of tests.
func lineHasIDAndGold(stdout string, id int) bool {
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		// Node-definition line looks like: `2 [label="#2 b" ...];`
		// Check the line starts with the id followed by " ["
		// (avoids matching edges like "2 -> 1").
		prefix := startsWithIDDefn(trimmed, id)
		if !prefix {
			continue
		}
		if strings.Contains(line, "gold") {
			return true
		}
	}
	return false
}

// startsWithIDDefn reports whether a (trimmed) line starts with the
// node-definition pattern "<id> [" for the given id. Used by tests
// to ignore edges (which look like "<id> -> <id>;").
func startsWithIDDefn(line string, id int) bool {
	prefix := ""
	switch {
	case id < 10:
		prefix = string(rune('0'+id)) + " ["
	default:
		// Fall back to fmt — fine for small ids in tests.
		prefix = ""
		for v := id; v > 0; v /= 10 {
			prefix = string(rune('0'+v%10)) + prefix
		}
		prefix += " ["
	}
	return strings.HasPrefix(line, prefix)
}

// TestExportGraphDotHighlightOnTSKGraph: the `tsk graph --format dot
// --highlight <id>` surface (the original graph verb) also accepts
// --highlight and produces identical output to the export verb.
func TestExportGraphDotHighlightOnTSKGraph(t *testing.T) {
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
	graphOut, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight", "2")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	exportOut, _, err := runCmd(t, dir, "export", "--graph-dot", "--highlight", "2")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if graphOut != exportOut {
		t.Fatalf("graph --format dot --highlight and export --graph-dot --highlight must be byte-identical\nGRAPH:\n%s\nEXPORT:\n%s", graphOut, exportOut)
	}
}

// TestExportGraphDotHighlightRejectedOnNonGraphFormat: --highlight
// outside --graph-dot must error.
func TestExportGraphDotHighlightRejectedOnNonGraphFormat(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "export", "--json", "--highlight", "1")
	if err == nil {
		t.Fatal("expected error: --highlight with --json")
	}
	if !strings.Contains(err.Error(), "--graph-dot") {
		t.Fatalf("error should mention --graph-dot, got: %v", err)
	}
}

// TestExportGraphDotHighlightMissingID: pointing --highlight at an
// id with no task surfaces a clear error.
func TestExportGraphDotHighlightMissingID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "export", "--graph-dot", "--highlight", "999")
	if err == nil {
		t.Fatal("expected error for missing --highlight id")
	}
	if !strings.Contains(err.Error(), "no task with id 999") {
		t.Fatalf("expected 'no task with id 999', got: %v", err)
	}
	if !strings.Contains(err.Error(), "--highlight") {
		t.Fatalf("error should be prefixed with --highlight context, got: %v", err)
	}
}

// TestExportGraphDotHighlightWithReachable: --highlight composes
// with --reachable. The reachable filter scopes the subgraph, and
// highlight styles one of the nodes inside it.
func TestExportGraphDotHighlightWithReachable(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// 2 -> 1 chain; 4 -> 3 chain (disjoint).
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Scope to subgraph from #2 (includes #2 and #1), highlight #1.
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot", "--reachable", "2", "--highlight", "1")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !lineHasIDAndGold(stdout, 1) {
		t.Fatalf("#1 should be highlighted gold in subgraph, got:\n%s", stdout)
	}
	// #3 and #4 are outside the reachable scope, so they should not
	// even appear in the output.
	if strings.Contains(stdout, "4 -> 3;") {
		t.Fatalf("unrelated chain leaked into reachable subgraph, got:\n%s", stdout)
	}
}

// TestExportGraphDotHighlightOverridesBlockedStyle: even when the
// highlighted node would otherwise be styled red (blocked), the
// gold focus marker wins.
func TestExportGraphDotHighlightOverridesBlockedStyle(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq-open", "blocked"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// #2 is now blocked (depends on open #1).
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot", "--highlight", "2")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	// Highlight should win: #2's line has "gold" not "red".
	if !lineHasIDAndGold(stdout, 2) {
		t.Fatalf("#2 should have gold (highlight) styling, got:\n%s", stdout)
	}
	// #2's line must NOT have the red marker (would mean the
	// switch order is wrong).
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if !startsWithIDDefn(trimmed, 2) {
			continue
		}
		if strings.Contains(line, `color="red"`) {
			t.Fatalf("highlight must override blocked-red style; got both:\n%s", line)
		}
	}
}

// TestExportGraphDotNoHighlightDoesNotAddStyle: with no --highlight
// flag, the output is byte-identical to the previous default (no
// gold markers anywhere). This is the regression guard against the
// highlight feature accidentally leaking into the default path.
func TestExportGraphDotNoHighlightDoesNotAddStyle(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if strings.Contains(stdout, "gold") {
		t.Fatalf("no --highlight passed; output must not contain 'gold', got:\n%s", stdout)
	}
	if strings.Contains(stdout, "penwidth=2") {
		t.Fatalf("no --highlight passed; output must not contain 'penwidth=2', got:\n%s", stdout)
	}
}

// TestExportGraphDotHighlightSilentWhenFilteredOut: when --reachable
// scopes the graph such that the highlight id isn't in it, the
// command should NOT error — the user's filter is the source of
// truth. Output stays clean (no gold marker for an absent node).
func TestExportGraphDotHighlightSilentWhenFilteredOut(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// 2 -> 1 chain; 4 -> 3 chain. Highlight #3 but filter to
	// reachable=2 (which only includes #1 and #2).
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot", "--reachable", "2", "--highlight", "3")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	// #3 isn't in the rendered subgraph, so no gold should appear.
	if strings.Contains(stdout, "gold") {
		t.Fatalf("filtered-out highlight should not appear in output, got:\n%s", stdout)
	}
}
