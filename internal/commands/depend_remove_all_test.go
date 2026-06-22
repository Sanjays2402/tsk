package commands

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestDependRemoveAllScrubFromEveryDependent: the global sweep.
// Several tasks depend on #1; --remove-all #1 must drop #1 from
// every dependent's DependsOn in a single Save.
func TestDependRemoveAllScrubFromEveryDependent(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// #2, #3, #4 each depend on #1.
	for _, id := range []string{"2", "3", "4"} {
		if _, _, err := runCmd(t, dir, "depend", id, "--on", "1"); err != nil {
			t.Fatalf("depend %s: %v", id, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--remove-all")
	if err != nil {
		t.Fatalf("depend 1 --remove-all: %v", err)
	}
	if !strings.Contains(stdout, "scrubbed #1 from 3 task(s)") {
		t.Fatalf("expected 'scrubbed #1 from 3 task(s)', got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	for _, t2 := range s.Tasks {
		for _, dep := range t2.DependsOn {
			if dep == 1 {
				t.Fatalf("task #%d still lists #1 after --remove-all: deps=%v", t2.ID, t2.DependsOn)
			}
		}
	}
}

// TestDependRemoveAllPreservesOtherDeps: scrubbing #1 must NOT
// touch a dependent's other prereqs. If #3 depends on [1,2,5],
// after --remove-all #1, deps must be [2,5] in that order.
func TestDependRemoveAllPreservesOtherDeps(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d", "e"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// #5 depends on 1,2,3 (set via --on so the list is sorted: 1,2,3).
	if _, _, err := runCmd(t, dir, "depend", "5", "--on", "1,2,3"); err != nil {
		t.Fatalf("depend 5: %v", err)
	}
	// #4 depends on just 1.
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "1"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "1", "--remove-all"); err != nil {
		t.Fatalf("depend 1 --remove-all: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	t4 := s.ByID(4)
	if t4 == nil {
		t.Fatal("task #4 missing")
	}
	if len(t4.DependsOn) != 0 {
		t.Fatalf("#4 should have empty deps after --remove-all (only had #1), got %v", t4.DependsOn)
	}
	t5 := s.ByID(5)
	if t5 == nil {
		t.Fatal("task #5 missing")
	}
	if len(t5.DependsOn) != 2 || t5.DependsOn[0] != 2 || t5.DependsOn[1] != 3 {
		t.Fatalf("#5 deps after --remove-all should be [2,3] (order preserved), got %v", t5.DependsOn)
	}
}

// TestDependRemoveAllNoOpOnUnreferencedID: scrubbing an id that no
// task depends on is a clean no-op with a clear message and no
// .bak chain churn.
func TestDependRemoveAllNoOpOnUnreferencedID(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// #2 depends on nothing related to #1.
	stdout, _, err := runCmd(t, dir, "depend", "1", "--remove-all")
	if err != nil {
		t.Fatalf("depend 1 --remove-all: %v", err)
	}
	if !strings.Contains(stdout, "no tasks depend on #1") {
		t.Fatalf("expected 'no tasks depend on #1', got:\n%s", stdout)
	}
}

// TestDependRemoveAllMissingIDAlsoNoOp: --remove-all on an id NOT
// in the store is a no-op (vacuously: "nothing depends on a
// missing id"). The use case is "scrub a freshly-removed task"
// — forcing an existence check first would defeat the ergonomic.
func TestDependRemoveAllMissingIDAlsoNoOp(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "99", "--remove-all")
	if err != nil {
		t.Fatalf("depend 99 --remove-all (missing id should be no-op): %v", err)
	}
	if !strings.Contains(stdout, "no tasks depend on #99") {
		t.Fatalf("expected 'no tasks depend on #99', got:\n%s", stdout)
	}
}

// TestDependRemoveAllJSON: structured output for scripts.
// {"id": N, "touched": [<ids>]}. Empty touched = [] not null.
func TestDependRemoveAllJSON(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend 3: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--remove-all", "--json")
	if err != nil {
		t.Fatalf("depend 1 --remove-all --json: %v", err)
	}
	var doc struct {
		ID      int   `json:"id"`
		Touched []int `json:"touched"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("bad json output: %v\n%s", err, stdout)
	}
	if doc.ID != 1 {
		t.Fatalf("expected id=1, got %d", doc.ID)
	}
	if len(doc.Touched) != 2 {
		t.Fatalf("expected touched=[2,3], got %v", doc.Touched)
	}
	if doc.Touched[0] != 2 || doc.Touched[1] != 3 {
		t.Fatalf("touched order should be ascending [2,3], got %v", doc.Touched)
	}
}

// TestDependRemoveAllJSONEmptyTouchedArray: when nothing changes,
// json must emit "touched": [] not "touched": null — defensive
// against `jq '.touched | length'` consumers.
func TestDependRemoveAllJSONEmptyTouchedArray(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--remove-all", "--json")
	if err != nil {
		t.Fatalf("depend 1 --remove-all --json: %v", err)
	}
	if !strings.Contains(stdout, `"touched": []`) {
		t.Fatalf("expected 'touched: []' in JSON, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "null") {
		t.Fatalf("touched must never be null:\n%s", stdout)
	}
}

// TestDependRemoveAllRejectsMutexFlags: --remove-all is mutually
// exclusive with every other mutation flag and every read-only
// flag. Each combination must exit-2 with a clear message.
func TestDependRemoveAllRejectsMutexFlags(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	mutexCases := [][]string{
		{"depend", "1", "--remove-all", "--on", "2"},
		{"depend", "1", "--remove-all", "--add", "2"},
		{"depend", "1", "--remove-all", "--remove", "2"},
		{"depend", "1", "--remove-all", "--clear"},
		{"depend", "1", "--remove-all", "--tree"},
		{"depend", "1", "--remove-all", "--justify"},
		{"depend", "1", "--remove-all", "--upstream"},
	}
	for _, args := range mutexCases {
		_, _, err := runCmd(t, dir, args...)
		if err == nil {
			t.Fatalf("expected error for combo %v", args)
		}
		var ec ExitCoder
		if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
			t.Fatalf("combo %v: expected exit 2, got %v", args, err)
		}
	}
}

// TestDependRemoveAllRequiresID: without a positional id the flag
// has no target — must exit-2.
func TestDependRemoveAllRequiresID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "depend", "--remove-all")
	if err == nil {
		t.Fatal("expected error for --remove-all without id")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestDependRemoveAllSingleSave: a multi-touch run must persist
// in ONE store.Save (the .bak chain doesn't get N entries for N
// touched tasks). Side-effect check: after the run, the .bak file
// reflects the PRE-RUN state (single snapshot), not an intermediate
// per-task state. Done by counting bytes; if Save ran per-task,
// the .bak would only show the last-but-one mutation. With a
// single Save it shows the original.
func TestDependRemoveAllSingleSave(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	for _, id := range []string{"2", "3", "4"} {
		if _, _, err := runCmd(t, dir, "depend", id, "--on", "1"); err != nil {
			t.Fatalf("depend %s: %v", id, err)
		}
	}
	// Snapshot the pre-run file content for comparison.
	preRun := readFile(t, filepath.Join(dir, ".tsk.md"))
	if _, _, err := runCmd(t, dir, "depend", "1", "--remove-all"); err != nil {
		t.Fatalf("depend 1 --remove-all: %v", err)
	}
	bak := readFile(t, filepath.Join(dir, ".tsk.md.bak"))
	if bak != preRun {
		t.Fatalf("expected .bak to match pre-run (single Save), got drift:\nPRE-RUN:\n%s\nBAK:\n%s", preRun, bak)
	}
}
