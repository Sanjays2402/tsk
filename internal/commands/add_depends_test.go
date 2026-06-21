package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestAddDependsPersistsAtCreation: `tsk add --depends 1,2` should
// land the new task with DependsOn={1,2} in one shot — no follow-up
// `tsk depend` call needed.
func TestAddDependsPersistsAtCreation(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq-a", "prereq-b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "add", "dependent", "--depends", "1,2")
	if err != nil {
		t.Fatalf("add --depends: %v", err)
	}
	if !strings.Contains(stdout, "depends on #1, #2") {
		t.Fatalf("expected friendly 'depends on' annotation, got:\n%s", stdout)
	}
	// File should have depends:1,2 in meta.
	body := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(body, "depends:1,2") {
		t.Fatalf("expected 'depends:1,2' in file, got:\n%s", body)
	}
	// Round-trip through store.Load.
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	got := s.Tasks[2].DependsOn
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("expected DependsOn={1,2}, got %v", got)
	}
}

// TestAddDependsAcceptsHashAndDedupes: "#1,#1,2,#2" should reduce
// to {1, 2}.
func TestAddDependsAcceptsHashAndDedupes(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "add", "tt", "--depends", "#1,#1,2,#2"); err != nil {
		t.Fatalf("add --depends with hash+dupes: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	got := s.Tasks[2].DependsOn
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("expected DependsOn={1,2} after dedupe, got %v", got)
	}
}

// TestAddDependsRejectsMissingID: --depends pointing to a non-
// existent task must fail BEFORE the new task lands. The store
// stays at its pre-call task count.
func TestAddDependsRejectsMissingID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// 99 doesn't exist.
	_, _, err := runCmd(t, dir, "add", "y", "--depends", "99")
	if err == nil {
		t.Fatal("expected error for missing dep id")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2 (usage error), got %v", err)
	}
	// Store must still have exactly one task.
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if len(s.Tasks) != 1 {
		t.Fatalf("expected 1 task (no half-commit), got %d", len(s.Tasks))
	}
	if s.Tasks[0].Title != "x" {
		t.Fatalf("expected original task preserved, got %q", s.Tasks[0].Title)
	}
}

// TestAddDependsRejectsBadToken: a non-numeric token should error
// up-front without touching the store.
func TestAddDependsRejectsBadToken(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "add", "y", "--depends", "banana,1")
	if err == nil {
		t.Fatal("expected error for non-numeric dep token")
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if len(s.Tasks) != 1 {
		t.Fatalf("expected 1 task after bad --depends, got %d", len(s.Tasks))
	}
}

// TestAddDependsBlocksDone: a task added with --depends must
// actually be blocked from `done` until prereqs close, proving the
// deps are wired through the same enforcement path as `tsk depend`.
func TestAddDependsBlocksDone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "dependent", "--depends", "1"); err != nil {
		t.Fatalf("add --depends: %v", err)
	}
	// Done #2 should fail with the dependency error.
	_, _, err := runCmd(t, dir, "done", "2")
	if err == nil {
		t.Fatal("expected blocked-by error closing #2")
	}
	if !strings.Contains(err.Error(), "#1") {
		t.Fatalf("expected error to name blocker #1, got: %v", err)
	}
	// After closing #1, #2 can close.
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done prereq: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done dependent after prereq: %v", err)
	}
}

// TestAddDependsComposesWithOtherFlags: --depends should compose
// with -p/-t/-d/-n (no flag eats another's value, no flag wins
// silently).
func TestAddDependsComposesWithOtherFlags(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "prereq"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "full",
		"-p", "urgent",
		"-t", "dev",
		"-t", "blocker",
		"-d", "2099-12-31",
		"-n", "watch this one",
		"--depends", "1"); err != nil {
		t.Fatalf("add full: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if len(s.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(s.Tasks))
	}
	got := s.Tasks[1]
	if got.Title != "full" {
		t.Fatalf("title: want full, got %q", got.Title)
	}
	if got.Priority.String() != "urgent" {
		t.Fatalf("priority: want urgent, got %s", got.Priority)
	}
	if len(got.DependsOn) != 1 || got.DependsOn[0] != 1 {
		t.Fatalf("DependsOn: want [1], got %v", got.DependsOn)
	}
	if len(got.Tags) != 2 {
		t.Fatalf("tags: want 2, got %v", got.Tags)
	}
	if got.Due == nil {
		t.Fatal("expected due date set")
	}
	if got.Notes != "watch this one" {
		t.Fatalf("notes: want 'watch this one', got %q", got.Notes)
	}
}

// TestAddDependsEmptyValueIsNoop: --depends "" should behave just
// like the flag wasn't passed (no DependsOn, no fancy "depends on"
// annotation, same output shape).
func TestAddDependsEmptyValueIsNoop(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runCmd(t, dir, "add", "x", "--depends", "")
	if err != nil {
		t.Fatalf("add --depends empty: %v", err)
	}
	if strings.Contains(stdout, "depends on") {
		t.Fatalf("empty --depends should not add 'depends on' note, got:\n%s", stdout)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.Tasks[0].HasDependencies() {
		t.Fatalf("expected no DependsOn for empty --depends, got %v", s.Tasks[0].DependsOn)
	}
}
