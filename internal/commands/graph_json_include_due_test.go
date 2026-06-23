package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGraphJSONIncludeDueAddsField: --include-due adds a per-node
// "due" field carrying the canonical YYYY-MM-DD string. Tasks
// with no due date leave the field absent (omitempty drops the
// empty string).
func TestGraphJSONIncludeDueAddsField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "due-soon", "-d", "2026-07-01"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "no-due"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--include-due")
	if err != nil {
		t.Fatalf("graph --json --include-due: %v", err)
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
		got[n.ID] = n.Due
	}
	if got[1] != "2026-07-01" {
		t.Errorf("node #1 due: want 2026-07-01, got %q", got[1])
	}
	// #2 has no due — the field should be absent (empty string
	// after parse because of omitempty + Go zero value).
	if got[2] != "" {
		t.Errorf("node #2 due: want empty (no due), got %q", got[2])
	}
	// Confirm at byte level too: #2 should NOT have a "due" key.
	body := struct {
		Nodes []map[string]any `json:"nodes"`
	}{}
	if err := json.Unmarshal([]byte(stdout), &body); err != nil {
		t.Fatalf("parse raw: %v", err)
	}
	for _, n := range body.Nodes {
		idF, _ := n["id"].(float64)
		id := int(idF)
		_, hasDue := n["due"]
		if id == 2 && hasDue {
			t.Errorf("node #2 (no due) should NOT carry due key, got: %v", n)
		}
		if id == 1 && !hasDue {
			t.Errorf("node #1 (has due) should carry due key, got: %v", n)
		}
	}
}

// TestGraphJSONIncludeDueDefaultIsAbsent: without --include-due
// the historical envelope shape is preserved — no "due" key on
// any node. Critical for backward compat with existing snapshot
// fixtures.
func TestGraphJSONIncludeDueDefaultIsAbsent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-d", "2026-07-01"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json")
	if err != nil {
		t.Fatalf("graph --json (default): %v", err)
	}
	if strings.Contains(stdout, "\"due\"") {
		t.Fatalf("default JSON envelope should NOT contain due field, got:\n%s", stdout)
	}
}

// TestGraphJSONIncludeDueRequiresJSON: --include-due without
// --json is rejected at the usage layer (exit 2). The flag is
// exclusively a modifier for the JSON envelope path.
func TestGraphJSONIncludeDueRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--include-due")
	if err == nil {
		t.Fatal("expected error for --include-due without --json")
	}
	if !strings.Contains(err.Error(), "--include-due") {
		t.Fatalf("expected error to mention --include-due, got %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestGraphJSONIncludeDueComposesWithOtherIncludes: all three
// opt-ins stack — the envelope gains priority, tags, and due on
// every applicable node with no interference. Verifies the
// modular flag design holds as the set grows.
func TestGraphJSONIncludeDueComposesWithOtherIncludes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-p", "urgent", "-t", "ship", "-d", "2026-08-15"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-p", "low", "-t", "later", "-d", "2026-12-01"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--include-priority", "--include-tags", "--include-due")
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
	for _, n := range doc.Nodes {
		prios[n.ID] = n.Priority
		if n.Tags != nil {
			tags[n.ID] = *n.Tags
		}
		dues[n.ID] = n.Due
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
	if prios[2] != "low" {
		t.Errorf("node #2 priority: want low, got %q", prios[2])
	}
	if len(tags[2]) != 1 || tags[2][0] != "later" {
		t.Errorf("node #2 tags: want [later], got %v", tags[2])
	}
	if dues[2] != "2026-12-01" {
		t.Errorf("node #2 due: want 2026-12-01, got %q", dues[2])
	}
}

// TestGraphJSONIncludeDueCompactJSON: --include-due composes with
// --compact-json — single-line record with the due field inline.
func TestGraphJSONIncludeDueCompactJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ship", "-d", "2026-07-04"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--compact-json", "--include-due")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	body := strings.TrimRight(stdout, "\n")
	if strings.Contains(body, "\n") {
		t.Fatalf("compact output should be single-line, got:\n%s", stdout)
	}
	if !strings.Contains(body, "\"due\":\"2026-07-04\"") {
		t.Errorf("expected due:\"2026-07-04\" inline in compact body, got: %s", body)
	}
}

