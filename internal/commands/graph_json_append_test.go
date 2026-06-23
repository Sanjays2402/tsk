package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGraphJSONAppendCreatesFreshFile: the first --append call
// to a non-existent file creates it with the .jsonl convention
// (single compact record, one trailing newline). Same UX as
// shell `>>` to a missing file — no pre-create needed.
func TestGraphJSONAppendCreatesFreshFile(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "snap.jsonl")
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--append")
	if err != nil {
		t.Fatalf("graph --json --output --append: %v", err)
	}
	if !strings.Contains(stdout, "appended ") || !strings.Contains(stdout, "snap.jsonl") || !strings.Contains(stdout, "format=jsonl") {
		t.Fatalf("expected appended-to-file confirmation, got: %s", stdout)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Exactly one line (terminated by trailing newline from json.Encoder).
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line in fresh JSONL file, got %d:\n%s", len(lines), body)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(lines[0]), &doc); err != nil {
		t.Fatalf("parse first line: %v", err)
	}
	if doc.RootID != 1 {
		t.Errorf("expected root_id=1, got %d", doc.RootID)
	}
}

// TestGraphJSONAppendBuildsHistoryAcrossCalls: three sequential
// --append calls produce three records in the file, in call
// order. This is the snapshot-history use case — over time the
// file builds up a chronological record of impact-analysis
// queries.
func TestGraphJSONAppendBuildsHistoryAcrossCalls(t *testing.T) {
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
	outPath := filepath.Join(dir, "history.jsonl")
	// Three consecutive snapshot calls, each rooted at a
	// different task.
	for _, root := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "graph", "--upstream-of", root, "--json", "--output", outPath, "--append"); err != nil {
			t.Fatalf("append #%s: %v", root, err)
		}
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines after 3 appends, got %d:\n%s", len(lines), body)
	}
	// Parse each line and confirm root order matches call order.
	wantRoots := []int{1, 2, 3}
	for i, line := range lines {
		var doc subgraphDoc
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			t.Fatalf("parse line %d: %v\nline: %s", i, err, line)
		}
		if doc.RootID != wantRoots[i] {
			t.Errorf("line %d: expected root_id=%d, got %d", i, wantRoots[i], doc.RootID)
		}
		if doc.Direction != "upstream-of" {
			t.Errorf("line %d: expected direction=upstream-of, got %q", i, doc.Direction)
		}
	}
}

// TestGraphJSONAppendImpliesCompact: even without explicit
// --compact-json, --append produces single-line records. The
// implicit upgrade keeps the on-disk shape valid JSONL.
func TestGraphJSONAppendImpliesCompact(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "auto-compact.jsonl")
	// Note: --append without --compact-json
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--append"); err != nil {
		t.Fatalf("append: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Same single-line guarantee as the compact-explicit case.
	trimmed := strings.TrimRight(string(body), "\n")
	if strings.Contains(trimmed, "\n") {
		t.Fatalf("expected single-line on-disk record (compact implied), got:\n%s", body)
	}
}

// TestGraphJSONAppendRequiresJSON: --append without --json is a
// usage error. Catches typos where the user forgot --json and
// would otherwise get a confusing "this only works on the JSON
// path" surprise.
func TestGraphJSONAppendRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "x.jsonl")
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--output", outPath, "--append")
	if err == nil {
		t.Fatal("expected error for --append without --json")
	}
	if !strings.Contains(err.Error(), "--append only applies to --json") {
		t.Fatalf("expected append-requires-json error, got: %v", err)
	}
}

// TestGraphJSONAppendRequiresOutput: --append without --output
// is a usage error. The append target file is required —
// without it the command would have nowhere to append.
func TestGraphJSONAppendRequiresOutput(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--append")
	if err == nil {
		t.Fatal("expected error for --append without --output")
	}
	if !strings.Contains(err.Error(), "--append requires --output") {
		t.Fatalf("expected append-requires-output error, got: %v", err)
	}
}

// TestGraphJSONAppendAcceptsBothExtensions: .json and .jsonl are
// both accepted in append mode. .jsonl is the canonical streaming-
// JSON extension, but .json is allowed for users who already
// named their target file that way.
func TestGraphJSONAppendAcceptsBothExtensions(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, ext := range []string{".json", ".jsonl", ".JSON", ".JSONL"} {
		out := filepath.Join(dir, "append-ext"+ext)
		if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", out, "--append"); err != nil {
			t.Fatalf("append %s: %v", ext, err)
		}
		if _, err := os.Stat(out); err != nil {
			t.Errorf("expected %s to exist: %v", out, err)
		}
	}
}

// TestGraphJSONAppendRejectsBadExtension: paths with non-json
// extensions are still rejected — the validation matrix grows
// to include .jsonl, but everything else (svg, dot, txt, bare)
// still fails with a clear error.
func TestGraphJSONAppendRejectsBadExtension(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, bad := range []string{"foo.svg", "foo.dot", "foo.txt", "foo"} {
		out := filepath.Join(dir, bad)
		_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", out, "--append")
		if err == nil {
			t.Errorf("expected rejection for --append --output=%q, got nil", bad)
			continue
		}
		if !strings.Contains(err.Error(), ".json or .jsonl") {
			t.Errorf("expected append-extension error for %q, got: %v", bad, err)
		}
	}
}

// TestGraphJSONAppendValidatesExtensionMatrix: direct check of
// the helper's matrix for the appendMode=true branch. Mirrors
// TestValidateGraphOutputJSONExtensionMatrix's table-test style
// for the existing non-append path.
func TestValidateGraphOutputJSONExtensionAppendMatrix(t *testing.T) {
	cases := []struct {
		path    string
		wantErr bool
	}{
		{"foo.json", false},
		{"foo.jsonl", false},
		{"FOO.JSON", false},
		{"FOO.JSONL", false},
		{"path/to/foo.json", false},
		{"path/to/foo.jsonl", false},
		{"foo.svg", true},
		{"foo.txt", true},
		{"foo.dot", true},
		{"foo", true},
	}
	for _, c := range cases {
		err := validateGraphOutputJSONExtension(c.path, true)
		if (err != nil) != c.wantErr {
			t.Errorf("path=%q (append=true): wantErr=%v got %v", c.path, c.wantErr, err)
		}
	}
}

// TestGraphJSONAppendComposesWithCompact: passing --compact-json
// explicitly with --append is redundant but harmless (the
// implied compact wins; the explicit doesn't conflict). No
// usage error, normal behavior.
func TestGraphJSONAppendComposesWithExplicitCompact(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "redundant.jsonl")
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--compact-json", "--output", outPath, "--append"); err != nil {
		t.Fatalf("append with explicit compact: %v", err)
	}
	body, _ := os.ReadFile(outPath)
	trimmed := strings.TrimRight(string(body), "\n")
	if strings.Contains(trimmed, "\n") {
		t.Errorf("expected single-line, got:\n%s", body)
	}
}

// TestGraphJSONAppendBufferAtomicOnRenderFailure: extension
// validation runs BEFORE OpenFile, so a typo'd path never even
// creates the file. Regression guard that the append path
// preserves the atomic-write contract that --output already
// follows.
func TestGraphJSONAppendAtomicOnExtensionMismatch(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "bad.svg")
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--append")
	if err == nil {
		t.Fatal("expected rejection, got nil")
	}
	if _, statErr := os.Stat(outPath); statErr == nil {
		t.Errorf("expected NO file created on extension rejection")
	}
}
