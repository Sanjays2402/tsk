package commands

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestGraphJSONIncludeStartedAddsField: --include-started adds a
// per-node "started" field carrying an RFC3339 timestamp. Tasks
// that have never been started leave the field absent.
func TestGraphJSONIncludeStartedAddsField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "in-flight"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "not-started"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--include-started")
	if err != nil {
		t.Fatalf("graph --json --include-started: %v", err)
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
		got[n.ID] = n.Started
	}
	if got[1] == "" {
		t.Errorf("node #1 (started) should have started timestamp, got empty")
	}
	if _, err := time.Parse(time.RFC3339, got[1]); err != nil {
		t.Errorf("node #1 started should be valid RFC3339: %v (got %q)", err, got[1])
	}
	if got[2] != "" {
		t.Errorf("node #2 (not started) started: want empty, got %q", got[2])
	}
	// Confirm at byte level too: #2 should NOT have a "started" key.
	body := struct {
		Nodes []map[string]any `json:"nodes"`
	}{}
	if err := json.Unmarshal([]byte(stdout), &body); err != nil {
		t.Fatalf("parse raw: %v", err)
	}
	for _, n := range body.Nodes {
		idF, _ := n["id"].(float64)
		id := int(idF)
		_, hasStarted := n["started"]
		if id == 2 && hasStarted {
			t.Errorf("node #2 (not started) should NOT carry started key, got: %v", n)
		}
		if id == 1 && !hasStarted {
			t.Errorf("node #1 (started) should carry started key, got: %v", n)
		}
	}
}

// TestGraphJSONIncludeStartedDefaultIsAbsent: without
// --include-started the historical envelope shape is preserved.
func TestGraphJSONIncludeStartedDefaultIsAbsent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json")
	if err != nil {
		t.Fatalf("graph --json (default): %v", err)
	}
	if strings.Contains(stdout, "\"started\"") {
		t.Fatalf("default JSON envelope should NOT contain started field, got:\n%s", stdout)
	}
}

// TestGraphJSONIncludeStartedRequiresJSON: --include-started
// without --json is rejected at the usage layer (exit 2).
func TestGraphJSONIncludeStartedRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--include-started")
	if err == nil {
		t.Fatal("expected error for --include-started without --json")
	}
	if !strings.Contains(err.Error(), "--include-started") {
		t.Fatalf("expected error to mention --include-started, got %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestGraphJSONIncludeStartedComposesWithAllOtherIncludes: all
// FIVE opt-ins stack — the envelope gains priority, tags, due,
// completed, and started together. Demonstrates the modular
// opt-in design holds at full saturation.
func TestGraphJSONIncludeStartedComposesWithAllOtherIncludes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "in-flight", "-p", "urgent", "-t", "ship", "-d", "2026-08-15"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "shipped-prereq", "-p", "low", "-t", "later", "-d", "2026-12-01"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	// #1 depends on #2 (so #1 is the reachable root pointing at #2's
	// already-done state). #2 ships first, then #1 starts.
	if _, _, err := runCmd(t, dir, "depend", "1", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--include-priority", "--include-tags", "--include-due", "--include-completed", "--include-started")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	gotPrios := map[int]string{}
	gotTags := map[int][]string{}
	gotDues := map[int]string{}
	gotCompl := map[int]string{}
	gotStarted := map[int]string{}
	for _, n := range doc.Nodes {
		gotPrios[n.ID] = n.Priority
		if n.Tags != nil {
			gotTags[n.ID] = *n.Tags
		}
		gotDues[n.ID] = n.Due
		gotCompl[n.ID] = n.Completed
		gotStarted[n.ID] = n.Started
	}
	if gotPrios[1] != "urgent" {
		t.Errorf("node #1 priority: want urgent, got %q", gotPrios[1])
	}
	if gotStarted[1] == "" {
		t.Errorf("node #1 (started) should carry started ts, got empty")
	}
	if gotCompl[1] != "" {
		t.Errorf("node #1 (in-flight, not done) completed: want empty, got %q", gotCompl[1])
	}
	if gotCompl[2] == "" {
		t.Errorf("node #2 (done) should carry completed ts, got empty")
	}
	if gotStarted[2] != "" {
		t.Errorf("node #2 (done; started never set) started: want empty, got %q", gotStarted[2])
	}
}

// TestGraphJSONIncludeStartedCompactJSON: composes with
// --compact-json — single-line record with the started field
// inline.
func TestGraphJSONIncludeStartedCompactJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "in-flight"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--compact-json", "--include-started")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	body := strings.TrimRight(stdout, "\n")
	if strings.Contains(body, "\n") {
		t.Fatalf("compact output should be single-line, got:\n%s", stdout)
	}
	if !strings.Contains(body, "\"started\":\"") {
		t.Errorf("expected started key inline in compact body, got: %s", body)
	}
}

// TestGraphJSONIncludeStartedDanglingNodeOmits: dangling-edge
// nodes have no task to read started from. The field is absent
// regardless of the flag.
func TestGraphJSONIncludeStartedDanglingNodeOmits(t *testing.T) {
	n := subgraphNode{ID: 99, Title: "(missing)", Done: false}
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "started") {
		t.Errorf("dangling node should omit started field, got: %s", b)
	}
	realN := subgraphNode{ID: 1, Title: "real", Done: false, Started: "2026-06-23T09:42:00Z"}
	b2, err := json.Marshal(realN)
	if err != nil {
		t.Fatalf("marshal real: %v", err)
	}
	if !strings.Contains(string(b2), "\"started\":\"2026-06-23T09:42:00Z\"") {
		t.Errorf("real node with started should serialize the field, got: %s", b2)
	}
}

