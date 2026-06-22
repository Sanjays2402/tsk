package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestPauseAllByTagOnlyPausesMatching: --all --tag work pauses only
// in-progress tasks carrying the tag, leaves other wip tasks alone.
func TestPauseAllByTagOnlyPausesMatching(t *testing.T) {
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
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--tag", "work")
	if err != nil {
		t.Fatalf("pause --all --tag work: %v", err)
	}
	if !strings.Contains(stdout, "stopped 2 task(s)") {
		t.Fatalf("expected 'stopped 2 task(s)' (only work-tagged), got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.ByID(1).Started != nil {
		t.Fatal("#1 (work) should be paused")
	}
	if s.ByID(2).Started != nil {
		t.Fatal("#2 (work) should be paused")
	}
	if s.ByID(3).Started == nil {
		t.Fatal("#3 (home) should NOT have been paused")
	}
}

// TestPauseAllByPriorityOnlyPausesMatching: --all --priority urgent
// covers the priority-filter half (mirrors start --all's coverage).
func TestPauseAllByPriorityOnlyPausesMatching(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "u1", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "lo", "-p", "low"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "u2", "-p", "urgent"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start %s: %v", id, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--priority", "urgent")
	if err != nil {
		t.Fatalf("pause --all --priority urgent: %v", err)
	}
	if !strings.Contains(stdout, "stopped 2 task(s)") {
		t.Fatalf("expected 'stopped 2 task(s)' (only urgent), got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.ByID(2).Started == nil {
		t.Fatal("#2 (low) should NOT have been paused")
	}
}

// TestPauseAllTagAndPriorityIntersects: when both filters set, only
// tasks matching BOTH (intersection) get paused. Same compose
// semantics as start --all.
func TestPauseAllTagAndPriorityIntersects(t *testing.T) {
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
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--tag", "work", "--priority", "high")
	if err != nil {
		t.Fatalf("pause --all combined: %v", err)
	}
	if !strings.Contains(stdout, "stopped 1 task(s)") {
		t.Fatalf("expected 1 (work AND high), got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.ByID(1).Started != nil {
		t.Fatal("#1 (work+high) should be paused")
	}
	if s.ByID(2).Started == nil {
		t.Fatal("#2 (work but low) should NOT be paused")
	}
	if s.ByID(3).Started == nil {
		t.Fatal("#3 (high but home) should NOT be paused")
	}
}

// TestPauseAllNoFilterIsBackwardCompatible: --all with no filter
// keeps the original "pause every in-progress task" behavior.
// Regression guard against the runPauseAll refactor.
func TestPauseAllNoFilterIsBackwardCompatible(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	for _, id := range []string{"1", "2", "3"} {
		if _, _, err := runCmd(t, dir, "start", id); err != nil {
			t.Fatalf("start: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all")
	if err != nil {
		t.Fatalf("pause --all (no filter): %v", err)
	}
	if !strings.Contains(stdout, "stopped 3 task(s)") {
		t.Fatalf("expected all 3 paused (backward compat), got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	for _, task := range s.Tasks {
		if task.Started != nil {
			t.Fatalf("task #%d still started", task.ID)
		}
	}
}

// TestPauseAllNoFilterEmptyWipUnchanged: --all with no filter and
// no wip tasks reports "no in-progress tasks" (backward compat).
func TestPauseAllNoFilterEmptyWipUnchanged(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all")
	if err != nil {
		t.Fatalf("pause --all empty: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress tasks") {
		t.Fatalf("expected 'no in-progress tasks', got:\n%s", stdout)
	}
}

// TestPauseAllFilteredEmptyMatchUsesFilterWording: when WIP tasks
// exist but none match the filter, the empty message includes the
// filter summary (so a typo is immediately visible).
func TestPauseAllFilteredEmptyMatchUsesFilterWording(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--tag", "ghost")
	if err != nil {
		t.Fatalf("pause filtered empty: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress tasks match") {
		t.Fatalf("expected 'no in-progress tasks match', got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tag=ghost") {
		t.Fatalf("expected filter summary in message, got:\n%s", stdout)
	}
}

// TestPauseFilterFlagsRequireAll: --tag/--priority without --all is
// rejected (single-id pause is already explicit; bare per-id pause
// with --tag would be confusingly different from --all + filter).
func TestPauseFilterFlagsRequireAll(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "pause", "1", "--tag", "work")
	if err == nil {
		t.Fatal("expected error for --tag without --all on single-id pause")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestPauseAllBadPriorityErrors: a malformed --priority value
// surfaces at parsePendingPriority-time with exit 2.
func TestPauseAllBadPriorityErrors(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "pause", "--all", "--priority", "wat")
	if err == nil {
		t.Fatal("expected error for bad --priority")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestPauseAllFilteredDoneTasksExcluded: a task that was once
// in-progress but is now done shouldn't enter the filter's reach.
// Mirrors the existing PauseAllSkipsDoneTasks guard but with a
// filter applied.
func TestPauseAllFilteredDoneTasksExcluded(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "open-work", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "done-work", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "2"); err != nil {
		t.Fatalf("start 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "--all", "--tag", "work")
	if err != nil {
		t.Fatalf("pause --all --tag work with done: %v", err)
	}
	if !strings.Contains(stdout, "stopped 1 task(s)") {
		t.Fatalf("expected 1 paused (done excluded), got:\n%s", stdout)
	}
}
