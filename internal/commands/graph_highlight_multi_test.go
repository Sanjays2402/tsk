package commands

import (
	"strings"
	"testing"
)

// TestGraphHighlightMultiID: --highlight 1,3 spotlights both nodes
// with the gold/bold style. Both must carry the style; neither
// other node should.
func TestGraphHighlightMultiID(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta", "gamma", "delta"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// Create some edges so the graph isn't empty: 4 depends on 1,2,3.
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1,2,3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight", "1,3")
	if err != nil {
		t.Fatalf("graph --highlight 1,3: %v", err)
	}
	// #1 and #3 each get a "gold" line. #2 and #4 must NOT.
	if !lineHasGold(stdout, "1 ") {
		t.Fatalf("expected node #1 to carry gold style, got:\n%s", stdout)
	}
	if !lineHasGold(stdout, "3 ") {
		t.Fatalf("expected node #3 to carry gold style, got:\n%s", stdout)
	}
	if lineHasGold(stdout, "2 ") {
		t.Fatalf("node #2 must NOT carry gold style, got:\n%s", stdout)
	}
	if lineHasGold(stdout, "4 ") {
		t.Fatalf("node #4 must NOT carry gold style, got:\n%s", stdout)
	}
}

// TestGraphHighlightSingleIDStillWorks: backward-compat — a single
// "7" passed to the now-string highlight must still spotlight node
// #7 exactly as the old int flag did.
func TestGraphHighlightSingleIDStillWorks(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight", "1")
	if err != nil {
		t.Fatalf("graph --highlight 1: %v", err)
	}
	if !lineHasGold(stdout, "1 ") {
		t.Fatalf("single-id highlight #1 should still work, got:\n%s", stdout)
	}
	if lineHasGold(stdout, "2 ") {
		t.Fatalf("only #1 should be highlighted, not #2:\n%s", stdout)
	}
}

// TestGraphHighlightHashPrefixTolerated: --highlight "#1,#3" must
// work the same as "1,3" — matches the depend --on / next --skip
// convention of accepting "#N" tokens for users coming from
// `tsk show #7` muscle memory.
func TestGraphHighlightHashPrefixTolerated(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight", "#1,#2")
	if err != nil {
		t.Fatalf("graph --highlight #1,#2: %v", err)
	}
	for _, want := range []string{"1 ", "2 "} {
		if !lineHasGold(stdout, want) {
			t.Fatalf("expected node %q gold under #-prefixed highlight, got:\n%s", want, stdout)
		}
	}
}

// TestGraphHighlightInvalidIDExitsCleanly: a bogus id in the CSV
// surfaces at the flag layer with exit 2 — silently rendering a
// graph with no spotlight would hide typos.
func TestGraphHighlightInvalidIDExitsCleanly(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight", "1,abc")
	if err == nil {
		t.Fatal("expected error for non-numeric id in --highlight")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestGraphHighlightMissingIDErrors: every id in the CSV must
// exist in the store. A missing id should not silently render
// without a spotlight — that hides the typo.
func TestGraphHighlightMissingIDErrors(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight", "1,99")
	if err == nil {
		t.Fatal("expected error for missing id in --highlight")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Fatalf("error should name the missing id 99, got: %v", err)
	}
}

// TestGraphHighlightDuplicatesCollapse: --highlight 1,1,1 is the
// same as --highlight 1 — duplicates dedupe via the set.
func TestGraphHighlightDuplicatesCollapse(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight", "1,1,1")
	if err != nil {
		t.Fatalf("graph --highlight 1,1,1: %v", err)
	}
	// Count gold lines for node #1 — must be exactly one. (Edge
	// lines like "1 -> 2;" don't carry gold so the [label= filter
	// is just belt-and-suspenders.)
	count := 0
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "1 ") {
			continue
		}
		if !strings.Contains(line, "[label=") {
			continue
		}
		if strings.Contains(line, "gold") {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 gold node line for #1 (dedup), got %d in:\n%s", count, stdout)
	}
}

// TestGraphHighlightOverridesRedBlocked: a blocked task that is
// also highlighted must render gold, not red. The whole reason for
// the override-style policy.
func TestGraphHighlightOverridesRedBlocked(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "blocked-thing"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Without highlight, #2 would render red (it's open and depends
	// on #1 which is still open). With highlight, #2 must render
	// gold; the "color=red" attr must NOT appear on the #2 line.
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight", "2")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "2 ") {
			continue
		}
		// Edge lines look like "2 -> 1;". Node lines look like
		// "2 [label=...]". Only check node lines.
		if !strings.Contains(line, "[label=") {
			continue
		}
		if !strings.Contains(line, "gold") {
			t.Fatalf("#2 node line should carry gold style:\n%s", line)
		}
		if strings.Contains(line, `color="red"`) {
			t.Fatalf("#2 node line should NOT carry red style (highlight overrides):\n%s", line)
		}
	}
}

