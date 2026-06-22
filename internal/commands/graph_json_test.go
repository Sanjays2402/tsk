package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// graphSubgraphDocJSON mirrors subgraphDoc for test decoding without
// exporting the production type. Stable schema — if the field shape
// changes in production, this struct must change in lockstep.
type graphSubgraphDocJSON struct {
	RootID    int    `json:"root_id"`
	Direction string `json:"direction"`
	Nodes     []struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
		Done  bool   `json:"done"`
	} `json:"nodes"`
	Edges []struct {
		From int `json:"from"`
		To   int `json:"to"`
	} `json:"edges"`
	Filter string `json:"filter,omitempty"`
}

// TestGraphReachableJSONEnvelope: --reachable + --json emits the
// stable JSON envelope with the root, every transitive prereq, and
// every in-chain edge.
func TestGraphReachableJSONEnvelope(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root", "mid", "top"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// chain: #3 -> #2 -> #1 (top depends on mid depends on root)
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2->1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend 3->2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "3", "--json")
	if err != nil {
		t.Fatalf("graph --reachable 3 --json: %v", err)
	}
	var doc graphSubgraphDocJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json unmarshal: %v\nout:\n%s", err, stdout)
	}
	if doc.RootID != 3 {
		t.Errorf("root_id: want 3, got %d", doc.RootID)
	}
	if doc.Direction != "reachable" {
		t.Errorf("direction: want reachable, got %q", doc.Direction)
	}
	// Nodes: all three (root + every transitive prereq).
	if len(doc.Nodes) != 3 {
		t.Fatalf("nodes: want 3, got %d:\n%s", len(doc.Nodes), stdout)
	}
	// Sorted ascending by id.
	for i := 0; i < 3; i++ {
		if doc.Nodes[i].ID != i+1 {
			t.Errorf("nodes[%d].id = %d; want %d", i, doc.Nodes[i].ID, i+1)
		}
	}
	// Edges: #3->#2 and #2->#1, sorted by (from, to).
	if len(doc.Edges) != 2 {
		t.Fatalf("edges: want 2, got %d", len(doc.Edges))
	}
	// edges sorted by from asc → #2->#1 first, then #3->#2
	if doc.Edges[0].From != 2 || doc.Edges[0].To != 1 {
		t.Errorf("edges[0]: want 2->1, got %d->%d", doc.Edges[0].From, doc.Edges[0].To)
	}
	if doc.Edges[1].From != 3 || doc.Edges[1].To != 2 {
		t.Errorf("edges[1]: want 3->2, got %d->%d", doc.Edges[1].From, doc.Edges[1].To)
	}
	if doc.Filter != "" {
		t.Errorf("filter: want empty (no --open), got %q", doc.Filter)
	}
}

// TestGraphUpstreamOfJSONEnvelope: --upstream-of + --json reports
// the inverse direction. Same envelope shape, different direction
// label and different nodes/edges (transitive dependents of root).
func TestGraphUpstreamOfJSONEnvelope(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root", "mid", "top"} {
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
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json")
	if err != nil {
		t.Fatalf("graph --upstream-of 1 --json: %v", err)
	}
	var doc graphSubgraphDocJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json unmarshal: %v\nout:\n%s", err, stdout)
	}
	if doc.RootID != 1 {
		t.Errorf("root_id: want 1, got %d", doc.RootID)
	}
	if doc.Direction != "upstream-of" {
		t.Errorf("direction: want upstream-of, got %q", doc.Direction)
	}
	// All three nodes should appear — #1 plus every dependent.
	if len(doc.Nodes) != 3 {
		t.Fatalf("nodes: want 3, got %d", len(doc.Nodes))
	}
}

// TestGraphSubgraphJSONEmptyRootHasNoEdgesButOneNode: when a task
// has no dependents and the user asks --upstream-of with --json,
// the envelope still includes the root node in nodes[] so the
// consumer always has a non-null answer to "did this id exist?".
func TestGraphSubgraphJSONEmptyRootHasNoEdgesButOneNode(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json")
	if err != nil {
		t.Fatalf("upstream-of lonely --json: %v", err)
	}
	var doc graphSubgraphDocJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json unmarshal: %v\nout:\n%s", err, stdout)
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("nodes: want 1 (the root itself), got %d:\n%s", len(doc.Nodes), stdout)
	}
	if doc.Nodes[0].ID != 1 || doc.Nodes[0].Title != "lonely" {
		t.Errorf("node[0]: want {1 lonely}, got {%d %q}", doc.Nodes[0].ID, doc.Nodes[0].Title)
	}
	if len(doc.Edges) != 0 {
		t.Errorf("edges: want 0 for empty subgraph, got %d", len(doc.Edges))
	}
	// edges must be [] not null — verified by checking JSON output literal.
	if !strings.Contains(stdout, `"edges": []`) {
		t.Errorf("expected edges: [] (empty array, not null), got:\n%s", stdout)
	}
}

