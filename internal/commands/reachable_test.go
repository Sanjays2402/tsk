package commands

import (
	"strings"
	"testing"
)

// TestReachableMatchesGraphReachable: the new top-level command and
// the existing `graph --reachable` flag must produce byte-identical
// output for the same query. The whole point of having a top-level
// alias is discoverability — the BEHAVIOR must stay synchronized.
func TestReachableMatchesGraphReachable(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"deep", "mid", "top"} {
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
	graphOut, _, err := runCmd(t, dir, "graph", "--reachable", "3")
	if err != nil {
		t.Fatalf("graph --reachable: %v", err)
	}
	reachOut, _, err := runCmd(t, dir, "reachable", "3")
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	if graphOut != reachOut {
		t.Fatalf("reachable must match graph --reachable byte-for-byte.\ngraph:\n%s\nreachable:\n%s", graphOut, reachOut)
	}
}

// TestReachableShowsTransitivePrereqs: a chain 3 -> 2 -> 1 asked
// from #3 should emit every edge in the transitive closure.
func TestReachableShowsTransitivePrereqs(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"deep", "mid", "top", "other"} {
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
	stdout, _, err := runCmd(t, dir, "reachable", "3")
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	for _, want := range []string{"#3 -> #2", "#2 -> #1"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q, got:\n%s", want, stdout)
		}
	}
	// Unrelated task #4 must not appear.
	if strings.Contains(stdout, "#4") {
		t.Fatalf("unrelated task #4 must be absent, got:\n%s", stdout)
	}
}

// TestReachableUnknownID: a non-existent id should error.
func TestReachableUnknownID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "reachable", "99")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

// TestReachableDOTFormat: the --format dot flag forwards into the
// shared emitter.
func TestReachableDOTFormat(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "reachable", "2", "--format", "dot")
	if err != nil {
		t.Fatalf("reachable --format dot: %v", err)
	}
	if !strings.HasPrefix(stdout, "digraph tsk {") {
		t.Fatalf("expected DOT skeleton, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "2 -> 1;") {
		t.Fatalf("expected '2 -> 1;' edge, got:\n%s", stdout)
	}
}

// TestReachableOpenComposes: --open should compose with the
// reachable filter the same way it does on `graph --reachable
// --open`.
func TestReachableOpenComposes(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"deep", "mid", "top"} {
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
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "reachable", "3", "--open")
	if err != nil {
		t.Fatalf("reachable --open: %v", err)
	}
	if !strings.Contains(stdout, "#3 -> #2") {
		t.Fatalf("expected '#3 -> #2', got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#2 -> #1") {
		t.Fatalf("done-prereq edge must drop under --open, got:\n%s", stdout)
	}
}

// TestReachableNoDeps: a root with no prereqs of its own should
// surface the explicit "no dependencies reachable from #N" message.
func TestReachableNoDeps(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "reachable", "1")
	if err != nil {
		t.Fatalf("reachable: %v", err)
	}
	if !strings.Contains(stdout, "no dependencies reachable from #1") {
		t.Fatalf("expected specific empty message, got:\n%s", stdout)
	}
}

// TestReachableRejectsBadFormat: unknown --format value should error
// up-front.
func TestReachableRejectsBadFormat(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "reachable", "1", "--format", "yaml")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}
