package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMoveBasic verifies a single task moves from source to destination.
func TestMoveBasic(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	dst := filepath.Join(dstDir, ".tsk.md")

	if _, _, err := runCmd(t, srcDir, "add", "scratch task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, srcDir, "move", "1", "--to", dst)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if !strings.Contains(stdout, "moved 1 task") {
		t.Errorf("expected success message, got:\n%s", stdout)
	}
	// Source should now be empty (still listed, no tasks)
	srcOut, _, _ := runCmd(t, srcDir, "ls")
	if strings.Contains(srcOut, "scratch task") {
		t.Errorf("task still in source:\n%s", srcOut)
	}
	// Destination should have it
	dstBytes, _ := os.ReadFile(dst)
	if !strings.Contains(string(dstBytes), "scratch task") {
		t.Errorf("task not in destination:\n%s", dstBytes)
	}
}

// TestMoveMultipleIDs verifies multiple tasks move in one shot.
func TestMoveMultipleIDs(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	dst := filepath.Join(dstDir, ".tsk.md")

	for _, title := range []string{"task one", "task two", "task three"} {
		if _, _, err := runCmd(t, srcDir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, srcDir, "move", "1", "3", "--to", dst)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if !strings.Contains(stdout, "moved 2 task") {
		t.Errorf("expected 'moved 2 task', got:\n%s", stdout)
	}
	srcOut, _, _ := runCmd(t, srcDir, "ls")
	if !strings.Contains(srcOut, "task two") {
		t.Errorf("task two should remain in source:\n%s", srcOut)
	}
	if strings.Contains(srcOut, "task one") || strings.Contains(srcOut, "task three") {
		t.Errorf("moved tasks still in source:\n%s", srcOut)
	}
	dstBytes, _ := os.ReadFile(dst)
	if !strings.Contains(string(dstBytes), "task one") || !strings.Contains(string(dstBytes), "task three") {
		t.Errorf("destination missing tasks:\n%s", dstBytes)
	}
}

// TestMoveDryRun verifies --dry-run shows preview but doesn't touch files.
func TestMoveDryRun(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	dst := filepath.Join(dstDir, ".tsk.md")

	if _, _, err := runCmd(t, srcDir, "add", "preview task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, srcDir, "move", "1", "--to", dst, "--dry-run")
	if err != nil {
		t.Fatalf("move --dry-run: %v", err)
	}
	if !strings.Contains(stdout, "DRY RUN") {
		t.Errorf("expected DRY RUN in output:\n%s", stdout)
	}
	if !strings.Contains(stdout, "preview task") {
		t.Errorf("expected task title in preview:\n%s", stdout)
	}
	// Source should still have task
	srcOut, _, _ := runCmd(t, srcDir, "ls")
	if !strings.Contains(srcOut, "preview task") {
		t.Errorf("task should remain after dry-run:\n%s", srcOut)
	}
	// Destination should not exist
	if _, err := os.Stat(dst); err == nil {
		t.Errorf("destination should not exist after dry-run")
	}
}

// TestMoveRejectsMissingTo verifies --to is required.
func TestMoveRejectsMissingTo(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "move", "1")
	if err == nil {
		t.Fatal("expected error when --to is missing")
	}
}

// TestMoveRejectsSelfMove verifies you can't move to the same file.
func TestMoveRejectsSelfMove(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, ".tsk.md")
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "move", "1", "--to", src)
	if err == nil {
		t.Fatal("expected error for self-move")
	}
	if !strings.Contains(err.Error(), "same file") {
		t.Errorf("expected 'same file' error, got: %v", err)
	}
}

// TestMoveRejectsBadID verifies a bogus ID fails.
func TestMoveRejectsBadID(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	dst := filepath.Join(dstDir, ".tsk.md")
	if _, _, err := runCmd(t, srcDir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, srcDir, "move", "999", "--to", dst)
	if err == nil {
		t.Fatal("expected error for missing id")
	}
}

// TestMoveCreatesDestination verifies the destination file is created if absent.
func TestMoveCreatesDestination(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	dst := filepath.Join(dstDir, "subdir-that-exists", ".tsk.md")
	if err := os.Mkdir(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, _, err := runCmd(t, srcDir, "add", "new home"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, srcDir, "move", "1", "--to", dst); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Errorf("destination not created: %v", err)
	}
}

// TestMoveReassignsIDs verifies IDs are re-assigned in destination by default.
func TestMoveReassignsIDs(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	dst := filepath.Join(dstDir, ".tsk.md")

	// Destination already has tasks 1 and 2.
	if _, _, err := runCmd(t, dstDir, "add", "existing one"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dstDir, "add", "existing two"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Source has task id=1
	if _, _, err := runCmd(t, srcDir, "add", "incoming"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, srcDir, "move", "1", "--to", dst); err != nil {
		t.Fatalf("move: %v", err)
	}
	// Destination should have 3 tasks with unique IDs.
	dstBytes, _ := os.ReadFile(dst)
	out := string(dstBytes)
	for _, want := range []string{"existing one", "existing two", "incoming"} {
		if !strings.Contains(out, want) {
			t.Errorf("destination missing %q:\n%s", want, out)
		}
	}
	// Count "id:" appearances and verify all 3 IDs are distinct
	if strings.Count(out, "id:1") != 1 {
		t.Errorf("expected one id:1 in destination, got:\n%s", out)
	}
	if !strings.Contains(out, "id:3") {
		t.Errorf("incoming task should get id:3:\n%s", out)
	}
}

// TestMoveKeepIDsCollision verifies --keep-ids fails on collision.
func TestMoveKeepIDsCollision(t *testing.T) {
	srcDir := t.TempDir()
	dstDir := t.TempDir()
	dst := filepath.Join(dstDir, ".tsk.md")

	if _, _, err := runCmd(t, dstDir, "add", "existing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, srcDir, "add", "incoming"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, srcDir, "move", "1", "--to", dst, "--keep-ids")
	if err == nil {
		t.Fatal("expected collision error with --keep-ids")
	}
	if !strings.Contains(err.Error(), "id 1") {
		t.Errorf("expected collision error mentioning id 1, got: %v", err)
	}
}
