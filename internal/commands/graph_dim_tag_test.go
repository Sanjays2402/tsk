package commands

import (
	"strings"
	"testing"
)

// TestGraphDimTagPushesTaggedNodesToBackground: --dim-tag scaffold
// emits the dim style for every node carrying the tag. Sister of
// the highlight-tag tests but for the inverse "push to background"
// verb.
func TestGraphDimTagPushesTaggedNodesToBackground(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha", "-t", "scaffold"); err != nil {
		t.Fatalf("add alpha: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta"); err != nil {
		t.Fatalf("add beta: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "gamma", "-t", "scaffold"); err != nil {
		t.Fatalf("add gamma: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "delta"); err != nil {
		t.Fatalf("add delta: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1,2,3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--dim-tag", "scaffold")
	if err != nil {
		t.Fatalf("graph --dim-tag scaffold: %v", err)
	}
	if !lineHasDim(stdout, "1 ") {
		t.Fatalf("expected #1 (scaffold) dim, got:\n%s", stdout)
	}
	if !lineHasDim(stdout, "3 ") {
		t.Fatalf("expected #3 (scaffold) dim, got:\n%s", stdout)
	}
	if lineHasDim(stdout, "2 ") {
		t.Fatalf("#2 (no tag) must NOT be dim, got:\n%s", stdout)
	}
	if lineHasDim(stdout, "4 ") {
		t.Fatalf("#4 (no tag) must NOT be dim, got:\n%s", stdout)
	}
}

// TestGraphDimTagCaseInsensitive: tag matching is case-insensitive
// (mirrors --highlight-tag semantics and `tsk ls --tag`).
func TestGraphDimTagCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "scaffold"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--dim-tag", "SCAFFOLD")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if !lineHasDim(stdout, "1 ") {
		t.Fatalf("expected #1 dim under uppercase match, got:\n%s", stdout)
	}
}

// TestGraphDimTagUnionWithDimIDs: --dim 4 + --dim-tag scaffold renders
// BOTH the id and every tagged node in dim style (the union policy
// mirrors --highlight + --highlight-tag).
func TestGraphDimTagUnionWithDimIDs(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "scaffold"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "c"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "d"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1,2,3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot",
		"--dim", "3", "--dim-tag", "scaffold")
	if err != nil {
		t.Fatalf("graph union: %v", err)
	}
	if !lineHasDim(stdout, "1 ") {
		t.Fatalf("#1 (tagged) should be dim:\n%s", stdout)
	}
	if !lineHasDim(stdout, "3 ") {
		t.Fatalf("#3 (--dim) should be dim:\n%s", stdout)
	}
	if lineHasDim(stdout, "2 ") {
		t.Fatalf("#2 must NOT be dim:\n%s", stdout)
	}
	if lineHasDim(stdout, "4 ") {
		t.Fatalf("#4 must NOT be dim:\n%s", stdout)
	}
}

// TestGraphDimTagMissingTagRendersCleanly: a tag no task carries is
// NOT an error — same defensive policy as --highlight-tag.
func TestGraphDimTagMissingTagRendersCleanly(t *testing.T) {
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
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--dim-tag", "nonexistent")
	if err != nil {
		t.Fatalf("graph --dim-tag nonexistent: %v", err)
	}
	if strings.Contains(stdout, "dashed") {
		t.Fatalf("missing tag should produce no dim style, got:\n%s", stdout)
	}
}

// TestGraphDimTagRejectsOverlapWithHighlight: a node both dimmed-by-
// tag AND highlighted-by-id is still contradictory. The overlap
// check fires AFTER mergeDimTag so tag-resolved dim ids are in
// scope for the rejection.
func TestGraphDimTagRejectsOverlapWithHighlight(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "scaffold-thing", "-t", "scaffold"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "other"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--format", "dot",
		"--dim-tag", "scaffold", "--highlight", "1")
	if err == nil {
		t.Fatal("expected overlap error (#1 is both dim-tagged and highlighted)")
	}
	if !strings.Contains(err.Error(), "#1") {
		t.Fatalf("error should call out #1, got: %v", err)
	}
}

// TestGraphDimTagAsciiRejects: --dim-tag is DOT-only (matches
// --dim, --highlight, --highlight-tag).
func TestGraphDimTagAsciiRejects(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--dim-tag", "x")
	if err == nil {
		t.Fatal("expected error combining --dim-tag with ascii format")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestExportGraphDotDimTagMatchesGraph: the export verb honors
// --dim-tag the same way `tsk graph --format dot` does.
func TestExportGraphDotDimTagMatchesGraph(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "scaffold"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot", "--dim-tag", "scaffold")
	if err != nil {
		t.Fatalf("export --graph-dot --dim-tag: %v", err)
	}
	if !lineHasDim(stdout, "1 ") {
		t.Fatalf("expected #1 dim via export, got:\n%s", stdout)
	}
}

// TestGraphAndExportDimTagInLockstep: graph and export produce
// byte-identical output for the same --dim-tag input.
func TestGraphAndExportDimTagInLockstep(t *testing.T) {
	mkDir := func() string {
		d := t.TempDir()
		if _, _, err := runCmd(t, d, "add", "a", "-t", "scaffold"); err != nil {
			t.Fatalf("add a: %v", err)
		}
		if _, _, err := runCmd(t, d, "add", "b"); err != nil {
			t.Fatalf("add b: %v", err)
		}
		if _, _, err := runCmd(t, d, "add", "c", "-t", "scaffold"); err != nil {
			t.Fatalf("add c: %v", err)
		}
		if _, _, err := runCmd(t, d, "depend", "3", "--on", "1,2"); err != nil {
			t.Fatalf("depend: %v", err)
		}
		return d
	}
	graphDir := mkDir()
	exportDir := mkDir()
	graphOut, _, err := runCmd(t, graphDir, "graph", "--format", "dot", "--dim-tag", "scaffold")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	exportOut, _, err := runCmd(t, exportDir, "export", "--graph-dot", "--dim-tag", "scaffold")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if graphOut != exportOut {
		t.Fatalf("graph and export must produce byte-identical --dim-tag output\nGRAPH:\n%s\nEXPORT:\n%s", graphOut, exportOut)
	}
}
