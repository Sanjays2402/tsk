package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchiveMergeIntoCustomPath: --merge-into writes to the given
// file instead of the sibling .tsk.archive.md.
func TestArchiveMergeIntoCustomPath(t *testing.T) {
	dir := t.TempDir()
	customPath := filepath.Join(dir, "custom.archive.md")
	if _, _, err := runCmd(t, dir, "add", "project-a-task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--merge-into", customPath)
	if err != nil {
		t.Fatalf("archive --merge-into: %v", err)
	}
	if !strings.Contains(stdout, customPath) {
		t.Fatalf("output should mention the custom archive path, got:\n%s", stdout)
	}
	// Custom file exists and contains the task.
	body, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("read custom archive: %v", err)
	}
	if !strings.Contains(string(body), "project-a-task") {
		t.Fatalf("custom archive missing task:\n%s", string(body))
	}
	// Default sibling MUST NOT exist (we redirected the write).
	defaultPath := filepath.Join(dir, ".tsk.archive.md")
	if _, err := os.Stat(defaultPath); err == nil {
		t.Fatalf("default archive should not be created when --merge-into is set, but %s exists", defaultPath)
	}
}

// TestArchiveMergeIntoSecondBatchAppends: a second --merge-into call
// to the same file continues the id space and preserves the first
// batch's content.
func TestArchiveMergeIntoSecondBatchAppends(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shared.archive.md")
	// First batch.
	if _, _, err := runCmd(t, dir, "add", "first"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--merge-into", target); err != nil {
		t.Fatalf("archive 1: %v", err)
	}
	// Second batch in a DIFFERENT project dir, archiving into the
	// same target file.
	dir2 := t.TempDir()
	if _, _, err := runCmd(t, dir2, "add", "second"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if _, _, err := runCmd(t, dir2, "done", "1"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	if _, _, err := runCmd(t, dir2, "archive", "--all", "--merge-into", target); err != nil {
		t.Fatalf("archive 2: %v", err)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read shared archive: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "first") {
		t.Fatalf("first batch missing from shared archive:\n%s", s)
	}
	if !strings.Contains(s, "second") {
		t.Fatalf("second batch missing from shared archive:\n%s", s)
	}
	// Should have two distinct ids (1 and 2) in id:N comments.
	if !strings.Contains(s, "id:1") || !strings.Contains(s, "id:2") {
		t.Fatalf("expected ids 1 and 2 in shared archive, got:\n%s", s)
	}
}

// TestArchiveMergeIntoRefusesActiveStore: pointing --merge-into at
// the active .tsk.md must error out (would otherwise corrupt the
// active file).
func TestArchiveMergeIntoRefusesActiveStore(t *testing.T) {
	dir := t.TempDir()
	activePath := filepath.Join(dir, ".tsk.md")
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--all", "--merge-into", activePath)
	if err == nil {
		t.Fatal("expected error for --merge-into pointing at the active store")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
	if !strings.Contains(err.Error(), "active store") {
		t.Fatalf("error should explain the conflict, got: %v", err)
	}
}

// TestArchiveMergeIntoWithStrategyWeekly: --merge-into composes
// with --strategy weekly — the bucketed layout lands in the
// non-default file.
func TestArchiveMergeIntoWithStrategyWeekly(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "weekly.archive.md")
	if _, _, err := runCmd(t, dir, "add", "bucketed"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--strategy", "weekly", "--merge-into", target); err != nil {
		t.Fatalf("archive weekly merge-into: %v", err)
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "-W") {
		t.Fatalf("expected weekly -W marker in custom archive:\n%s", s)
	}
	if !strings.Contains(s, "bucketed") {
		t.Fatalf("expected task in custom archive:\n%s", s)
	}
}

// TestArchiveMergeIntoRelativePathResolvesAgainstStoreDir: a bare
// filename like "team.archive.md" resolves next to the active store,
// not the cwd.
func TestArchiveMergeIntoRelativePathResolvesAgainstStoreDir(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--merge-into", "team.archive.md")
	if err != nil {
		t.Fatalf("archive --merge-into relative: %v", err)
	}
	expectedPath := filepath.Join(dir, "team.archive.md")
	if !strings.Contains(stdout, expectedPath) {
		t.Fatalf("expected output to reference %q (resolved next to store), got:\n%s", expectedPath, stdout)
	}
	if _, err := os.Stat(expectedPath); err != nil {
		t.Fatalf("expected file at %s, got: %v", expectedPath, err)
	}
}

// TestArchiveMergeIntoDryRunSurfacesPath: dry-run output names the
// custom path so the user can verify before committing.
func TestArchiveMergeIntoDryRunSurfacesPath(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "preview.archive.md")
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all", "--dry-run", "--merge-into", target)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if !strings.Contains(stdout, target) {
		t.Fatalf("dry-run output should name the custom path, got:\n%s", stdout)
	}
	// File should NOT have been written.
	if _, err := os.Stat(target); err == nil {
		t.Fatalf("dry-run wrote file at %s, must not", target)
	}
}

// TestArchiveMergeIntoEmptyDefaultsToSibling: passing an empty
// --merge-into="" is treated the same as omitting the flag (default
// to sibling .tsk.archive.md). Defensive against shell-var typos.
func TestArchiveMergeIntoEmptyDefaultsToSibling(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all", "--merge-into", ""); err != nil {
		t.Fatalf("archive --merge-into '': %v", err)
	}
	defaultPath := filepath.Join(dir, ".tsk.archive.md")
	if _, err := os.Stat(defaultPath); err != nil {
		t.Fatalf("expected default archive at %s with empty --merge-into, got: %v", defaultPath, err)
	}
}
