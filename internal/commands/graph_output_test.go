package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGraphOutputWritesSVGFile: --output writes the SVG bytes to the
// named file when extension and format agree. The file on disk
// should be byte-identical to what stdout would have produced.
func TestGraphOutputWritesSVGFile(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	outPath := filepath.Join(dir, "deps.svg")
	stdout, _, err := runCmd(t, dir, "graph", "--format", "svg", "--output", outPath)
	if err != nil {
		t.Fatalf("graph --output: %v", err)
	}
	if !strings.Contains(stdout, "wrote ") || !strings.Contains(stdout, "deps.svg") {
		t.Fatalf("expected wrote-to-file confirmation, got: %s", stdout)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(body), `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Fatalf("expected SVG root in file, got:\n%s", string(body))
	}
	if !strings.Contains(string(body), "</svg>") {
		t.Fatalf("expected closing </svg> in file, got:\n%s", string(body))
	}
}

// TestGraphOutputWritesDotFile: same as SVG path but for DOT format.
// The .dot extension is accepted; the file contains GraphViz DOT
// syntax.
func TestGraphOutputWritesDotFile(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	outPath := filepath.Join(dir, "deps.dot")
	if _, _, err := runCmd(t, dir, "graph", "--format", "dot", "--output", outPath); err != nil {
		t.Fatalf("graph --output dot: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.HasPrefix(string(body), "digraph tsk {") {
		t.Fatalf("expected DOT header in file, got:\n%s", string(body))
	}
}

// TestGraphOutputAcceptsGVExtension: .gv is the alternate GraphViz
// extension and must be accepted alongside .dot.
func TestGraphOutputAcceptsGVExtension(t *testing.T) {
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
	outPath := filepath.Join(dir, "deps.gv")
	if _, _, err := runCmd(t, dir, "graph", "--format", "dot", "--output", outPath); err != nil {
		t.Fatalf("graph --output gv: %v", err)
	}
	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("expected .gv file written: %v", err)
	}
}

// TestGraphOutputAsciiAcceptsTxtAndExtensionless: ASCII format is
// the most permissive — both .txt and no-extension paths are valid.
func TestGraphOutputAsciiAcceptsTxtAndExtensionless(t *testing.T) {
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
	// .txt accepted
	txtPath := filepath.Join(dir, "graph.txt")
	if _, _, err := runCmd(t, dir, "graph", "--format", "ascii", "--output", txtPath); err != nil {
		t.Fatalf("graph --output txt: %v", err)
	}
	if _, err := os.Stat(txtPath); err != nil {
		t.Fatalf("expected .txt file written: %v", err)
	}
	// extensionless accepted (common for text dumps)
	bareDir := t.TempDir()
	if _, _, err := runCmd(t, bareDir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, bareDir, "add", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, bareDir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	barePath := filepath.Join(bareDir, "summary")
	if _, _, err := runCmd(t, bareDir, "graph", "--format", "ascii", "--output", barePath); err != nil {
		t.Fatalf("graph --output (no ext): %v", err)
	}
	if _, err := os.Stat(barePath); err != nil {
		t.Fatalf("expected extensionless file written: %v", err)
	}
}

// TestGraphOutputRejectsMismatchedExtension: --format svg with a
// --output ending in .dot is the silent-footgun case this feature
// exists to prevent. Should surface a clear usage error.
func TestGraphOutputRejectsMismatchedExtension(t *testing.T) {
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
	outPath := filepath.Join(dir, "deps.dot")
	_, _, err := runCmd(t, dir, "graph", "--format", "svg", "--output", outPath)
	if err == nil {
		t.Fatal("expected error for --format svg with .dot extension, got nil")
	}
	if !strings.Contains(err.Error(), "expects --output ending in .svg") {
		t.Fatalf("expected extension-mismatch error, got: %v", err)
	}
	// File should NOT have been created — atomic write contract.
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Fatalf("expected NO file on extension mismatch, but %s exists", outPath)
	}
}

// TestGraphOutputRejectsDotFormatWithSvgExtension: the inverse
// footgun — DOT bytes shouldn't land under a .svg name.
func TestGraphOutputRejectsDotFormatWithSvgExtension(t *testing.T) {
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
	outPath := filepath.Join(dir, "deps.svg")
	_, _, err := runCmd(t, dir, "graph", "--format", "dot", "--output", outPath)
	if err == nil {
		t.Fatal("expected error for --format dot with .svg extension, got nil")
	}
	if !strings.Contains(err.Error(), "expects --output ending in .dot or .gv") {
		t.Fatalf("expected extension-mismatch error, got: %v", err)
	}
}

// TestGraphOutputCaseInsensitiveExtension: a .SVG path is the same
// as .svg for validation purposes (filesystems don't all care about
// case, and users shouldn't either at the flag layer).
func TestGraphOutputCaseInsensitiveExtension(t *testing.T) {
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
	outPath := filepath.Join(dir, "deps.SVG")
	if _, _, err := runCmd(t, dir, "graph", "--format", "svg", "--output", outPath); err != nil {
		t.Fatalf("graph --output .SVG: %v", err)
	}
}

// TestGraphOutputRejectsJSONWithBadExtension: --json --output
// requires a .json extension. A path like graph.svg is rejected at
// exit 2 before any bytes hit disk so a typo doesn't silently
// deposit JSON content under the wrong filename.
//
// Sister of validateGraphOutputExtension's format/extension
// matrix: every format keyword has its own canonical extension(s);
// --json is now part of that family rather than the prior outright
// mutex with --output.
func TestGraphOutputRejectsJSONWithBadExtension(t *testing.T) {
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
	outPath := filepath.Join(dir, "graph.svg")
	_, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--output", outPath)
	if err == nil {
		t.Fatal("expected error for --json --output with non-.json path, got nil")
	}
	if !strings.Contains(err.Error(), "--json --output expects path ending in .json") {
		t.Fatalf("expected json-extension error, got: %v", err)
	}
	// Confirm no file landed on disk (atomic-write contract:
	// validation runs BEFORE render, so a typo never deposits
	// bytes under the wrong extension).
	if _, err := os.Stat(outPath); err == nil {
		t.Fatalf("did not expect %s to exist after extension rejection", outPath)
	}
}

// TestGraphOutputRespectsReachable: --output composes with
// --reachable so a `tsk graph --reachable 7 --format svg --output
// sub.svg` produces the subgraph SVG in one shot. Regression guard
// against the dispatch path forgetting to honor subgraph filters
// when writing to a file.
func TestGraphOutputRespectsReachable(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// Chain 2->1 and an unrelated 4->3 task.
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	outPath := filepath.Join(dir, "sub.svg")
	if _, _, err := runCmd(t, dir, "graph", "--format", "svg", "--reachable", "2", "--output", outPath); err != nil {
		t.Fatalf("graph --reachable --output: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read sub.svg: %v", err)
	}
	// #2 and #1 should be in the subgraph; #3 and #4 should not.
	// We check via the "#N</tspan>" pattern that wraps node labels —
	// avoids false positives on hex color codes like "#333" used in
	// the SVG style attributes.
	if !strings.Contains(string(body), "#1</tspan>") || !strings.Contains(string(body), "#2</tspan>") {
		t.Fatalf("expected #1 and #2 node labels in subgraph, got:\n%s", string(body))
	}
	if strings.Contains(string(body), "#3</tspan>") || strings.Contains(string(body), "#4</tspan>") {
		t.Fatalf("expected subgraph to exclude #3 and #4 node labels, got:\n%s", string(body))
	}
}

// TestValidateGraphOutputExtensionUnitMatrix exercises the helper
// directly so each format/extension combination is covered
// independently of the cobra flag plumbing.
func TestValidateGraphOutputExtensionUnitMatrix(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		format  string
		wantErr bool
	}{
		{"ascii-txt-ok", "graph.txt", "ascii", false},
		{"ascii-no-ext-ok", "summary", "ascii", false},
		{"ascii-svg-rejected", "graph.svg", "ascii", true},
		{"dot-dot-ok", "deps.dot", "dot", false},
		{"dot-gv-ok", "deps.gv", "dot", false},
		{"dot-svg-rejected", "deps.svg", "dot", true},
		{"dot-no-ext-rejected", "deps", "dot", true},
		{"svg-svg-ok", "deps.svg", "svg", false},
		{"svg-uppercase-svg-ok", "deps.SVG", "svg", false},
		{"svg-dot-rejected", "deps.dot", "svg", true},
		{"svg-no-ext-rejected", "deps", "svg", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateGraphOutputExtension(c.path, c.format)
			if c.wantErr && err == nil {
				t.Fatalf("expected error for path=%s format=%s", c.path, c.format)
			}
			if !c.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
