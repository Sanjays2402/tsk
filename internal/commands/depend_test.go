package commands

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestDependOnSetsAndPersists: --on persists `depends:` in meta and
// round-trips through store.Load.
func TestDependOnSetsAndPersists(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	body := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(body, "depends:1,2") {
		t.Fatalf("expected 'depends:1,2' in file, got:\n%s", body)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if got := s.Tasks[2].DependsOn; len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("expected DependsOn={1,2}, got %v", got)
	}
}

// TestDependBlocksDone: marking a blocked task done must FAIL with
// a usage-coded error and leave the file untouched.
func TestDependBlocksDone(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "blocked"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	_, _, err := runCmd(t, dir, "done", "2")
	if err == nil {
		t.Fatal("expected error marking blocked task done")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
	// Task 2 must NOT have been flipped.
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.Tasks[1].Done {
		t.Fatal("task 2 should still be open after blocked done")
	}
}

// TestDependBlocksUnblockedSucceeds: once the prereq is closed, done
// is allowed.
func TestDependBlockedAllowsAfterPrereqDone(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "blocked"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done prereq: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done blocked after prereq closed: %v", err)
	}
}

// TestDependSameBatchClose: `tsk done 1 2` should work when 2 depends
// on 1, because both are being closed in the same batch — the user's
// intent is clear and forcing arg order would be hostile.
func TestDependSameBatchClose(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "blocked"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2", "1"); err != nil {
		t.Fatalf("done batch (reversed order): %v", err)
	}
}

// TestDependRejectsSelf: a task can't depend on itself.
func TestDependRejectsSelf(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "depend", "1", "--on", "1")
	if err == nil {
		t.Fatal("expected error for self-dep")
	}
}

// TestDependRejectsDirectCycle: A depends on B → can't then set
// B depends on A.
func TestDependRejectsDirectCycle(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "1", "--on", "2"); err != nil {
		t.Fatalf("depend 1->2: %v", err)
	}
	_, _, err := runCmd(t, dir, "depend", "2", "--on", "1")
	if err == nil {
		t.Fatal("expected error for direct cycle 1↔2")
	}
}

// TestDependRejectsMissingID: deps must reference real tasks (the
// hand-edited dangling case is tolerated; the CLI is strict so the
// user doesn't typo).
func TestDependRejectsMissingID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "depend", "1", "--on", "99")
	if err == nil {
		t.Fatal("expected error for missing dep id")
	}
}

// TestDependAddAndRemove: incremental editing flows work.
func TestDependAddAndRemove(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--add", "2,3"); err != nil {
		t.Fatalf("depend --add: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if got := s.Tasks[3].DependsOn; len(got) != 3 {
		t.Fatalf("expected DependsOn len=3 after add, got %v", got)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--remove", "2"); err != nil {
		t.Fatalf("depend --remove: %v", err)
	}
	s, _ = store.Load(filepath.Join(dir, ".tsk.md"))
	for _, dep := range s.Tasks[3].DependsOn {
		if dep == 2 {
			t.Fatalf("expected 2 removed, got %v", s.Tasks[3].DependsOn)
		}
	}
}

// TestDependClear: --clear drops every dep.
func TestDependClear(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--clear"); err != nil {
		t.Fatalf("depend --clear: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if len(s.Tasks[1].DependsOn) != 0 {
		t.Fatalf("expected empty deps after clear, got %v", s.Tasks[1].DependsOn)
	}
	// And now done is unblocked.
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done after clear: %v", err)
	}
}

// TestDependInspectShowsOpenBlockers: the inspect mode separates the
// full dep list from the open-blockers subset.
func TestDependInspectShowsOpenBlockers(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done 1: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "3")
	if err != nil {
		t.Fatalf("depend inspect: %v", err)
	}
	if !strings.Contains(stdout, "#1") || !strings.Contains(stdout, "#2") {
		t.Fatalf("expected full dep list in inspect, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "open blockers: #2") {
		t.Fatalf("expected only #2 in open blockers, got:\n%s", stdout)
	}
}

// TestDependListSurfacesEveryBlocked: --list global view.
func TestDependListSurfacesEveryBlocked(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "--list", "--json")
	if err != nil {
		t.Fatalf("depend --list: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 blocked tasks, got %d: %v", len(rows), rows)
	}
}

// TestDependJSONShape: single-id inspect JSON has the documented fields.
func TestDependJSONShape(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "2", "--json")
	if err != nil {
		t.Fatalf("depend --json: %v", err)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(stdout), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	for _, k := range []string{"id", "depends_on", "blocking_open"} {
		if _, ok := obj[k]; !ok {
			t.Fatalf("missing key %q in JSON, got %v", k, obj)
		}
	}
}

// TestDependRoundTripPreservesAcrossSave: hand-edit deps in the file,
// load + save (via rename), assert deps survive.
func TestDependRoundTripPreservesAcrossSave(t *testing.T) {
	dir := t.TempDir()
	writeRawTasks(t, dir,
		"- [ ] prereq <!-- id:1 prio:medium -->",
		"- [ ] blocked <!-- id:2 prio:medium depends:1 -->",
	)
	if _, _, err := runCmd(t, dir, "rename", "2", "renamed"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	body := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(body, "depends:1") {
		t.Fatalf("expected 'depends:1' preserved across save, got:\n%s", body)
	}
}

// TestDependDanglingTolerated: a hand-edited dangling dep doesn't
// crash and doesn't block done (no task to wait on).
func TestDependDanglingTolerated(t *testing.T) {
	dir := t.TempDir()
	writeRawTasks(t, dir,
		"- [ ] x <!-- id:1 prio:medium depends:99 -->",
	)
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done with dangling dep should succeed: %v", err)
	}
}

// TestDependMutexFlags: --on + --add etc. is a usage error.
func TestDependMutexFlags(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	_, _, err := runCmd(t, dir, "depend", "2", "--on", "1", "--add", "1")
	if err == nil {
		t.Fatal("expected mutex error")
	}
}
