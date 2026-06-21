package commands

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestStartStampsStartedMeta: `tsk start <id>` sets Started and
// persists `started:<RFC3339>` in the file.
func TestStartStampsStartedMeta(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "the work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	body := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(body, "started:") {
		t.Fatalf("expected 'started:' in file, got:\n%s", body)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.Tasks[0].Started == nil {
		t.Fatal("expected Started set after start")
	}
	if !s.Tasks[0].IsInProgress() {
		t.Fatal("expected IsInProgress true")
	}
}

// TestStopClearsStarted: stop nulls Started; the meta key disappears.
func TestStopClearsStarted(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, _, err := runCmd(t, dir, "stop", "1"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	body := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(body, "started:") {
		t.Fatalf("expected 'started:' gone after stop, got:\n%s", body)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.Tasks[0].Started != nil {
		t.Fatal("expected Started nil after stop")
	}
}

// TestStartIdempotent: starting an already-started task is a no-op and
// preserves the original timestamp (defensive against accidentally
// zeroing elapsed time).
func TestStartIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	first := *s.Tasks[0].Started
	time.Sleep(1100 * time.Millisecond)
	stdout, _, err := runCmd(t, dir, "start", "1")
	if err != nil {
		t.Fatalf("start again: %v", err)
	}
	if !strings.Contains(stdout, "no change") {
		t.Fatalf("expected 'no change', got:\n%s", stdout)
	}
	s, _ = store.Load(filepath.Join(dir, ".tsk.md"))
	if !s.Tasks[0].Started.Equal(first) {
		t.Fatalf("started timestamp should be preserved, got %v want %v",
			s.Tasks[0].Started, first)
	}
}

// TestStartResetBumpsTimestamp: --reset bumps started: to now even if
// already started.
func TestStartResetBumpsTimestamp(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	first := *s.Tasks[0].Started
	time.Sleep(1100 * time.Millisecond)
	if _, _, err := runCmd(t, dir, "start", "1", "--reset"); err != nil {
		t.Fatalf("start --reset: %v", err)
	}
	s, _ = store.Load(filepath.Join(dir, ".tsk.md"))
	if !s.Tasks[0].Started.After(first) {
		t.Fatalf("--reset should bump started, before=%v after=%v",
			first, s.Tasks[0].Started)
	}
}

// TestDoneClearsStarted: marking a task done drops the Started
// timestamp (Completed wins; we don't keep stale data).
func TestDoneClearsStarted(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	body := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(body, "started:") {
		t.Fatalf("expected 'started:' cleared after done, got:\n%s", body)
	}
	if !strings.Contains(body, "completed:") {
		t.Fatalf("expected 'completed:' present, got:\n%s", body)
	}
}

// TestStartRejectsDoneTask: starting a done task is an error (forces
// the user to reopen first — implicit transitions hide bugs).
func TestStartRejectsDoneTask(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	_, _, err := runCmd(t, dir, "start", "1")
	if err == nil {
		t.Fatal("expected error starting a done task")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected ExitCode 2, got %v", err)
	}
}

// TestStartMultiID: 3 ids in one call, all flipped, single confirmation.
func TestStartMultiID(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "start", "1", "2", "3")
	if err != nil {
		t.Fatalf("start multi: %v", err)
	}
	if !strings.Contains(stdout, "started 3 task(s)") {
		t.Fatalf("expected '3 task(s)' confirmation, got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	for _, task := range s.Tasks {
		if task.Started == nil {
			t.Fatalf("task #%d not started", task.ID)
		}
	}
}

// TestStartRejectsBadID: validation up-front (no partial state).
func TestStartRejectsBadID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "start", "1", "99")
	if err == nil {
		t.Fatal("expected error for bad id")
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.Tasks[0].Started != nil {
		t.Fatal("partial apply leaked: task 1 was started despite failure")
	}
}

