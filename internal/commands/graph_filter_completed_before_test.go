package commands

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestGraphFilterCompletedBeforeTrimsRecentNodes: with
// --filter-completed-before 7d set, a node completed 2 days ago
// (RECENT, INSIDE the cutoff window) is filtered OUT, while a
// node completed 30 days ago (OLDER than the cutoff) survives.
// This is the inverse direction of --filter-completed-since.
func TestGraphFilterCompletedBeforeTrimsRecentNodes(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"recent-done", "old-done", "root"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// root (id=3) depends on 1 and 2.
	for _, dep := range []string{"1", "2"} {
		if _, _, err := runCmd(t, dir, "depend", "3", "--add", dep); err != nil {
			t.Fatalf("depend add %s: %v", dep, err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	old := time.Now().Add(-30 * 24 * time.Hour)
	t2 := s.ByID(2)
	t2.Completed = &old
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "3", "--json", "--filter-completed-before", "7d")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	kept := make(map[int]bool)
	for _, n := range doc.Nodes {
		kept[n.ID] = true
	}
	if !kept[3] {
		t.Errorf("expected root #3 to be kept; got nodes %+v", doc.Nodes)
	}
	if !kept[2] {
		t.Errorf("expected node #2 (old-done, 30d) to survive --filter-completed-before 7d; got nodes %+v", doc.Nodes)
	}
	if kept[1] {
		t.Errorf("expected node #1 (recent-done, 2s) to be filtered OUT under --filter-completed-before 7d; got nodes %+v", doc.Nodes)
	}
}

// TestGraphFilterCompletedBeforePreservesRoot: the root id is
// kept even when its own task isn't done (matches the symmetric
// semantic --filter-completed-since uses — root always present).
func TestGraphFilterCompletedBeforePreservesRoot(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-completed-before", "7d")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(doc.Nodes) != 1 || doc.Nodes[0].ID != 1 {
		t.Errorf("expected root #1 kept under filter; got %+v", doc.Nodes)
	}
}

// TestGraphFilterCompletedBeforeMarkerFieldPresent: the envelope
// gains a top-level filter_completed_before marker carrying the
// canonical humanized window when the flag is active. Scripts
// distinguish this from --filter-completed-since via the field
// name suffix.
func TestGraphFilterCompletedBeforeMarkerFieldPresent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-completed-before", "24h")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if doc.FilterCompletedBefore == "" {
		t.Errorf("expected filter_completed_before marker set; got empty")
	}
	if doc.FilterCompletedSince != "" {
		t.Errorf("expected filter_completed_since NOT set under --filter-completed-before; got %q", doc.FilterCompletedSince)
	}
}

// TestGraphFilterCompletedBeforeDefaultAbsentBackCompat: when the
// flag isn't passed, the envelope shape stays byte-identical to
// the historical form (no filter_completed_before key).
func TestGraphFilterCompletedBeforeDefaultAbsentBackCompat(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if strings.Contains(stdout, "filter_completed_before") {
		t.Errorf("expected no filter_completed_before key when flag is unset; got:\n%s", stdout)
	}
}

// TestGraphFilterCompletedBeforeRequiresJSON: the flag is rejected
// on the non-JSON (ascii/dot/svg) paths since they have no obvious
// "completed-before" rendering idiom.
func TestGraphFilterCompletedBeforeRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--filter-completed-before", "7d")
	if err == nil {
		t.Fatal("expected error for --filter-completed-before without --json")
	}
	if !strings.Contains(err.Error(), "filter-completed-before only applies to --json") {
		t.Errorf("expected 'only applies to --json' error; got: %v", err)
	}
}

// TestGraphFilterCompletedBeforeInvalidDuration: a bogus duration
// string is rejected with a clear naming of the offending value.
func TestGraphFilterCompletedBeforeInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-completed-before", "wat")
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
	if !strings.Contains(err.Error(), "invalid --filter-completed-before") {
		t.Errorf("expected 'invalid --filter-completed-before' error; got: %v", err)
	}
}

// TestGraphFilterCompletedBeforeZeroRejected: a zero/negative
// duration is rejected — a zero-window cutoff would have ambiguous
// semantics and is almost certainly a typo.
func TestGraphFilterCompletedBeforeZeroRejected(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-completed-before", "0s")
	if err == nil {
		t.Fatal("expected error for zero duration")
	}
	if !strings.Contains(err.Error(), "must be a positive duration") {
		t.Errorf("expected 'must be a positive duration' error; got: %v", err)
	}
}

// TestGraphFilterCompletedBeforeEmptyValueNoFilter: an empty
// --filter-completed-before string is a no-op (defensive against
// unset shell vars), matching the SINCE family's stance.
func TestGraphFilterCompletedBeforeEmptyValueNoFilter(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "root"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--add", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--filter-completed-before", "")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if len(doc.Nodes) != 2 {
		t.Errorf("expected empty --filter-completed-before to be no-op (2 nodes kept); got %+v", doc.Nodes)
	}
	if doc.FilterCompletedBefore != "" {
		t.Errorf("expected no marker for empty filter; got %q", doc.FilterCompletedBefore)
	}
}

// TestGraphFilterCompletedBeforeFixedWindowComposesWithSince: the
// SINCE and BEFORE filters on the same axis compose as AND — this
// is the fixed-width-window use case: `--filter-completed-since 30d
// --filter-completed-before 7d` keeps only completions in the
// 7-30 day band.
func TestGraphFilterCompletedBeforeFixedWindowComposesWithSince(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"ancient", "midband", "fresh", "root"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	for _, dep := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "depend", "4", "--add", dep); err != nil {
			t.Fatalf("depend: %v", err)
		}
	}
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "done", id); err != nil {
			t.Fatalf("done %s: %v", id, err)
		}
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	ancient := time.Now().Add(-60 * 24 * time.Hour)
	mid := time.Now().Add(-14 * 24 * time.Hour)
	t1 := s.ByID(1)
	t1.Completed = &ancient
	t2 := s.ByID(2)
	t2.Completed = &mid
	// t3 keeps the freshly-done timestamp (just now).
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "4", "--json",
		"--filter-completed-since", "30d", "--filter-completed-before", "7d")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	kept := make(map[int]bool)
	for _, n := range doc.Nodes {
		kept[n.ID] = true
	}
	if !kept[4] {
		t.Errorf("root #4 must survive; got %+v", doc.Nodes)
	}
	if !kept[2] {
		t.Errorf("expected #2 (midband, 14d) to survive fixed-window filter; got %+v", doc.Nodes)
	}
	if kept[1] {
		t.Errorf("expected #1 (ancient, 60d) to be filtered out (outside since=30d); got %+v", doc.Nodes)
	}
	if kept[3] {
		t.Errorf("expected #3 (fresh, ~0s) to be filtered out (inside before=7d); got %+v", doc.Nodes)
	}
	if doc.FilterCompletedSince == "" || doc.FilterCompletedBefore == "" {
		t.Errorf("expected both markers set on fixed-window composition; got since=%q before=%q",
			doc.FilterCompletedSince, doc.FilterCompletedBefore)
	}
}

// TestGraphFilterCompletedBeforeHelpMention: the --help text
// surfaces the new flag so the discoverability story matches the
// SINCE family.
func TestGraphFilterCompletedBeforeHelpMention(t *testing.T) {
	dir := t.TempDir()
	_, combined, err := runCmd(t, dir, "graph", "--help")
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(combined, "--filter-completed-before") {
		t.Errorf("expected --filter-completed-before in help text; got:\n%s", combined)
	}
}
