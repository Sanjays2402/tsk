package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestPathAnyDirectionFindsReverse: a chain that the directed
// search misses (because the user asked B → A but the dep is
// A → B) must be found when --any-direction is set.
//
// Setup: #2 depends on #1 (i.e. 2 → 1 directed). Without
// --any-direction `tsk path 1 2` returns no-path. With
// --any-direction it should find the chain.
func TestPathAnyDirectionFindsReverse(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "dependent"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Directed search must NOT find anything.
	_, _, err := runCmd(t, dir, "path", "1", "2")
	if err == nil {
		t.Fatal("expected no-path error in directed mode")
	}
	// --any-direction must succeed.
	stdout, _, err := runCmd(t, dir, "path", "1", "2", "--any-direction")
	if err != nil {
		t.Fatalf("path 1 2 --any-direction: %v", err)
	}
	if !strings.Contains(stdout, "#1") || !strings.Contains(stdout, "#2") {
		t.Fatalf("expected both ids in path, got:\n%s", stdout)
	}
}

// TestPathAnyDirectionStillSameDirectionWorks: if the directed
// search finds a chain, --any-direction returns the same shortest
// chain — same answer, just via a wider search. The shortest path
// in an undirected graph that contains a directed one IS the
// directed one (BFS in the wider graph is at most as long).
func TestPathAnyDirectionStillSameDirectionWorks(t *testing.T) {
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
	directed, _, err := runCmd(t, dir, "path", "3", "1", "--json")
	if err != nil {
		t.Fatalf("path directed: %v", err)
	}
	undirected, _, err := runCmd(t, dir, "path", "3", "1", "--any-direction", "--json")
	if err != nil {
		t.Fatalf("path undirected: %v", err)
	}
	var dirDoc, undDoc map[string]any
	if err := json.Unmarshal([]byte(directed), &dirDoc); err != nil {
		t.Fatalf("dir JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(undirected), &undDoc); err != nil {
		t.Fatalf("und JSON: %v", err)
	}
	dirPath := dirDoc["path"].([]any)
	undPath := undDoc["path"].([]any)
	if len(dirPath) != len(undPath) {
		t.Fatalf("expected same path length, got directed=%d undirected=%d", len(dirPath), len(undPath))
	}
	for i := range dirPath {
		if dirPath[i] != undPath[i] {
			t.Fatalf("path[%d] differs: directed=%v undirected=%v", i, dirPath[i], undPath[i])
		}
	}
}

// TestPathAnyDirectionZigZag: a chain that requires switching
// direction mid-traversal must work — that's the whole point.
//
// Setup:  4 → 3  AND  2 → 3  AND  2 → 1
// (3 depends on nothing; 4 and 2 both depend on 3; 2 depends on 1)
// Question: are #4 and #1 connected? Directed search 4 → 1: no.
// Undirected: 4 → 3 ← 2 → 1 — a 4-node connectivity chain.
func TestPathAnyDirectionZigZag(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// 4 depends on 3, 2 depends on 3 and 1.
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1,3"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	// Directed: no path from 4 to 1.
	_, _, err := runCmd(t, dir, "path", "4", "1")
	if err == nil {
		t.Fatal("expected no-path in directed mode")
	}
	// Undirected: walks 4 → 3 → 2 → 1 (or some 4-node chain).
	stdout, _, err := runCmd(t, dir, "path", "4", "1", "--any-direction", "--json")
	if err != nil {
		t.Fatalf("path zig-zag: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if found, _ := doc["found"].(bool); !found {
		t.Fatalf("expected found=true for zig-zag, got %v", doc)
	}
	path := doc["path"].([]any)
	if len(path) < 3 {
		t.Fatalf("expected zig-zag path of >=3 nodes, got %d:\n%s", len(path), stdout)
	}
	// Start and end are correct.
	if int(path[0].(float64)) != 4 || int(path[len(path)-1].(float64)) != 1 {
		t.Fatalf("expected path to start at 4 end at 1, got %v", path)
	}
}

// TestPathDirectionFieldInJSON: --json reports the search mode
// ("directed" by default, "any" with --any-direction) so consumers
// know which kind of result they got.
func TestPathDirectionFieldInJSON(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	directed, _, err := runCmd(t, dir, "path", "2", "1", "--json")
	if err != nil {
		t.Fatalf("path directed: %v", err)
	}
	var dirDoc map[string]any
	if err := json.Unmarshal([]byte(directed), &dirDoc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, directed)
	}
	if direction, _ := dirDoc["direction"].(string); direction != "directed" {
		t.Fatalf("expected direction=directed, got %v", dirDoc["direction"])
	}
	undirected, _, err := runCmd(t, dir, "path", "2", "1", "--any-direction", "--json")
	if err != nil {
		t.Fatalf("path any: %v", err)
	}
	var undDoc map[string]any
	if err := json.Unmarshal([]byte(undirected), &undDoc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, undirected)
	}
	if direction, _ := undDoc["direction"].(string); direction != "any" {
		t.Fatalf("expected direction=any, got %v", undDoc["direction"])
	}
}

// TestPathAnyDirectionNotFoundMessage: when even the undirected
// search finds nothing (no connectivity at all), the plain message
// reflects that — the user shouldn't be told "try --any-direction"
// when they already did.
func TestPathAnyDirectionNotFoundMessage(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// No deps between them at all.
	stdout, _, err := runCmd(t, dir, "path", "1", "2", "--any-direction")
	if err == nil {
		t.Fatal("expected no-path error for fully disconnected tasks")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %v", err)
	}
	if !strings.Contains(stdout, "even with --any-direction") {
		t.Fatalf("expected 'even with --any-direction' framing, got:\n%s", stdout)
	}
	// Directed default should still suggest the wider search.
	stdoutDir, _, err := runCmd(t, dir, "path", "1", "2")
	if err == nil {
		t.Fatal("expected no-path error in directed mode")
	}
	if !strings.Contains(stdoutDir, "try --any-direction") {
		t.Fatalf("expected 'try --any-direction' hint, got:\n%s", stdoutDir)
	}
}

// TestPathAnyDirectionFindsShortest: shortest-path semantics hold
// in undirected mode too. If both a 1-edge connection and a longer
// chain exist, BFS picks the short one regardless of direction.
func TestPathAnyDirectionFindsShortest(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// 2 → 1 (direct), and 3 → 2 → 1 (longer route to 1 via 2).
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "path", "1", "3", "--any-direction", "--json")
	if err != nil {
		t.Fatalf("path any-direction: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	path := doc["path"].([]any)
	// Shortest in undirected graph: 1 → 2 → 3 (3 nodes, 2 edges).
	if len(path) != 3 {
		t.Fatalf("expected shortest path of 3 nodes, got %d:\n%s", len(path), stdout)
	}
}
