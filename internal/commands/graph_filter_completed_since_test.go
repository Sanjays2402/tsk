package commands

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestGraphFilterCompletedSinceTrimsOldNodes: a task completed
// outside the recency window is dropped from the envelope.
func TestGraphFilterCompletedSinceTrimsOldNodes(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"old-done", "new-done", "root"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// Build dep chain: root (id=3) depends on old-done (id=1)
	// and new-done (id=2).
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	// Hand-edit task #1's Completed to 30 days ago.
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	old := time.Now().Add(-30 * 24 * time.Hour)
	t1 := s.ByID(1)
	if t1 == nil {
		t.Fatalf("missing task 1")
	}
	t1.Completed = &old
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json", "--filter-completed-since", "7d")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	// The root (id=1) should appear; tasks completed within 7d would too.
	// Task #3 (root, the unrelated upstream parent) — actually
	// --upstream-of 1 gives chain pointing AT 1; node 3 depends
	// on 1 so 3 is upstream of 1. 3 is open (not done) so it's
	// filtered out. We expect only the root (1) to survive.
	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node (root only), got %d:\nnodes: %+v", len(doc.Nodes), doc.Nodes)
	}
	if doc.Nodes[0].ID != 1 {
		t.Errorf("expected only root id=1 to survive, got %d", doc.Nodes[0].ID)
	}
}

// TestGraphFilterCompletedSincePreservesRoot: even when the root
// is itself not within the window (or not done), it always
// survives — the consumer asked about THIS root.
func TestGraphFilterCompletedSincePreservesRoot(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "open-root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Root is open (not done). With filter active, root should
	// still appear in the envelope.
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-completed-since", "7d")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node (root), got %d", len(doc.Nodes))
	}
	if doc.Nodes[0].ID != 1 {
		t.Errorf("expected root id=1, got %d", doc.Nodes[0].ID)
	}
}

// TestGraphFilterCompletedSinceDropsEdges: edges touching dropped
// nodes are removed from the envelope.
func TestGraphFilterCompletedSinceDropsEdges(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"old-prereq", "root"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Push #1's Completed back beyond the window.
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	old := time.Now().Add(-30 * 24 * time.Hour)
	t1 := s.ByID(1)
	t1.Completed = &old
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--filter-completed-since", "7d")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Only root (#2) survives; #1 was beyond window. Edge
	// 2->1 should be dropped (target #1 is gone).
	if len(doc.Edges) != 0 {
		t.Errorf("expected 0 edges (target dropped), got %d:\nedges: %+v", len(doc.Edges), doc.Edges)
	}
}

// TestGraphFilterCompletedSinceWithinWindowSurvives: a task
// completed JUST inside the window survives.
func TestGraphFilterCompletedSinceWithinWindowSurvives(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"fresh-done", "root"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Set #1's Completed to 1 hour ago (within 7d window).
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	fresh := time.Now().Add(-1 * time.Hour)
	s.ByID(1).Completed = &fresh
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--filter-completed-since", "7d")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Both root (#2) and the freshly-done #1 survive.
	if len(doc.Nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d:\n%+v", len(doc.Nodes), doc.Nodes)
	}
}

// TestGraphFilterCompletedSinceEnvelopeFieldPresent: the
// envelope has a top-level filter_completed_since field naming
// the active window.
func TestGraphFilterCompletedSinceEnvelopeFieldPresent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-completed-since", "7d")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if !strings.Contains(stdout, "filter_completed_since") {
		t.Errorf("expected filter_completed_since field in envelope, got:\n%s", stdout)
	}
}

// TestGraphFilterCompletedSinceDefaultAbsent: without the flag,
// no filter_completed_since field appears (back-compat).
func TestGraphFilterCompletedSinceDefaultAbsent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if strings.Contains(stdout, "filter_completed_since") {
		t.Errorf("default envelope should NOT contain filter_completed_since, got:\n%s", stdout)
	}
}

// TestGraphFilterCompletedSinceRequiresJSON: the flag without
// --json is a usage error.
func TestGraphFilterCompletedSinceRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--filter-completed-since", "7d")
	if err == nil {
		t.Fatal("expected error for --filter-completed-since without --json")
	}
	if !strings.Contains(err.Error(), "--filter-completed-since only applies to --json") {
		t.Fatalf("expected requires-json error, got: %v", err)
	}
}

// TestGraphFilterCompletedSinceRejectsInvalidDuration: a typo'd
// duration surfaces as a usage error.
func TestGraphFilterCompletedSinceRejectsInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-completed-since", "garbage")
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
	if !strings.Contains(err.Error(), "invalid --filter-completed-since") {
		t.Fatalf("expected invalid-duration error, got: %v", err)
	}
}

// TestGraphFilterCompletedSinceRejectsZero: zero/negative
// durations are rejected (vacuous filter, almost certainly a typo).
func TestGraphFilterCompletedSinceRejectsZero(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-completed-since", "0s")
	if err == nil {
		t.Fatal("expected error for 0s")
	}
	if !strings.Contains(err.Error(), "must be a positive duration") {
		t.Fatalf("expected positive-duration error, got: %v", err)
	}
}

// TestGraphFilterCompletedSinceComposesWithIncludeCompleted: the
// filter composes naturally with --include-completed so consumers
// can confirm WHICH completion dates kept each node.
func TestGraphFilterCompletedSinceComposesWithIncludeCompleted(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"fresh", "root"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	now := time.Now()
	s.ByID(1).Completed = &now
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--filter-completed-since", "7d", "--include-completed")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	// Find #1 in the surviving nodes; should have completed set.
	var foundDone bool
	for _, n := range doc.Nodes {
		if n.ID == 1 {
			foundDone = true
			if n.Completed == "" {
				t.Errorf("expected completed timestamp on surviving done node")
			}
		}
	}
	if !foundDone {
		t.Errorf("expected node #1 in envelope, got: %+v", doc.Nodes)
	}
}

// TestGraphFilterCompletedSinceEmptyValueNoFilter: empty value
// (e.g. unset shell var) means "no filter" — every node survives,
// envelope omits the marker field.
func TestGraphFilterCompletedSinceEmptyValueNoFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-completed-since", "")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if strings.Contains(stdout, "filter_completed_since") {
		t.Errorf("empty --filter-completed-since should be a no-op, got marker: %s", stdout)
	}
}
