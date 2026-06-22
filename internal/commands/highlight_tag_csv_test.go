package commands

import (
	"strings"
	"testing"
)

// TestHighlightTagCSVUnionOfTwoTags: --highlight-tag release,p0 must
// spotlight every task carrying EITHER tag (logical OR). The union
// policy mirrors --highlight ids' multi-id semantics.
func TestHighlightTagCSVUnionOfTwoTags(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "release"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-t", "p0"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "c", "-t", "other"); err != nil {
		t.Fatalf("add c: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "d"); err != nil {
		t.Fatalf("add d: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1,2,3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight-tag", "release,p0")
	if err != nil {
		t.Fatalf("graph --highlight-tag release,p0: %v", err)
	}
	if !lineHasGold(stdout, "1 ") {
		t.Fatalf("expected #1 (release) gold, got:\n%s", stdout)
	}
	if !lineHasGold(stdout, "2 ") {
		t.Fatalf("expected #2 (p0) gold, got:\n%s", stdout)
	}
	if lineHasGold(stdout, "3 ") {
		t.Fatalf("#3 (other tag) must NOT be gold, got:\n%s", stdout)
	}
	if lineHasGold(stdout, "4 ") {
		t.Fatalf("#4 (no tag) must NOT be gold, got:\n%s", stdout)
	}
}

// TestHighlightTagCSVUnionOfThreeTagsWithGaps: --highlight-tag
// release, p0, urgent matches any of the three. Whitespace around
// CSV tokens is trimmed; empty tokens are quietly dropped.
func TestHighlightTagCSVUnionOfThreeTagsWithGaps(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "release"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-t", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "c", "-t", "p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "d"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1,2,3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Note the spaces and the trailing comma — splitTagCSV should
	// tokenize cleanly.
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot",
		"--highlight-tag", " release, p0 ,urgent,")
	if err != nil {
		t.Fatalf("graph --highlight-tag with whitespace+empty: %v", err)
	}
	for _, id := range []string{"1 ", "2 ", "3 "} {
		if !lineHasGold(stdout, id) {
			t.Fatalf("expected #%s gold for any of release/urgent/p0, got:\n%s", id, stdout)
		}
	}
}

// TestHighlightTagCSVMissingAllTagsNoSpotlight: when none of the
// listed tags match any task, the spotlight is empty (no error,
// just no spotlight). Empty-tag policy mirrors the single-tag
// "missing tag renders cleanly" semantics.
func TestHighlightTagCSVMissingAllTagsNoSpotlight(t *testing.T) {
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
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot",
		"--highlight-tag", "ghost,missing")
	if err != nil {
		t.Fatalf("graph --highlight-tag with all-missing CSV: %v", err)
	}
	if strings.Contains(stdout, "gold") {
		t.Fatalf("all-missing CSV should produce no gold, got:\n%s", stdout)
	}
}

// TestHighlightTagCSVPartialMatchStillSpotlights: when ONE listed
// tag matches at least one task, the spotlight fires for that tag
// while the missing tags are quietly skipped (no error).
func TestHighlightTagCSVPartialMatchStillSpotlights(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot",
		"--highlight-tag", "ghost,real,phantom")
	if err != nil {
		t.Fatalf("graph --highlight-tag partial CSV: %v", err)
	}
	if !lineHasGold(stdout, "1 ") {
		t.Fatalf("expected #1 (real) gold, got:\n%s", stdout)
	}
}

// TestDimTagCSVUnionMirror: same multi-tag CSV semantics flow
// through --dim-tag. Sister test of TestHighlightTagCSVUnionOfTwoTags
// for the inverse verb.
func TestDimTagCSVUnionMirror(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "scaffold"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-t", "wip"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "c"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot",
		"--dim-tag", "scaffold,wip")
	if err != nil {
		t.Fatalf("graph --dim-tag CSV: %v", err)
	}
	if !lineHasDim(stdout, "1 ") || !lineHasDim(stdout, "2 ") {
		t.Fatalf("expected #1 and #2 dim under CSV, got:\n%s", stdout)
	}
	if lineHasDim(stdout, "3 ") {
		t.Fatalf("#3 must NOT be dim, got:\n%s", stdout)
	}
}

// TestHighlightTagCSVSingleTagRegression: the existing single-tag
// form (--highlight-tag release) must keep working unchanged after
// the CSV extension. Regression guard against the mergeTagsIntoSet
// refactor.
func TestHighlightTagCSVSingleTagRegression(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "release"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot",
		"--highlight-tag", "release")
	if err != nil {
		t.Fatalf("graph single-tag regression: %v", err)
	}
	if !lineHasGold(stdout, "1 ") {
		t.Fatalf("single-tag must still gold #1, got:\n%s", stdout)
	}
}

// TestExportHighlightTagCSVMatchesGraph: the export verb honors
// CSV --highlight-tag the same way graph does. Same scaffolding.
func TestExportHighlightTagCSVMatchesGraph(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "release"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-t", "p0"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot",
		"--highlight-tag", "release,p0")
	if err != nil {
		t.Fatalf("export --graph-dot --highlight-tag CSV: %v", err)
	}
	if !lineHasGold(stdout, "1 ") || !lineHasGold(stdout, "2 ") {
		t.Fatalf("expected #1 and #2 gold via export CSV, got:\n%s", stdout)
	}
}

// TestGraphAndExportHighlightTagCSVInLockstep: graph and export
// produce byte-identical output for the same multi-tag CSV input.
func TestGraphAndExportHighlightTagCSVInLockstep(t *testing.T) {
	mkDir := func() string {
		d := t.TempDir()
		if _, _, err := runCmd(t, d, "add", "a", "-t", "release"); err != nil {
			t.Fatalf("add a: %v", err)
		}
		if _, _, err := runCmd(t, d, "add", "b", "-t", "p0"); err != nil {
			t.Fatalf("add b: %v", err)
		}
		if _, _, err := runCmd(t, d, "add", "c"); err != nil {
			t.Fatalf("add c: %v", err)
		}
		if _, _, err := runCmd(t, d, "depend", "3", "--on", "1,2"); err != nil {
			t.Fatalf("depend: %v", err)
		}
		return d
	}
	graphDir := mkDir()
	exportDir := mkDir()
	graphOut, _, err := runCmd(t, graphDir, "graph", "--format", "dot",
		"--highlight-tag", "release,p0")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	exportOut, _, err := runCmd(t, exportDir, "export", "--graph-dot",
		"--highlight-tag", "release,p0")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if graphOut != exportOut {
		t.Fatalf("graph and export must produce byte-identical multi-tag CSV --highlight-tag output\nGRAPH:\n%s\nEXPORT:\n%s", graphOut, exportOut)
	}
}

// TestSplitTagCSVHelper: direct unit test of the tokenizer for the
// edge cases the integration tests imply.
func TestSplitTagCSVHelper(t *testing.T) {
	cases := []struct {
		raw  string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"release", []string{"release"}},
		{"release,p0", []string{"release", "p0"}},
		{" release , p0 ", []string{"release", "p0"}},
		{"release,,p0", []string{"release", "p0"}},
		{"release,", []string{"release"}},
		{",release", []string{"release"}},
		{",,", nil},
	}
	for _, c := range cases {
		got := splitTagCSV(c.raw)
		if len(got) != len(c.want) {
			t.Errorf("splitTagCSV(%q): len mismatch: got %v, want %v", c.raw, got, c.want)
			continue
		}
		for i, g := range got {
			if g != c.want[i] {
				t.Errorf("splitTagCSV(%q)[%d]: got %q, want %q", c.raw, i, g, c.want[i])
			}
		}
	}
}