// TestStopIdempotent: stopping a non-started task is a no-op.
func TestStopIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "stop", "1")
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if !strings.Contains(stdout, "no change") {
		t.Fatalf("expected 'no change', got:\n%s", stdout)
	}
}

// TestInProgressListsStartedTasks: the new view surfaces only
// in-progress tasks, sorted most-recent-start first.
func TestInProgressListsStartedTasks(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"first", "second", "third"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	if _, _, err := runCmd(t, dir, "start", "3"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "in-progress")
	if err != nil {
		t.Fatalf("in-progress: %v", err)
	}
	if !strings.Contains(stdout, "first") || !strings.Contains(stdout, "third") {
		t.Fatalf("expected both started tasks, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "second") {
		t.Fatalf("non-started 'second' should NOT appear:\n%s", stdout)
	}
	// Most-recent first: 'third' (just started) before 'first'.
	if strings.Index(stdout, "third") > strings.Index(stdout, "first") {
		t.Fatalf("expected most-recent first (third before first), got:\n%s", stdout)
	}
}

// TestInProgressEmpty: empty list message instead of silent output.
func TestInProgressEmpty(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "in-progress")
	if err != nil {
		t.Fatalf("in-progress: %v", err)
	}
	if !strings.Contains(stdout, "no in-progress") {
		t.Fatalf("expected 'no in-progress', got:\n%s", stdout)
	}
}

// TestInProgressJSON: --json emits a stable array; empty case is [].
func TestInProgressJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "in-progress", "--json")
	if err != nil {
		t.Fatalf("in-progress --json: %v", err)
	}
	got := strings.TrimSpace(stdout)
	if got != "[]" {
		t.Fatalf("expected '[]' for empty case, got %q", got)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err = runCmd(t, dir, "in-progress", "--json")
	if err != nil {
		t.Fatalf("in-progress --json: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(stdout), &arr); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 task in JSON, got %d", len(arr))
	}
}

// TestStartedSurfacesInShow: `tsk show <id>` reveals the started:
// line when set.
func TestStartedSurfacesInShow(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(stdout, "started:") {
		t.Fatalf("expected 'started:' line in show, got:\n%s", stdout)
	}
}

// TestStartedAliasWip: the 'wip' alias works.
func TestStartedAliasWip(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wip")
	if err != nil {
		t.Fatalf("wip: %v", err)
	}
	if !strings.Contains(stdout, "#1") {
		t.Fatalf("expected #1 in wip output, got:\n%s", stdout)
	}
}

// TestHumanizeElapsedBuckets: format choices across the unit thresholds.
func TestHumanizeElapsedBuckets(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "<1m"},
		{59 * time.Second, "<1m"},
		{1 * time.Minute, "1m"},
		{45 * time.Minute, "45m"},
		{1 * time.Hour, "1h"},
		{23 * time.Hour, "23h"},
		{24 * time.Hour, "1d"},
		{72 * time.Hour, "3d"},
	}
	for _, c := range cases {
		if got := humanizeElapsed(c.d); got != c.want {
			t.Errorf("humanizeElapsed(%s) = %q want %q", c.d, got, c.want)
		}
	}
}

// TestStartedRoundtripsAcrossSave: write a started: meta by hand,
// load, re-save, and assert the key is still there. Guards against
// the meta key being dropped on the next save (which would silently
// lose the timestamp).
func TestStartedRoundtripsAcrossSave(t *testing.T) {
	dir := t.TempDir()
	ts := time.Now().UTC().Format(time.RFC3339)
	writeRawTasks(t, dir,
		"- [ ] hand <!-- id:1 prio:high started:"+ts+" -->",
	)
	// Trigger a Save (renames are a one-shot mutation).
	if _, _, err := runCmd(t, dir, "rename", "1", "renamed"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	body := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(body, "started:") {
		t.Fatalf("expected 'started:' preserved across save, got:\n%s", body)
	}
}
