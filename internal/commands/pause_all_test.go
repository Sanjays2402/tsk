package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestPauseAllClearsEveryStarted: with --all, every task that is
// currently in-progress is paused in a single Save. Tasks that
// were never started stay untouched (Started already nil).
func TestPauseAllClearsEveryStarted(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// Start 1, 2, 3. Leave 4 alone.
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all")
	if err != nil {
		t.Fatalf("pause --all: %v", err)
	}
	if !strings.Contains(stdout, "stopped 3 task(s)") {
		t.Fatalf("expected 'stopped 3 task(s)' (pause --all delegates to runStartStop), got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	for _, task := range s.Tasks {
		if task.Started != nil {
			t.Fatalf("task #%d still started after pause --all", task.ID)
		}
	}
}

// TestPauseAllEmptySet: with no tasks in-progress, --all prints the
// same "no in-progress tasks" message `tsk wip` uses so the two
// verbs answer the empty case identically.
func TestPauseAllEmptySet(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all")
	if err != nil {
		t.Fatalf("pause --all on empty set: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress tasks") {
		t.Fatalf("expected 'no in-progress tasks' on empty set, got:\n%s", stdout)
	}
}

// TestPauseAllRejectsPositionalIDs: --all and positional ids are
// mutually exclusive — combining them would hide a typo (e.g.
// `tsk pause --all 3` could plausibly mean "everything except 3").
func TestPauseAllRejectsPositionalIDs(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "pause", "--all", "1")
	if err == nil {
		t.Fatal("expected error combining --all with positional id")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage error), got %v", err)
	}
}

// TestPauseNoArgsErrors: pause with no args and no --all is a
// usage error (used to be ExactArgs >=1 in cobra; explicit message
// now lives in the RunE).
func TestPauseNoArgsErrors(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "pause")
	if err == nil {
		t.Fatal("expected error with no args + no --all")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage error), got %v", err)
	}
}

// TestPauseAllSkipsDoneTasks: a done task that happened to once be
// in-progress should not be in the wip set anymore (done clears
// started:), so pause --all should ignore it. Belt-and-suspenders
// regression — protects against the in-progress filter ever
// drifting from runStartStop's "no transitions on done" guard.
func TestPauseAllSkipsDoneTasks(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// #2 is open + not started. Set should be empty.
	stdout, _, err := runCmd(t, dir, "pause", "--all")
	if err != nil {
		t.Fatalf("pause --all: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress tasks") {
		t.Fatalf("expected 'no in-progress tasks' (done #1 cleared started, #2 never started), got:\n%s", stdout)
	}
}

// TestInProgressIDsHelperOrdering: the helper that resolves the
// in-progress set must return ids sorted ascending so the
// runStartStop summary line is reproducible and tests are stable.
func TestInProgressIDsHelperOrdering(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// Start in reverse-id order to make sure the helper sorts.
	for _, id := range []string{"3", "2", "1"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	ids := inProgressIDs(s.Tasks)
	if len(ids) != 3 {
		t.Fatalf("expected 3 in-progress ids, got %d", len(ids))
	}
	for i, want := range []int{1, 2, 3} {
		if ids[i] != want {
			t.Fatalf("ids[%d]: want %d, got %d (full slice %v)", i, want, ids[i], ids)
		}
	}
}
