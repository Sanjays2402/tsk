package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestFreezeHidesFromDefaultLs: freezing a task hides it from default
// ls (it's a waiting task now) and surfaces it via --include-waiting.
func TestFreezeHidesFromDefaultLs(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "visible"); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "frozen"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "freeze", "2"); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if strings.Contains(stdout, "frozen") {
		t.Fatalf("frozen task should be hidden by default ls:\n%s", stdout)
	}
	if !strings.Contains(stdout, "visible") {
		t.Fatalf("non-frozen task should still appear:\n%s", stdout)
	}
	// --include-waiting surfaces it.
	stdout, _, err = runCmd(t, dir, "ls", "--include-waiting")
	if err != nil {
		t.Fatalf("ls --include-waiting: %v", err)
	}
	if !strings.Contains(stdout, "frozen") {
		t.Fatalf("--include-waiting should surface frozen tasks:\n%s", stdout)
	}
}

// TestFreezePersistsAsWaitMetadata: the freeze sentinel date
// (2099-12-31) lands in the meta block under the `wait:` key — so
// existing wait machinery (filters, --clear, --list) works without
// any new persisted state.
func TestFreezePersistsAsWaitMetadata(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "freeze", "1"); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	body := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(body, "wait:2099-12-31") {
		t.Fatalf("expected wait:2099-12-31 sentinel in file, got:\n%s", body)
	}
	// And it round-trips through Load.
	s, err := store.Load(filepath.Join(dir, ".tsk.md"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !IsFrozen(s.Tasks[0]) {
		t.Fatalf("expected IsFrozen on reload, got WaitUntil=%v", s.Tasks[0].WaitUntil)
	}
}

// TestFreezeMultipleIDsAtOnce: cobra MinimumNArgs(1) plus the
// parseTaskIDs dedupe should let `freeze 3 5 7` flip three tasks in
// a single Save.
func TestFreezeMultipleIDsAtOnce(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "freeze", "1", "2", "3")
	if err != nil {
		t.Fatalf("freeze multi: %v", err)
	}
	if !strings.Contains(stdout, "frozen 3 task(s)") {
		t.Fatalf("expected '3 task(s)' confirmation, got:\n%s", stdout)
	}
}

// TestFreezeIdempotent: freezing an already-frozen task is a no-op
// (the "3 already frozen" message guards against people thinking
// they accidentally double-applied something).
func TestFreezeIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "freeze", "1"); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "freeze", "1")
	if err != nil {
		t.Fatalf("re-freeze: %v", err)
	}
	if !strings.Contains(stdout, "no change") {
		t.Fatalf("expected 'no change' on re-freeze, got:\n%s", stdout)
	}
}

// TestThawClearsWait: thaw is the inverse of freeze; it should
// clear the wait date and the task should re-appear in ls.
func TestThawClearsWait(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "deferred"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "freeze", "1"); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if strings.Contains(stdout, "deferred") {
		t.Fatalf("frozen task should be hidden, got:\n%s", stdout)
	}
	if _, _, err := runCmd(t, dir, "thaw", "1"); err != nil {
		t.Fatalf("thaw: %v", err)
	}
	stdout, _, err = runCmd(t, dir, "ls")
	if err != nil {
		t.Fatalf("ls after thaw: %v", err)
	}
	if !strings.Contains(stdout, "deferred") {
		t.Fatalf("thawed task should re-appear:\n%s", stdout)
	}
}

// TestThawHandlesNonFrozenWaiting: a task waiting on a normal date
// (not the freeze sentinel) is also clearable via thaw — the verb
// works on any wait, not just freeze-shaped ones.
func TestThawHandlesNonFrozenWaiting(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "wait", "1", "2099-01-01"); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if _, _, err := runCmd(t, dir, "thaw", "1"); err != nil {
		t.Fatalf("thaw: %v", err)
	}
	s, err := store.Load(filepath.Join(dir, ".tsk.md"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.Tasks[0].WaitUntil != nil {
		t.Fatalf("expected WaitUntil cleared, got %v", s.Tasks[0].WaitUntil)
	}
}

// TestThawIdempotent: thawing a non-waiting task is a no-op.
func TestThawIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "thaw", "1")
	if err != nil {
		t.Fatalf("thaw: %v", err)
	}
	if !strings.Contains(stdout, "no change") {
		t.Fatalf("expected 'no change' on thaw of non-frozen, got:\n%s", stdout)
	}
}

// TestFreezeRejectsBadID: a non-existent id aborts before any save
// (so a multi-id call with one bad id leaves the file untouched).
func TestFreezeRejectsBadID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "freeze", "1", "99")
	if err == nil {
		t.Fatal("expected error for non-existent id")
	}
	// Task 1 must NOT have been touched.
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if IsFrozen(s.Tasks[0]) {
		t.Fatal("partial apply leaked: task 1 was frozen despite the call failing")
	}
}

// TestIsFrozenSentinelOnly: only the exact freeze sentinel date
// counts. A wait on 2099-12-30 is "waiting", not "frozen".
func TestIsFrozenSentinelOnly(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "wait", "1", "2099-12-30"); err != nil {
		t.Fatalf("wait: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if IsFrozen(s.Tasks[0]) {
		t.Fatal("a non-sentinel wait date should not count as frozen")
	}
}

// TestFreezeWaitListSurfacesIt: `tsk wait --list` shows frozen tasks
// (they ARE waiting tasks, just with a far-future date).
func TestFreezeWaitListSurfacesIt(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "deep-freeze"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "freeze", "1"); err != nil {
		t.Fatalf("freeze: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "wait", "--list")
	if err != nil {
		t.Fatalf("wait --list: %v", err)
	}
	if !strings.Contains(stdout, "deep-freeze") {
		t.Fatalf("wait --list should surface frozen tasks:\n%s", stdout)
	}
	if !strings.Contains(stdout, "2099-12-31") {
		t.Fatalf("wait --list should show the freeze sentinel date:\n%s", stdout)
	}
}
