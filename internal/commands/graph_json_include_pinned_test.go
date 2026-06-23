package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGraphJSONIncludePinnedSurfacesTrueOnPinnedTasks: a pinned
// task gets pinned=true in the JSON envelope when --include-pinned
// is set.
func TestGraphJSONIncludePinnedSurfacesTrueOnPinnedTasks(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "important"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pin", "1"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--include-pinned")
	if err != nil {
		t.Fatalf("graph --json --include-pinned: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(doc.Nodes) == 0 {
		t.Fatal("expected at least one node")
	}
	if doc.Nodes[0].Pinned == nil {
		t.Fatalf("expected pinned field present (flag is on), got nil:\n%s", stdout)
	}
	if !*doc.Nodes[0].Pinned {
		t.Errorf("expected pinned=true for pinned task, got %v", *doc.Nodes[0].Pinned)
	}
}

// TestGraphJSONIncludePinnedFalseOnUnpinnedTasks: an unpinned
// task gets pinned=false (present, false) when --include-pinned
// is set — distinguishable from "flag off" (field absent).
func TestGraphJSONIncludePinnedFalseOnUnpinnedTasks(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "plain"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--include-pinned")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Nodes[0].Pinned == nil {
		t.Fatalf("expected pinned field present (flag is on), got nil for unpinned task")
	}
	if *doc.Nodes[0].Pinned {
		t.Errorf("expected pinned=false for unpinned task, got %v", *doc.Nodes[0].Pinned)
	}
	// Belt and suspenders: the on-disk byte representation should
	// contain "pinned":false so jq pipelines can distinguish.
	if !strings.Contains(stdout, "\"pinned\": false") {
		t.Errorf("expected 'pinned: false' in raw JSON, got:\n%s", stdout)
	}
}

// TestGraphJSONIncludePinnedDefaultAbsent: without --include-pinned
// the envelope is byte-identical to the historical shape (no
// "pinned" field). Back-compat guard.
func TestGraphJSONIncludePinnedDefaultAbsent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "any"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pin", "1"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if strings.Contains(stdout, "pinned") {
		t.Fatalf("default envelope should NOT contain 'pinned' field, got:\n%s", stdout)
	}
}

// TestGraphJSONIncludePinnedRequiresJSON: --include-pinned without
// --json is a usage error.
func TestGraphJSONIncludePinnedRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--include-pinned")
	if err == nil {
		t.Fatal("expected error for --include-pinned without --json")
	}
	if !strings.Contains(err.Error(), "--include-pinned only applies to --json") {
		t.Fatalf("expected include-pinned-requires-json error, got: %v", err)
	}
}

// TestGraphJSONIncludePinnedActivatedByIncludeAll: --include-all
// flips on every opt-in including the new --include-pinned. Byte-
// level check that the envelope contains pinned when --include-all
// is set (without explicit --include-pinned).
func TestGraphJSONIncludePinnedActivatedByIncludeAll(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "all-fields"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pin", "1"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--include-all")
	if err != nil {
		t.Fatalf("graph --include-all: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Nodes[0].Pinned == nil || !*doc.Nodes[0].Pinned {
		t.Errorf("expected --include-all to flip on pinned (true for pinned task), got %v", doc.Nodes[0].Pinned)
	}
	// Confirm priority/tags/etc. also surface (saturation guard).
	if doc.Nodes[0].Priority == "" {
		t.Errorf("expected --include-all to flip on priority too, got empty string")
	}
}

// TestGraphJSONIncludePinnedComposesWithCompletedStarted: pinned
// composes with the other opt-ins. Multi-field activation produces
// the union of all requested fields.
func TestGraphJSONIncludePinnedComposesWithOtherOptIns(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "compose"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pin", "1"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pri", "1", "urgent"); err != nil {
		t.Fatalf("pri: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--include-pinned", "--include-priority")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Nodes[0].Pinned == nil || !*doc.Nodes[0].Pinned {
		t.Errorf("expected pinned=true, got %v", doc.Nodes[0].Pinned)
	}
	if doc.Nodes[0].Priority != "urgent" {
		t.Errorf("expected priority=urgent, got %q", doc.Nodes[0].Priority)
	}
}

// TestGraphJSONIncludePinnedDanglingNodeOmitted: a dangling-edge
// "(missing)" node has no task to read pinned from, so the field
// is absent even when --include-pinned is set.
func TestGraphJSONIncludePinnedDanglingNodeOmitted(t *testing.T) {
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
	// Remove task #1, leaving #2 with a dangling DependsOn edge.
	if _, _, err := runCmd(t, dir, "rm", "1"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--include-pinned")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Find the dangling node (Title=="(missing)"); its Pinned
	// should be nil (omitted).
	var foundMissing bool
	for _, n := range doc.Nodes {
		if n.Title == "(missing)" {
			foundMissing = true
			if n.Pinned != nil {
				t.Errorf("dangling node should omit pinned field, got %v", *n.Pinned)
			}
		}
	}
	if !foundMissing {
		t.Errorf("expected a dangling (missing) node, got nodes: %+v", doc.Nodes)
	}
}

// TestGraphJSONIncludePinnedCompactJSON: --compact-json with
// --include-pinned produces a single-line record carrying the
// pinned field. JSONL composition smoke test.
func TestGraphJSONIncludePinnedCompactJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "compact"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pin", "1"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--include-pinned", "--compact-json")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	trimmed := strings.TrimRight(stdout, "\n")
	if strings.Contains(trimmed, "\n") {
		t.Fatalf("expected single-line compact output, got:\n%s", stdout)
	}
	// Confirm pinned is present in the compact form.
	if !strings.Contains(trimmed, `"pinned":true`) {
		t.Errorf("expected compact form to contain 'pinned:true', got:\n%s", stdout)
	}
}

// TestGraphJSONIncludePinnedHelpMentionsFlag: --help text covers
// --include-pinned.
func TestGraphJSONIncludePinnedHelpMentionsFlag(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runCmd(t, dir, "graph", "--help")
	if err != nil {
		t.Fatalf("graph --help: %v", err)
	}
	if !strings.Contains(stdout, "--include-pinned") {
		t.Errorf("expected --include-pinned in help, got:\n%s", stdout)
	}
}

// TestGraphJSONIncludePinnedUpstreamOfDirection: works equally well
// in --upstream-of direction (the inverse subgraph extractor).
func TestGraphJSONIncludePinnedUpstreamOfDirection(t *testing.T) {
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
	if _, _, err := runCmd(t, dir, "pin", "2"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json", "--include-pinned")
	if err != nil {
		t.Fatalf("graph --upstream-of: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Find node 2 (the pinned dependent).
	var found bool
	for _, n := range doc.Nodes {
		if n.ID == 2 {
			found = true
			if n.Pinned == nil || !*n.Pinned {
				t.Errorf("expected node 2 to be pinned=true, got %v", n.Pinned)
			}
		}
	}
	if !found {
		t.Errorf("expected node 2 in upstream-of envelope")
	}
}