// TestGraphSubgraphJSONIncludesOpenFilter: when --open is passed,
// the JSON envelope includes filter: "open" so consumers can tell
// the preview filtered out done-task noise.
func TestGraphSubgraphJSONIncludesOpenFilter(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root", "mid"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--open", "--json")
	if err != nil {
		t.Fatalf("--open --json: %v", err)
	}
	var doc graphSubgraphDocJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json unmarshal: %v\nout:\n%s", err, stdout)
	}
	if doc.Filter != "open" {
		t.Errorf("filter: want \"open\", got %q", doc.Filter)
	}
}

// TestGraphJSONRejectedWithoutRoot: --json with neither --reachable
// nor --upstream-of is rejected at the flag layer (usage error,
// exit 2). The JSON shape is a per-root envelope; without a root
// there's no useful structure to emit.
func TestGraphJSONRejectedWithoutRoot(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--json")
	if err == nil {
		t.Fatal("expected --json without --reachable/--upstream-of to be rejected")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage), got %v", err)
	}
}

// TestGraphJSONDeterministicNodeOrder: a diamond shape with three
// transitively-dependent tasks should emit nodes in ascending-id
// order regardless of the order edges were created. Determinism
// matters for jq pipelines that key off array indices.
func TestGraphJSONDeterministicNodeOrder(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root", "deploy", "ship", "release"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// Add deps in REVERSE order to make sure the JSON output
	// doesn't reflect insertion order.
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json")
	if err != nil {
		t.Fatalf("upstream-of diamond --json: %v", err)
	}
	var doc graphSubgraphDocJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json unmarshal: %v\nout:\n%s", err, stdout)
	}
	for i := 0; i < len(doc.Nodes)-1; i++ {
		if doc.Nodes[i].ID >= doc.Nodes[i+1].ID {
			t.Errorf("nodes not sorted asc by id: %v", doc.Nodes)
		}
	}
	for i := 0; i < len(doc.Edges)-1; i++ {
		a := doc.Edges[i]
		b := doc.Edges[i+1]
		if a.From > b.From || (a.From == b.From && a.To > b.To) {
			t.Errorf("edges not sorted: %v", doc.Edges)
		}
	}
}

// TestGraphJSONReachableExcludesOtherChains: --reachable + --json
// only includes the subgraph reachable from the root — unrelated
// chains are NOT included in nodes/edges (no impact-analysis
// pollution from unrelated subgraphs).
func TestGraphJSONReachableExcludesOtherChains(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// Two disconnected chains: 2->1 and 4->3.
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json")
	if err != nil {
		t.Fatalf("--reachable 2 --json: %v", err)
	}
	var doc graphSubgraphDocJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json unmarshal: %v\nout:\n%s", err, stdout)
	}
	// Only #1 and #2 should appear.
	if len(doc.Nodes) != 2 {
		t.Fatalf("nodes: want 2 (only chain {1,2}), got %d:\n%s", len(doc.Nodes), stdout)
	}
	for _, n := range doc.Nodes {
		if n.ID == 3 || n.ID == 4 {
			t.Errorf("unrelated chain node #%d leaked into reachable JSON", n.ID)
		}
	}
}

// TestGraphJSONDoneFlagPreserved: a done task in the subgraph
// surfaces with done=true so consumers can branch on completion
// state without re-querying tsk.
func TestGraphJSONDoneFlagPreserved(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"root", "dep"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json")
	if err != nil {
		t.Fatalf("reachable --json: %v", err)
	}
	var doc graphSubgraphDocJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("json unmarshal: %v\nout:\n%s", err, stdout)
	}
	var found bool
	for _, n := range doc.Nodes {
		if n.ID == 1 {
			found = true
			if !n.Done {
				t.Errorf("#1 should be done=true in JSON, got %+v", n)
			}
		}
	}
	if !found {
		t.Errorf("#1 missing from nodes")
	}
}

// TestGraphJSONRespectsReachableUpstreamMutex: --reachable +
// --upstream-of together is still rejected, even with --json on.
// Regression: --json shouldn't slip past the mutex check.
func TestGraphJSONRespectsReachableUpstreamMutex(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--upstream-of", "1", "--json")
	if err == nil {
		t.Fatal("expected mutex rejection")
	}
}

// TestGraphSubgraphJSONDoesNotMutate: invoking the JSON path must
// not write anything to disk — verify by byte-for-byte comparison
// of the .tsk.md before and after the invocation.
func TestGraphSubgraphJSONDoesNotMutate(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	before := readFile(t, dir+"/.tsk.md")
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json"); err != nil {
		t.Fatalf("reachable --json: %v", err)
	}
	after := readFile(t, dir+"/.tsk.md")
	if before != after {
		t.Errorf("graph --json mutated .tsk.md")
	}
}
