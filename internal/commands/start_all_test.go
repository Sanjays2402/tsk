package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestStartAllByTagStartsMatching: --all --tag X stamps started:
// on every open task with the tag and leaves the rest alone.
func TestStartAllByTagStartsMatching(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha", "-t", "work"); err != nil {
		t.Fatalf("add alpha: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta", "-t", "work"); err != nil {
		t.Fatalf("add beta: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "gamma", "-t", "home"); err != nil {
		t.Fatalf("add gamma: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work")
	if err != nil {
		t.Fatalf("start --all --tag work: %v", err)
	}
	if !strings.Contains(stdout, "started 2 task(s)") {
		t.Fatalf("expected 'started 2 task(s)', got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	for _, t2 := range s.Tasks {
		switch t2.ID {
		case 1, 2:
			if t2.Started == nil {
				t.Fatalf("#%d should be started", t2.ID)
			}
		case 3:
			if t2.Started != nil {
				t.Fatalf("#%d (home tag) should NOT be started", t2.ID)
			}
		}
	}
}

// TestStartAllByPriorityStartsMatching: --all --priority urgent
// covers the priority-filter half of the verb.
func TestStartAllByPriorityStartsMatching(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "urgent1", "-p", "urgent"); err != nil {
		t.Fatalf("add urgent1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "low1", "-p", "low"); err != nil {
		t.Fatalf("add low1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "urgent2", "-p", "urgent"); err != nil {
		t.Fatalf("add urgent2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--priority", "urgent")
	if err != nil {
		t.Fatalf("start --all --priority urgent: %v", err)
	}
	if !strings.Contains(stdout, "started 2 task(s)") {
		t.Fatalf("expected 'started 2 task(s)', got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.ByID(1).Started == nil || s.ByID(3).Started == nil {
		t.Fatal("urgent tasks should be started")
	}
	if s.ByID(2).Started != nil {
		t.Fatal("low task should NOT be started")
	}
}

// TestStartAllTagAndPriorityIntersects: when both filters set,
// only tasks matching BOTH (intersection) get started — matches
// the depend --pending tag+priority compose semantics.
func TestStartAllTagAndPriorityIntersects(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "work-high", "-t", "work", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "work-low", "-t", "work", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "home-high", "-t", "home", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work", "--priority", "high")
	if err != nil {
		t.Fatalf("start --all combined: %v", err)
	}
	if !strings.Contains(stdout, "started 1 task(s)") {
		t.Fatalf("expected exactly 1 (work AND high), got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.ByID(1).Started == nil {
		t.Fatal("#1 (work+high) should be started")
	}
	if s.ByID(2).Started != nil {
		t.Fatal("#2 (work but low) should NOT be started")
	}
	if s.ByID(3).Started != nil {
		t.Fatal("#3 (high but home) should NOT be started")
	}
}

// TestStartAllRequiresFilter: --all with no --tag and no --priority
// is rejected — the whole design refuses "start every open task
// in the store". Exit 2 (usage error).
func TestStartAllRequiresFilter(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "start", "--all")
	if err == nil {
		t.Fatal("expected error for --all with no filter")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 (usage error), got %v", err)
	}
}

// TestStartAllRejectsPositionalIDs: --all + positional ids is
// rejected — combining them would hide a typo (`tsk start --all 3`
// could mean "start everything except 3" or "start everything AND 3").
func TestStartAllRejectsPositionalIDs(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "start", "--all", "--tag", "x", "1")
	if err == nil {
		t.Fatal("expected error combining --all with positional id")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestStartAllEmptySetCleanNoop: a filter that matches no open
// tasks exits 0 with a clear "no open tasks match" line — typos
// in --tag must NOT crash a wrapper script.
func TestStartAllEmptySetCleanNoop(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "nonexistent")
	if err != nil {
		t.Fatalf("empty-set --all should not error, got: %v", err)
	}
	if !strings.Contains(stdout, "no open tasks match") {
		t.Fatalf("expected 'no open tasks match', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tag=nonexistent") {
		t.Fatalf("expected filter summary in empty message, got:\n%s", stdout)
	}
}

// TestStartAllExcludesDone: done tasks must NOT be in the
// start --all set — start/done is meaningless once a task is done.
func TestStartAllExcludesDone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "open", "-t", "work"); err != nil {
		t.Fatalf("add open: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "done-already", "-t", "work"); err != nil {
		t.Fatalf("add done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work")
	if err != nil {
		t.Fatalf("start --all: %v", err)
	}
	if !strings.Contains(stdout, "started 1 task(s)") {
		t.Fatalf("expected exactly 1 started (done excluded), got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.ByID(2).Started != nil {
		t.Fatal("#2 is done and must not have started: stamped")
	}
}

// TestStartAllIdempotentOnAlreadyStarted: tasks that are already
// started stay in the set but contribute zero changes (same
// idempotency contract as runStartStop's per-id path). The
// summary reads as "no change" when nothing actually flipped.
func TestStartAllIdempotentOnAlreadyStarted(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start 1: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "--all", "--tag", "work")
	if err != nil {
		t.Fatalf("start --all again: %v", err)
	}
	if !strings.Contains(stdout, "no change") {
		t.Fatalf("expected 'no change' for already-started, got:\n%s", stdout)
	}
}

// TestStartAllBadPriorityErrors: a malformed --priority value
// surfaces at parseAtFlag-time with exit 2 — silently degrading
// to no-filter would be confusing.
func TestStartAllBadPriorityErrors(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "start", "--all", "--priority", "wat")
	if err == nil {
		t.Fatal("expected error for bad --priority")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestStartAllPlainStartUnchanged: the original `tsk start <id>`
// path keeps working unchanged after the --all addition.
// Regression guard against the RunE refactor.
func TestStartAllPlainStartUnchanged(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "start", "1")
	if err != nil {
		t.Fatalf("plain start: %v", err)
	}
	if !strings.Contains(stdout, "started 1 task(s)") {
		t.Fatalf("plain start regression, got:\n%s", stdout)
	}
}
