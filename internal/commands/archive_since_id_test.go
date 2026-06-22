package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveSinceIDArchivesBelowCutoff: --since-id 3 archives every
// Done task with id < 3 (i.e. #1 and #2) and leaves higher ids alone.
func TestArchiveSinceIDArchivesBelowCutoff(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d", "e"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// Done 1,2,3 — only 1 and 2 should archive at cutoff=3.
	if _, _, err := runCmd(t, dir, "done", "1", "2", "3"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--since-id", "3")
	if err != nil {
		t.Fatalf("archive --since-id 3: %v", err)
	}
	if !strings.Contains(stdout, "archived 2 task(s)") {
		t.Fatalf("expected 'archived 2 task(s)' (#1,#2), got:\n%s", stdout)
	}
	active := readFile(t, filepath.Join(dir, ".tsk.md"))
	// #3 is still in active (id == cutoff is NOT below).
	if !strings.Contains(active, "- [x] c") {
		t.Fatalf("expected #3 (c) preserved in active (id==cutoff), got:\n%s", active)
	}
	// #4, #5 still in active too (open).
	if !strings.Contains(active, "- [ ] d") || !strings.Contains(active, "- [ ] e") {
		t.Fatalf("expected #4 (d) and #5 (e) preserved, got:\n%s", active)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	if !strings.Contains(archive, "- [x] a") || !strings.Contains(archive, "- [x] b") {
		t.Fatalf("expected #1,#2 in archive, got:\n%s", archive)
	}
}

// TestArchiveSinceIDSkipsOpenTasks: open tasks below the id cutoff
// are NOT archived. Only Done tasks qualify, same as every other
// archive selector.
func TestArchiveSinceIDSkipsOpenTasks(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// Only #1 is done. #2 is below cutoff but open.
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--since-id", "3")
	if err != nil {
		t.Fatalf("archive --since-id 3: %v", err)
	}
	if !strings.Contains(stdout, "archived 1 task(s)") {
		t.Fatalf("expected exactly 1 archived (#1, the only Done one), got:\n%s", stdout)
	}
	active := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(active, "- [ ] b") {
		t.Fatalf("open #2 must stay in active even though id<cutoff:\n%s", active)
	}
}

// TestArchiveSinceIDIgnoresCompletionTimestamp: the whole point of
// the id-axis is to skip the time check. Tasks WITHOUT a Completed
// timestamp (hand-edited or older) still qualify if their id is
// below the cutoff. Regression against any future refactor that
// might fold the time-axis check back into the id-axis path.
func TestArchiveSinceIDIgnoresCompletionTimestamp(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Hand-strip the completed timestamp to simulate a hand-edited
	// or imported task.
	body := readFile(t, filepath.Join(dir, ".tsk.md"))
	stripped := strings.Replace(body, " completed:", " stripped_completed:", 1)
	if stripped == body {
		t.Skip("test setup couldn't find a completed: stamp to strip — store layout changed?")
	}
	if err := os.WriteFile(filepath.Join(dir, ".tsk.md"), []byte(stripped), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--since-id", "2")
	if err != nil {
		t.Fatalf("archive --since-id 2: %v", err)
	}
	if !strings.Contains(stdout, "archived 1 task(s)") {
		t.Fatalf("expected #1 archived despite missing completed stamp, got:\n%s", stdout)
	}
}

// TestArchiveSinceIDMutuallyExclusiveWithAll: --since-id and --all
// pick different axes; combining them is a usage error.
func TestArchiveSinceIDMutuallyExclusiveWithAll(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--since-id", "5", "--all")
	if err == nil {
		t.Fatal("expected error combining --since-id and --all")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestArchiveSinceIDMutuallyExclusiveWithExplicitOlderThan: passing
// both --since-id and an explicit --older-than is rejected (two
// different axes). The DEFAULT --older-than=30d that just lives in
// flag-defaults must NOT trigger the rejection — only an explicit
// user-supplied flag should.
func TestArchiveSinceIDMutuallyExclusiveWithExplicitOlderThan(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--since-id", "5", "--older-than", "7d")
	if err == nil {
		t.Fatal("expected error combining --since-id and explicit --older-than")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestArchiveSinceIDDefaultOlderThanIgnored: just --since-id with
// no --older-than is fine — the default flag-value for --older-than
// must NOT block the id-axis run. Critical regression: if we'd
// checked olderThan != "" instead of Changed("older-than"), every
// --since-id call would fail.
func TestArchiveSinceIDDefaultOlderThanIgnored(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// No --older-than passed — the default "30d" must be ignored.
	stdout, _, err := runCmd(t, dir, "archive", "--since-id", "2")
	if err != nil {
		t.Fatalf("plain --since-id 2 should not error on default --older-than: %v", err)
	}
	if !strings.Contains(stdout, "archived 1 task(s)") {
		t.Fatalf("expected #1 archived, got:\n%s", stdout)
	}
}

// TestArchiveSinceIDDryRun: --since-id composes with --dry-run.
// Reports what WOULD be archived without writing.
func TestArchiveSinceIDDryRun(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	before := readFile(t, filepath.Join(dir, ".tsk.md"))
	stdout, _, err := runCmd(t, dir, "archive", "--since-id", "3", "--dry-run")
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(stdout, "would archive 2 task(s)") {
		t.Fatalf("expected 'would archive 2 task(s)' in dry-run, got:\n%s", stdout)
	}
	after := readFile(t, filepath.Join(dir, ".tsk.md"))
	if before != after {
		t.Fatal("dry-run --since-id mutated active file")
	}
}

// TestArchiveSinceIDEmptyMatch: --since-id below any done id is a
// clean no-op with the standard "no tasks to archive" message.
func TestArchiveSinceIDEmptyMatch(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// cutoff=1 → strictly less than 1 → nothing matches.
	stdout, _, err := runCmd(t, dir, "archive", "--since-id", "1")
	if err != nil {
		t.Fatalf("archive --since-id 1: %v", err)
	}
	if !strings.Contains(stdout, "no tasks to archive") {
		t.Fatalf("expected no-op message, got:\n%s", stdout)
	}
}

// TestArchiveSinceIDNegativeRejected: --since-id with a negative
// value is a usage error.
func TestArchiveSinceIDNegativeRejected(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--since-id", "-3")
	if err == nil {
		t.Fatal("expected error for negative --since-id")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestArchiveSinceIDComposesWithStrategy: --since-id selection
// layers cleanly on top of bucketed --strategy. The selected
// tasks land in their per-bucket sections in the archive.
func TestArchiveSinceIDComposesWithStrategy(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--since-id", "3", "--strategy", "yearly")
	if err != nil {
		t.Fatalf("archive --since-id 3 --strategy yearly: %v", err)
	}
	if !strings.Contains(stdout, "strategy=yearly") {
		t.Fatalf("expected strategy=yearly in output, got:\n%s", stdout)
	}
	archive := readFile(t, filepath.Join(dir, ".tsk.archive.md"))
	// Yearly bucket header should be present (e.g. "## 2026").
	if !strings.Contains(archive, "## ") {
		t.Fatalf("expected a bucket header (## YYYY) in archive, got:\n%s", archive)
	}
	if !strings.Contains(archive, "- [x] a") || !strings.Contains(archive, "- [x] b") {
		t.Fatalf("expected #1,#2 in archive under yearly strategy, got:\n%s", archive)
	}
}

// (no helpers needed — uses os.WriteFile directly.)
