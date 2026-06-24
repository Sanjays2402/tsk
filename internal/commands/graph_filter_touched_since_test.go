package commands

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestGraphFilterTouchedSinceShortcutForBothFilters: with --filter-
// touched-since 7d, the envelope behaves identically to setting
// BOTH --filter-completed-since 7d AND --filter-started-since 7d.
// This is the ergonomic-shortcut shape: one flag flips both axes.
func TestGraphFilterTouchedSinceShortcutForBothFilters(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"recent-done", "recent-start", "old-done", "old-start", "root"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// root (id=5) depends on 1, 2, 3, 4.
	for _, dep := range []string{"1", "2", "3", "4"} {
		if _, _, err := runCmd(t, dir, "depend", "5", "--add", dep); err != nil {
			t.Fatalf("depend add %s: %v", dep, err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "3"); err != nil {
		t.Fatalf("done 3: %v", err)
	}
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

	// One flag, same effect as both: --filter-touched-since 7d
	// should give: root 5 (always) + 1 (recent done) + 2 (recent start),
	// dropping 3 (old done) and 4 (old start).
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "5", "--json", "--filter-touched-since", "7d")
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
	if !kept[5] || !kept[1] || !kept[2] {
		t.Errorf("expected nodes 5, 1, 2 to be kept; got %+v", doc.Nodes)
	}
	if kept[3] || kept[4] {
		t.Errorf("expected nodes 3, 4 (old) to be filtered; got %+v", doc.Nodes)
	}
}

// TestGraphFilterTouchedSinceSetsBothMarkers: both
// filter_completed_since and filter_started_since marker fields
// are set on the envelope when --filter-touched-since is in use.
// This keeps the on-disk JSON shape backward-compatible with
// scripts that look for the individual markers (no new
// filter_touched_since field is added — the shortcut is purely
// at the CLI layer).
func TestGraphFilterTouchedSinceSetsBothMarkers(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-touched-since", "24h")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	if doc.FilterCompletedSince == "" {
		t.Errorf("expected filter_completed_since marker set, got empty")
	}
	if doc.FilterStartedSince == "" {
		t.Errorf("expected filter_started_since marker set, got empty")
	}
	if doc.FilterCompletedSince != doc.FilterStartedSince {
		t.Errorf("expected the two markers to match under --filter-touched-since, got completed=%q started=%q",
			doc.FilterCompletedSince, doc.FilterStartedSince)
	}
}

// TestGraphFilterTouchedSinceIndividualWins: when --filter-completed-
// since is ALSO set explicitly alongside --filter-touched-since, the
// individual flag wins for that axis (composition rule). This lets
// users mix the shortcut for one axis with a precise per-axis value
// for the other ("completions in 24h OR starts in 7d").
func TestGraphFilterTouchedSinceIndividualWins(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-completed-since", "1h", "--filter-touched-since", "7d")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	// completed-since should reflect 1h (the explicit value),
	// not 7d (touched). started-since should reflect 7d
	// (touched filling in the unset axis).
	if !strings.Contains(doc.FilterCompletedSince, "1h") && !strings.Contains(doc.FilterCompletedSince, "60m") {
		t.Errorf("expected filter_completed_since to reflect 1h, got %q", doc.FilterCompletedSince)
	}
	if !strings.Contains(doc.FilterStartedSince, "7d") && !strings.Contains(doc.FilterStartedSince, "168h") {
		t.Errorf("expected filter_started_since to reflect 7d, got %q", doc.FilterStartedSince)
	}
}

// TestGraphFilterTouchedSinceRequiresJSON: --filter-touched-since
// without --json is a usage error.
func TestGraphFilterTouchedSinceRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--filter-touched-since", "24h")
	if err == nil {
		t.Fatal("expected error for --filter-touched-since without --json")
	}
	if !strings.Contains(err.Error(), "filter-touched-since only applies to --json") {
		t.Errorf("expected useful error, got: %v", err)
	}
}

// TestGraphFilterTouchedSinceInvalidDuration: invalid duration
// strings produce a clean usage error.
func TestGraphFilterTouchedSinceInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-touched-since", "bogus")
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
	if !strings.Contains(err.Error(), "invalid --filter-touched-since") {
		t.Errorf("expected useful error, got: %v", err)
	}
}

// TestGraphFilterTouchedSinceZeroRejected: zero/negative durations
// are rejected as a usage error.
func TestGraphFilterTouchedSinceZeroRejected(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-touched-since", "0s")
	if err == nil {
		t.Fatal("expected error for zero duration")
	}
	if !strings.Contains(err.Error(), "must be a positive duration") {
		t.Errorf("expected useful error, got: %v", err)
	}
}

// TestGraphFilterTouchedSinceEmptyValue: empty value (defensive
// against unset shell vars) is a no-op.
func TestGraphFilterTouchedSinceEmptyValue(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-touched-since", "")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if strings.Contains(stdout, "filter_completed_since") || strings.Contains(stdout, "filter_started_since") {
		t.Errorf("empty --filter-touched-since should not set markers, got:\n%s", stdout)
	}
}

// TestGraphFilterTouchedSinceComposesWithAppend: --filter-touched-
// since works on the JSONL append path (same semantic as the
// individual filters that already compose with --append).
func TestGraphFilterTouchedSinceComposesWithAppend(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "recent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "1", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "2"); err != nil {
		t.Fatalf("start: %v", err)
	}
	path := filepath.Join(dir, "snap.jsonl")
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--filter-touched-since", "1h", "--output", path, "--append"); err != nil {
		t.Fatalf("graph append: %v", err)
	}
	data := readFile(t, path)
	if !strings.Contains(data, "filter_completed_since") {
		t.Errorf("appended record should carry filter_completed_since marker, got:\n%s", data)
	}
	if !strings.Contains(data, "filter_started_since") {
		t.Errorf("appended record should carry filter_started_since marker, got:\n%s", data)
	}
}

// TestGraphFilterTouchedSinceHelpMentionsFlag: --help text mentions
// the new flag for discoverability.
func TestGraphFilterTouchedSinceHelpMentionsFlag(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runCmd(t, dir, "graph", "--help")
	if err != nil {
		t.Fatalf("graph --help: %v", err)
	}
	if !strings.Contains(stdout, "--filter-touched-since") {
		t.Errorf("--help should mention --filter-touched-since, got:\n%s", stdout)
	}
}
