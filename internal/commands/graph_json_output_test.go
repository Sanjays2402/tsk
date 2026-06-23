package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGraphJSONOutputWritesReachableEnvelope: --reachable + --json
// + --output writes the subgraph envelope to the named .json file.
// The bytes on disk are the same as stdout would have produced —
// verified by parsing the JSON and checking the shape end-to-end.
func TestGraphJSONOutputWritesReachableEnvelope(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root-prereq", "ship-it"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// #2 depends on #1: --reachable 2 should pick up both.
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	outPath := filepath.Join(dir, "impact.json")
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--output", outPath)
	if err != nil {
		t.Fatalf("graph --json --output: %v", err)
	}
	if !strings.Contains(stdout, "wrote ") || !strings.Contains(stdout, "impact.json") || !strings.Contains(stdout, "format=json") {
		t.Fatalf("expected wrote-to-file confirmation with format=json, got: %s", stdout)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse JSON: %v\nbody:\n%s", err, body)
	}
	if doc.RootID != 2 {
		t.Errorf("expected root_id=2, got %d", doc.RootID)
	}
	if doc.Direction != "reachable" {
		t.Errorf("expected direction=reachable, got %q", doc.Direction)
	}
	// Reachable from #2 = {#2 (root), #1 (its prereq)}.
	if len(doc.Nodes) != 2 {
		t.Fatalf("expected 2 nodes (root + prereq), got %d: %+v", len(doc.Nodes), doc.Nodes)
	}
	if len(doc.Edges) != 1 {
		t.Fatalf("expected 1 edge (2 -> 1), got %d: %+v", len(doc.Edges), doc.Edges)
	}
}

// TestGraphJSONOutputWritesUpstreamOfEnvelope: same primitive but
// for the inverse subgraph direction (--upstream-of). The envelope's
// direction field reflects the actual flag the user passed so
// downstream consumers can branch on it.
func TestGraphJSONOutputWritesUpstreamOfEnvelope(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// b depends on a, c depends on b — upstream-of a = {a, b, c}.
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	outPath := filepath.Join(dir, "upstream.json")
	if _, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json", "--output", outPath); err != nil {
		t.Fatalf("graph --upstream-of --json --output: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if doc.Direction != "upstream-of" {
		t.Errorf("expected direction=upstream-of, got %q", doc.Direction)
	}
	if doc.RootID != 1 {
		t.Errorf("expected root_id=1, got %d", doc.RootID)
	}
	// Three nodes (root + two upstream dependents).
	if len(doc.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d: %+v", len(doc.Nodes), doc.Nodes)
	}
}

// TestGraphJSONOutputAtomicOnRenderFailure: validation runs BEFORE
// render, so a typo'd path (extension mismatch) never deposits any
// bytes. This is the same atomic-write contract every other tsk
// write path follows — regression guard so a future refactor
// doesn't accidentally write then validate.
func TestGraphJSONOutputAtomicOnExtensionMismatch(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Bare path (no extension) — should be rejected, no file
	// landed.
	outPath := filepath.Join(dir, "impact")
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath)
	if err == nil {
		t.Fatal("expected error for extensionless --json --output, got nil")
	}
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Fatalf("expected NO file at %s after extension rejection", outPath)
	}
}

// TestGraphJSONOutputRequiresSubgraphRoot: --json without
// --reachable/--upstream-of has no shape (the envelope is per-
// root), so --json --output without a root should still be
// rejected — the same check that gates --json alone. Sanity guard.
func TestGraphJSONOutputRequiresSubgraphRoot(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "impact.json")
	_, _, err := runCmd(t, dir, "graph", "--json", "--output", outPath)
	if err == nil {
		t.Fatal("expected error for --json without --reachable/--upstream-of")
	}
	if !strings.Contains(err.Error(), "--json only applies to --reachable or --upstream-of") {
		t.Fatalf("expected per-root requirement error, got: %v", err)
	}
}

// TestGraphJSONOutputCaseInsensitiveExtension: .JSON / .Json /
// .json all accepted. Filesystems on macOS are case-insensitive
// by default; treating extensions case-sensitively here would
// reject perfectly valid uppercase paths.
func TestGraphJSONOutputCaseInsensitiveExtension(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, ext := range []string{".json", ".JSON", ".Json"} {
		out := filepath.Join(dir, "case"+ext)
		if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", out); err != nil {
			t.Fatalf("graph %s: %v", ext, err)
		}
		if _, err := os.Stat(out); err != nil {
			t.Errorf("expected %s to exist: %v", out, err)
		}
	}
}

// TestGraphJSONOutputBytesMatchStdout: writing to file and writing
// to stdout produce IDENTICAL bytes (modulo any trailing newline
// from print convention). Critical for snapshot tests / CI gates
// that compare jq output to a saved fixture — the two paths must
// stay in sync.
func TestGraphJSONOutputBytesMatchStdout(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdoutOut, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json")
	if err != nil {
		t.Fatalf("graph stdout: %v", err)
	}
	outPath := filepath.Join(dir, "sub.json")
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--output", outPath); err != nil {
		t.Fatalf("graph --output: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// The stdout path goes through the same encoder so the bytes
	// should be a strict equality match.
	if string(body) != stdoutOut {
		t.Fatalf("stdout vs file mismatch:\nstdout:\n%s\nfile:\n%s", stdoutOut, body)
	}
}

// TestValidateGraphOutputJSONExtensionMatrix: spot-check the
// helper directly so future contributors see the matrix in one
// place. Mirrors validateGraphOutputExtension's matrix style.
func TestValidateGraphOutputJSONExtensionMatrix(t *testing.T) {
	cases := []struct {
		path    string
		wantErr bool
	}{
		{"foo.json", false},
		{"FOO.JSON", false},
		{"path/with/dirs/file.json", false},
		{"foo.svg", true},
		{"foo.txt", true},
		{"foo", true}, // bare paths rejected (no .ext at all)
		{"foo.dot", true},
	}
	for _, c := range cases {
		err := validateGraphOutputJSONExtension(c.path, false)
		if (err != nil) != c.wantErr {
			t.Errorf("path=%q: wantErr=%v got %v", c.path, c.wantErr, err)
		}
	}
}