// TestGraphJSONIncludeStartedClearsOnDone: when a task transitions
// from started -> done, the started field clears. The graph
// envelope should reflect: completed is set, started is absent.
// Verifies the model.Task done semantics propagate to the envelope.
func TestGraphJSONIncludeStartedClearsOnDone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work-then-done"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--include-started", "--include-completed")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Nodes))
	}
	n := doc.Nodes[0]
	if n.Started != "" {
		t.Errorf("done task started should clear, got %q", n.Started)
	}
	if n.Completed == "" {
		t.Errorf("done task should carry completed, got empty")
	}
}

// TestGraphJSONIncludeStartedUpstreamOf: works identically for
// --upstream-of (the inverse subgraph direction).
func TestGraphJSONIncludeStartedUpstreamOf(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add root: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "downstream"); err != nil {
		t.Fatalf("add downstream: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json", "--include-started")
	if err != nil {
		t.Fatalf("graph --upstream-of --include-started: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if doc.Direction != "upstream-of" {
		t.Errorf("direction: want upstream-of, got %q", doc.Direction)
	}
	started := map[int]string{}
	for _, n := range doc.Nodes {
		started[n.ID] = n.Started
	}
	if started[1] == "" {
		t.Errorf("node #1 (started) should carry started ts, got empty")
	}
	if started[2] != "" {
		t.Errorf("node #2 (not started) started: want empty, got %q", started[2])
	}
}

// TestGraphJSONIncludeStartedAppendModeStreamingShape: composes
// with --append — the JSONL stream carries the started field on
// every applicable record.
func TestGraphJSONIncludeStartedAppendModeStreamingShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "in-flight"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	outPath := dir + "/history.jsonl"
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--append", "--include-started"); err != nil {
		t.Fatalf("graph append: %v", err)
	}
	body := readFile(t, outPath)
	line := strings.TrimRight(body, "\n")
	if strings.Contains(line, "\n") {
		t.Fatalf("append output should be single-line, got:\n%s", body)
	}
	if !strings.Contains(line, "\"started\":\"") {
		t.Errorf("expected started key inline in compact append record, got: %s", line)
	}
}
