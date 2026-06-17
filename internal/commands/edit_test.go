package commands

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestEditInvokesEditor points $EDITOR at a harmless command (`true`) and
// asserts the edit command resolves the store and runs the editor without
// error. This exercises newEditCmd, which was almost entirely uncovered.
func TestEditInvokesEditor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on POSIX 'true'/'false' commands")
	}
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Setenv("EDITOR", "true")
	if _, _, err := runCmd(t, dir, "edit"); err != nil {
		t.Fatalf("edit with EDITOR=true should succeed, got: %v", err)
	}
}

// TestEditPropagatesEditorFailure asserts a non-zero editor exit surfaces as an
// error wrapped with the "editor:" prefix.
func TestEditPropagatesEditorFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on POSIX 'true'/'false' commands")
	}
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	t.Setenv("EDITOR", "false")
	_, _, err := runCmd(t, dir, "edit")
	if err == nil {
		t.Fatal("edit with EDITOR=false should return an error")
	}
}

// TestEditRequiresExistingStore asserts edit fails clearly when there is no
// .tsk.md to open (resolveStore requireExisting=true path).
func TestEditRequiresExistingStore(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("relies on POSIX 'true' command")
	}
	dir := t.TempDir()
	t.Setenv("EDITOR", "true")
	// No init: the scratch .tsk.md does not exist yet.
	_, _, err := runCmd(t, dir, "edit")
	if err == nil {
		t.Fatal("edit should fail when no .tsk.md exists")
	}
}

// TestEditOpensStorePath confirms the editor is handed the real store path by
// using a tiny shell shim that appends a marker to a sentinel file. This
// proves edit passes s.Path (not some default) to the editor process.
func TestEditOpensStorePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell shim")
	}
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	sentinel := filepath.Join(dir, "opened-path.txt")
	shim := filepath.Join(dir, "fake-editor.sh")
	script := "#!/bin/sh\nprintf '%s' \"$1\" > " + shellQuote(sentinel) + "\n"
	if err := os.WriteFile(shim, []byte(script), 0o755); err != nil {
		t.Fatalf("write shim: %v", err)
	}
	t.Setenv("EDITOR", shim)
	if _, _, err := runCmd(t, dir, "edit"); err != nil {
		t.Fatalf("edit: %v", err)
	}
	got, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("editor shim did not record a path: %v", err)
	}
	wantSuffix := filepath.Join(dir, ".tsk.md")
	if string(got) != wantSuffix {
		t.Fatalf("editor received %q, want %q", string(got), wantSuffix)
	}
}

// shellQuote single-quotes a string for safe interpolation into the /bin/sh
// shim above. Paths from t.TempDir() never contain single quotes, but quoting
// keeps the shim robust.
func shellQuote(s string) string {
	return "'" + s + "'"
}
