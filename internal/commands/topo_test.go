package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTopoEmitsPrereqsFirst: a basic chain 3 → 2 → 1 should emit
// the prereq (#1) first, then #2, then #3 — the user can walk the
// list straight through without `tsk done` refusals.
func TestTopoEmitsPrereqsFirst(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"deep", "middle", "top"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2->1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend 3->2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo")
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	i1 := strings.Index(stdout, "#1 ")
	i2 := strings.Index(stdout, "#2 ")
	i3 := strings.Index(stdout, "#3 ")
	if !(i1 >= 0 && i2 > i1 && i3 > i2) {
		t.Fatalf("expected topo order #1 < #2 < #3, got positions (%d, %d, %d):\n%s",
			i1, i2, i3, stdout)
	}
}

// TestTopoNoDepsFollowsTieBreak: when no tasks have deps, the
// emitted order matches `tsk top` — pin > priority desc > earliest
// due > lowest id. Asserts the inside-layer ordering is right.
func TestTopoNoDepsFollowsTieBreak(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "low task", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "urgent task", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "high task", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo")
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	iUrgent := strings.Index(stdout, "urgent task")
	iHigh := strings.Index(stdout, "high task")
	iLow := strings.Index(stdout, "low task")
	if !(iUrgent >= 0 && iHigh > iUrgent && iLow > iHigh) {
		t.Fatalf("expected urgent < high < low, got (%d, %d, %d):\n%s",
			iUrgent, iHigh, iLow, stdout)
	}
}

// TestTopoSkipsDoneByDefault: done tasks are excluded by default —
// the whole point of topo is "what should I do?".
func TestTopoSkipsDoneByDefault(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo")
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	if strings.Contains(stdout, "alpha") {
		t.Fatalf("done task should be excluded by default, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "beta") {
		t.Fatalf("open task should appear, got:\n%s", stdout)
	}
}

// TestTopoAllIncludesDone: --all flips the policy.
func TestTopoAllIncludesDone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--all")
	if err != nil {
		t.Fatalf("topo --all: %v", err)
	}
	if !strings.Contains(stdout, "alpha") || !strings.Contains(stdout, "beta") {
		t.Fatalf("--all should include done task, got:\n%s", stdout)
	}
}

// TestTopoSatisfiedDepNotBlocking: when prereq is done, the
// dependent task should be in the "ready" set immediately. Same
// policy as unmetBlockers — closed prereqs don't count.
func TestTopoSatisfiedDepNotBlocking(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "dependent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo")
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	if !strings.Contains(stdout, "dependent") {
		t.Fatalf("dependent should appear since prereq is done, got:\n%s", stdout)
	}
}

