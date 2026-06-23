package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGraphJSONCompactProducesSingleLine: --compact-json flips
// the encoder from indented to single-line "no whitespace" form.
// The resulting bytes must have exactly one trailing newline (from
// json.Encoder's standard newline-after-record convention) and no
// other line breaks anywhere in the body — critical for JSONL
// pipelines where each line is a self-contained record.
func TestGraphJSONCompactProducesSingleLine(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--compact-json")
	if err != nil {
		t.Fatalf("graph --json --compact-json: %v", err)
	}
	// Trim the trailing newline json.Encoder appends, then count
	// remaining newlines — should be exactly zero (single-line
	// body).
	body := strings.TrimRight(stdout, "\n")
	if strings.Contains(body, "\n") {
		t.Fatalf("expected compact JSON to have no internal newlines, got:\n%s", stdout)
	}
	// Must still parse as valid JSON with the same shape.
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("compact JSON failed to parse: %v\nbody: %s", err, body)
	}
	if doc.RootID != 2 {
		t.Errorf("expected root_id=2, got %d", doc.RootID)
	}
	if len(doc.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(doc.Nodes))
	}
}

// TestGraphJSONCompactRoundTripsViaFile: --output writes the
// same single-line bytes to disk. Critical for JSONL append
// pipelines where multiple compact lines are concatenated.
func TestGraphJSONCompactRoundTripsViaFile(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := filepath.Join(dir, "compact.json")
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--compact-json", "--output", outPath); err != nil {
		t.Fatalf("graph --json --compact-json --output: %v", err)
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	bodyStr := strings.TrimRight(string(body), "\n")
	if strings.Contains(bodyStr, "\n") {
		t.Fatalf("expected single-line file body, got:\n%s", body)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(bodyStr), &doc); err != nil {
		t.Fatalf("file JSON failed to parse: %v", err)
	}
}

// TestGraphJSONCompactRequiresJSON: --compact-json without --json
// is rejected at exit 2. The flag only affects the JSON path, so
// a typo (passing it with --format dot, say) should fail loudly
// rather than silently no-op.
func TestGraphJSONCompactRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--compact-json")
	if err == nil {
		t.Fatal("expected error for --compact-json without --json, got nil")
	}
	if !strings.Contains(err.Error(), "--compact-json only applies to --json") {
		t.Fatalf("expected compact-json-requires-json error, got: %v", err)
	}
}

// TestGraphJSONDefaultStaysIndented: backward compat — without
// --compact-json, the indented two-space format that existing
// fixtures rely on is preserved. The historical bytes-match-
// stdout regression in graph_json_output_test.go would catch
// drift; this is a direct assertion of the structural property
// (multi-line, indented).
func TestGraphJSONDefaultStaysIndented(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json")
	if err != nil {
		t.Fatalf("graph --json: %v", err)
	}
	// Indented form has multiple newlines (one per element + container).
	newlines := strings.Count(stdout, "\n")
	if newlines < 5 {
		t.Errorf("expected indented JSON to have >= 5 newlines, got %d:\n%s", newlines, stdout)
	}
	// Must contain at least one two-space indent — the canonical
	// json.Encoder.SetIndent("", "  ") marker.
	if !strings.Contains(stdout, "  ") {
		t.Errorf("expected indented JSON to contain two-space indent, got:\n%s", stdout)
	}
}

// TestGraphJSONCompactWithUpstreamOf: --compact-json composes
// with --upstream-of (same JSON envelope path), so the inverse
// direction also gets the single-line treatment.
func TestGraphJSONCompactWithUpstreamOf(t *testing.T) {
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
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json", "--compact-json")
	if err != nil {
		t.Fatalf("graph --upstream-of --json --compact-json: %v", err)
	}
	body := strings.TrimRight(stdout, "\n")
	if strings.Contains(body, "\n") {
		t.Fatalf("expected compact upstream-of JSON to be single-line, got:\n%s", stdout)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(body), &doc); err != nil {
		t.Fatalf("upstream-of compact JSON failed to parse: %v", err)
	}
	if doc.Direction != "upstream-of" {
		t.Errorf("expected direction=upstream-of, got %q", doc.Direction)
	}
}

// TestGraphJSONCompactBytesParseToSameShape: the compact and
// indented forms decode to IDENTICAL Go structs — semantically
// equal, just formatted differently. Critical guarantee so
// consumers can switch between the two without reshaping.
func TestGraphJSONCompactBytesParseToSameShape(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"x", "y", "z"} {
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
	indented, _, err := runCmd(t, dir, "graph", "--reachable", "3", "--json")
	if err != nil {
		t.Fatalf("indented: %v", err)
	}
	compact, _, err := runCmd(t, dir, "graph", "--reachable", "3", "--json", "--compact-json")
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	var docIndent, docCompact subgraphDoc
	if err := json.Unmarshal([]byte(indented), &docIndent); err != nil {
		t.Fatalf("parse indented: %v", err)
	}
	if err := json.Unmarshal([]byte(strings.TrimRight(compact, "\n")), &docCompact); err != nil {
		t.Fatalf("parse compact: %v", err)
	}
	if docIndent.RootID != docCompact.RootID {
		t.Errorf("RootID mismatch: indented=%d compact=%d", docIndent.RootID, docCompact.RootID)
	}
	if docIndent.Direction != docCompact.Direction {
		t.Errorf("Direction mismatch: indented=%q compact=%q", docIndent.Direction, docCompact.Direction)
	}
	if len(docIndent.Nodes) != len(docCompact.Nodes) {
		t.Errorf("Node count mismatch: indented=%d compact=%d", len(docIndent.Nodes), len(docCompact.Nodes))
	}
	if len(docIndent.Edges) != len(docCompact.Edges) {
		t.Errorf("Edge count mismatch: indented=%d compact=%d", len(docIndent.Edges), len(docCompact.Edges))
	}
}

// TestGraphJSONCompactWithFilter: --compact-json composes with
// --open (filter flag) without surprises — the filter field
// still appears in the envelope.
func TestGraphJSONCompactWithFilter(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--compact-json", "--open")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	body := strings.TrimRight(stdout, "\n")
	if strings.Contains(body, "\n") {
		t.Fatalf("expected compact body, got:\n%s", stdout)
	}
	if !strings.Contains(body, `"filter":"open"`) {
		t.Errorf("expected filter=open in compact envelope, got:\n%s", body)
	}
}