// TestExportGraphDOTHighlightMultiID: same multi-id contract on
// the central data-out verb. Same parsing, same set, same render.
func TestExportGraphDOTHighlightMultiID(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1,2,3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot", "--highlight", "1,3")
	if err != nil {
		t.Fatalf("export --graph-dot --highlight 1,3: %v", err)
	}
	if !lineHasGold(stdout, "1 ") {
		t.Fatalf("expected #1 gold via export, got:\n%s", stdout)
	}
	if !lineHasGold(stdout, "3 ") {
		t.Fatalf("expected #3 gold via export, got:\n%s", stdout)
	}
	if lineHasGold(stdout, "2 ") {
		t.Fatalf("#2 must NOT be gold via export, got:\n%s", stdout)
	}
}

// TestGraphAndExportHighlightInLockstep: byte-identical output
// between `tsk graph --format dot --highlight 1,3` and
// `tsk export --graph-dot --highlight 1,3` — regression against
// the two surfaces ever drifting.
func TestGraphAndExportHighlightInLockstep(t *testing.T) {
	mkDir := func() string {
		d := t.TempDir()
		for _, title := range []string{"a", "b", "c", "d"} {
			if _, _, err := runCmd(t, d, "add", title); err != nil {
				t.Fatalf("add: %v", err)
			}
		}
		if _, _, err := runCmd(t, d, "depend", "4", "--on", "1,2,3"); err != nil {
			t.Fatalf("depend: %v", err)
		}
		return d
	}
	graphDir := mkDir()
	exportDir := mkDir()
	graphOut, _, err := runCmd(t, graphDir, "graph", "--format", "dot", "--highlight", "1,3")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	exportOut, _, err := runCmd(t, exportDir, "export", "--graph-dot", "--highlight", "1,3")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if graphOut != exportOut {
		t.Fatalf("graph and export must produce byte-identical multi-highlight output\nGRAPH:\n%s\nEXPORT:\n%s", graphOut, exportOut)
	}
}

// TestParseHighlightCSVEmptyReturnsNil: empty input must return
// (nil, nil) so the printGraphDOT membership check (highlightSet[id])
// reads as "no highlight" (a nil map lookup returns zero value).
func TestParseHighlightCSVEmptyReturnsNil(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Use the real CLI to validate the empty-string path through
	// parseHighlightCSV (just call graph with no --highlight).
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot")
	if err != nil {
		t.Fatalf("graph (no highlight): %v", err)
	}
	if strings.Contains(stdout, "gold") {
		t.Fatalf("no highlight should produce no gold styles, got:\n%s", stdout)
	}
}

// lineHasGold returns true if any node line in the DOT output for
// the given id prefix (e.g. "1 ") carries the gold style fillcolor.
// Skips edge lines (those have "->" not "[label=").
func lineHasGold(dot, idPrefix string) bool {
	for _, line := range strings.Split(dot, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, idPrefix) {
			continue
		}
		if !strings.Contains(line, "[label=") {
			continue
		}
		if strings.Contains(line, "gold") {
			return true
		}
	}
	return false
}
