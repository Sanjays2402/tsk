package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestPauseClearsStarted: pause behaves exactly like stop — clears
// the started: timestamp on a previously-started task.
func TestPauseClearsStarted(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "1")
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if !strings.Contains(stdout, "stopped 1 task(s)") {
		t.Fatalf("expected 'stopped 1 task(s)' (pause shares stop's body), got:\n%s", stdout)
	}
	body := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(body, "started:") {
		t.Fatalf("expected 'started:' gone after pause, got:\n%s", body)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.Tasks[0].Started != nil {
		t.Fatal("expected Started nil after pause")
	}
}

// TestPauseIdempotent: pausing a never-started task is a no-op,
// matching stop's contract.
func TestPauseIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "1")
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	if !strings.Contains(stdout, "no change") {
		t.Fatalf("expected 'no change' on never-started task, got:\n%s", stdout)
	}
}

// TestPauseMultiID: multi-id support comes free from runStartStop's
// shared body — verify the wiring still surfaces it through pause.
func TestPauseMultiID(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "start", "1", "2", "3"); err != nil {
		t.Fatalf("start multi: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "pause", "1", "2", "3")
	if err != nil {
		t.Fatalf("pause multi: %v", err)
	}
	if !strings.Contains(stdout, "3 task(s)") {
		t.Fatalf("expected '3 task(s)' confirmation, got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	for _, task := range s.Tasks {
		if task.Started != nil {
			t.Fatalf("task #%d still started after pause", task.ID)
		}
	}
}

// TestPauseStopProduceSameOutput: pause and stop must produce
// byte-identical output for the same input. This is the regression
// guard against future drift between the two surfaces — they share
// runStartStop today, but a refactor could split them.
func TestPauseStopProduceSameOutput(t *testing.T) {
	mkDir := func() string {
		d := t.TempDir()
		if _, _, err := runCmd(t, d, "add", "thing"); err != nil {
			t.Fatalf("add: %v", err)
		}
		if _, _, err := runCmd(t, d, "start", "1"); err != nil {
			t.Fatalf("start: %v", err)
		}
		return d
	}
	pauseDir := mkDir()
	stopDir := mkDir()
	pauseOut, _, err := runCmd(t, pauseDir, "pause", "1")
	if err != nil {
		t.Fatalf("pause: %v", err)
	}
	stopOut, _, err := runCmd(t, stopDir, "stop", "1")
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if pauseOut != stopOut {
		t.Fatalf("pause and stop must produce identical output\nPAUSE:\n%s\nSTOP:\n%s", pauseOut, stopOut)
	}
}

// TestPauseAliasHold: the 'hold' alias works.
func TestPauseAliasHold(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, _, err := runCmd(t, dir, "hold", "1"); err != nil {
		t.Fatalf("hold: %v", err)
	}
	body := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(body, "started:") {
		t.Fatalf("'hold' alias should clear started:, got:\n%s", body)
	}
}

// TestPauseRejectsDoneTask: must inherit start/stop's "no transitions
// on done tasks" guard.
func TestPauseRejectsDoneTask(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "pause", "1")
	if err == nil {
		t.Fatal("expected error pausing a done task")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}
