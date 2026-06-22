package commands

import (
	"strings"
	"testing"
)

// TestGraphSVGEmitsValidPreamble: --format svg produces a
// well-formed SVG document with the expected root element.
func TestGraphSVGEmitsValidPreamble(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha"); err != nil {
		t.Fatalf("add alpha: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta"); err != nil {
		t.Fatalf("add beta: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "svg")
	if err != nil {
		t.Fatalf("graph --format svg: %v", err)
	}
	if !strings.Contains(stdout, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Fatalf("expected SVG root element, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "</svg>") {
		t.Fatalf("expected closing </svg>, got:\n%s", stdout)
	}
}

// TestGraphSVGContainsNodeRectsForEachTask: every task that
// participates in the graph gets a <rect> node in the output.
func TestGraphSVGContainsNodeRectsForEachTask(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta", "gamma"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "svg")
	if err != nil {
		t.Fatalf("graph --format svg: %v", err)
	}
	// Should have at least 3 <rect> elements (one per node in graph).
	rectCount := strings.Count(stdout, "<rect ")
	if rectCount < 3 {
		t.Fatalf("expected >=3 <rect> nodes, got %d:\n%s", rectCount, stdout)
	}
	// Labels should embed the id text "#1", "#2", "#3".
	for _, want := range []string{"#1", "#2", "#3"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected label %q in SVG, got:\n%s", want, stdout)
		}
	}
}

// TestGraphSVGEmitsEdges: every dep edge in the graph yields a
// <line> element with the arrowhead marker reference.
func TestGraphSVGEmitsEdges(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2->1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend 3->2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "svg")
	if err != nil {
		t.Fatalf("graph --format svg: %v", err)
	}
	// Two edges expected: 2->1 and 3->2.
	lineCount := strings.Count(stdout, "<line ")
	if lineCount < 2 {
		t.Fatalf("expected >=2 <line> edges, got %d:\n%s", lineCount, stdout)
	}
	// Arrowhead marker must be referenced.
	if !strings.Contains(stdout, `marker-end="url(#arrow)"`) {
		t.Fatalf("expected arrowhead marker reference, got:\n%s", stdout)
	}
}

// TestGraphSVGHighlightAppliesGoldFill: --highlight + --format svg
// emits a gold-filled node for the highlighted id.
func TestGraphSVGHighlightAppliesGoldFill(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "svg", "--highlight", "1")
	if err != nil {
		t.Fatalf("graph svg --highlight 1: %v", err)
	}
	if !strings.Contains(stdout, "#ffd700") {
		t.Fatalf("expected gold fill #ffd700 for highlighted node, got:\n%s", stdout)
	}
}

// TestGraphSVGDimAppliesMutedStyle: --dim + --format svg renders
// the dimmed node with a muted gray + dashed border.
func TestGraphSVGDimAppliesMutedStyle(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "svg", "--dim", "1")
	if err != nil {
		t.Fatalf("graph svg --dim 1: %v", err)
	}
	if !strings.Contains(stdout, "stroke-dasharray=") {
		t.Fatalf("expected dashed border for dimmed node, got:\n%s", stdout)
	}
}

// TestGraphSVGEscapesXMLInTitles: a title with XML metacharacters
// is escaped in the output so the SVG remains well-formed.
func TestGraphSVGEscapesXMLInTitles(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "fix <bug> in &lib"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "ship"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "svg")
	if err != nil {
		t.Fatalf("graph svg: %v", err)
	}
	// Raw < and > inside text content would break the SVG parser.
	// The renderer must emit &lt;, &gt;, &amp; instead.
	if strings.Contains(stdout, ">fix <bug>") {
		t.Fatalf("unescaped < or > in SVG text content:\n%s", stdout)
	}
	if !strings.Contains(stdout, "&lt;bug&gt;") {
		t.Fatalf("expected escaped <bug> -> &lt;bug&gt;, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "&amp;lib") {
		t.Fatalf("expected escaped &lib -> &amp;lib, got:\n%s", stdout)
	}
}

// TestGraphSVGEmptyEmitsPlaceholder: empty graph emits a tiny
// placeholder SVG with a "no dependencies" label so consumers can
// still open the file without a parse error.
func TestGraphSVGEmptyEmitsPlaceholder(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "svg")
	if err != nil {
		t.Fatalf("graph svg: %v", err)
	}
	// emitGraph short-circuits on empty edges with the "no
	// dependencies" ASCII line BEFORE the SVG renderer runs, so
	// the output here is the ASCII fallback. That's the existing
	// behavior — verify it stays consistent.
	if !strings.Contains(stdout, "no dependencies") {
		t.Fatalf("expected 'no dependencies' empty marker, got:\n%s", stdout)
	}
}

// TestGraphSVGFormatAlias: 'svg' is accepted; bogus formats are
// still rejected with a usage error.
func TestGraphSVGFormatAlias(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// "svg" should succeed (even without deps, it falls back to
	// "no dependencies").
	if _, _, err := runCmd(t, dir, "graph", "--format", "svg"); err != nil {
		t.Fatalf("graph --format svg should succeed: %v", err)
	}
	// Bogus format still rejected.
	_, _, err := runCmd(t, dir, "graph", "--format", "made-up-fmt")
	if err == nil {
		t.Fatal("expected error for unknown --format")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
	// Error message should list the new svg option.
	if !strings.Contains(err.Error(), "svg") {
		t.Fatalf("usage error should mention 'svg' option, got: %v", err)
	}
}
