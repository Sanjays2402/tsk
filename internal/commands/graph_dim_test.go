package commands

import (
	"strings"
	"testing"
)

// TestGraphDimRendersGrayDashed: --dim 2 emits the gray dashed style
// for #2 and leaves the other nodes unchanged. Sister of the
// --highlight tests but for the inverse "push to background" verb.
func TestGraphDimRendersGrayDashed(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta", "gamma"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--dim", "2")
	if err != nil {
		t.Fatalf("graph --dim 2: %v", err)
	}
	if !lineHasDim(stdout, "2 ") {
		t.Fatalf("expected #2 to carry dim style, got:\n%s", stdout)
	}
	if lineHasDim(stdout, "1 ") || lineHasDim(stdout, "3 ") {
		t.Fatalf("only #2 should be dim, got:\n%s", stdout)
	}
}

// TestGraphDimCSVMultiID: --dim accepts CSV (same parser as
// --highlight). Every listed id gets the dim style.
func TestGraphDimCSVMultiID(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1,2,3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--dim", "1,2")
	if err != nil {
		t.Fatalf("graph --dim 1,2: %v", err)
	}
	if !lineHasDim(stdout, "1 ") || !lineHasDim(stdout, "2 ") {
		t.Fatalf("expected #1 and #2 dim, got:\n%s", stdout)
	}
	if lineHasDim(stdout, "3 ") || lineHasDim(stdout, "4 ") {
		t.Fatalf("#3 and #4 must NOT be dim, got:\n%s", stdout)
	}
}

// TestGraphDimHashPrefixTolerated: --dim #2,#3 works the same as
// --dim 2,3 (same CSV parser as --highlight).
func TestGraphDimHashPrefixTolerated(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--dim", "#1,#2")
	if err != nil {
		t.Fatalf("graph --dim #1,#2: %v", err)
	}
	if !lineHasDim(stdout, "1 ") || !lineHasDim(stdout, "2 ") {
		t.Fatalf("expected #1 and #2 dim with hash prefix, got:\n%s", stdout)
	}
}

// TestGraphDimMissingIDErrors: --dim with an id not in the store is
// a usage error — same defensive shape as --highlight.
func TestGraphDimMissingIDErrors(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--format", "dot", "--dim", "99")
	if err == nil {
		t.Fatal("expected error for missing dim id")
	}
	if !strings.Contains(err.Error(), "--dim") {
		t.Fatalf("error should mention --dim, got: %v", err)
	}
}

// TestGraphDimInvalidIDErrors: --dim with a non-numeric token is a
// usage error with --dim: in the message (NOT --highlight:).
func TestGraphDimInvalidIDErrors(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--format", "dot", "--dim", "abc")
	if err == nil {
		t.Fatal("expected error for non-numeric dim token")
	}
	if !strings.Contains(err.Error(), "--dim") {
		t.Fatalf("error must say --dim (not --highlight), got: %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestGraphDimOverlapWithHighlightRejected: a node listed in BOTH
// --dim and --highlight is contradictory intent — reject up-front
// with a clear error rather than silently letting one style win.
func TestGraphDimOverlapWithHighlightRejected(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--format", "dot",
		"--dim", "1,2", "--highlight", "2")
	if err == nil {
		t.Fatal("expected error for dim/highlight overlap")
	}
	if !strings.Contains(err.Error(), "#2") {
		t.Fatalf("error should call out the conflicting id #2, got: %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestGraphDimAndHighlightDisjointWork: when dim and highlight ids
// are disjoint, BOTH styles render (each on the appropriate node).
func TestGraphDimAndHighlightDisjointWork(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot",
		"--dim", "1", "--highlight", "3")
	if err != nil {
		t.Fatalf("graph dim+highlight: %v", err)
	}
	if !lineHasDim(stdout, "1 ") {
		t.Fatalf("expected #1 dim, got:\n%s", stdout)
	}
	if !lineHasGold(stdout, "3 ") {
		t.Fatalf("expected #3 gold, got:\n%s", stdout)
	}
	if lineHasDim(stdout, "3 ") {
		t.Fatalf("#3 must NOT be dim, got:\n%s", stdout)
	}
}

// TestGraphDimRejectedOnASCII: --dim is DOT-only — combining with
// ascii is a usage error (matches --highlight / --highlight-tag).
func TestGraphDimRejectedOnASCII(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--dim", "1")
	if err == nil {
		t.Fatal("expected error combining --dim with ascii format")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestExportGraphDotDimMatchesGraph: the export verb honors --dim
// the same way `tsk graph --format dot` does. Same scaffolding.
func TestExportGraphDotDimMatchesGraph(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot", "--dim", "1")
	if err != nil {
		t.Fatalf("export --graph-dot --dim 1: %v", err)
	}
	if !lineHasDim(stdout, "1 ") {
		t.Fatalf("expected #1 dim via export, got:\n%s", stdout)
	}
}

// TestGraphAndExportDimInLockstep: graph and export produce byte-
// identical output for the same --dim input. Regression against the
// two surfaces ever drifting on the new flag.
func TestGraphAndExportDimInLockstep(t *testing.T) {
	mkDir := func() string {
		d := t.TempDir()
		for _, title := range []string{"a", "b", "c"} {
			if _, _, err := runCmd(t, d, "add", title); err != nil {
				t.Fatalf("add %s: %v", title, err)
			}
		}
		if _, _, err := runCmd(t, d, "depend", "3", "--on", "1,2"); err != nil {
			t.Fatalf("depend: %v", err)
		}
		return d
	}
	graphDir := mkDir()
	exportDir := mkDir()
	graphOut, _, err := runCmd(t, graphDir, "graph", "--format", "dot", "--dim", "1,2")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	exportOut, _, err := runCmd(t, exportDir, "export", "--graph-dot", "--dim", "1,2")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if graphOut != exportOut {
		t.Fatalf("graph and export must produce byte-identical --dim output\nGRAPH:\n%s\nEXPORT:\n%s", graphOut, exportOut)
	}
}

// TestGraphNoDimMeansNoDimStyle: a graph rendered without --dim
// must NOT carry the dim style anywhere — regression against the
// dim block accidentally firing on nil.
func TestGraphNoDimMeansNoDimStyle(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot")
	if err != nil {
		t.Fatalf("graph (no dim): %v", err)
	}
	// "lightgray" is in done-style too, but only when there's a
	// done task. Here we use only open tasks so it must be absent.
	if strings.Contains(stdout, "lightgray") {
		t.Fatalf("no dim should produce no lightgray fill, got:\n%s", stdout)
	}
}

// lineHasDim returns true if any node line in the DOT output for
// the given id prefix (e.g. "1 ") carries the dim style (dashed +
// gray color attribute combination). Mirrors lineHasGold for the
// dim-style assertions.
func lineHasDim(dot, idPrefix string) bool {
	for _, line := range strings.Split(dot, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, idPrefix) {
			continue
		}
		if !strings.Contains(line, "[label=") {
			continue
		}
		if strings.Contains(line, "dashed") && strings.Contains(line, `fontcolor="gray"`) {
			return true
		}
	}
	return false
}
