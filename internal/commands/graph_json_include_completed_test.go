package commands

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestGraphJSONIncludeCompletedAddsField: --include-completed adds
// a per-node "completed" field carrying an RFC3339 timestamp. Open
// tasks leave the field absent (omitempty drops the empty string).
func TestGraphJSONIncludeCompletedAddsField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "shipped"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "still-open"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--include-completed")
	if err != nil {
		t.Fatalf("graph --json --include-completed: %v", err)
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
		got[n.ID] = n.Completed
	}
	if got[1] == "" {
		t.Errorf("node #1 (done) should have completed timestamp, got empty")
	}
	if _, err := time.Parse(time.RFC3339, got[1]); err != nil {
		t.Errorf("node #1 completed should be valid RFC3339: %v (got %q)", err, got[1])
	}
	if got[2] != "" {
		t.Errorf("node #2 (open) completed: want empty, got %q", got[2])
	}
	// Confirm at byte level too: #2 should NOT have a "completed" key.
	body := struct {
		Nodes []map[string]any `json:"nodes"`
	}{}
	if err := json.Unmarshal([]byte(stdout), &body); err != nil {
		t.Fatalf("parse raw: %v", err)
	}
	for _, n := range body.Nodes {
		idF, _ := n["id"].(float64)
		id := int(idF)
		_, hasCompleted := n["completed"]
		if id == 2 && hasCompleted {
			t.Errorf("node #2 (open) should NOT carry completed key, got: %v", n)
		}
		if id == 1 && !hasCompleted {
			t.Errorf("node #1 (done) should carry completed key, got: %v", n)
		}
	}
}

// TestGraphJSONIncludeCompletedDefaultIsAbsent: without
// --include-completed the historical envelope shape is preserved —
// no "completed" key on any node. Critical for backward compat
// with existing snapshot fixtures.
func TestGraphJSONIncludeCompletedDefaultIsAbsent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json")
	if err != nil {
		t.Fatalf("graph --json (default): %v", err)
	}
	if strings.Contains(stdout, "\"completed\"") {
		t.Fatalf("default JSON envelope should NOT contain completed field, got:\n%s", stdout)
	}
}

// TestGraphJSONIncludeCompletedRequiresJSON: --include-completed
// without --json is rejected at the usage layer (exit 2). The flag
// is exclusively a modifier for the JSON envelope path.
func TestGraphJSONIncludeCompletedRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--include-completed")
	if err == nil {
		t.Fatal("expected error for --include-completed without --json")
	}
	if !strings.Contains(err.Error(), "--include-completed") {
		t.Fatalf("expected error to mention --include-completed, got %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestGraphJSONIncludeCompletedComposesWithOtherIncludes: all
// four opt-ins stack — the envelope gains priority, tags, due,
// and completed on every applicable node with no interference.
func TestGraphJSONIncludeCompletedComposesWithOtherIncludes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "shipped", "-p", "urgent", "-t", "ship", "-d", "2026-08-15"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "open", "-p", "low", "-t", "later", "-d", "2026-12-01"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--include-priority", "--include-tags", "--include-due", "--include-completed")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	prios := map[int]string{}
	tags := map[int][]string{}
	dues := map[int]string{}
	completes := map[int]string{}
	for _, n := range doc.Nodes {
		prios[n.ID] = n.Priority
		if n.Tags != nil {
			tags[n.ID] = *n.Tags
		}
		dues[n.ID] = n.Due
		completes[n.ID] = n.Completed
	}
	if prios[1] != "urgent" {
		t.Errorf("node #1 priority: want urgent, got %q", prios[1])
	}
	if len(tags[1]) != 1 || tags[1][0] != "ship" {
		t.Errorf("node #1 tags: want [ship], got %v", tags[1])
	}
	if dues[1] != "2026-08-15" {
		t.Errorf("node #1 due: want 2026-08-15, got %q", dues[1])
	}
	if completes[1] == "" {
		t.Errorf("node #1 (done) should carry completed, got empty")
	}
	if completes[2] != "" {
		t.Errorf("node #2 (open) completed: want empty, got %q", completes[2])
	}
}

// TestGraphJSONIncludeCompletedCompactJSON: composes with
// --compact-json — single-line record with the completed field
// inline.
func TestGraphJSONIncludeCompletedCompactJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ship"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--compact-json", "--include-completed")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	body := strings.TrimRight(stdout, "\n")
	if strings.Contains(body, "\n") {
		t.Fatalf("compact output should be single-line, got:\n%s", stdout)
	}
	if !strings.Contains(body, "\"completed\":\"") {
		t.Errorf("expected completed key inline in compact body, got: %s", body)
	}
}

// TestGraphJSONIncludeCompletedUpstreamOf: works identically for
// --upstream-of (the inverse subgraph direction).
func TestGraphJSONIncludeCompletedUpstreamOf(t *testing.T) {
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
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json", "--include-completed")
	if err != nil {
		t.Fatalf("graph --upstream-of --include-completed: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if doc.Direction != "upstream-of" {
		t.Errorf("direction: want upstream-of, got %q", doc.Direction)
	}
	if len(doc.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(doc.Nodes))
	}
	completes := map[int]string{}
	for _, n := range doc.Nodes {
		completes[n.ID] = n.Completed
	}
	if completes[1] == "" {
		t.Errorf("node #1 (done) should carry completed, got empty")
	}
	if completes[2] != "" {
		t.Errorf("node #2 (open) completed: want empty, got %q", completes[2])
	}
}

// TestGraphJSONIncludeCompletedDanglingNodeOmits: dangling-edge
// nodes have no task to read completed from. The field is absent
// regardless of the flag.
func TestGraphJSONIncludeCompletedDanglingNodeOmits(t *testing.T) {
	n := subgraphNode{ID: 99, Title: "(missing)", Done: false}
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "completed") {
		t.Errorf("dangling node should omit completed field, got: %s", b)
	}
	realN := subgraphNode{ID: 1, Title: "real", Done: true, Completed: "2026-06-23T09:42:00Z"}
	b2, err := json.Marshal(realN)
	if err != nil {
		t.Fatalf("marshal real: %v", err)
	}
	if !strings.Contains(string(b2), "\"completed\":\"2026-06-23T09:42:00Z\"") {
		t.Errorf("real node with completed should serialize the field, got: %s", b2)
	}
}

// TestGraphJSONIncludeCompletedRFC3339Format: every completed
// timestamp serialized in the envelope must be RFC3339-parseable.
// Stability contract: jq pipelines that compare timestamps need a
// reliable format.
func TestGraphJSONIncludeCompletedRFC3339Format(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 3; i++ {
		title := "done-task"
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
		if _, _, err := runCmd(t, dir, "done", "1"); err != nil && i == 1 {
			t.Fatalf("done first: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "3", "--json", "--include-completed")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, n := range doc.Nodes {
		if n.Completed == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339, n.Completed); err != nil {
			t.Errorf("node #%d completed %q is not RFC3339-parseable: %v", n.ID, n.Completed, err)
		}
	}
}
