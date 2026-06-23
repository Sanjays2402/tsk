package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGraphJSONIncludeTagsAddsField: --include-tags adds a per-node
// "tags" field carrying the alphabetized tag array. Every real
// task's node gains the field; tasks with no tags get an empty
// array `[]` (not omitted, not null) so jq `.tags | length` works
// uniformly.
func TestGraphJSONIncludeTagsAddsField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work,urgent"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-t", "home"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--include-tags")
	if err != nil {
		t.Fatalf("graph --json --include-tags: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(doc.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(doc.Nodes))
	}
	got := map[int][]string{}
	for _, n := range doc.Nodes {
		if n.Tags == nil {
			t.Errorf("node #%d: Tags pointer should be non-nil when --include-tags is set", n.ID)
			continue
		}
		got[n.ID] = *n.Tags
	}
	// Tags should be alphabetized.
	if len(got[1]) != 2 || got[1][0] != "urgent" || got[1][1] != "work" {
		t.Errorf("node #1 tags: want [urgent work] (alphabetized), got %v", got[1])
	}
	if len(got[2]) != 1 || got[2][0] != "home" {
		t.Errorf("node #2 tags: want [home], got %v", got[2])
	}
}

// TestGraphJSONIncludeTagsEmptyTagArray: a task with no tags
// produces "tags": [] in the JSON envelope (not null, not omitted)
// when --include-tags is set. The empty array is the contract so
// downstream jq pipelines like `.tags | length` work without
// branching on type.
func TestGraphJSONIncludeTagsEmptyTagArray(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "tagless"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--include-tags")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	// Direct byte-level check: the JSON should contain "tags": []
	// (with the compact/indented form both showing the empty
	// array shape).
	if !strings.Contains(stdout, "\"tags\": []") && !strings.Contains(stdout, "\"tags\":[]") {
		t.Errorf("expected explicit empty array `[]` for tagless task, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "\"tags\":null") || strings.Contains(stdout, "\"tags\": null") {
		t.Errorf("tags should NOT serialize as null, got:\n%s", stdout)
	}
	// Also confirm via struct unmarshal.
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(doc.Nodes))
	}
	if doc.Nodes[0].Tags == nil {
		t.Error("Tags pointer should be non-nil (empty array, not absent)")
	} else if len(*doc.Nodes[0].Tags) != 0 {
		t.Errorf("Tags should be empty, got %v", *doc.Nodes[0].Tags)
	}
}

// TestGraphJSONIncludeTagsDefaultIsAbsent: without --include-tags
// the historical envelope shape is preserved — no "tags" key on
// any node. Critical for backward compat with existing snapshot
// fixtures / jq pipelines that don't expect the new field.
func TestGraphJSONIncludeTagsDefaultIsAbsent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json")
	if err != nil {
		t.Fatalf("graph --json (default): %v", err)
	}
	if strings.Contains(stdout, "\"tags\"") {
		t.Fatalf("default JSON envelope should NOT contain tags field, got:\n%s", stdout)
	}
	// Confirm via raw-map parse too.
	body := struct {
		Nodes []map[string]any `json:"nodes"`
	}{}
	if err := json.Unmarshal([]byte(stdout), &body); err != nil {
		t.Fatalf("parse: %v", err)
	}
	for i, n := range body.Nodes {
		if _, hasT := n["tags"]; hasT {
			t.Errorf("node %d unexpectedly carries tags key: %v", i, n)
		}
	}
}

// TestGraphJSONIncludeTagsRequiresJSON: --include-tags without
// --json is rejected at the usage layer (exit 2). The flag is
// exclusively a modifier for the JSON envelope path; combining it
// with the ASCII or DOT renderer would have no defined meaning.
func TestGraphJSONIncludeTagsRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--include-tags")
	if err == nil {
		t.Fatal("expected error for --include-tags without --json")
	}
	if !strings.Contains(err.Error(), "--include-tags") {
		t.Fatalf("expected error to mention --include-tags, got %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2 (usage error), got %v", err)
	}
}

// TestGraphJSONIncludeTagsComposesWithIncludePriority: both opt-in
// modifiers stack independently — the envelope gains both
// priority and tags on every real-task node, with no interference.
func TestGraphJSONIncludeTagsComposesWithIncludePriority(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-p", "urgent", "-t", "work"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-p", "low", "-t", "home,errand"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--include-priority", "--include-tags")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	prios := map[int]string{}
	tags := map[int][]string{}
	for _, n := range doc.Nodes {
		prios[n.ID] = n.Priority
		if n.Tags != nil {
			tags[n.ID] = *n.Tags
		}
	}
	if prios[1] != "urgent" {
		t.Errorf("node #1 priority: want urgent, got %q", prios[1])
	}
	if prios[2] != "low" {
		t.Errorf("node #2 priority: want low, got %q", prios[2])
	}
	if len(tags[1]) != 1 || tags[1][0] != "work" {
		t.Errorf("node #1 tags: want [work], got %v", tags[1])
	}
	if len(tags[2]) != 2 || tags[2][0] != "errand" || tags[2][1] != "home" {
		t.Errorf("node #2 tags: want [errand home] (alphabetized), got %v", tags[2])
	}
}

