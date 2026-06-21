package commands

import (
	"strings"
	"testing"
)

// TestGraphEmptyStore: with zero deps in the store, the command
// prints a friendly message rather than emitting an empty digraph
// or no output at all.
func TestGraphEmptyStore(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if !strings.Contains(stdout, "no dependencies") {
		t.Fatalf("expected 'no dependencies', got:\n%s", stdout)
	}
}

// TestGraphASCIIAdjacency: ascii format renders each dep as a
// "#from -> #to" row in sorted order.
func TestGraphASCIIAdjacency(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// 3 depends on 1,2.
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if !strings.Contains(stdout, "#3 -> #1, #2") {
		t.Fatalf("expected '#3 -> #1, #2', got:\n%s", stdout)
	}
	// Title is included for the source so the line is self-documenting.
	if !strings.Contains(stdout, "c") {
		t.Fatalf("expected source title 'c' in row, got:\n%s", stdout)
	}
}

// TestGraphDoneSection: completed source tasks land in a separate
// "(done):" section so the active work isn't visually buried.
func TestGraphDoneSection(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// Open 4 deps on 3; done 2 deps on 1.
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	openIdx := strings.Index(stdout, "#4 -> #3")
	sectionIdx := strings.Index(stdout, "(done):")
	doneIdx := strings.Index(stdout, "#2 -> #1")
	if openIdx < 0 || sectionIdx < 0 || doneIdx < 0 {
		t.Fatalf("missing expected rows (open=%d, section=%d, done=%d):\n%s",
			openIdx, sectionIdx, doneIdx, stdout)
	}
	if !(openIdx < sectionIdx && sectionIdx < doneIdx) {
		t.Fatalf("expected open row before (done): before done row, got positions %d/%d/%d:\n%s",
			openIdx, sectionIdx, doneIdx, stdout)
	}
}

// TestGraphOpenFilter: --open drops done tasks AND edges to done
// tasks (those deps are satisfied — they no longer block anything).
func TestGraphOpenFilter(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// 3 depends on 1 (open) and 2 (will be done).
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--open")
	if err != nil {
		t.Fatalf("graph --open: %v", err)
	}
	// The remaining live edge is #3 -> #1 only.
	if !strings.Contains(stdout, "#3 -> #1") {
		t.Fatalf("expected '#3 -> #1', got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#2") {
		t.Fatalf("--open should drop edges to done #2, got:\n%s", stdout)
	}
}

// TestGraphDOTSyntax: --format dot produces valid-ish DOT skeleton
// with the right shape (digraph header, node declarations, edges,
// closing brace).
func TestGraphDOTSyntax(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot")
	if err != nil {
		t.Fatalf("graph dot: %v", err)
	}
	wantFragments := []string{
		"digraph tsk {",
		"rankdir=LR",
		"1 [label=",
		"2 [label=",
		"2 -> 1",
		"alpha",
		"beta",
		"}",
	}
	for _, want := range wantFragments {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected DOT to contain %q, got:\n%s", want, stdout)
		}
	}
}

// TestGraphDOTBlockedStyling: an open task with at least one open
// prereq should get the "red" outline (visual chokepoint marker).
func TestGraphDOTBlockedStyling(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "blocked", "satisfied", "done"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// 2 (blocked) depends on 1 (open).
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	// 3 (satisfied) depends on 4 (will be done).
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "4"); err != nil {
		t.Fatalf("depend 3: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "4"); err != nil {
		t.Fatalf("done 4: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--format", "dot")
	if err != nil {
		t.Fatalf("graph dot: %v", err)
	}
	// The node line for id 2 should carry color="red"; the one for
	// id 3 (satisfied) should not.
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "2 [label=") {
			if !strings.Contains(line, `color="red"`) {
				t.Fatalf("blocked id=2 should be red, got: %q", line)
			}
		}
		if strings.HasPrefix(strings.TrimSpace(line), "3 [label=") {
			if strings.Contains(line, `color="red"`) {
				t.Fatalf("satisfied id=3 should NOT be red, got: %q", line)
			}
		}
		if strings.HasPrefix(strings.TrimSpace(line), "4 [label=") {
			if !strings.Contains(line, "lightgray") {
				t.Fatalf("done id=4 should be filled lightgray, got: %q", line)
			}
		}
	}
}

// TestGraphRejectsUnknownFormat: unknown --format value is a usage
// error (exit 2).
func TestGraphRejectsUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--format", "yaml")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestGraphSortedDeterministic: with multiple sources and multiple
// deps each, the output ordering is always (source asc, then dep
// asc). Repeated runs produce identical bytes.
func TestGraphSortedDeterministic(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d", "e"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "5", "--on", "4,2,1"); err != nil {
		t.Fatalf("depend 5: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3,1"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	a, _, _ := runCmd(t, dir, "graph")
	b, _, _ := runCmd(t, dir, "graph")
	if a != b {
		t.Fatalf("graph output should be deterministic, got\n--A--\n%s\n--B--\n%s", a, b)
	}
	// Source order: 4 comes before 5 (sorted asc).
	row4 := strings.Index(a, "#4 -> ")
	row5 := strings.Index(a, "#5 -> ")
	if row4 < 0 || row5 < 0 {
		t.Fatalf("expected both #4 and #5 rows present, got positions %d/%d:\n%s", row4, row5, a)
	}
	if row5 <= row4 {
		t.Fatalf("expected #4 row before #5 row, got positions %d/%d:\n%s", row4, row5, a)
	}
	// Within #5, deps are sorted asc: 1, 2, 4.
	if !strings.Contains(a, "#5 -> #1, #2, #4") {
		t.Fatalf("expected '#5 -> #1, #2, #4' (sorted), got:\n%s", a)
	}
}
