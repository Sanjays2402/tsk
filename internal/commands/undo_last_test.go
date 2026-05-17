package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCmdWithStdin is like runCmd but pipes the given reader as stdin.
func runCmdWithStdin(t *testing.T, tmpDir string, stdin io.Reader, args ...string) (stdout, combined string, err error) {
	t.Helper()
	root := NewRoot()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetIn(stdin)
	full := append([]string{"--file", filepath.Join(tmpDir, ".tsk.md")}, args...)
	root.SetArgs(full)
	err = root.Execute()
	return out.String(), out.String() + errb.String(), err
}

// TestUndoLastBasic verifies after add + rm + undo-last, the task returns.
func TestUndoLastBasic(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "snapshot me"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "rm", "1"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	// Verify gone
	lsOut, _, _ := runCmd(t, dir, "ls")
	if strings.Contains(lsOut, "snapshot me") {
		t.Fatalf("task should be removed before undo-last:\n%s", lsOut)
	}
	if _, _, err := runCmd(t, dir, "undo-last", "--yes"); err != nil {
		t.Fatalf("undo-last: %v", err)
	}
	lsOut2, _, _ := runCmd(t, dir, "ls")
	if !strings.Contains(lsOut2, "snapshot me") {
		t.Errorf("task should be back after undo-last:\n%s", lsOut2)
	}
}

// TestUndoLastIsInvolutive verifies undo-last twice returns to pre-undo state.
func TestUndoLastIsInvolutive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "first task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "rm", "1"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	// Undo: task comes back
	if _, _, err := runCmd(t, dir, "undo-last", "--yes"); err != nil {
		t.Fatalf("undo-last #1: %v", err)
	}
	if out, _, _ := runCmd(t, dir, "ls"); !strings.Contains(out, "first task") {
		t.Fatalf("after undo #1 task should exist:\n%s", out)
	}
	// Undo again: task removed again
	if _, _, err := runCmd(t, dir, "undo-last", "--yes"); err != nil {
		t.Fatalf("undo-last #2: %v", err)
	}
	if out, _, _ := runCmd(t, dir, "ls"); strings.Contains(out, "first task") {
		t.Errorf("after undo #2 task should be gone:\n%s", out)
	}
}

// TestUndoLastNoSnapshot verifies running with no .bak fails cleanly.
func TestUndoLastNoSnapshot(t *testing.T) {
	dir := t.TempDir()
	// Create empty .tsk.md but no .bak
	if err := os.WriteFile(filepath.Join(dir, ".tsk.md"), []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _, err := runCmd(t, dir, "undo-last", "--yes")
	if err == nil {
		t.Fatal("expected error with no snapshot")
	}
	if !strings.Contains(err.Error(), "no snapshot") {
		t.Errorf("expected 'no snapshot' in error, got: %v", err)
	}
}

// TestUndoLastConfirmAbort verifies prompt without --yes aborts on "n".
func TestUndoLastConfirmAbort(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "to delete"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "rm", "1"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	// Use a custom stdin with "n\n"
	stdout, _, err := runCmdWithStdin(t, dir, bytes.NewBufferString("n\n"), "undo-last")
	if err != nil {
		t.Fatalf("undo-last: %v", err)
	}
	if !strings.Contains(stdout, "aborted") {
		t.Errorf("expected aborted, got:\n%s", stdout)
	}
	// Task should still be gone
	lsOut, _, _ := runCmd(t, dir, "ls")
	if strings.Contains(lsOut, "to delete") {
		t.Errorf("task should remain deleted after abort:\n%s", lsOut)
	}
}

// TestSaveCreatesBak verifies that Save() automatically writes the .bak snapshot.
func TestSaveCreatesBak(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "snapshot trigger"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// After first add, .bak shouldn't exist (file didn't exist before).
	if _, err := os.Stat(filepath.Join(dir, ".tsk.md.bak")); err == nil {
		t.Errorf("expected no .bak on first save")
	}
	// Second save → .bak should exist.
	if _, _, err := runCmd(t, dir, "add", "second task"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".tsk.md.bak")); err != nil {
		t.Errorf(".bak should exist after second save: %v", err)
	}
}