// TestGraphJSONIncludeTagsCompactJSON: --include-tags composes
// with --compact-json — single-line record with the tags field
// inline. Tests the JSONL-pipeline path that benefits most from
// the new field.
func TestGraphJSONIncludeTagsCompactJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--compact-json", "--include-tags")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	body := strings.TrimRight(stdout, "\n")
	if strings.Contains(body, "\n") {
		t.Fatalf("compact output should be single-line, got:\n%s", stdout)
	}
	if !strings.Contains(body, "\"tags\":[\"work\"]") {
		t.Errorf("expected tags:[\"work\"] inline in compact body, got: %s", body)
	}
}

// TestGraphJSONIncludeTagsUpstreamOf: the flag works identically
// for --upstream-of (the inverse subgraph direction). Same
// envelope shape, same tags field, no direction-specific behavior.
func TestGraphJSONIncludeTagsUpstreamOf(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root", "-t", "milestone"); err != nil {
		t.Fatalf("add root: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "downstream", "-t", "ship"); err != nil {
		t.Fatalf("add downstream: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json", "--include-tags")
	if err != nil {
		t.Fatalf("graph --upstream-of --include-tags: %v", err)
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
	tags := map[int][]string{}
	for _, n := range doc.Nodes {
		if n.Tags == nil {
			t.Errorf("node #%d: Tags pointer should be non-nil with --include-tags", n.ID)
			continue
		}
		tags[n.ID] = *n.Tags
	}
	if len(tags[1]) != 1 || tags[1][0] != "milestone" {
		t.Errorf("node #1 tags: want [milestone], got %v", tags[1])
	}
	if len(tags[2]) != 1 || tags[2][0] != "ship" {
		t.Errorf("node #2 tags: want [ship], got %v", tags[2])
	}
}

// TestGraphJSONIncludeTagsDanglingNodeOmits: a dangling-edge
// "(missing)" node has no task to read tags from. The field is
// omitted entirely from the JSON (omitempty drops the nil
// pointer), keeping the dangling-node shape minimal regardless of
// whether the flag is set. We can't easily produce a true dangling
// edge through the CLI (`tsk depend --on N` validates N exists),
// so test the emit logic directly: a subgraphNode with Tags==nil
// marshals without a "tags" key, even when other nodes have it.
func TestGraphJSONIncludeTagsDanglingNodeOmits(t *testing.T) {
	n := subgraphNode{ID: 99, Title: "(missing)", Done: false}
	b, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "tags") {
		t.Errorf("dangling node should omit tags field, got: %s", b)
	}
	// And verify that a real node with an empty Tags slice DOES
	// serialize the field (the contract we promise jq pipelines).
	tags := []string{}
	realN := subgraphNode{ID: 1, Title: "real", Done: false, Tags: &tags}
	b2, err := json.Marshal(realN)
	if err != nil {
		t.Fatalf("marshal real: %v", err)
	}
	if !strings.Contains(string(b2), "\"tags\":[]") {
		t.Errorf("real node with empty tags should serialize as []: got %s", b2)
	}
}

// TestGraphJSONIncludeTagsAppendModeStreamingShape: --include-tags
// composes with --append — the JSONL stream carries the tags
// field on every real-task node. Confirms the opt-in shape
// survives the implicit compact-upgrade that --append imposes.
func TestGraphJSONIncludeTagsAppendModeStreamingShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "release"); err != nil {
		t.Fatalf("add: %v", err)
	}
	outPath := dir + "/history.jsonl"
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--append", "--include-tags"); err != nil {
		t.Fatalf("graph append: %v", err)
	}
	// One-line file; line carries tags:["release"] for #1.
	body := readFile(t, outPath)
	line := strings.TrimRight(body, "\n")
	if strings.Contains(line, "\n") {
		t.Fatalf("append output should be single-line, got:\n%s", body)
	}
	if !strings.Contains(line, "\"tags\":[\"release\"]") {
		t.Errorf("expected tags:[\"release\"] in compact append record, got: %s", line)
	}
}
