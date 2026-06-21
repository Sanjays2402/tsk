package commands

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/store"
)

// TestMergeBasic: notes concatenate, tags union, victim disappears.
func TestMergeBasic(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha", "-t", "work"); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta", "-t", "personal"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "note", "1", "first note"); err != nil {
		t.Fatalf("note 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "note", "2", "second note"); err != nil {
		t.Fatalf("note 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "merge", "1", "2"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if len(s.Tasks) != 1 {
		t.Fatalf("expected 1 task after merge, got %d", len(s.Tasks))
	}
	t1 := s.ByID(1)
	if t1 == nil {
		t.Fatal("survivor #1 missing after merge")
	}
	// Notes concatenated with provenance line.
	if !strings.Contains(t1.Notes, "first note") || !strings.Contains(t1.Notes, "second note") {
		t.Fatalf("expected both notes preserved, got: %q", t1.Notes)
	}
	if !strings.Contains(t1.Notes, "--- merged from #2 ---") {
		t.Fatalf("expected provenance separator, got: %q", t1.Notes)
	}
	// Tags union (NormalizeTags sorts alphabetically).
	if !t1.HasTag("work") || !t1.HasTag("personal") {
		t.Fatalf("expected union tags {work, personal}, got %v", t1.Tags)
	}
	// Victim is gone.
	if s.ByID(2) != nil {
		t.Fatal("victim #2 should be removed after merge")
	}
}

// TestMergeBackRefRewrite: a third task that depended on the victim
// should now depend on the survivor instead.
func TestMergeBackRefRewrite(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"survivor", "victim", "downstream"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// downstream (3) depends on victim (2).
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "merge", "1", "2"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	t3 := s.ByID(3)
	if t3 == nil {
		t.Fatal("downstream #3 missing")
	}
	if len(t3.DependsOn) != 1 || t3.DependsOn[0] != 1 {
		t.Fatalf("expected downstream now depends on survivor #1, got %v", t3.DependsOn)
	}
}

// TestMergeBackRefDedupe: if a third task depended on BOTH survivor
// and victim, after rewrite it should depend on survivor exactly
// once (not twice).
func TestMergeBackRefDedupe(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"survivor", "victim", "downstream"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// downstream depends on BOTH survivor and victim.
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "merge", "1", "2"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	t3 := s.ByID(3)
	if len(t3.DependsOn) != 1 || t3.DependsOn[0] != 1 {
		t.Fatalf("expected DependsOn=[1] after dedup, got %v", t3.DependsOn)
	}
}

// TestMergeSelfMergeRejected: merging an id into itself is a usage
// error so typos don't silently no-op.
func TestMergeSelfMergeRejected(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "merge", "1", "1")
	if err == nil {
		t.Fatal("expected error for self-merge")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestMergeMutualDepRejected: if survivor depends on victim (or
// vice versa), the merge is refused so the user clears the
// relationship explicitly first.
func TestMergeMutualDepRejected(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "1", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	_, _, err := runCmd(t, dir, "merge", "1", "2")
	if err == nil {
		t.Fatal("expected error when survivor depends on victim")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestMergeDryRun: --dry-run prints a plan but does NOT write.
func TestMergeDryRun(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "merge", "1", "2", "--dry-run")
	if err != nil {
		t.Fatalf("merge dry-run: %v", err)
	}
	if !strings.Contains(stdout, "DRY RUN") {
		t.Fatalf("expected 'DRY RUN' header, got:\n%s", stdout)
	}
	// File should still have both tasks.
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.ByID(2) == nil {
		t.Fatal("victim #2 should still exist after dry-run")
	}
}

// TestMergePreferVictim: with --prefer victim, conflicting scalar
// fields take the victim's value.
func TestMergePreferVictim(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha", "-p", "low"); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta", "-p", "high"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "merge", "1", "2", "--prefer", "victim"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	t1 := s.ByID(1)
	if t1.Priority.String() != "high" {
		t.Fatalf("expected survivor priority=high after --prefer victim, got %s", t1.Priority)
	}
}

// TestMergeNoteOnlyKeepsSurvivorScalars: --note-only leaves the
// survivor's scalar fields untouched even when they conflict.
func TestMergeNoteOnlyKeepsSurvivorScalars(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "alpha", "-p", "low"); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "beta", "-p", "urgent"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "note", "2", "important context"); err != nil {
		t.Fatalf("note: %v", err)
	}
	// Use --prefer victim AND --note-only — note-only should win.
	if _, _, err := runCmd(t, dir, "merge", "1", "2", "--prefer", "victim", "--note-only"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	t1 := s.ByID(1)
	if t1.Priority.String() != "low" {
		t.Fatalf("expected survivor priority unchanged (low) under --note-only, got %s", t1.Priority)
	}
	if !strings.Contains(t1.Notes, "important context") {
		t.Fatalf("expected victim's notes folded in, got: %q", t1.Notes)
	}
}

// TestMergeMissingID: refusing on a non-existent id is a normal
// error (not a usage error — the input was syntactically fine).
func TestMergeMissingID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "merge", "1", "999")
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

// TestMergeDepUnion: deps of victim are folded into survivor.
func TestMergeDepUnion(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"survivor", "victim", "prereqA", "prereqB"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// survivor (1) deps on prereqA (3); victim (2) deps on prereqB (4).
	if _, _, err := runCmd(t, dir, "depend", "1", "--on", "3"); err != nil {
		t.Fatalf("depend 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "4"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "merge", "1", "2"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	t1 := s.ByID(1)
	wantDeps := map[int]bool{3: true, 4: true}
	if len(t1.DependsOn) != 2 {
		t.Fatalf("expected DependsOn length 2 (union), got %v", t1.DependsOn)
	}
	for _, d := range t1.DependsOn {
		if !wantDeps[d] {
			t.Fatalf("unexpected dep %d, wanted union {3,4}, got %v", d, t1.DependsOn)
		}
	}
}

// TestMergeUnknownPreferRejected: --prefer must be one of the known
// modes; anything else is a usage error.
func TestMergeUnknownPreferRejected(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	_, _, err := runCmd(t, dir, "merge", "1", "2", "--prefer", "magic")
	if err == nil {
		t.Fatal("expected error for unknown --prefer")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestMergeUndoLastReverts: the merge writes via the normal Save
// pipeline so `undo-last` reverts it.
func TestMergeUndoLastReverts(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "merge", "1", "2"); err != nil {
		t.Fatalf("merge: %v", err)
	}
	// Confirm 2 is gone.
	s, _ := store.Load(filepath.Join(dir, ".tsk.md"))
	if s.ByID(2) != nil {
		t.Fatal("victim #2 should be removed after merge")
	}
	if _, _, err := runCmd(t, dir, "undo-last", "--yes"); err != nil {
		t.Fatalf("undo-last: %v", err)
	}
	s, _ = store.Load(filepath.Join(dir, ".tsk.md"))
	if s.ByID(2) == nil {
		t.Fatal("victim #2 should be restored after undo-last")
	}
}