// TestGraphJSONIncludeDueUpstreamOf: works identically for
// --upstream-of (the inverse subgraph direction). Same envelope,
// same field, no direction-specific behavior.
func TestGraphJSONIncludeDueUpstreamOf(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root", "-d", "2026-08-01"); err != nil {
		t.Fatalf("add root: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "downstream"); err != nil {
		t.Fatalf("add downstream: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json", "--include-due")
	if err != nil {
		t.Fatalf("graph --upstream-of --include-due: %v", err)
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
	dues := map[int]string{}
	for _, n := range doc.Nodes {
		dues[n.ID] = n.Due
	}
	if dues[1] != "2026-08-01" {
		t.Errorf("node #1 due: want 2026-08-01, got %q", dues[1])
	}
	if dues[2] != "" {
		t.Errorf("node #2 due: want empty (no due), got %q", dues[2])
	}
}

// TestGraphJSONIncludeDueDanglingNodeOmits: dangling-edge nodes
// have no task to read due from. The field is absent from the
// JSON regardless of the flag. Unit-level: a subgraphNode with
// Due=="" marshals without the key (omitempty drops it).
func TestGraphJSONIncludeDueDanglingNodeOmits(t *testing.T) {
	n := subgraphNode{ID: 99, Title: "(missing)", Done: false}
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "due") {
		t.Errorf("dangling node should omit due field, got: %s", b)
	}
	// A real node with Due set should serialize the field.
	realN := subgraphNode{ID: 1, Title: "real", Done: false, Due: "2026-07-01"}
	b2, err := json.Marshal(realN)
	if err != nil {
		t.Fatalf("marshal real: %v", err)
	}
	if !strings.Contains(string(b2), "\"due\":\"2026-07-01\"") {
		t.Errorf("real node with due should serialize the field, got: %s", b2)
	}
}

// TestGraphJSONIncludeDueAppendModeStreamingShape: --include-due
// composes with --append — the JSONL stream carries the field on
// every applicable record.
func TestGraphJSONIncludeDueAppendModeStreamingShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ship", "-d", "2026-09-01"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := dir + "/history.jsonl"
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--append", "--include-due"); err != nil {
		t.Fatalf("graph append: %v", err)
	}
	body := readFile(t, outPath)
	line := strings.TrimRight(body, "\n")
	if strings.Contains(line, "\n") {
		t.Fatalf("append output should be single-line, got:\n%s", body)
	}
	if !strings.Contains(line, "\"due\":\"2026-09-01\"") {
		t.Errorf("expected due:\"2026-09-01\" in compact append record, got: %s", line)
	}
}

// TestGraphJSONIncludeDueLexicographicJqFilter: the YYYY-MM-DD
// format makes string comparison equivalent to date comparison.
// Sanity-check the contract by parsing multiple nodes and
// verifying string comparison yields the right ordering.
func TestGraphJSONIncludeDueLexicographicJqFilter(t *testing.T) {
	dir := t.TempDir()
	dates := []string{"2026-12-01", "2026-07-04", "2026-03-15"}
	for i, d := range dates {
		title := "task-" + d
		_ = i
		if _, _, err := runCmd(t, dir, "add", title, "-d", d); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// Chain them so all three appear in one subgraph.
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "3", "--json", "--include-due")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	gotDues := map[int]string{}
	for _, n := range doc.Nodes {
		gotDues[n.ID] = n.Due
	}
	// Verify lexicographic comparison: "2026-03-15" < "2026-07-04"
	// < "2026-12-01" — the contract the include-due field promises
	// jq pipelines. Tasks were added in the order December, July,
	// March (ids 1, 2, 3), so the lexicographic relation is
	// #3 < #2 < #1.
	if !(gotDues[3] < gotDues[2]) {
		t.Errorf("lexicographic: expected %q < %q", gotDues[3], gotDues[2])
	}
	if !(gotDues[2] < gotDues[1]) {
		t.Errorf("lexicographic: expected %q < %q", gotDues[2], gotDues[1])
	}
}
