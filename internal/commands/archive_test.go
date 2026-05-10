package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readArchive returns the contents of the sibling archive file in tmpDir.
func readArchive(t *testing.T, tmpDir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(tmpDir, ".tsk.archive.md"))
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	return string(b)
}

func TestArchiveAllMovesDoneTasks(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}

	stdout, _, err := runCmd(t, dir, "archive", "--all")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !strings.Contains(stdout, "archived 2 task(s)") {
		t.Fatalf("expected archived 2, got:\n%s", stdout)
	}

	active := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(active, "- [x]") {
		t.Errorf("active should have no done tasks, got:\n%s", active)
	}
	if !strings.Contains(active, "- [ ] c") {
		t.Errorf("expected c preserved, got:\n%s", active)
	}
	// Active task IDs preserved (c was id 3).
	if !strings.Contains(active, "id:3") {
		t.Errorf("active task id should remain 3, got:\n%s", active)
	}

	archive := readArchive(t, dir)
	if !strings.HasPrefix(archive, "# tsk archive") {
		t.Errorf("archive missing header, got:\n%s", archive)
	}
	if !strings.Contains(archive, "- [x] a") || !strings.Contains(archive, "- [x] b") {
		t.Errorf("archive missing tasks, got:\n%s", archive)
	}
	// Fresh sequential IDs in archive (1, 2).
	if !strings.Contains(archive, "id:1") || !strings.Contains(archive, "id:2") {
		t.Errorf("archive ids should be 1,2, got:\n%s", archive)
	}
}

func TestArchiveDryRunChangesNothing(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	before := readFile(t, filepath.Join(dir, ".tsk.md"))

	stdout, _, err := runCmd(t, dir, "archive", "--all", "--dry-run")
	if err != nil {
		t.Fatalf("archive dry-run: %v", err)
	}
	if !strings.Contains(stdout, "would archive") {
		t.Fatalf("expected dry-run preamble, got:\n%s", stdout)
	}
	after := readFile(t, filepath.Join(dir, ".tsk.md"))
	if before != after {
		t.Errorf("dry-run mutated active file:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if _, err := os.Stat(filepath.Join(dir, ".tsk.archive.md")); !os.IsNotExist(err) {
		t.Errorf("dry-run created archive file: err=%v", err)
	}
}

func TestArchiveOlderThanRespectsAge(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "fresh"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Just-completed → 30d cutoff should skip it.
	stdout, _, err := runCmd(t, dir, "archive", "--older-than", "30d")
	if err != nil {
		t.Fatalf("archive older-than: %v", err)
	}
	if !strings.Contains(stdout, "no tasks to archive") {
		t.Fatalf("expected no-op, got:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, ".tsk.archive.md")); !os.IsNotExist(err) {
		t.Errorf("expected no archive file, err=%v", err)
	}
}

func TestArchiveOlderThanZeroArchives(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// 0d → cutoff is "now"; just-completed task is technically before now.
	stdout, _, err := runCmd(t, dir, "archive", "--older-than", "0d")
	if err != nil {
		t.Fatalf("archive 0d: %v", err)
	}
	if !strings.Contains(stdout, "archived 1 task(s)") {
		t.Fatalf("expected archive, got:\n%s", stdout)
	}
}

func TestArchiveContinuesIDsAcrossRuns(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// First batch: done 1
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all"); err != nil {
		t.Fatalf("archive: %v", err)
	}
	arch1 := readArchive(t, dir)
	if !strings.Contains(arch1, "id:1") {
		t.Errorf("first archive id should be 1, got:\n%s", arch1)
	}
	// Second batch: done 2 (which is still id 2 in active, since archive didn't renumber)
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "archive", "--all"); err != nil {
		t.Fatalf("archive 2: %v", err)
	}
	arch2 := readArchive(t, dir)
	// New entry should be id:2 in archive (continued from max=1).
	if !strings.Contains(arch2, "- [x] b") {
		t.Errorf("expected b in archive, got:\n%s", arch2)
	}
	if strings.Count(arch2, "id:1") != 1 || strings.Count(arch2, "id:2") != 1 {
		t.Errorf("archive should have ids 1 and 2, got:\n%s", arch2)
	}
}

func TestArchiveNoDoneTasksIsNoOp(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "archive", "--all")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !strings.Contains(stdout, "no tasks to archive") {
		t.Fatalf("expected no-op message, got:\n%s", stdout)
	}
}

func TestPurgeRefusesWithoutSelection(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "purge")
	if err == nil {
		t.Fatal("expected refusal without --done or --id, got nil")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Errorf("expected ExitCode 2, got %v", err)
	}
}

func TestPurgeDoneRemovesOnlyDone(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"keep1", "killme", "keep2"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "purge", "--done")
	if err != nil {
		t.Fatalf("purge --done: %v", err)
	}
	if !strings.Contains(stdout, "purged 1 task(s)") {
		t.Fatalf("expected purged 1, got:\n%s", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(content, "killme") {
		t.Errorf("killme should be gone:\n%s", content)
	}
	if !strings.Contains(content, "keep1") || !strings.Contains(content, "keep2") {
		t.Errorf("survivors missing:\n%s", content)
	}
	// Surviving IDs stable.
	if !strings.Contains(content, "id:1") || !strings.Contains(content, "id:3") {
		t.Errorf("expected stable ids 1 and 3, got:\n%s", content)
	}
}

func TestPurgeDryRun(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	before := readFile(t, filepath.Join(dir, ".tsk.md"))
	stdout, _, err := runCmd(t, dir, "purge", "--done", "--dry-run")
	if err != nil {
		t.Fatalf("purge dry-run: %v", err)
	}
	if !strings.Contains(stdout, "would delete") || !strings.Contains(stdout, "would purge") {
		t.Fatalf("expected dry-run output, got:\n%s", stdout)
	}
	after := readFile(t, filepath.Join(dir, ".tsk.md"))
	if before != after {
		t.Errorf("dry-run mutated file")
	}
}

func TestPurgeByID(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "purge", "--id", "1", "--id", "3"); err != nil {
		t.Fatalf("purge by id: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(content, "- [ ] a") || strings.Contains(content, "- [ ] c") {
		t.Errorf("ids 1,3 should be gone:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] b") {
		t.Errorf("id 2 should survive:\n%s", content)
	}
}

func TestPurgeByIDMissing(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "purge", "--id", "999")
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

func TestPurgeOlderThanRestrictsDone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// Just completed, should not match a 30d cutoff.
	stdout, _, err := runCmd(t, dir, "purge", "--done", "--older-than", "30d")
	if err != nil {
		t.Fatalf("purge older-than: %v", err)
	}
	if !strings.Contains(stdout, "no tasks to purge") {
		t.Fatalf("expected no-op, got:\n%s", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [x] a") {
		t.Errorf("task should still exist:\n%s", content)
	}
}

func TestArchiveBadOlderThan(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "archive", "--older-than", "garbage")
	if err == nil {
		t.Fatal("expected error for bad --older-than")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Errorf("expected ExitCode 2, got %v", err)
	}
}
