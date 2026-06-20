package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRenameUpdatesTitle(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "old title"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "rename", "1", "shiny new title")
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if !strings.Contains(stdout, `#1 title "old title" -> "shiny new title"`) {
		t.Fatalf("expected transition line, got: %q", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [ ] shiny new title") {
		t.Fatalf("expected new title on disk, got:\n%s", content)
	}
	if strings.Contains(content, "old title") {
		t.Fatalf("old title should be gone, got:\n%s", content)
	}
}

func TestRenameJoinsArgsWithoutQuotes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Multi-word title without explicit quoting.
	if _, _, err := runCmd(t, dir, "rename", "1", "buy", "more", "milk"); err != nil {
		t.Fatalf("rename multi-word: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [ ] buy more milk") {
		t.Fatalf("expected joined title on disk, got:\n%s", content)
	}
}

func TestRenameAlias(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "old"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "retitle", "1", "fresh"); err != nil {
		t.Fatalf("retitle alias: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [ ] fresh") {
		t.Fatalf("alias should work, got:\n%s", content)
	}
}

func TestRenameUnchangedIsNoop(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "same"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "rename", "1", "same")
	if err != nil {
		t.Fatalf("rename same: %v", err)
	}
	if !strings.Contains(stdout, "title unchanged") {
		t.Fatalf("expected unchanged notice, got: %q", stdout)
	}
}

func TestRenameTrimmedSpaceIsNoop(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "tidy"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Leading/trailing whitespace should be trimmed; matches existing title.
	stdout, _, err := runCmd(t, dir, "rename", "1", "  tidy  ")
	if err != nil {
		t.Fatalf("rename trimmed: %v", err)
	}
	if !strings.Contains(stdout, "title unchanged") {
		t.Fatalf("trimmed whitespace should match, got: %q", stdout)
	}
}

func TestRenameRejectsEmptyTitle(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "rename", "1", "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only title")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

func TestRenameRejectsBadID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "rename", "abc", "new")
	if err == nil {
		t.Fatal("expected error for non-numeric id")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2 for usage error, got %v", err)
	}
}

func TestRenameUnknownID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "rename", "999", "anything")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestRenamePreservesMetadata(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "old", "-p", "high", "-t", "dev", "-d", "2099-12-31"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "rename", "1", "renamed"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	for _, want := range []string{"prio:high", "due:2099-12-31", "tags:dev", "- [ ] renamed"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected %q preserved in:\n%s", want, content)
		}
	}
}
