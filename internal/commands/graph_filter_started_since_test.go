package commands

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestGraphFilterStartedSinceTrimsOldNodes: an in-progress task
// whose Started timestamp is OUTSIDE the recency window is dropped
// from the envelope. Sister of --filter-completed-since for the
// in-progress (OPEN) side of the work-state pair.
func TestGraphFilterStartedSinceTrimsOldNodes(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"old-wip", "new-wip", "root"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// root (#3) depends on old-wip (#1) and new-wip (#2).
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Start both prereqs.
	if _, _, err := runCmd(t, dir, "start", "1", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Push #1's Started 30 days back.
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	old := time.Now().Add(-30 * 24 * time.Hour)
	t1 := s.ByID(1)
	if t1 == nil {
		t.Fatalf("missing task 1")
	}
	t1.Started = &old
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// --upstream-of 1 gives the chain pointing AT 1; node 3
	// depends on 1 so 3 appears upstream of 1. We also have
	// node 2 (sibling prereq for 3) showing up in the
	// upstream-of-1 subgraph via 3->2 edge collection.
	// Actually --upstream-of 1 picks edges where DependsOn
	// transitively names 1. Only 3->1 satisfies that for #1.
	// So nodes in the upstream-of-1 envelope: {1 (root), 3}.
	// Filter --filter-started-since 7d: drops node 3 (never
	// started). Root #1 always survives. Result: just root.
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json", "--filter-started-since", "7d")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node (root only — old-wip filtered, root preserved), got %d:\nnodes: %+v", len(doc.Nodes), doc.Nodes)
	}
	if doc.Nodes[0].ID != 1 {
		t.Errorf("expected root id=1 to survive, got %d", doc.Nodes[0].ID)
	}
}

// TestGraphFilterStartedSincePreservesRoot: even when the root is
// not itself in-progress (or never started), it always survives —
// the consumer asked about THIS root.
func TestGraphFilterStartedSincePreservesRoot(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "open-root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Root is open (not started). With filter active, root
	// should still appear in the envelope.
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-started-since", "7d")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(doc.Nodes) != 1 {
		t.Fatalf("expected 1 node (root), got %d", len(doc.Nodes))
	}
	if doc.Nodes[0].ID != 1 {
		t.Errorf("expected root id=1 preserved, got %d", doc.Nodes[0].ID)
	}
}

// TestGraphFilterStartedSinceFreshNodeSurvives: a task started
// WITHIN the window (e.g. an hour ago) is preserved.
func TestGraphFilterStartedSinceFreshNodeSurvives(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add root: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "fresh-wip"); err != nil {
		t.Fatalf("add fresh: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "1", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	// Default Started is "now", so well within any reasonable window.
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-started-since", "1h")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(doc.Nodes) != 2 {
		t.Fatalf("expected 2 nodes (root + fresh-wip), got %d:\nnodes: %+v", len(doc.Nodes), doc.Nodes)
	}
}

// TestGraphFilterStartedSinceMarkerField: the envelope's
// filter_started_since field carries the canonical humanized
// duration when active.
func TestGraphFilterStartedSinceMarkerField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-started-since", "24h")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if doc.FilterStartedSince == "" {
		t.Errorf("expected filter_started_since marker to be set, got empty")
	}
}

// TestGraphFilterStartedSinceDefaultAbsent: the
// filter_started_since marker field is OMITTED from the envelope
// when the filter is not in use (omitempty contract).
func TestGraphFilterStartedSinceDefaultAbsent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	// Byte-level check: the field name should NOT appear in
	// the envelope.
	if strings.Contains(stdout, "filter_started_since") {
		t.Errorf("filter_started_since should be omitted by default, got:\n%s", stdout)
	}
}

// TestGraphFilterStartedSinceRequiresJSON: --filter-started-since
// without --json is a usage error (the filter only makes sense on
// the JSON envelope path).
func TestGraphFilterStartedSinceRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--filter-started-since", "24h")
	if err == nil {
		t.Fatal("expected error for --filter-started-since without --json")
	}
	if !strings.Contains(err.Error(), "filter-started-since only applies to --json") {
		t.Errorf("expected useful error, got: %v", err)
	}
}

// TestGraphFilterStartedSinceInvalidDuration: invalid duration
// strings produce a clean usage error.
func TestGraphFilterStartedSinceInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-started-since", "bogus")
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
	if !strings.Contains(err.Error(), "invalid --filter-started-since") {
		t.Errorf("expected useful error, got: %v", err)
	}
}

