package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGraphJSONIncludePriorityAddsField: --include-priority adds a
// per-node "priority" field carrying the canonical string form. Every
// real task gets a non-empty value; the field maps to the same
// shape `tsk show --json` uses so jq selectors work identically
// across the two surfaces.
func TestGraphJSONIncludePriorityAddsField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-p", "urgent"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-p", "high"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--include-priority")
	if err != nil {
		t.Fatalf("graph --json --include-priority: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(doc.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(doc.Nodes))
	}
	got := map[int]string{}
	for _, n := range doc.Nodes {
		got[n.ID] = n.Priority
	}
	if got[1] != "urgent" {
		t.Errorf("node #1 priority: want urgent, got %q", got[1])
	}
	if got[2] != "high" {
		t.Errorf("node #2 priority: want high, got %q", got[2])
	}
}

// TestGraphJSONIncludePriorityDefaultIsAbsent: without
// --include-priority the historical envelope shape is preserved —
// no "priority" key on any node. Critical for backward compat with
// existing snapshot fixtures / jq pipelines.
func TestGraphJSONIncludePriorityDefaultIsAbsent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-p", "urgent"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-p", "high"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json")
	if err != nil {
		t.Fatalf("graph --json (default): %v", err)
	}
	if strings.Contains(stdout, "\"priority\"") {
		t.Fatalf("default JSON envelope should NOT contain priority field, got:\n%s", stdout)
	}
	// Also confirm priority field is absent at the struct level
	// (omitempty drops it when the raw value is empty).
	var raw []map[string]any
	body := struct {
		Nodes []map[string]any `json:"nodes"`
	}{}
	if err := json.Unmarshal([]byte(stdout), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for i, n := range body.Nodes {
		if _, hasP := n["priority"]; hasP {
			t.Errorf("node %d unexpectedly carries priority key: %v", i, n)
		}
	}
	_ = raw
}

// TestGraphJSONIncludePriorityRequiresJSON: --include-priority
// without --json is rejected at the usage layer (exit 2). The flag
// is exclusively a modifier for the JSON envelope path; combining
// it with the ASCII or DOT renderer would have no defined meaning.
func TestGraphJSONIncludePriorityRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--include-priority")
	if err == nil {
		t.Fatal("expected error for --include-priority without --json")
	}
	if !strings.Contains(err.Error(), "--include-priority") {
		t.Fatalf("expected error to mention --include-priority, got %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2 (usage error), got %v", err)
	}
}

// TestGraphJSONIncludePriorityWithCompact: --include-priority
// composes with --compact-json — both modifiers are independent
// and their combined effect is a single-line JSON record carrying
// the priority field on every node.
func TestGraphJSONIncludePriorityWithCompact(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-p", "low"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-p", "medium"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--compact-json", "--include-priority")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	body := strings.TrimRight(stdout, "\n")
	if strings.Contains(body, "\n") {
		t.Fatalf("compact output should be single-line, got:\n%s", stdout)
	}
	if !strings.Contains(body, "\"priority\":\"low\"") {
		t.Errorf("expected priority:\"low\" in compact body, got: %s", body)
	}
	if !strings.Contains(body, "\"priority\":\"medium\"") {
		t.Errorf("expected priority:\"medium\" in compact body, got: %s", body)
	}
}

// TestGraphJSONIncludePriorityUpstreamOf: the flag works
// identically for --upstream-of (the inverse subgraph direction).
// Same envelope shape, same priority field, no surprises.
func TestGraphJSONIncludePriorityUpstreamOf(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root", "-p", "urgent"); err != nil {
		t.Fatalf("add root: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "downstream", "-p", "medium"); err != nil {
		t.Fatalf("add downstream: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json", "--include-priority")
	if err != nil {
		t.Fatalf("upstream-of: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if doc.Direction != "upstream-of" {
		t.Errorf("direction: want upstream-of, got %q", doc.Direction)
	}
	got := map[int]string{}
	for _, n := range doc.Nodes {
		got[n.ID] = n.Priority
	}
	if got[1] != "urgent" || got[2] != "medium" {
		t.Errorf("priorities want #1=urgent #2=medium, got #1=%q #2=%q", got[1], got[2])
	}
}

// TestGraphJSONIncludePriorityWithAppend: the flag composes with
// --append, so each JSONL record carries priority when the user
// opted in. Useful for snapshot-history pipelines that want to
// track priority drift over time.
func TestGraphJSONIncludePriorityWithAppend(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-p", "high"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-p", "low"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	outPath := dir + "/snap.jsonl"
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--include-priority", "--output", outPath, "--append"); err != nil {
		t.Fatalf("append: %v", err)
	}
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--include-priority", "--output", outPath, "--append"); err != nil {
		t.Fatalf("append second: %v", err)
	}
	body := readFile(t, outPath)
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 jsonl records, got %d:\n%s", len(lines), body)
	}
	for i, line := range lines {
		if !strings.Contains(line, "\"priority\":\"high\"") || !strings.Contains(line, "\"priority\":\"low\"") {
			t.Errorf("line %d: missing priority fields:\n%s", i, line)
		}
	}
}

// TestGraphJSONIncludePriorityDanglingNode: a dangling-edge node
// (referenced but not present in the store) keeps an empty
// priority that omitempty drops from the envelope. The historical
// "(missing)" title placeholder is unchanged.
func TestGraphJSONIncludePriorityDanglingNode(t *testing.T) {
	// We can't easily produce a true dangling edge through the CLI
	// since `tsk depend --on N` validates N exists. But we can
	// directly test the emit logic by constructing a store with a
	// dangling edge in-memory and verifying the emit output.
	//
	// Skip the integration form and inline a unit-level
	// assertion: when ByID returns nil for an id, the resulting
	// node carries Priority=="" regardless of --include-priority.
	// This is exercised by the dangling-node branch in the loop;
	// the omitempty tag suppresses the empty value from the JSON
	// shape.
	//
	// We verify by constructing the shape directly: a subgraphNode
	// with Priority=="" marshals without a "priority" key.
	n := subgraphNode{ID: 99, Title: "(missing)", Done: false}
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "priority") {
		t.Errorf("dangling node should omit priority field, got: %s", b)
	}
}

// TestGraphJSONIncludePriorityAllFourValues: every model.Priority
// value renders to its canonical string. Catches drift in the
// model.Priority.String() side or a future encoder change.
func TestGraphJSONIncludePriorityAllFourValues(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"low", "medium", "high", "urgent"} {
		if _, _, err := runCmd(t, dir, "add", p, "-p", p); err != nil {
			t.Fatalf("add %s: %v", p, err)
		}
	}
	// Chain them: 1 <- 2 <- 3 <- 4 so all four are reachable from 4.
	for i := 2; i <= 4; i++ {
		from, to := i, i-1
		if _, _, err := runCmd(t, dir, "depend", itoa(from), "--on", itoa(to)); err != nil {
			t.Fatalf("depend %d->%d: %v", from, to, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "4", "--json", "--include-priority")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Nodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(doc.Nodes))
	}
	want := map[int]string{1: "low", 2: "medium", 3: "high", 4: "urgent"}
	for _, n := range doc.Nodes {
		if want[n.ID] != n.Priority {
			t.Errorf("node #%d priority: want %q, got %q", n.ID, want[n.ID], n.Priority)
		}
	}
}
