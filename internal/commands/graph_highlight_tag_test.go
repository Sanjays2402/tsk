package commands

import (
	"strings"
	"testing"
)

// TestGraphHighlightTagSpotlightsTaggedNodes: --highlight-tag X
// gold-styles every node carrying tag X. Other nodes remain plain
// (or in their done/blocked state).
func TestGraphHighlightTagSpotlightsTaggedNodes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha", "-t", "release"); err != nil {
		t.Fatalf("add alpha: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta"); err != nil {
		t.Fatalf("add beta: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "gamma", "-t", "release"); err != nil {
		t.Fatalf("add gamma: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "delta"); err != nil {
		t.Fatalf("add delta: %v", err)
	}
	// Make a graph so nodes get rendered: 4 depends on 1,2,3.
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1,2,3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight-tag", "release")
	if err != nil {
		t.Fatalf("graph --highlight-tag release: %v", err)
	}
	if !lineHasGold(stdout, "1 ") {
		t.Fatalf("expected #1 (release) gold, got:\n%s", stdout)
	}
	if !lineHasGold(stdout, "3 ") {
		t.Fatalf("expected #3 (release) gold, got:\n%s", stdout)
	}
	if lineHasGold(stdout, "2 ") {
		t.Fatalf("#2 (no tag) must NOT be gold, got:\n%s", stdout)
	}
	if lineHasGold(stdout, "4 ") {
		t.Fatalf("#4 (no tag) must NOT be gold, got:\n%s", stdout)
	}
}

// TestGraphHighlightTagCaseInsensitive: tag matching is case-
// insensitive (same semantics as tsk ls --tag). "RELEASE" matches
// tasks tagged "release".
func TestGraphHighlightTagCaseInsensitive(t *testing.T) {
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
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight-tag", "RELEASE")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if !lineHasGold(stdout, "1 ") {
		t.Fatalf("expected #1 gold under uppercase match, got:\n%s", stdout)
	}
}

// TestGraphHighlightTagUnionWithHighlightIDs: --highlight 42 plus
// --highlight-tag release renders BOTH the id and every tagged
// node in gold (the union policy).
func TestGraphHighlightTagUnionWithHighlightIDs(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "release"); err != nil {
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
	// #1 has the release tag; #3 picked via --highlight. Union: both gold.
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot",
		"--highlight", "3", "--highlight-tag", "release")
	if err != nil {
		t.Fatalf("graph union: %v", err)
	}
	if !lineHasGold(stdout, "1 ") {
		t.Fatalf("#1 (tagged) should be gold:\n%s", stdout)
	}
	if !lineHasGold(stdout, "3 ") {
		t.Fatalf("#3 (--highlight) should be gold:\n%s", stdout)
	}
	if lineHasGold(stdout, "2 ") {
		t.Fatalf("#2 must NOT be gold:\n%s", stdout)
	}
	if lineHasGold(stdout, "4 ") {
		t.Fatalf("#4 must NOT be gold:\n%s", stdout)
	}
}

// TestGraphHighlightTagMissingTagRendersCleanly: a tag no task
// carries is NOT an error — the graph renders without any
// spotlight. The user notices the missing highlight and adjusts.
func TestGraphHighlightTagMissingTagRendersCleanly(t *testing.T) {
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
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight-tag", "nonexistent")
	if err != nil {
		t.Fatalf("graph --highlight-tag nonexistent: %v", err)
	}
	if strings.Contains(stdout, "gold") {
		t.Fatalf("missing tag should produce no gold styles, got:\n%s", stdout)
	}
}

// TestGraphHighlightTagOnlyAsciiRejects: --highlight-tag is DOT-
// only (just like --highlight). Combining with ascii is a usage
// error so callers don't silently miss the spotlight.
func TestGraphHighlightTagOnlyAsciiRejects(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--highlight-tag", "x")
	if err == nil {
		t.Fatal("expected error combining --highlight-tag with ascii format")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestExportGraphDotHighlightTag: the central data-out verb honors
// --highlight-tag the same way `tsk graph --format dot` does.
// Same scaffolding, same scoring, same spotlight.
func TestExportGraphDotHighlightTag(t *testing.T) {
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
	stdout, _, err := runCmd(t, dir, "export", "--graph-dot", "--highlight-tag", "release")
	if err != nil {
		t.Fatalf("export --graph-dot --highlight-tag: %v", err)
	}
	if !lineHasGold(stdout, "1 ") {
		t.Fatalf("expected #1 gold via export, got:\n%s", stdout)
	}
}

// TestGraphAndExportHighlightTagInLockstep: graph and export
// produce byte-identical output for the same --highlight-tag input.
// Regression against the two surfaces ever drifting on the new
// flag.
func TestGraphAndExportHighlightTagInLockstep(t *testing.T) {
	mkDir := func() string {
		d := t.TempDir()
		if _, _, err := runCmd(t, d, "add", "a", "-t", "release"); err != nil {
			t.Fatalf("add a: %v", err)
		}
		if _, _, err := runCmd(t, d, "add", "b"); err != nil {
			t.Fatalf("add b: %v", err)
		}
		if _, _, err := runCmd(t, d, "add", "c", "-t", "release"); err != nil {
			t.Fatalf("add c: %v", err)
		}
		if _, _, err := runCmd(t, d, "depend", "3", "--on", "1,2"); err != nil {
			t.Fatalf("depend: %v", err)
		}
		return d
	}
	graphDir := mkDir()
	exportDir := mkDir()
	graphOut, _, err := runCmd(t, graphDir, "graph", "--format", "dot", "--highlight-tag", "release")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	exportOut, _, err := runCmd(t, exportDir, "export", "--graph-dot", "--highlight-tag", "release")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if graphOut != exportOut {
		t.Fatalf("graph and export must produce byte-identical --highlight-tag output\nGRAPH:\n%s\nEXPORT:\n%s", graphOut, exportOut)
	}
}

// TestGraphHighlightTagOverridesRedBlocked: a blocked task that is
// also matched by the highlight-tag must render gold, not red.
// Same override policy as the id-based highlight.
func TestGraphHighlightTagOverridesRedBlocked(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "blocked-thing", "-t", "release"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot", "--highlight-tag", "release")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "2 ") {
			continue
		}
		if !strings.Contains(line, "[label=") {
			continue
		}
		if !strings.Contains(line, "gold") {
			t.Fatalf("#2 should be gold (highlight-tag wins over blocked-red):\n%s", line)
		}
		if strings.Contains(line, `color="red"`) {
			t.Fatalf("#2 should NOT carry red (highlight-tag overrides):\n%s", line)
		}
	}
}