// TestGraphFilterStartedSinceZeroRejected: a zero-window filter
// would drop every node and is almost certainly a typo.
func TestGraphFilterStartedSinceZeroRejected(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-started-since", "0s")
	if err == nil {
		t.Fatal("expected error for zero duration")
	}
	if !strings.Contains(err.Error(), "must be a positive duration") {
		t.Errorf("expected useful error, got: %v", err)
	}
}

// TestGraphFilterStartedSinceEmptyValue: empty string (defensive
// against unset shell vars) is a no-op (no filter).
func TestGraphFilterStartedSinceEmptyValue(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-started-since", "")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if !strings.Contains(stdout, "filter_started_since") == false {
		// double-check: empty value should NOT set the marker
	}
	if strings.Contains(stdout, "filter_started_since") {
		t.Errorf("empty --filter-started-since should not set marker, got:\n%s", stdout)
	}
}

// TestGraphFilterStartedSinceUnionWithCompletedSince: when BOTH
// filters are active, the composition is UNION — a node survives
// if EITHER recently-completed OR recently-started. This is the
// "what's actively moving?" use case the two filters were
// designed for.
func TestGraphFilterStartedSinceUnionWithCompletedSince(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"recently-done", "recently-started", "old-done", "old-wip", "root"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// root (id=5) depends on each of 1, 2, 3, 4. Use --add so
	// each call appends (--on would replace each time, leaving
	// only the last dep).
	for _, dep := range []string{"1", "2", "3", "4"} {
		if _, _, err := runCmd(t, dir, "depend", "5", "--add", dep); err != nil {
			t.Fatalf("depend 5 add %s: %v", dep, err)
		}
	}
	// Mark #1 and #3 done; #1 stays recent, #3 we'll backdate.
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "3"); err != nil {
		t.Fatalf("done 3: %v", err)
	}
	// Start #2 (recent) and #4 (will backdate).
	if _, _, err := runCmd(t, dir, "start", "2", "4"); err != nil {
		t.Fatalf("start: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	old := time.Now().Add(-30 * 24 * time.Hour)
	t3 := s.ByID(3)
	t3.Completed = &old
	t4 := s.ByID(4)
	t4.Started = &old
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// reachable 5 = {5, 1, 2, 3, 4}. Filter UNION keeps:
	// root 5 (always) + 1 (recent done) + 2 (recent start).
	// Drops 3 (old done) and 4 (old start).
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "5", "--json", "--filter-completed-since", "7d", "--filter-started-since", "7d")
	if err != nil {
		t.Fatalf("graph reachable 5: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	kept := make(map[int]bool)
	for _, n := range doc.Nodes {
		kept[n.ID] = true
	}
	if !kept[5] {
		t.Errorf("expected root 5 to be kept, nodes=%+v", doc.Nodes)
	}
	if !kept[1] {
		t.Errorf("expected node 1 (recent done) to be kept under union, nodes=%+v", doc.Nodes)
	}
	if !kept[2] {
		t.Errorf("expected node 2 (recent start) to be kept under union, nodes=%+v", doc.Nodes)
	}
	if kept[3] {
		t.Errorf("did not expect node 3 (old done) to be kept, nodes=%+v", doc.Nodes)
	}
	if kept[4] {
		t.Errorf("did not expect node 4 (old start) to be kept, nodes=%+v", doc.Nodes)
	}
	// Both marker fields should be set on the envelope.
	if doc.FilterCompletedSince == "" {
		t.Errorf("expected filter_completed_since marker, got empty")
	}
	if doc.FilterStartedSince == "" {
		t.Errorf("expected filter_started_since marker, got empty")
	}
}

// TestGraphFilterStartedSinceComposesWithIncludeStarted: the
// recency filter combines naturally with --include-started so the
// surviving nodes carry the same started field they were filtered
// on (useful for jq pipelines that want to see the timestamp).
func TestGraphFilterStartedSinceComposesWithIncludeStarted(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "fresh-wip"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "1", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-started-since", "1h", "--include-started")
	if err != nil {
		t.Fatalf("graph composition: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	// Find node 2; should have a non-empty started field.
	var hasStarted bool
	for _, n := range doc.Nodes {
		if n.ID == 2 {
			if n.Started != "" {
				hasStarted = true
			}
		}
	}
	if !hasStarted {
		t.Errorf("expected node 2 to carry a non-empty 'started' field after --include-started, got nodes=%+v", doc.Nodes)
	}
}

// TestGraphFilterStartedSinceHelpMentionsFlag: --help text mentions
// the new flag for discoverability.
func TestGraphFilterStartedSinceHelpMentionsFlag(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runCmd(t, dir, "graph", "--help")
	if err != nil {
		t.Fatalf("graph --help: %v", err)
	}
	if !strings.Contains(stdout, "--filter-started-since") {
		t.Errorf("--help should mention --filter-started-since, got:\n%s", stdout)
	}
}
