package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManGeneratesPagesToDefaultDir asserts that the default invocation
// writes manpages to ./man and includes at least the root + a few known
// subcommands.
func TestManGeneratesPagesToDefaultDir(t *testing.T) {
	dir := t.TempDir()
	prevWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(prevWd) }()

	if _, _, err := runCmd(t, dir, "man"); err != nil {
		t.Fatalf("man: %v", err)
	}
	manDir := filepath.Join(dir, "man")
	entries, err := os.ReadDir(manDir)
	if err != nil {
		t.Fatalf("read man dir: %v", err)
	}
	// At least 20 pages: root + ~40 subcommands typically.
	if len(entries) < 20 {
		t.Fatalf("expected lots of manpages, got %d", len(entries))
	}
	// Must include tsk.1 (root) and at least one well-known subcommand.
	want := []string{"tsk.1", "tsk-add.1", "tsk-pin.1", "tsk-last.1"}
	for _, w := range want {
		if _, err := os.Stat(filepath.Join(manDir, w)); err != nil {
			t.Errorf("missing manpage %s", w)
		}
	}
}

// TestManCustomDir writes to an explicit --dir and verifies content.
func TestManCustomDir(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "custom-man")
	stdout, _, err := runCmd(t, dir, "man", "--dir", out)
	if err != nil {
		t.Fatalf("man --dir: %v", err)
	}
	if !strings.Contains(stdout, "wrote ") {
		t.Fatalf("expected success report, got %q", stdout)
	}
	// Spot-check the root page contains the version source line.
	body, err := os.ReadFile(filepath.Join(out, "tsk.1"))
	if err != nil {
		t.Fatalf("read tsk.1: %v", err)
	}
	if !strings.Contains(string(body), "tsk") {
		t.Fatalf("manpage missing 'tsk' reference:\n%s", body[:200])
	}
	// roff manpage format starts with `.TH` directive (TH = Title Header).
	if !strings.Contains(string(body), ".TH") {
		t.Fatalf("manpage missing .TH roff directive:\n%s", body[:200])
	}
}

// TestManInstallRequiresYes asserts --install alone errors.
func TestManInstallRequiresYes(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runCmd(t, dir, "man", "--install")
	if err == nil {
		t.Fatal("expected error for --install without --yes")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected --yes hint, got %v", err)
	}
}

// TestManInstallConflictsWithDir asserts --dir and --install reject each other.
func TestManInstallConflictsWithDir(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runCmd(t, dir, "man", "--install", "--yes", "--dir", "/tmp/whatever")
	if err == nil {
		t.Fatal("expected error for --dir + --install")
	}
}

// TestManSection lets the user pick section 8, asserts file suffix follows.
func TestManSection(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "man-sec8")
	if _, _, err := runCmd(t, dir, "man", "--dir", out, "--section", "8"); err != nil {
		t.Fatalf("man --section 8: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "tsk.8")); err != nil {
		t.Fatalf("expected tsk.8: %v", err)
	}
}