// TestTopoCycleAnnotation: a hand-edited 3-cycle (which the writer
// won't catch) must surface as "(cycle)" annotations at the tail,
// never silently dropped or looped forever.
func TestTopoCycleAnnotation(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta", "gamma"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "1", "--on", "2"); err != nil {
		t.Fatalf("depend 1->2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "3"); err != nil {
		t.Fatalf("depend 2->3: %v", err)
	}
	// Splice 3 → 1 directly into the file to close the cycle.
	path := filepath.Join(dir, ".tsk.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(string(body), "\n")
	for i, l := range lines {
		if strings.Contains(l, "id:3 ") && !strings.Contains(l, "depends:") {
			lines[i] = strings.Replace(l, "-->", "depends:1 -->", 1)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo")
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	if !strings.Contains(stdout, "(cycle)") {
		t.Fatalf("expected '(cycle)' annotation in output, got:\n%s", stdout)
	}
	// All three should still be present.
	for _, want := range []string{"alpha", "beta", "gamma"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q to appear (cycles are emitted, not dropped), got:\n%s", want, stdout)
		}
	}
}

// TestTopoJSONShape: --json emits a stable array of objects with
// id/title/priority/done. Empty pool → "[]\n".
func TestTopoJSONShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--json")
	if err != nil {
		t.Fatalf("topo --json: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d:\n%s", len(rows), stdout)
	}
	if int(rows[0]["id"].(float64)) != 1 || int(rows[1]["id"].(float64)) != 2 {
		t.Fatalf("expected ids [1, 2], got:\n%s", stdout)
	}
	// Required keys.
	for _, key := range []string{"id", "title", "priority", "done"} {
		if _, ok := rows[0][key]; !ok {
			t.Fatalf("missing key %q in row 0", key)
		}
	}
}

// TestTopoJSONEmpty: caught-up store emits "[]\n", not "null".
func TestTopoJSONEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--json")
	if err != nil {
		t.Fatalf("topo --json empty: %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("expected '[]' for empty, got %q", stdout)
	}
}

// TestTopoIDsOnly: --ids emits comma-separated ids for pipelining.
func TestTopoIDsOnly(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend 3->2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2->1: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--ids")
	if err != nil {
		t.Fatalf("topo --ids: %v", err)
	}
	got := strings.TrimSpace(stdout)
	if got != "1,2,3" {
		t.Fatalf("expected '1,2,3', got %q", got)
	}
}

// TestTopoMutuallyExclusiveOutputs: --json + --ids should fail
// with a usage error (exit 2).
func TestTopoMutuallyExclusiveOutputs(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "topo", "--json", "--ids")
	if err == nil {
		t.Fatal("expected error for --json + --ids combo")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestTopoDOTFormat: --format dot emits valid DOT scaffolding with
// nodes + edges. We're not parsing the DOT (out of scope), just
// asserting the obvious skeletons.
func TestTopoDOTFormat(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo", "--format", "dot")
	if err != nil {
		t.Fatalf("topo --format dot: %v", err)
	}
	if !strings.HasPrefix(stdout, "digraph tsk_topo {") {
		t.Fatalf("expected DOT skeleton, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "2 -> 1;") {
		t.Fatalf("expected edge 2 -> 1, got:\n%s", stdout)
	}
}

// TestTopoEmptyStore: no tasks at all yields "no tasks".
func TestTopoEmptyStore(t *testing.T) {
	dir := t.TempDir()
	// Need an init or the resolveStore call complains; create an empty .tsk.md.
	path := filepath.Join(dir, ".tsk.md")
	if err := os.WriteFile(path, []byte("# tasks\n\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo")
	if err != nil {
		t.Fatalf("topo empty: %v", err)
	}
	if !strings.Contains(stdout, "no tasks") {
		t.Fatalf("expected 'no tasks', got:\n%s", stdout)
	}
}

// TestTopoBraidPrereqsRespectsTieBreak: two parallel roots A
// (urgent) and B (low) both feed into C. Topo should emit A
// before B (urgent wins the tie), then C.
func TestTopoBraidPrereqsRespectsTieBreak(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "urgent root", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "low root", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "merge"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo")
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	iUrgent := strings.Index(stdout, "urgent root")
	iLow := strings.Index(stdout, "low root")
	iMerge := strings.Index(stdout, "merge")
	if !(iUrgent >= 0 && iLow > iUrgent && iMerge > iLow) {
		t.Fatalf("expected urgent < low < merge, got (%d, %d, %d):\n%s",
			iUrgent, iLow, iMerge, stdout)
	}
}

// TestTopoDanglingDepTreatedSatisfied: a task whose only dep is a
// missing id should NOT be blocked — matches unmetBlockers' policy
// (dangling refs are surfaced by `tsk lint`, not enforcement).
func TestTopoDanglingDepTreatedSatisfied(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Hand-edit: set depends:99 on task 1 (id 99 doesn't exist).
	path := filepath.Join(dir, ".tsk.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(string(body), "\n")
	for i, l := range lines {
		if strings.Contains(l, "id:1 ") && !strings.Contains(l, "depends:") {
			lines[i] = strings.Replace(l, "-->", "depends:99 -->", 1)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "topo")
	if err != nil {
		t.Fatalf("topo: %v", err)
	}
	if !strings.Contains(stdout, "real") {
		t.Fatalf("expected real to appear (dangling dep is satisfied), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "(cycle)") {
		t.Fatalf("dangling deps should NOT trigger cycle annotation, got:\n%s", stdout)
	}
}
